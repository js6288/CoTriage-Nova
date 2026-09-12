package io.cotriage.nova.server;

import static io.cotriage.nova.server.Values.*;

import io.cotriage.nova.common.Json;
import java.nio.charset.StandardCharsets;
import java.sql.*;
import java.time.*;
import java.util.*;
import org.springframework.core.io.ClassPathResource;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.support.GeneratedKeyHolder;
import org.springframework.transaction.support.TransactionTemplate;

/** SQL column names and public JSON remain compatible with the original GORM tables. */
public class ConfigRepository {
  final JdbcTemplate jdbc;
  final TransactionTemplate tx;
  private static final Set<String> PREDICTIONS =
      Set.of(
          "NORMAL",
          "ABNORMAL",
          "APP_ERROR",
          "NET_ERROR",
          "DISK_ERROR",
          "INFRA_ERROR",
          "SECURITY",
          "UNKNOWN");
  static final Map<String, String> TABLES =
      Map.of(
          "edge-providers",
          "edge_providers",
          "cloud-providers",
          "cloud_providers",
          "edge-classifiers",
          "classifier_configs",
          "edge-rules",
          "edge_rules",
          "edge-templates",
          "edge_templates",
          "edge-rule-sets",
          "edge_rule_sets",
          "edge-template-sets",
          "edge_template_sets");

  public ConfigRepository(JdbcTemplate jdbc) {
    this.jdbc = jdbc;
    this.tx =
        new TransactionTemplate(
            new DataSourceTransactionManager(Objects.requireNonNull(jdbc.getDataSource())));
    this.tx.setTimeout(10);
  }

  public void initialize() {
    io.cotriage.nova.common.Schema.ensure(jdbc, new ClassPathResource("schema.sql"));
    seedBuiltinRules();
  }

  static String table(String kind) {
    var t = TABLES.get(kind);
    if (t == null) throw new ApiFailure(404, "not found");
    return t;
  }

  List<Map<String, Object>> rows(String sql, Object... args) {
    return jdbc.query(sql, (rs, i) -> row(rs), args);
  }

  static Map<String, Object> row(ResultSet rs) throws SQLException {
    var m = new LinkedHashMap<String, Object>();
    var md = rs.getMetaData();
    for (int i = 1; i <= md.getColumnCount(); i++) {
      var key = md.getColumnLabel(i).toLowerCase(Locale.ROOT);
      Object v = rs.getObject(i);
      if (v instanceof Timestamp t) v = t.toInstant().toString();
      else if (v instanceof LocalDateTime t) v = t.toInstant(ZoneOffset.UTC).toString();
      else if (v instanceof OffsetDateTime t) v = t.toInstant().toString();
      else if (v instanceof byte[] b) v = new String(b, StandardCharsets.UTF_8);
      else if (v instanceof Clob c) v = c.getSubString(1, (int) c.length());
      if (Set.of(
                  "is_active",
                  "enabled",
                  "processed_by_router",
                  "corrected_edge",
                  "added_actionable_info",
                  "confirmed_high_risk")
              .contains(key)
          && v != null) v = Boolean.TRUE.equals(v) || v instanceof Number n && n.intValue() != 0;
      m.put(key, v);
    }
    return m;
  }

  long count(String sql, Object... args) {
    var n = jdbc.queryForObject(sql, Number.class, args);
    return n == null ? 0 : n.longValue();
  }

  Map<String, Object> raw(String kind, long id) {
    var rows =
        rows(
            "SELECT * FROM "
                + table(kind)
                + " WHERE id=?"
                + (kind.endsWith("-sets") || kind.equals("edge-providers")
                    ? ""
                    : " AND deleted_at IS NULL"),
            id);
    if (rows.isEmpty()) throw new ApiFailure(404, "not found");
    return rows.getFirst();
  }

  public Map<String, Object> get(String kind, long id) {
    return present(kind, raw(kind, id));
  }

  static Object parseStored(Object value) {
    var decoded = json(Objects.toString(value, "null"));
    return decoded instanceof String s ? json(s) : decoded;
  }

