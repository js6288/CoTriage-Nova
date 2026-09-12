package io.cotriage.nova.gateway;

import java.util.*;
import org.springframework.dao.DuplicateKeyException;
import org.springframework.jdbc.core.JdbcTemplate;

public class GatewayStore {
  final JdbcTemplate db;

  public GatewayStore(JdbcTemplate db) {
    this.db = db;
  }

  public boolean exists(String eventID) {
    Long n =
        db.queryForObject(
            "SELECT COUNT(*) FROM cloud_feedback WHERE event_id=?", Long.class, eventID);
    return n != null && n > 0;
  }

  public Map<String, Object> get(String eventID) {
    return db.queryForMap("SELECT * FROM cloud_feedback WHERE event_id=? LIMIT 1", eventID);
  }

  public void insert(Map<String, Object> values) {
    String columns = String.join(",", values.keySet()),
        slots = String.join(",", Collections.nCopies(values.size(), "?"));
    try {
      db.update(
          "INSERT INTO cloud_feedback (" + columns + ") VALUES (" + slots + ")",
          values.values().toArray());
    } catch (DuplicateKeyException ignored) {
      /* Same event uniqueness behavior as GORM TranslateError. */
    }
  }
}
