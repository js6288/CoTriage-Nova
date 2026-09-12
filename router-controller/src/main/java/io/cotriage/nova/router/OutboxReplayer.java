package io.cotriage.nova.router;

import static io.cotriage.nova.router.Events.*;

import io.lettuce.core.api.sync.RedisCommands;
import java.time.*;
import java.util.*;
import java.util.function.Supplier;

public class OutboxReplayer {
  @FunctionalInterface
  public interface Publisher {
    void publish(Map<String, Object> row);
  }

  final RouterConfig cfg;
  final RouterStore store;
  final Publisher publisher;
  Supplier<Instant> clock = Instant::now;

  public OutboxReplayer(RouterConfig cfg, RouterStore store, Publisher publisher) {
    this.cfg = cfg;
    this.store = store;
    this.publisher = publisher;
  }

  public static Publisher redis(RedisCommands<String, String> redis, String stream) {
    return row -> {
      Map<String, String> values = new LinkedHashMap<>();
      for (String k :
          List.of(
              "event_id", "request_id", "idempotency_key", "payload_json", "route_decision_json"))
        values.put(k, str(row, k));
      values.put("priority_score", Double.toString(num(row, "priority_score")));
      values.put("source", "router_outbox");
      redis.xadd(stream.isEmpty() ? "cotriage.events.cloud.req" : stream, values);
    };
  }

  public Duration backoff(long attempt) {
    Duration d = cfg.retryBase;
    for (long i = 1; i < Math.max(attempt, 1); i++) {
      d = d.multipliedBy(2);
      if (d.compareTo(Duration.ofMinutes(1)) > 0) return Duration.ofMinutes(1);
    }
    return d;
  }

  public void replayOnce() {
    Instant now = clock.get();
    store.requeueExpired(now);
    RuntimeException first = null;
    for (Map<String, Object> candidate : store.due(cfg.replayerBatch, now))
      try {
        Optional<Map<String, Object>> locked =
            store.lock(
                (long) num(candidate, "id"),
                cfg.consumer.isEmpty() ? "router-controller" : cfg.consumer,
                now,
                now.plus(cfg.lockTTL));
        if (locked.isEmpty()) continue;
        Map<String, Object> row = locked.get();
        long id = (long) num(row, "id");
        try {
          publisher.publish(row);
        } catch (RuntimeException e) {
          long max = (long) num(row, "max_attempts");
          if (max <= 0) max = cfg.maxAttempts;
          if (num(row, "attempt") >= max) store.dlq(id, e.getMessage(), clock.get());
          else
            store.retry(
                id,
                clock.get().plus(backoff((long) num(row, "attempt"))),
                e.getMessage(),
                clock.get());
          continue;
        }
        store.sent(id, clock.get());
      } catch (RuntimeException e) {
        if (first == null) first = e;
      }
    if (first != null) throw first;
  }
}