  static Map<String, Object> present(String kind, Map<String, Object> row) {
    var out = new LinkedHashMap<>(row);
    if (kind.equals("edge-providers")) out.put("healthcheck_path", out.remove("health_path"));
    if (kind.equals("cloud-providers")) {
      out.remove("created_by");
      out.remove("updated_by");
      if (out.get("deleted_at") == null) out.remove("deleted_at");
      if (out.get("last_publish_at") == null) out.remove("last_publish_at");
    }
    if (kind.equals("edge-classifiers")
        || kind.equals("edge-rules")
        || kind.equals("edge-templates")) out.remove("deleted_at");
    if (out.containsKey("keywords_json"))
      out.put("keywords", parseStored(out.remove("keywords_json")));
    if (out.containsKey("evidence_json"))
      out.put("evidence", parseStored(out.remove("evidence_json")));
    if (kind.endsWith("-sets")) {
      out.remove("items_json");
      out.remove("item_ids_json");
    }
    return out;
  }

  public Map<String, Object> list(String kind, Map<String, String> q) {
    var t = table(kind);
    var args = new ArrayList<Object>();
    var where = new StringBuilder(" WHERE deleted_at IS NULL");
    boolean cloud = kind.equals("cloud-providers");
    if (!cloud) {
      for (var filter : List.of("status", "edge_pred")) {
        if (!str(q, filter).isBlank()
            && (!filter.equals("edge_pred")
                || kind.equals("edge-rules")
                || kind.equals("edge-templates"))) {
          where.append(" AND ").append(filter).append("=?");
          args.add(trim(q, filter));
        }
      }
      if (kind.equals("edge-providers") && !str(q, "active").isBlank()) {
        where.append(" AND is_active=?");
        args.add(Set.of("1", "true").contains(trim(q, "active")));
      }
      if (!trim(q, "q").isEmpty()) {
        var cols =
            kind.equals("edge-rules")
                ? List.of("name", "pattern", "advice")
                : kind.equals("edge-templates")
                    ? List.of("template_text", "raw_example", "advice")
                    : kind.equals("edge-providers")
                        ? List.of("name")
                        : List.of("name", "model", "endpoint");
        where
            .append(" AND (")
            .append(String.join(" OR ", cols.stream().map(c -> c + " LIKE ?").toList()))
            .append(")");
        cols.forEach(c -> args.add("%" + trim(q, "q") + "%"));
      }
    }
    long total = count("SELECT COUNT(*) FROM " + t + where, args.toArray());
    int page = integer(q, "page", 1, Integer.MIN_VALUE, Integer.MAX_VALUE),
        requestedSize = integer(q, "page_size", 20, Integer.MIN_VALUE, Integer.MAX_VALUE);
    if (page <= 0) page = 1;
    if (requestedSize <= 0) requestedSize = 20;
    int size = Math.min(200, requestedSize);
    var order =
        kind.equals("edge-rules")
            ? "priority ASC,updated_at DESC"
            : "updated_at DESC" + (kind.equals("edge-providers") ? ",id DESC" : "");
    var sql = "SELECT * FROM " + t + where + " ORDER BY " + order;
    if (!cloud) {
      sql += " LIMIT ? OFFSET ?";
      args.add(size);
      args.add((long) (page - 1) * size);
    }
    var items = rows(sql, args.toArray()).stream().map(r -> present(kind, r)).toList();
    return obj(
        "items",
        items,
        "page",
        cloud ? 1 : page,
        "page_size",
        cloud ? items.size() : requestedSize,
        "total",
        total);
  }

  long insert(String table, Map<String, Object> data) {
    var keys = new ArrayList<>(data.keySet());
    var holder = new GeneratedKeyHolder();
    jdbc.update(
        connection -> {
          var ps =
              connection.prepareStatement(
                  "INSERT INTO "
                      + table
                      + " ("
                      + String.join(",", keys)
                      + ") VALUES ("
                      + String.join(",", Collections.nCopies(keys.size(), "?"))
                      + ")",
                  Statement.RETURN_GENERATED_KEYS);
          for (int i = 0; i < keys.size(); i++) ps.setObject(i + 1, data.get(keys.get(i)));
          return ps;
        },
        holder);
    return Objects.requireNonNull(holder.getKey()).longValue();
  }

  void update(String table, long id, Map<String, Object> data) {
    var values = new ArrayList<>(data.values());
    values.add(id);
    jdbc.update(
        "UPDATE "
            + table
            + " SET "
            + String.join(",", data.keySet().stream().map(k -> k + "=?").toList())
            + " WHERE id=?",
        values.toArray());
  }

