package io.cotriage.nova.router;

import io.cotriage.nova.common.Env;
import java.time.Duration;

public class RouterConfig {
  public String redis = Env.get("REDIS_ADDR", "redis:6379"),
      inferred = Env.get("INFERRED_STREAM", "cotriage.events.inferred"),
      routed = Env.get("ROUTED_STREAM", "cotriage.events.routed"),
      cloud = Env.get("CLOUD_REQ_STREAM", "cotriage.events.cloud.req"),
      dlq = Env.get("DLQ_STREAM", "cotriage.events.dlq"),
      group = Env.get("CONSUMER_GROUP", "router-g"),
      consumer = Env.get("CONSUMER_NAME", "router-controller-1"),
      dsn =
          Env.get(
              "MYSQL_DSN",
              "cotriage:cotriage-pass@tcp(mysql.cotriage.svc.cluster.local:3306)/cotriage?charset=utf8mb4&parseTime=true&loc=UTC"),
      healthURL = Env.get("CLOUD_HEALTH_URL", "");
  public Duration snapshotInterval = duration("STATE_SNAPSHOT_INTERVAL", 60),
      dbPingTimeout = duration("DB_PING_TIMEOUT", 4),
      redisPingTimeout = duration("REDIS_PING_TIMEOUT", 2),
      healthTimeout = duration("CLOUD_HEALTH_TIMEOUT", 2),
      budgetRefillInterval = duration("BUDGET_REFILL_INTERVAL", 60),
      estimatedLatency = duration("ESTIMATED_CLOUD_LATENCY", 1),
      replayerInterval = positive(duration("OUTBOX_REPLAYER_INTERVAL", 2), 2),
      lockTTL = positive(duration("OUTBOX_LOCK_TTL", 30), 30),
      retryBase = positive(duration("OUTBOX_RETRY_BASE_DELAY", 5), 5),
      feedbackInterval = positive(duration("FEEDBACK_PROCESSOR_INTERVAL", 5), 5),
      stateWindow = duration("STATE_WINDOW", 600),
      block = duration("WORKER_BLOCK_TIMEOUT", 2),
      migrationTimeout = duration("ROUTER_MIGRATION_TIMEOUT", 300);
  public int healthWindow = Env.integer("CLOUD_HEALTH_WINDOW", 20),
      queueMax = Env.integer("QUEUE_PRESSURE_MAX", 1000),
      rateLimit = Env.integer("CLOUD_RATE_LIMIT", 60),
      replayerBatch = positive(Env.integer("OUTBOX_REPLAYER_BATCH", 20), 20),
      maxAttempts = positive(Env.integer("OUTBOX_MAX_ATTEMPTS", 5), 5),
      feedbackBatch = positive(Env.integer("FEEDBACK_PROCESSOR_BATCH", 20), 20),
      batch = positive(Env.integer("WORKER_BATCH", 20), 20);
  public long maxlen = Env.longValue("ROUTER_STREAM_MAXLEN", 100000);
  public double minuteBudget = Env.decimal("CLOUD_BUDGET_MINUTE", 60),
      hourBudget = Env.decimal("CLOUD_BUDGET_HOUR", 1000),
      dayBudget = Env.decimal("CLOUD_BUDGET_DAY", 10000),
      auditRatio = Env.decimal("MIN_CLOUD_RATIO_FOR_AUDIT", .05),
      gainAlpha = Env.decimal("GAIN_UCB_ALPHA", .1);
  public boolean autoMigrate = Env.integer("AUTO_MIGRATE", 1) != 0,
      replayerEnabled = Env.integer("OUTBOX_REPLAYER_ENABLED", 1) != 0,
      feedbackEnabled = Env.integer("FEEDBACK_PROCESSOR_ENABLED", 1) != 0;

  static Duration duration(String name, long seconds) {
    return Env.duration(name, Duration.ofSeconds(seconds));
  }

  static Duration positive(Duration d, long seconds) {
    return d.isZero() || d.isNegative() ? Duration.ofSeconds(seconds) : d;
  }

  static int positive(int n, int fallback) {
    return n <= 0 ? fallback : n;
  }
}
