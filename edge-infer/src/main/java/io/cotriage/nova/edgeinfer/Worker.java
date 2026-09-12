package io.cotriage.nova.edgeinfer;

import static io.cotriage.nova.edgeinfer.Models.*;

import java.io.IOException;
import java.time.Duration;
import java.time.Instant;
import java.util.Map;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

final class Worker {
  private static final Logger LOG = LoggerFactory.getLogger(Worker.class);
  private static final Duration DAY = Duration.ofHours(24);
  private final RedisPort redis;
  private final Config cfg;
  private final Engine engine;

  Worker(RedisPort redis, Config cfg, Engine engine) {
    this.redis = redis;
    this.cfg = cfg;
    this.engine = engine;
  }

  void run() throws InterruptedException {
    long claimPeriod = cfg.claimIdleTime().toNanos() / 2,
        cleanupPeriod = cfg.redisCleanupEvery().toNanos();
    // Go time.NewTicker panics for this otherwise-unvalidated configuration.
    if (claimPeriod <= 0 || cleanupPeriod <= 0)
      throw new IllegalArgumentException("non-positive interval for ticker");
    long nextClaim = System.nanoTime() + claimPeriod,
        nextCleanup = System.nanoTime() + cleanupPeriod;
    while (!Thread.currentThread().isInterrupted()) {
      ensureRedisReady();
      long now = System.nanoTime();
      if (now >= nextClaim) {
        try {
          claimPending();
        } catch (Exception ex) {
          LOG.warn("[edge-infer] claim pending: {}", ex.getMessage());
        }
        nextClaim = nextTick(nextClaim, claimPeriod, now);
      } else if (now >= nextCleanup) {
        try {
          cleanup();
        } catch (Exception ex) {
          LOG.warn("[edge-infer] redis cleanup: {}", ex.getMessage());
        }
        nextCleanup = nextTick(nextCleanup, cleanupPeriod, now);
      }
      try {
        for (var message :
            redis.readGroup(
                cfg.rawStream(), cfg.consumerGroup(), cfg.consumerName(), 16, cfg.blockTimeout()))
          handle(message);
      } catch (RuntimeException ex) {
        if (Thread.currentThread().isInterrupted()) return;
        LOG.warn(
            "[edge-infer] read group: {}; retry in {}", ex.getMessage(), cfg.redisRetryDelay());
        sleep(cfg.redisRetryDelay());
      }
    }
  }

  private static long nextTick(long previous, long period, long now) {
    return previous + ((now - previous) / period + 1) * period;
  }

  void ensureRedisReady() throws InterruptedException {
    while (!Thread.currentThread().isInterrupted()) {
      try {
        redis.ping();
      } catch (RuntimeException ex) {
        LOG.warn(
            "[edge-infer] redis unavailable: {}; retry in {}",
            ex.getMessage(),
            cfg.redisRetryDelay());
        sleep(cfg.redisRetryDelay());
        continue;
      }
      try {
        ensureGroup();
        return;
      } catch (RuntimeException ex) {
        LOG.warn(
            "[edge-infer] ensure consumer group: {}; retry in {}",
            ex.getMessage(),
            cfg.redisRetryDelay());
        sleep(cfg.redisRetryDelay());
      }
    }
    throw new InterruptedException("worker stopped");
  }

  void ensureGroup() {
    try {
      redis.createGroup(cfg.rawStream(), cfg.consumerGroup());
    } catch (RuntimeException ex) {
      if (ex.getMessage() == null || !ex.getMessage().startsWith("BUSYGROUP")) throw ex;
    }
  }

  void cleanup() {
    redis.trim(cfg.rawStream(), cfg.rawMaxLen());
    redis.trim(cfg.inferredStream(), cfg.inferredMaxLen());
    redis.trim(cfg.dlqStream(), cfg.dlqMaxLen());
    bestEffortExpire();
  }

  void claimPending() {
    for (var message :
        redis.autoClaim(
            cfg.rawStream(), cfg.consumerGroup(), cfg.consumerName(), cfg.claimIdleTime(), 16))
      handle(message);
  }