  static Timestamp now() {
    return Timestamp.from(Instant.now());
  }

  static void required(Map<String, Object> req, String... fields) {
    for (String f : fields)
      if (trim(req, f).isEmpty()) throw new ApiFailure(400, f + " is required");
  }

  public Map<String, Object> create(String kind, Map<String, Object> req) {
    req = requestFields(kind, req, false);
    if (kind.endsWith("-sets")) return createSet(kind, req);
    var data = editable(kind, req, null);
    var timestamp = now();
    data.put("created_at", timestamp);
    data.put("updated_at", timestamp);
    data.put("deleted_at", null);
    if (isProvider(kind)) {
      data.put("is_active", false);
      data.put("config_version", 0L);
      data.put("etag", "");
    }
    long id = insert(table(kind), data);
    return get(kind, id);
  }

  public Map<String, Object> patch(String kind, long id, Map<String, Object> req) {
    req = requestFields(kind, req, true);
    var current = raw(kind, id);
    var data = editable(kind, req, current);
    data.put("updated_at", now());
    update(table(kind), id, data);
    return get(kind, id);
  }

  static boolean isProvider(String kind) {
    return kind.equals("edge-providers")
        || kind.equals("cloud-providers")
        || kind.equals("edge-classifiers");
  }

  static Map<String, Object> requestFields(String kind, Map<String, Object> input, boolean patch) {
    String fields =
        switch (kind) {
          case "edge-providers" ->
              "name,provider,endpoint,model,timeout_ms,max_tokens,temperature,top_p,seed,healthcheck_path,response_path"
                  + (patch ? ",status,is_active" : "");
          case "cloud-providers" ->
              "name,provider,endpoint,model,api_key_env,api_key,timeout_ms,max_tokens,temperature,top_p,seed,health_path,response_path"
                  + (patch ? ",status" : "");
          case "edge-classifiers" -> "name,endpoint,model,timeout_ms,threshold,status";
          case "edge-rules" ->
              "name,description,match_type,pattern,keywords,target_field,edge_pred,severity,confidence,evidence,advice_code,advice,priority,enabled,status";
          case "edge-templates" ->
              "template_text,raw_example,source,edge_pred,severity,confidence,evidence,advice_code,advice,status";
          case "from-cluster" ->
              "template_text,sample_raw_log,source,edge_pred,max_severity,avg_confidence,evidence,advice_code,advice,status";
          case "edge-rule-sets", "edge-template-sets" -> "name,description,item_ids";
          default -> throw new ApiFailure(404, "not found");
        };
    var out = new LinkedHashMap<String, Object>();
    for (var key : fields.split(",")) if (input.containsKey(key)) out.put(key, input.get(key));
    for (var key : List.of("evidence", "keywords"))
      if (out.get(key) instanceof List<?> values)
        out.put(key, values.stream().map(v -> v == null ? "" : v).toList());
    return out;
  }

