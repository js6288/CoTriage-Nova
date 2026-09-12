package io.cotriage.nova.router;

import static io.cotriage.nova.router.Events.*;

import io.cotriage.nova.common.Json;
import java.sql.Timestamp;
import java.time.*;
import java.util.*;
import org.springframework.core.io.ClassPathResource;
import org.springframework.jdbc.core.JdbcTemplate;

/** Deliberately keeps the original individual SQL statements and commit boundaries. */
public class RouterStore {
  final JdbcTemplate db;

  public RouterStore(JdbcTemplate db) {
    this.db = db;
  }

  public void ensureSchema() {
    io.cotriage.nova.common.Schema.ensure(db, new ClassPathResource("router-schema.sql"));
  }

  static Timestamp ts(Instant i) {
    return Timestamp.from(i);
  }

  static Timestamp parse(String s) {
    try {
      return ts(OffsetDateTime.parse(s).toInstant());
    } catch (Exception e) {
      return null;
    }
  }

  public void insert(String table, Map<String, Object> values, boolean ignore) {
    String columns = String.join(",", values.keySet()),
        slots = String.join(",", Collections.nCopies(values.size(), "?"));
    db.update(
        "INSERT INTO "
            + table
            + " ("
            + columns
            + ") VALUES ("
            + slots
            + ")"
            + (ignore ? " ON DUPLICATE KEY UPDATE id=id" : ""),
        values.values().toArray());
  }

  public void decision(Events.Inferred in, RaiseEngine.Decision d, Instant now, String hash) {
    insert(
        "router_decisions",
        Json.object(
            "event_id",
            in.event_id,
            "trace_id",
            in.trace_id,
            "schema_version",
            in.schema_version,
            "source",
            in.source,
            "equipment",
            in.equipment,
            "event_ts",
            parse(in.event_ts),
            "inferred_at",
            parse(in.inferred_at),
            "template_id",
            in.template_id,
            "raw_log_hash",
            hash,
            "edge_pred",
            in.edge_pred,
            "severity",
            in.severity,
            "confidence",
            in.confidence,
            "edge_uncertainty",
            in.uncertainty,
            "inference_path",
            in.inference_path,
            "model_version",
            in.model_version,
            "rule_version",
            in.rule_version,
            "action",
            d.Action(),
            "reason",
            d.Reason(),
            "policy_ref",
            d.PolicyRef(),
            "score",
            d.Score(),
            "threshold",
            d.Threshold(),
            "cloud_health",
            d.CloudHealth(),
            "raise_version",
            d.RaiseVersion(),
            "latency_ms",
            in.latency_ms,
            "routed_at",
            ts(now),
            "created_at",
            ts(now),
            "updated_at",
            ts(now)),
        true);
  }

  public void outbox(Events.Routed r, int maxAttempts, Instant now) {
    insert(
        "cloud_outbox",
        Json.object(
            "event_id",
            r.input().event_id,
            "request_id",
            "cloud-" + r.input().event_id,
            "idempotency_key",
            "router-cloud-" + r.input().event_id,
            "payload_json",
            Json.write(r),
            "route_decision_json",
            Json.write(r.decision()),
            "priority_score",
            r.decision().Score(),
            "defer_reason",
            r.decision().Reason(),
            "status",
            "PENDING",
            "attempt",
            0,
            "max_attempts",
            maxAttempts,
            "deadline_at",
            ts(now.plusSeconds(600)),
            "created_at",
            ts(now),
            "updated_at",
            ts(now),
            "locked_by",
            "",
            "last_error",
            "",
            "dlq_reason",
            ""),
        true);
  }

  public void snapshot(
      String type, String key, String payload, String version, String metadata, Instant now) {
    insert(
        "router_state_snapshots",
        Json.object(
            "snapshot_type",
            type,
            "snapshot_key",
            key,
            "payload_json",
            payload,
            "version",
            version,
            "checksum",
            "",
            "is_active",
            true,
            "metadata_json",
            metadata == null ? "" : metadata,
            "created_at",
            ts(now)),
        false);
  }

