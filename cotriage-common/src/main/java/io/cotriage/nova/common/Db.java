package io.cotriage.nova.common;

import com.zaxxer.hikari.HikariConfig;
import com.zaxxer.hikari.HikariDataSource;
import java.net.URLDecoder;
import java.nio.charset.StandardCharsets;
import java.util.LinkedHashMap;
import java.util.Map;
import org.springframework.jdbc.core.JdbcTemplate;

/** Explicit adapter for existing go-sql-driver/mysql TCP DSNs. */
public final class Db {
  public record ConnectionSpec(String url, String username, String password) {}

  private Db() {}

  public static ConnectionSpec parse(String dsn) {
    int marker = dsn.lastIndexOf("@tcp("), close = marker < 0 ? -1 : dsn.indexOf(")/", marker + 5);
    if (marker < 0 || close < 0)
      throw new IllegalArgumentException(
          "MYSQL_DSN must use user:password@tcp(host:port)/database");
    String auth = dsn.substring(0, marker);
    int colon = auth.indexOf(':');
    String user = colon < 0 ? auth : auth.substring(0, colon),
        password = colon < 0 ? "" : auth.substring(colon + 1);
    String endpoint = dsn.substring(marker + 5, close), databaseAndQuery = dsn.substring(close + 2);
    int queryIndex = databaseAndQuery.indexOf('?');
    String database = queryIndex < 0 ? databaseAndQuery : databaseAndQuery.substring(0, queryIndex);
    Map<String, String> params = new LinkedHashMap<>();
    params.put("characterEncoding", "UTF-8");
    params.put("connectionTimeZone", "UTC");
    params.put("preserveInstants", "true");
    params.put("connectTimeout", "4000");
    params.put("allowPublicKeyRetrieval", "true");
    params.put("useSSL", "false");
    if (queryIndex >= 0)
      for (String item : databaseAndQuery.substring(queryIndex + 1).split("&")) {
        String[] pair = item.split("=", 2);
        String key = URLDecoder.decode(pair[0], StandardCharsets.UTF_8),
            value = pair.length > 1 ? URLDecoder.decode(pair[1], StandardCharsets.UTF_8) : "";
        switch (key) {
          case "parseTime", "charset" -> {}
          case "loc" -> params.put("connectionTimeZone", value);
          case "timeout" ->
              params.put("connectTimeout", Long.toString(Env.parseDuration(value).toMillis()));
          case "readTimeout" ->
              params.put("socketTimeout", Long.toString(Env.parseDuration(value).toMillis()));
          case "writeTimeout" -> {} // Connector/J has no independent write timeout.
          case "tls" -> {
            params.put("useSSL", Boolean.toString(!value.equals("false")));
            params.put("verifyServerCertificate", Boolean.toString(!value.equals("skip-verify")));
          }
          default -> params.put(key, value);
        }
      }
    StringBuilder query = new StringBuilder();
    params.forEach(
        (k, v) -> {
          if (!query.isEmpty()) query.append('&');
          query.append(k).append('=').append(java.net.URLEncoder.encode(v, StandardCharsets.UTF_8));
        });
    return new ConnectionSpec(
        "jdbc:mysql://" + endpoint + "/" + database + "?" + query, user, password);
  }

  public static HikariDataSource open(String dsn) {
    var spec = parse(dsn);
    var config = new HikariConfig();
    config.setJdbcUrl(spec.url());
    config.setUsername(spec.username());
    config.setPassword(spec.password());
    config.setMaximumPoolSize(10);
    config.setMinimumIdle(0);
    config.setMaxLifetime(300_000);
    config.setConnectionTimeout(4_000);
    config.setInitializationFailTimeout(4_000);
    return new HikariDataSource(config);
  }

  public static JdbcTemplate jdbc(String dsn) {
    return new JdbcTemplate(open(dsn));
  }
}