  Map<String, Object> editable(String kind, Map<String, Object> req, Map<String, Object> current) {
    validateWireTypes(req);
    boolean provider = kind.endsWith("providers"), cloud = kind.equals("cloud-providers");
    var out = new LinkedHashMap<String, Object>();
    if (provider) {
      if (current == null) {
        for (var key : List.of("name", "provider", "endpoint", "model", "timeout_ms", "max_tokens"))
          if (str(req, key).isEmpty()) throw new ApiFailure(400, key + " is required");
        if (cloud) {
          for (var key : List.of("api_key_env", "api_key"))
            if (str(req, key).isEmpty()) throw new ApiFailure(400, key + " is required");
        } else {
          if (str(req, "response_path").isEmpty())
            throw new ApiFailure(400, "response_path is required");
          if (dec(req, "temperature") == 0 || dec(req, "top_p") == 0)
            throw new ApiFailure(400, "temperature and top_p are required");
        }
        if (num(req, "timeout_ms") == 0 || num(req, "max_tokens") == 0)
          throw new ApiFailure(400, "timeout_ms and max_tokens are required");
      }
      var fields =
          new ArrayList<>(
              List.of(
                  "name",
                  "provider",
                  "endpoint",
                  "model",
                  "timeout_ms",
                  "max_tokens",
                  "temperature",
                  "top_p",
                  "seed",
                  "health_path",
                  "response_path",
                  "status"));
      if (cloud) fields.addAll(List.of("api_key_env", "api_key"));
      else fields.add("is_active");
      for (var f : fields) {
        String input = f.equals("health_path") && !cloud ? "healthcheck_path" : f;
        Object value = current == null ? null : current.get(f);
        // Go encoding/json leaves the outer ** pointer nil for explicit JSON null as well as
        // absence.
        if (req.get(input) != null) value = req.get(input);
        if (value instanceof String s
            && (Set.of("name", "endpoint", "model", "response_path", "api_key_env", "api_key")
                    .contains(f)
                || cloud && f.equals("provider"))) value = s.trim();
        if (value == null && !Set.of("seed", "health_path").contains(f))
          value =
              Set.of("temperature", "top_p", "timeout_ms", "max_tokens").contains(f)
                  ? 0
                  : f.equals("is_active") ? false : "";
        out.put(f, value);
      }
      if (current == null) out.put("status", "draft");
      if (cloud) {
        if (current == null && dec(out, "top_p") == 0) out.put("top_p", .9);
        if (current == null && str(out, "response_path").isBlank())
          out.put("response_path", "choices[0].message.content");
        if (current != null && yes(current, "is_active"))
          throw new ApiFailure(409, "provider state conflict");
        if (current != null && trim(req, "api_key").isEmpty())
          out.put("api_key", current.get("api_key"));
        validateCloud(out);
        if (!Set.of("draft", "published", "disabled").contains(str(out, "status")))
          throw new ApiFailure(400, "invalid provider");
      } else {
        if (yes(out, "is_active") && !str(out, "status").equals("published"))
          throw new ApiFailure(400, "invalid state");
        if (Set.of("disabled", "deleted", "draft").contains(str(out, "status")))
          out.put("is_active", false);
      }
    } else if (kind.equals("edge-classifiers")) {
      required(req, "name", "endpoint", "model", "timeout_ms");
      if (num(req, "timeout_ms") <= 0 || dec(req, "threshold") < 0 || dec(req, "threshold") > 1)
        throw new ApiFailure(400, "bad request");
      for (var f : List.of("name", "endpoint", "model")) out.put(f, trim(req, f));
      out.put("timeout_ms", num(req, "timeout_ms"));
      out.put("threshold", dec(req, "threshold"));
      var status = def(str(req, "status"), current == null ? "draft" : str(current, "status"));
      if (!Set.of("draft", "published", "disabled", "deleted").contains(status))
        throw new ApiFailure(400, "bad request");
      out.put("status", status);
      if (current != null)
        out.put("is_active", status.equals("published") && yes(current, "is_active"));
    } else {
      required(req, "edge_pred");
      if (!PREDICTIONS.contains(str(req, "edge_pred"))
          || num(req, "severity") < 0
          || num(req, "severity") > 100
          || dec(req, "confidence") < 0
          || dec(req, "confidence") > 1) throw new ApiFailure(400, "bad request");
      out.put("edge_pred", str(req, "edge_pred"));
      out.put("severity", num(req, "severity"));
      out.put("confidence", dec(req, "confidence"));
      out.put("evidence_json", Json.write(req.get("evidence")));
      out.put("advice_code", trim(req, "advice_code"));
      out.put("advice", trim(req, "advice"));
      out.put(
          "status", def(str(req, "status"), current == null ? "draft" : str(current, "status")));
      if (kind.equals("edge-rules")) {
        required(req, "name", "match_type");
        var type = trim(req, "match_type");
        if (!Set.of("contains", "regex", "all_keywords", "any_keywords").contains(type))
          throw new ApiFailure(400, "bad request");
        if (Set.of("regex", "contains").contains(type)) required(req, "pattern");
        else if (Values.list(req.get("keywords")).isEmpty())
          throw new ApiFailure(400, "bad request");
        if (type.equals("regex"))
          try {
            com.google.re2j.Pattern.compile(str(req, "pattern"));
          } catch (Exception e) {
            throw new ApiFailure(400, "bad request: invalid regex");
          }
        out.put("name", trim(req, "name"));
        out.put("description", trim(req, "description"));
        out.put("match_type", type);
        out.put("pattern", trim(req, "pattern"));
        out.put("keywords_json", Json.write(req.get("keywords")));
        out.put("target_field", def(str(req, "target_field"), "template_text"));
        out.put("priority", num(req, "priority"));
        out.put("enabled", yes(req, "enabled"));
      } else {
        required(req, "template_text");
        out.put("template_id", hash(trim(req, "template_text")).substring(0, 16));
        out.put("template_text", trim(req, "template_text"));
        out.put("raw_example", trim(req, "raw_example"));
        out.put("source", def(str(req, "source"), "manual"));
      }
    }
    return out;
  }