  void handle(RedisPort.Message message) {
    try {
      process(message);
    } catch (Exception ex) {
      long attempt;
      try {
        attempt = redis.increment(attemptKey(), message.id());
      } catch (RuntimeException countError) {
        LOG.warn(
            "[edge-infer] message={} error={}; update attempt: {}",
            message.id(),
            ex.getMessage(),
            countError.getMessage());
        return;
      }
      bestEffortExpire();
      if (attempt < cfg.maxAttempts()) {
        LOG.warn(
            "[edge-infer] message={} attempt={}/{} error={}",
            message.id(),
            attempt,
            cfg.maxAttempts(),
            ex.getMessage());
        return;
      }
      try {
        moveToDlq(message, attempt, ex);
      } catch (RuntimeException dlqError) {
        LOG.warn("[edge-infer] message={} move to DLQ: {}", message.id(), dlqError.getMessage());
        return;
      }
      ack(message.id());
      return;
    }
    ack(message.id());
  }

  void process(RedisPort.Message message) throws Exception {
    String eventId = message.values().get("event_id"), payload = message.values().get("payload");
    if (eventId == null || eventId.isEmpty()) throw new IOException("stream event_id is missing");
    if (payload == null || payload.isEmpty()) throw new IOException("payload is missing");
    RawEvent raw;
    try {
      raw = EdgeJson.read(payload, RawEvent.class);
      if (raw == null) raw = new RawEvent();
    } catch (IOException ex) {
      throw new IOException("decode payload: " + ex.getMessage(), ex);
    }
    if (!raw.eventId.equals(eventId))
      throw new IOException(
          "event_id mismatch: stream="
              + EdgeJson.write(eventId)
              + " payload="
              + EdgeJson.write(raw.eventId));
    boolean processed;
    try {
      processed = redis.exists(processedKey(raw.eventId));
    } catch (RuntimeException ex) {
      throw new IOException("check idempotency key: " + ex.getMessage(), ex);
    }
    if (processed) return;
    InferredEvent result;
    try {
      result = engine.infer(raw);
    } catch (Exception ex) {
      throw new IOException("infer event: " + ex.getMessage(), ex);
    }
    String body = EdgeJson.write(result.toMap());
    try {
      redis.add(
          cfg.inferredStream(),
          cfg.inferredMaxLen(),
          Map.of("event_id", raw.eventId, "payload", body));
    } catch (RuntimeException ex) {
      throw new IOException("publish inferred event: " + ex.getMessage(), ex);
    }
    // Intentionally separate from XADD and ACK: preserves the original duplicate window on SET
    // failure.
    try {
      redis.set(processedKey(raw.eventId), "1", DAY);
    } catch (RuntimeException ex) {
      throw new IOException("write idempotency key: " + ex.getMessage(), ex);
    }
  }

  void moveToDlq(RedisPort.Message message, long attempt, Exception cause) {
    redis.add(
        cfg.dlqStream(),
        cfg.dlqMaxLen(),
        Map.of(
            "source_stream",
            cfg.rawStream(),
            "source_message_id",
            message.id(),
            "consumer_group",
            cfg.consumerGroup(),
            "attempt",
            Long.toString(attempt),
            "error",
            cause.getMessage(),
            "original_values",
            EdgeJson.write(message.values()),
            "failed_at",
            Instant.now().toString()));
  }

  private void ack(String id) {
    try {
      redis.ackAndDeleteAttempt(cfg.rawStream(), cfg.consumerGroup(), id, attemptKey());
    } catch (RuntimeException ex) {
      LOG.warn("[edge-infer] message={} ACK: {}", id, ex.getMessage());
    }
  }

  private void bestEffortExpire() {
    try {
      redis.expire(attemptKey(), DAY);
    } catch (RuntimeException ignored) {
    }
  }

  String attemptKey() {
    return "cotriage:edge-infer:attempts:" + cfg.consumerGroup();
  }

  static String processedKey(String eventId) {
    return "cotriage:edge-infer:processed:" + eventId;
  }

  private static void sleep(Duration duration) throws InterruptedException {
    Thread.sleep(duration);
  }
}
