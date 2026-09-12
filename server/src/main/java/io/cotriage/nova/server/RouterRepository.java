package io.cotriage.nova.server;

import static io.cotriage.nova.server.Values.*;

import java.sql.Timestamp;
import java.time.*;
import java.util.*;
import org.springframework.dao.DataAccessException;
import org.springframework.jdbc.core.JdbcTemplate;

public class RouterRepository {
  final JdbcTemplate jdbc;

  public RouterRepository(JdbcTemplate jdbc) {
    this.jdbc = jdbc;
  }

  public boolean available() {
    return jdbc != null;
  }

  private static final Map<String, String> FIELDS =
      Map.of(
          "decisions",
              "id,event_id,trace_id,schema_version,source,equipment,event_ts,inferred_at,template_id,raw_log_hash,edge_pred,severity,confidence,edge_uncertainty,inference_path,model_version,rule_version,action,reason,policy_ref,score,threshold,cloud_health,raise_version,latency_ms,routed_at,created_at,updated_at",
          "outbox",
              "id,event_id,request_id,idempotency_key,payload_json,route_decision_json,priority_score,defer_reason,status,attempt,max_attempts,next_retry_at,deadline_at,created_at,updated_at,locked_by,locked_until,sent_at,done_at,last_error,last_error_at,dlq_reason",
          "cloud-feedback",
              "id,event_id,request_id,source,equipment,template_id,edge_pred,edge_confidence,cloud_model,cloud_pred,root_cause,recommendation,explanation,cloud_confidence,cloud_latency_ms,token_usage,cost,reward,reward_reason,corrected_edge,added_actionable_info,confirmed_high_risk,processed_by_router,processed_at,created_at",
          "state",
              "id,snapshot_type,snapshot_key,payload_json,version,checksum,is_active,restored_at,metadata_json,created_at");

  static Map<String, Object> present(String kind, Map<String, Object> row) {
    var out = new LinkedHashMap<String, Object>();
    for (var k : FIELDS.get(kind).split(",")) {
      Object value = row.get(k);
      if (value == null) {
        if (Set.of(
                "event_ts",
                "inferred_at",
                "next_retry_at",
                "deadline_at",
                "locked_until",
                "sent_at",
                "done_at",
                "last_error_at",
                "processed_at",
                "restored_at")
            .contains(k)) value = null;
        else if (Set.of("created_at", "updated_at", "routed_at").contains(k))
          value = "0001-01-01T00:00:00Z";
        else if (Set.of(
                "is_active",
                "corrected_edge",
                "added_actionable_info",
                "confirmed_high_risk",
                "processed_by_router")
            .contains(k)) value = false;
        else if (Set.of(
                "id",
                "severity",
                "latency_ms",
                "attempt",
                "max_attempts",
                "cloud_latency_ms",
                "token_usage",
                "confidence",
                "edge_uncertainty",
                "score",
                "threshold",
                "priority_score",
                "edge_confidence",
                "cloud_confidence",
                "cost",
                "reward")
            .contains(k)) value = 0;
        else value = "";
      }
      out.put(k, value);
    }
    return out;
  }

  List<Map<String, Object>> rows(String sql, Object... args) {
    if (jdbc == null) return List.of();
    try {
      return jdbc.query(sql, (rs, i) -> ConfigRepository.row(rs), args);
    } catch (DataAccessException e) {
      if (missingTable(e)) return List.of();
      throw new ApiFailure(503, "router store query: " + e.getMostSpecificCause().getMessage());
    }
  }

  static boolean missingTable(Throwable e) {
    for (var c = e; c != null; c = c.getCause()) {
      var m = Objects.toString(c.getMessage(), "").toLowerCase(Locale.ROOT);
      if (m.contains("doesn't exist")
          || m.contains("does not exist")
          || m.contains("unknown table")
          || m.contains("table") && m.contains("not found")) return true;
    }
    return false;
  }