  static void validateCloud(Map<String, Object> p) {
    if (!Set.of("openai_compat", "deepseek", "qwen", "custom_http").contains(str(p, "provider"))
        || trim(p, "name").isEmpty()
        || trim(p, "endpoint").isEmpty()
        || trim(p, "model").isEmpty()
        || !trim(p, "api_key_env").matches("[A-Za-z_][A-Za-z0-9_]*")
        || num(p, "timeout_ms") <= 0
        || num(p, "max_tokens") <= 0
        || dec(p, "temperature") < 0
        || dec(p, "temperature") > 2
        || dec(p, "top_p") <= 0
        || dec(p, "top_p") > 1) throw new ApiFailure(400, "invalid provider");
  }

  static void validateWireTypes(Map<String, Object> p) {
    for (var k :
        List.of(
            "timeout_ms",
            "max_tokens",
            "temperature",
            "top_p",
            "seed",
            "threshold",
            "severity",
            "confidence",
            "priority",
            "max_severity",
            "avg_confidence"))
      if (p.get(k) != null && !(p.get(k) instanceof Number))
        throw new ApiFailure(400, "invalid numeric field: " + k);
    for (var k : List.of("is_active", "enabled"))
      if (p.get(k) != null && !(p.get(k) instanceof Boolean))
        throw new ApiFailure(400, "invalid boolean field: " + k);
    for (var k : List.of("evidence", "keywords"))
      if (p.get(k) != null
          && (!(p.get(k) instanceof List<?> list)
              || list.stream().anyMatch(v -> !(v instanceof String))))
        throw new ApiFailure(400, "invalid string array: " + k);
    for (var k :
        List.of(
            "name",
            "provider",
            "endpoint",
            "model",
            "health_path",
            "healthcheck_path",
            "response_path",
            "status",
            "api_key_env",
            "api_key",
            "description",
            "match_type",
            "pattern",
            "target_field",
            "edge_pred",
            "advice_code",
            "advice",
            "template_text",
            "raw_example",
            "sample_raw_log",
            "source"))
      if (p.get(k) != null && !(p.get(k) instanceof String))
        throw new ApiFailure(400, "invalid string field: " + k);
    for (var k :
        List.of("timeout_ms", "max_tokens", "seed", "severity", "priority", "max_severity"))
      if (p.get(k) instanceof Number n
          && (n instanceof Float
              || n instanceof Double
              || n instanceof java.math.BigDecimal
              || n instanceof java.math.BigInteger b && b.bitLength() > 63))
        throw new ApiFailure(400, "invalid integer field: " + k);
  }

  public void delete(String kind, long id) {
    var p = raw(kind, id);
    if (isProvider(kind) && yes(p, "is_active"))
      throw new ApiFailure(
          409,
          kind.equals("cloud-providers")
              ? "provider state conflict"
              : kind.equals("edge-classifiers")
                  ? "active classifier cannot be deleted"
                  : "active provider cannot be deleted");
    var updates = obj("status", "deleted", "deleted_at", now(), "updated_at", now());
    if (kind.equals("edge-providers")) updates.put("is_active", false);
    update(table(kind), id, updates);
  }

