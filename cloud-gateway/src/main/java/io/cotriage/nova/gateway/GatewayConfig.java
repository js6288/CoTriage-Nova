package io.cotriage.nova.gateway;

import io.cotriage.nova.common.Env;
import java.time.Duration;

public class GatewayConfig {
  public boolean enabled = integer("CLOUD_GATEWAY_ENABLED", 1) != 0;
  public String server = env("SERVER_BASE_URL", "http://server:8080"),
      endpoint = env("CLOUD_CONFIG_ENDPOINT", "/api/v1/cloud-config"),
      configToken = env("CLOUD_GATEWAY_CONFIG_TOKEN", "").strip(),
      redis = env("REDIS_ADDR", "redis:6379"),
      request = env("CLOUD_REQ_STREAM", "cotriage.events.cloud.req"),
      response = env("CLOUD_RESP_STREAM", "cotriage.events.cloud.resp"),
      dlq = env("DLQ_STREAM", "cotriage.events.dlq"),
      context = env("CLOUD_CONTEXT_SOURCE_STREAM", "cotriage.events.inferred"),
      group = env("CLOUD_GATEWAY_GROUP", "cloud-gateway-g"),
      consumer = env("CLOUD_GATEWAY_CONSUMER", "cloud-gateway-1"),
      dsn =
          env(
              "MYSQL_DSN",
              "cotriage:cotriage-pass@tcp(mysql.cotriage.svc.cluster.local:3306)/cotriage?charset=utf8mb4&parseTime=true&loc=UTC");
  public Duration poll = duration("CLOUD_CONFIG_POLL_INTERVAL", 10),
      configTimeout = duration("CLOUD_CONFIG_TIMEOUT", 5),
      block = duration("CLOUD_GATEWAY_BLOCK_TIMEOUT", 2),
      retry = duration("CLOUD_GATEWAY_REDIS_RETRY_DELAY", 3),
      readTimeout = duration("CLOUD_GATEWAY_REDIS_READ_TIMEOUT", 10),
      window = duration("CLOUD_CONTEXT_WINDOW", 300);
  public int batch = integer("CLOUD_GATEWAY_BATCH", 10),
      concurrency = integer("CLOUD_GATEWAY_CONCURRENCY", 2),
      maxEvents = integer("CLOUD_CONTEXT_MAX_EVENTS", 20),
      maxTokens = integer("CLOUD_CONTEXT_MAX_TOKENS", 3000),
      rawMaxChars = integer("CLOUD_RAW_LOG_MAX_CHARS", 2000),
      latencySLA = integer("CLOUD_LATENCY_SLA_MS", 5000),
      tokenBudget = integer("CLOUD_TOKEN_BUDGET_PER_REQ", 2000);
  public long maxlen = longValue("CLOUD_STREAM_MAXLEN", 100000);

  public GatewayConfig() {
    if (readTimeout.compareTo(block) <= 0) readTimeout = block.plusSeconds(5);
  }

  // This module's Go env helper preserves every nonempty value, including whitespace.
  static String env(String name, String fallback) {
    String value = System.getenv(name);
    return value == null || value.isEmpty() ? fallback : value;
  }

  static int integer(String name, int fallback) {
    try {
      return Integer.parseInt(env(name, ""));
    } catch (NumberFormatException e) {
      return fallback;
    }
  }

  static long longValue(String name, long fallback) {
    try {
      return Long.parseLong(env(name, ""));
    } catch (NumberFormatException e) {
      return fallback;
    }
  }

  static Duration duration(String name, long seconds) {
    try {
      return Env.parseDuration(env(name, ""));
    } catch (IllegalArgumentException | ArithmeticException e) {
      return Duration.ofSeconds(seconds);
    }
  }
}