  long count(String table, String condition, Object... args) {
    var rows =
        rows(
            "SELECT COUNT(*) count FROM "
                + table
                + (condition.isEmpty() ? "" : " WHERE " + condition),
            args);
    return rows.isEmpty() ? 0 : num(rows.getFirst(), "count");
  }

  public Map<String, Object> list(String kind, Map<String, String> q) {
    var table =
        switch (kind) {
          case "decisions" -> "router_decisions";
          case "outbox" -> "cloud_outbox";
          case "cloud-feedback" -> "cloud_feedback";
          default -> throw new ApiFailure(404, "not found");
        };
    int limit = integer(q, "limit", 50, 1, 200), offset = integer(q, "offset", 0, 0, 10000);
    var filters = new ArrayList<String>();
    var args = new ArrayList<Object>();
    for (var f :
        kind.equals("decisions")
            ? List.of("action", "template_id", "source")
            : kind.equals("outbox") ? List.of("status") : List.<String>of())
      if (!trim(q, f).isEmpty()) {
        filters.add(f + "=?");
        args.add(trim(q, f));
      }
    String where = String.join(" AND ", filters);
    long total = count(table, where, args.toArray());
    String order =
        kind.equals("decisions")
            ? "routed_at DESC,id DESC"
            : kind.equals("outbox")
                ? "priority_score DESC,created_at DESC"
                : "created_at DESC,id DESC";
    args.add(limit);
    args.add(offset);
    var items =
        rows(
            "SELECT * FROM "
                + table
                + (where.isEmpty() ? "" : " WHERE " + where)
                + " ORDER BY "
                + order
                + " LIMIT ? OFFSET ?",
            args.toArray());
    return obj(
        "items",
        items.stream().map(r -> present(kind, r)).toList(),
        "total",
        total,
        "limit",
        limit,
        "offset",
        offset,
        "degraded",
        !available());
  }

  public Map<String, Object> decision(String eventId) {
    var items =
        rows("SELECT * FROM router_decisions WHERE event_id=? ORDER BY id LIMIT 1", eventId);
    if (items.isEmpty()) throw new ApiFailure(404, "router decision not found");
    return present("decisions", items.getFirst());
  }

  public Map<String, Object> state(String type) {
    var items =
        rows(
            "SELECT * FROM router_state_snapshots"
                + (type.isBlank() ? "" : " WHERE snapshot_type=?")
                + " ORDER BY is_active DESC,created_at DESC,id DESC LIMIT 1",
            type.isBlank() ? new Object[0] : new Object[] {type});
    if (items.isEmpty()) return obj("available", false);
    var item = present("state", items.getFirst());
    var out = obj("available", true, "item", item);
    if (validJson(str(item, "payload_json"))) out.put("payload", json(str(item, "payload_json")));
    return out;
  }

  public Map<String, Object> summary() {
    var out =
        obj(
            "routed_total",
            0L,
            "local_total",
            0L,
            "cloud_total",
            0L,
            "defer_total",
            0L,
            "local_ratio",
            0d,
            "cloud_ratio",
            0d,
            "defer_ratio",
            0d,
            "outbox_pending",
            0L,
            "outbox_retry",
            0L,
            "outbox_dlq",
            0L,
            "p95_route_latency_ms",
            0L,
            "average_score",
            0d,
            "average_threshold",
            0d,
            "last_action",
            "");
    var totals =
        rows(
            "SELECT COUNT(*) routed_total,AVG(score) average_score,AVG(threshold) average_threshold"
                + " FROM router_decisions");
    if (totals.isEmpty()) return out;
    totals.getFirst().forEach((k, v) -> out.put(k, v == null ? 0 : v));
    long total = num(out, "routed_total");
    for (var action : List.of("LOCAL", "CLOUD", "DEFER")) {
      long n = count("router_decisions", "action=?", action);
      out.put(action.toLowerCase() + "_total", n);
      out.put(action.toLowerCase() + "_ratio", total > 0 ? (double) n / total : 0);
    }
    for (var status : List.of("PENDING", "RETRY", "DLQ"))
      out.put("outbox_" + status.toLowerCase(), count("cloud_outbox", "status=?", status));
    var latencies =
        rows(
                "SELECT latency_ms FROM router_decisions WHERE latency_ms IS NOT NULL ORDER BY"
                    + " routed_at DESC LIMIT 500")
            .stream()
            .map(r -> num(r, "latency_ms"))
            .toList();
    out.put("p95_route_latency_ms", percentile(latencies, 95));
    var last =
        rows(
            "SELECT action,routed_at FROM router_decisions ORDER BY routed_at DESC,id DESC LIMIT"
                + " 1");
    if (!last.isEmpty()) {
      out.put("last_action", last.getFirst().get("action"));
      out.put("last_routed_at", last.getFirst().get("routed_at"));
    }
    var snapshots =
        rows(
            "SELECT created_at,version FROM router_state_snapshots WHERE snapshot_type IN"
                + " ('router_state','system_state') ORDER BY created_at DESC,id DESC LIMIT 1");
    if (!snapshots.isEmpty()) {
      out.put("latest_state_snapshot_at", snapshots.getFirst().get("created_at"));
      out.put("latest_state_snapshot_version", snapshots.getFirst().get("version"));
    }
    return out;
  }