  public Map<String, Object> activate(String kind, long id) {
    // Provider stores look up/validate before the original transaction; classifiers and sets do so
    // inside it.
    var before = kind.endsWith("providers") ? raw(kind, id) : null;
    if (kind.equals("edge-providers")
        && Set.of("disabled", "deleted").contains(str(before, "status")))
      throw new ApiFailure(400, "invalid state");
    if (kind.equals("cloud-providers")) validateCloud(before);
    return tx.execute(
        status -> {
          var t = table(kind);
          var p = before == null ? raw(kind, id) : before;
          boolean set = kind.endsWith("-sets");
          if (!set
              && !kind.equals("cloud-providers")
              && Set.of("disabled", "deleted").contains(str(p, "status")))
            throw new ApiFailure(
                400, kind.equals("edge-classifiers") ? "bad request" : "invalid state");
          if (kind.equals("cloud-providers")) validateCloud(p);
          long version =
              count(
                  "SELECT COALESCE(MAX(config_version),0)+1 FROM "
                      + t
                      + (set ? "" : " FOR UPDATE"));
          String tag = etag(kind, version, p);
          if (set)
            jdbc.update(
                "UPDATE " + t + " SET status='archived',updated_at=? WHERE status='active'", now());
          else
            jdbc.update(
                "UPDATE " + t + " SET is_active=false,updated_at=? WHERE is_active=true", now());
          var updates =
              obj(
                  "status",
                  set ? "active" : "published",
                  "config_version",
                  version,
                  "etag",
                  tag,
                  "updated_at",
                  now());
          if (!set) updates.put("is_active", true);
          if (kind.endsWith("providers")) updates.put("last_publish_at", now());
          update(t, id, updates);
          if (set)
            return obj("version", version, "etag", tag, "items", parseStored(p.get("items_json")));
          var active = kind.equals("edge-providers") ? committedEdgeProvider(id) : get(kind, id);
          if (kind.equals("edge-providers"))
            return obj(
                "config_version",
                version,
                "etag",
                tag,
                "active_provider",
                active,
                "classifier",
                null,
                "rules",
                obj("version", 0, "etag", "", "items", null),
                "templates",
                obj("version", 0, "etag", "", "items", null));
          return active;
        });
  }

  Map<String, Object> committedEdgeProvider(long id) {
    // Observable legacy behavior: s.Get in Go uses the global pool while Activate has not
    // committed.
    // A separate JDBC connection therefore returns the prior committed provider, with the NEW outer
    // version/ETag.
    try (var connection = Objects.requireNonNull(jdbc.getDataSource()).getConnection();
        var statement = connection.prepareStatement("SELECT * FROM edge_providers WHERE id=?")) {
      statement.setQueryTimeout(8);
      statement.setLong(1, id);
      try (var result = statement.executeQuery()) {
        if (!result.next()) throw new ApiFailure(404, "not found");
        return present("edge-providers", row(result));
      }
    } catch (SQLException e) {
      throw new IllegalStateException("read committed provider", e);
    }
  }

  static String etag(String kind, long v, Map<String, Object> p) {
    if (kind.endsWith("-sets")) {
      var prefix = kind.equals("edge-rule-sets") ? "rules" : "templates";
      return "W/\""
          + prefix
          + "-"
          + v
          + "-"
          + hash(prefix + ":" + v + ":" + str(p, "items_json")).substring(0, 8)
          + "\"";
    }
    if (kind.equals("cloud-providers"))
      return "W/\"cloud-config-"
          + v
          + "-"
          + hash(str(p, "endpoint") + "|" + str(p, "model") + "|" + str(p, "api_key_env") + "|" + v)
              .substring(0, 8)
          + "\"";
    if (kind.equals("edge-classifiers"))
      return "W/\"classifier-"
          + v
          + "-"
          + hash(String.format(
                  Locale.ROOT,
                  "%d|%s|%s|%s|%d|%f",
                  v,
                  str(p, "name"),
                  str(p, "endpoint"),
                  str(p, "model"),
                  num(p, "timeout_ms"),
                  dec(p, "threshold")))
              .substring(0, 8)
          + "\"";
    String content =
        String.format(
            Locale.ROOT,
            "%d|%s|%s|%s|%s|%d|%d|%f|%f|%s|%s",
            v,
            str(p, "name"),
            str(p, "provider"),
            str(p, "endpoint"),
            str(p, "model"),
            num(p, "timeout_ms"),
            num(p, "max_tokens"),
            dec(p, "temperature"),
            dec(p, "top_p"),
            str(p, "health_path"),
            str(p, "response_path"));
    if (p.get("seed") != null) content += "|seed=" + num(p, "seed");
    return "W/\"" + v + "-" + hash(content).substring(0, 6) + "\"";
  }