  public Optional<String> latestState() {
    return db
        .queryForList(
            "SELECT payload_json FROM router_state_snapshots WHERE snapshot_type='router_state' AND"
                + " snapshot_key='default' ORDER BY is_active DESC,created_at DESC,id DESC LIMIT 1")
        .stream()
        .map(m -> str(m, "payload_json"))
        .findFirst();
  }

  public long pendingCount() {
    Long n =
        db.queryForObject(
            "SELECT COUNT(*) FROM cloud_outbox WHERE status IN ('PENDING','RETRY','SENDING')",
            Long.class);
    return n == null ? 0 : n;
  }

  public List<Map<String, Object>> due(int limit, Instant now) {
    return db.queryForList(
        "SELECT * FROM cloud_outbox WHERE status IN ('PENDING','RETRY') AND (next_retry_at IS NULL"
            + " OR next_retry_at<=?) AND (locked_until IS NULL OR locked_until<=?) ORDER BY"
            + " priority_score DESC,deadline_at IS NULL,deadline_at ASC,created_at ASC LIMIT ?",
        ts(now),
        ts(now),
        limit);
  }

  public Optional<Map<String, Object>> lock(long id, String worker, Instant now, Instant until) {
    int n =
        db.update(
            "UPDATE cloud_outbox SET"
                + " status='SENDING',attempt=COALESCE(attempt,0)+1,locked_by=?,locked_until=?,updated_at=?"
                + " WHERE id=? AND status IN ('PENDING','RETRY') AND (next_retry_at IS NULL OR"
                + " next_retry_at<=?) AND (locked_until IS NULL OR locked_until<=?)",
            worker,
            ts(until),
            ts(now),
            id,
            ts(now),
            ts(now));
    if (n == 0) return Optional.empty();
    return db.queryForList("SELECT * FROM cloud_outbox WHERE id=?", id).stream().findFirst();
  }

  public void requeueExpired(Instant now) {
    db.update(
        "UPDATE cloud_outbox SET"
            + " status='RETRY',next_retry_at=?,locked_by='',locked_until=NULL,last_error='outbox"
            + " sending lock expired',last_error_at=?,updated_at=? WHERE status='SENDING' AND"
            + " locked_until IS NOT NULL AND locked_until<=?",
        ts(now),
        ts(now),
        ts(now),
        ts(now));
  }

  public void sent(long id, Instant now) {
    db.update(
        "UPDATE cloud_outbox SET"
            + " status='SENT',sent_at=?,done_at=?,locked_by='',locked_until=NULL,updated_at=? WHERE"
            + " id=? AND status='SENDING'",
        ts(now),
        ts(now),
        ts(now),
        id);
  }

  public void retry(long id, Instant next, String error, Instant now) {
    db.update(
        "UPDATE cloud_outbox SET"
            + " status='RETRY',next_retry_at=?,locked_by='',locked_until=NULL,last_error=?,last_error_at=?,updated_at=?"
            + " WHERE id=? AND status='SENDING'",
        ts(next),
        error,
        ts(now),
        ts(now),
        id);
  }

  public void dlq(long id, String error, Instant now) {
    db.update(
        "UPDATE cloud_outbox SET"
            + " status='DLQ',done_at=?,locked_by='',locked_until=NULL,last_error=?,last_error_at=?,dlq_reason=?,updated_at=?"
            + " WHERE id=? AND status='SENDING'",
        ts(now),
        error,
        ts(now),
        error,
        ts(now),
        id);
  }

  public List<Map<String, Object>> feedback(int limit) {
    return db.queryForList(
        "SELECT * FROM cloud_feedback WHERE processed_by_router=false OR processed_by_router IS"
            + " NULL ORDER BY created_at ASC,id ASC LIMIT ?",
        limit);
  }

  public boolean claimFeedback(long id, Instant now) {
    return db.update(
            "UPDATE cloud_feedback SET processed_by_router=true,processed_at=? WHERE id=? AND"
                + " (processed_by_router=false OR processed_by_router IS NULL)",
            ts(now),
            id)
        > 0;
  }

  public void reward(long id, double reward, String reason, Instant now) {
    db.update(
        "UPDATE cloud_feedback SET reward=?,reward_reason=?,processed_by_router=true,processed_at=?"
            + " WHERE id=?",
        reward,
        reason,
        ts(now),
        id);
  }
}