  public Map<String, Object> competition() {
    var m = new LinkedHashMap<String, Object>();
    var routing =
        rows(
            "SELECT COUNT(*) routed_total,SUM(action='LOCAL') local_total,SUM(action='CLOUD')"
                + " cloud_total,SUM(action='DEFER') defer_total,COUNT(DISTINCT NULLIF(source,''))"
                + " distinct_sources,COUNT(DISTINCT NULLIF(equipment,'')) distinct_equipment FROM"
                + " router_decisions");
    if (routing.isEmpty()) return m;
    m.putAll(routing.getFirst());
    var feedback =
        rows(
            "SELECT COUNT(*) cloud_samples,COALESCE(AVG(cloud_latency_ms),0)"
                + " cloud_latency_avg_ms,COALESCE(SUM(token_usage),0)"
                + " cloud_token_total,COALESCE(SUM(cost),0)"
                + " cloud_cost_total,SUM(UPPER(edge_pred)=UPPER(cloud_pred))"
                + " edge_cloud_agree,SUM(UPPER(edge_pred)<>UPPER(cloud_pred))"
                + " edge_cloud_conflict,SUM(UPPER(edge_pred)<>UPPER(cloud_pred) AND"
                + " processed_by_router=1) conflict_resolved,SUM(added_actionable_info=1)"
                + " actionable_total,SUM(corrected_edge=1) corrected_total,COALESCE(AVG(reward),0)"
                + " reward_average FROM cloud_feedback");
    if (feedback.isEmpty()) return m;
    m.putAll(feedback.getFirst());
    var outbox =
        rows(
            "SELECT COUNT(*) outbox_total,SUM(status IN ('SENT','DONE'))"
                + " outbox_success,SUM(status='RETRY') outbox_retry,SUM(status='DLQ') outbox_dlq"
                + " FROM cloud_outbox");
    if (!outbox.isEmpty()) m.putAll(outbox.getFirst());
    return m;
  }

  public void saveSnapshot(String payload, String window, Instant at) {
    if (jdbc != null) {
      jdbc.update(
          "INSERT INTO competition_metric_snapshots (`window`,payload_json,collected_at) VALUES"
              + " (?,?,?)",
          window,
          payload,
          Timestamp.from(at));
      jdbc.update(
          "DELETE FROM competition_metric_snapshots WHERE collected_at<?",
          Timestamp.from(at.minus(Duration.ofDays(7))));
    }
  }

  public Map<String, Object> history(int limit) {
    var items = new ArrayList<Object>();
    for (var row :
        rows(
            "SELECT payload_json FROM competition_metric_snapshots ORDER BY collected_at DESC LIMIT"
                + " ?",
            limit)) {
      var payload = str(row, "payload_json");
      if (validJson(payload)) items.add(json(payload));
    }
    return obj("items", items, "total", items.size());
  }
}