  public Map<String, Object> active(String kind) {
    var rows =
        rows(
            "SELECT * FROM "
                + table(kind)
                + " WHERE is_active=true AND status='published' AND deleted_at IS NULL ORDER BY id"
                + " LIMIT 1");
    return rows.isEmpty() ? null : present(kind, rows.getFirst());
  }

  Map<String, Object> activeSet(String kind) {
    var rows =
        rows(
            "SELECT * FROM "
                + table(kind)
                + " WHERE status='active' ORDER BY config_version DESC LIMIT 1");
    if (rows.isEmpty()) return obj("version", 0, "etag", "", "items", List.of());
    var p = rows.getFirst();
    return obj(
        "version",
        p.get("config_version"),
        "etag",
        p.get("etag"),
        "items",
        parseStored(p.get("items_json")));
  }

  public Map<String, Object> edgeConfig() {
    var provider = active("edge-providers");
    var rules = activeSet("edge-rule-sets");
    var templates = activeSet("edge-template-sets");
    var classifier = active("edge-classifiers");
    long version =
        Math.max(
            provider == null ? 0 : num(provider, "config_version"),
            Math.max(num(rules, "version"), num(templates, "version")));
    if (classifier != null) version = Math.max(version, num(classifier, "config_version"));
    var tag =
        combine(
            provider == null ? "" : str(provider, "etag"),
            str(rules, "etag"),
            str(templates, "etag"));
    tag = combine(tag, classifier == null ? "" : str(classifier, "etag"));
    return obj(
        "config_version",
        version,
        "etag",
        tag,
        "active_provider",
        provider,
        "classifier",
        classifier,
        "rules",
        rules,
        "templates",
        templates);
  }

  static String combine(String... parts) {
    return "W/\"edge-config-" + hash("|" + String.join("|", parts)).substring(0, 16) + "\"";
  }

  Map<String, Object> createSet(String kind, Map<String, Object> req) {
    validateWireTypes(req);
    if (req.get("item_ids") != null
        && (!(req.get("item_ids") instanceof List<?> values)
            || values.stream().anyMatch(v -> !(v instanceof Integer || v instanceof Long))))
      throw new ApiFailure(400, "invalid integer array: item_ids");
    if (str(req, "name").isEmpty()) throw new ApiFailure(400, "name is required");
    var ids = Values.list(req.get("item_ids"));
    String itemKind = kind.equals("edge-rule-sets") ? "edge-rules" : "edge-templates";
    var query =
        "SELECT * FROM " + table(itemKind) + " WHERE deleted_at IS NULL AND status<>'deleted'";
    if (!ids.isEmpty())
      query += " AND id IN (" + String.join(",", Collections.nCopies(ids.size(), "?")) + ")";
    if (itemKind.equals("edge-rules")) query += " AND enabled=true ORDER BY priority ASC";
    else query += " ORDER BY updated_at DESC";
    var items = rows(query, ids.toArray()).stream().map(r -> present(itemKind, r)).toList();
    var data =
        obj(
            "name",
            trim(req, "name"),
            "description",
            trim(req, "description"),
            "status",
            "draft",
            "config_version",
            0,
            "etag",
            "",
            "item_ids_json",
            Json.write(req.get("item_ids")),
            "items_json",
            Json.write(items),
            "item_count",
            items.size(),
            "created_at",
            now(),
            "updated_at",
            now());
    return get(kind, insert(table(kind), data));
  }

  public Map<String, Object> fromCluster(Map<String, Object> req) {
    req = requestFields("from-cluster", req, false);
    validateWireTypes(req);
    var item =
        obj(
            "template_text",
            trim(req, "template_text"),
            "raw_example",
            trim(req, "sample_raw_log"),
            "source",
            def(str(req, "source"), "abnormal_cluster"),
            "edge_pred",
            str(req, "edge_pred"),
            "severity",
            num(req, "max_severity"),
            "confidence",
            dec(req, "avg_confidence"),
            "evidence",
            req.get("evidence"),
            "advice_code",
            trim(req, "advice_code"),
            "advice",
            trim(req, "advice"),
            "status",
            def(str(req, "status"), "draft"));
    editable("edge-templates", item, null);
    var existing =
        rows(
            "SELECT * FROM edge_templates WHERE template_id=? AND deleted_at IS NULL ORDER BY id"
                + " LIMIT 1",
            hash(trim(req, "template_text")).substring(0, 16));
    return existing.isEmpty()
        ? obj("created", true, "item", create("edge-templates", item))
        : obj("created", false, "item", present("edge-templates", existing.getFirst()));
  }

  void seedBuiltinRules() {
    var definitions =
        List.of(
            obj(
                "name",
                "内置-OOM/内存不足",
                "pattern",
                "out of memory|oom[- ]?killed|cannot allocate memory|memory allocation failed",
                "edge_pred",
                "INFRA_ERROR",
                "severity",
                95,
                "confidence",
                .99,
                "evidence",
                List.of("out of memory", "oom killed"),
                "advice_code",
                "CHECK_MEMORY",
                "advice",
                "检查容器/进程内存限制、节点内存水位和近期内存突增进程。",
                "priority",
                1000),
            obj(
                "name",
                "内置-磁盘空间不足",
                "pattern",
                "no space left on device|disk (?:is )?full|insufficient disk"
                    + " space|inode.*(full|exhausted)",
                "edge_pred",
                "DISK_ERROR",
                "severity",
                94,
                "confidence",
                .99,
                "evidence",
                List.of("disk full"),
                "advice_code",
                "CHECK_DISK_SPACE",
                "advice",
                "检查磁盘空间、inode、日志滚动策略和临时文件清理情况。",
                "priority",
                990),
            obj(
                "name",
                "内置-网络连接失败",
                "pattern",
                "connection refused|connection reset by peer|network is unreachable|no route to"
                    + " host|connection timed out",
                "edge_pred",
                "NET_ERROR",
                "severity",
                82,
                "confidence",
                .95,
                "evidence",
                List.of("network connection failure"),
                "advice_code",
                "CHECK_NETWORK_AND_SERVICE",
                "advice",
                "检查目标服务监听状态、Service/Endpoint、网络策略和跨节点连通性。",
                "priority",
                900),
            obj(
                "name",
                "内置-请求超时",
                "pattern",
                "context deadline exceeded|i/o timeout|request timed out|timeout awaiting"
                    + " headers|gateway timeout",
                "edge_pred",
                "NET_ERROR",
                "severity",
                76,
                "confidence",
                .9,
                "evidence",
                List.of("timeout"),
                "advice_code",
                "CHECK_TIMEOUT_AND_LATENCY",
                "advice",
                "检查依赖服务延迟、网络抖动、超时配置和下游负载。",
                "priority",
                860),
            obj(
                "name",
                "内置-认证鉴权失败",
                "pattern",
                "authentication failed|access denied|unauthorized|permission"
                    + " denied|forbidden|invalid token|token expired",
                "edge_pred",
                "SECURITY",
                "severity",
                78,
                "confidence",
                .92,
                "evidence",
                List.of("auth failure"),
                "advice_code",
                "CHECK_AUTH",
                "advice",
                "检查访问凭据、Token 有效期、账号权限、RBAC 和异常登录来源。",
                "priority",
                840));
    var ids = new ArrayList<Object>();
    for (var rule : definitions) {
      rule.putAll(
          obj(
              "description",
              "系统内置规则，可作为初始分诊能力直接启用。",
              "match_type",
              "regex",
              "target_field",
              "template_text",
              "enabled",
              true,
              "status",
              "published"));
      var found =
          rows(
              "SELECT id,enabled FROM edge_rules WHERE name=? AND deleted_at IS NULL ORDER BY id"
                  + " LIMIT 1",
              str(rule, "name"));
      if (found.isEmpty()) ids.add(create("edge-rules", rule).get("id"));
      else if (yes(found.getFirst(), "enabled")) ids.add(found.getFirst().get("id"));
    }
    if (!ids.isEmpty() && count("SELECT COUNT(*) FROM edge_rule_sets WHERE status='active'") == 0) {
      var set =
          createSet(
              "edge-rule-sets",
              obj(
                  "name",
                  "内置常用规则集",
                  "description",
                  "系统预置的常见日志分诊规则，可直接启用作为 RULE 路径默认规则集。",
                  "item_ids",
                  ids));
      activate("edge-rule-sets", num(set, "id"));
    }
  }
}
