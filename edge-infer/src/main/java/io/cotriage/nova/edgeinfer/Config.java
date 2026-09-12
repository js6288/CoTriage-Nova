package io.cotriage.nova.edgeinfer;

import io.cotriage.nova.common.Env;
import java.net.InetAddress;
import java.time.Duration;
import java.util.Map;

public record Config(
    String redisAddr,
    String rawStream,
    String inferredStream,
    String dlqStream,
    String consumerGroup,
    String consumerName,
    String rulesFile,
    String templatesFile,
    String edgeConfigUrl,
    String classifierUrl,
    String llmUrl,
    Duration templateCacheTtl,
    Duration configRefresh,
    Duration configTimeout,
    Duration classifierTimeout,
    Duration llmTimeout,
    Duration blockTimeout,
    Duration claimIdleTime,
    long maxAttempts,
    int logMaxBytes,
    long rawMaxLen,
    long inferredMaxLen,
    long dlqMaxLen,
    Duration redisRetryDelay,
    Duration redisCleanupEvery) {
  public static Config load() {
    return load(System.getenv(), hostname());
  }

  static Config load(Map<String, String> env, String hostname) {
    Config result =
        new Config(
            get(env, "REDIS_ADDR", "redis:6379"),
            get(env, "RAW_STREAM", "cotriage.events.raw"),
            get(env, "INFERRED_STREAM", "cotriage.events.inferred"),
            get(env, "DLQ_STREAM", "cotriage.events.dlq"),
            get(env, "CONSUMER_GROUP", "edge-infer-g"),
            get(env, "CONSUMER_NAME", hostname),
            get(env, "RULES_FILE", ""),
            get(env, "TEMPLATES_FILE", ""),
            get(env, "EDGE_CONFIG_URL", ""),
            get(env, "CLASSIFIER_URL", ""),
            get(env, "LLM_URL", ""),
            duration(env, "TEMPLATE_CACHE_TTL", "5m"),
            duration(env, "EDGE_CONFIG_REFRESH_INTERVAL", "30s"),
            duration(env, "EDGE_CONFIG_TIMEOUT", "3s"),
            duration(env, "CLASSIFIER_TIMEOUT", "1500ms"),
            duration(env, "LLM_TIMEOUT", "3s"),
            duration(env, "BLOCK_TIMEOUT", "2s"),
            duration(env, "CLAIM_IDLE_TIME", "30s"),
            number(env, "MAX_ATTEMPTS", 3),
            Math.toIntExact(number(env, "LOG_MAX_BYTES", 8192)),
            number(env, "RAW_STREAM_MAXLEN", 100000),
            number(env, "INFERRED_STREAM_MAXLEN", 100000),
            number(env, "DLQ_STREAM_MAXLEN", 10000),
            duration(env, "REDIS_RETRY_DELAY", "3s"),
            duration(env, "REDIS_CLEANUP_EVERY", "1m"));
    result.validate();
    return result;
  }

  private void validate() {
    if (consumerName.isEmpty())
      throw new IllegalArgumentException("CONSUMER_NAME must not be empty");
    if (maxAttempts < 1) throw new IllegalArgumentException("MAX_ATTEMPTS must be at least 1");
    if (logMaxBytes < 128) throw new IllegalArgumentException("LOG_MAX_BYTES must be at least 128");
    if (rawMaxLen < 1 || inferredMaxLen < 1 || dlqMaxLen < 1)
      throw new IllegalArgumentException("stream MAXLEN values must be at least 1");
    positive(redisRetryDelay, "REDIS_RETRY_DELAY");
    positive(redisCleanupEvery, "REDIS_CLEANUP_EVERY");
    if (!edgeConfigUrl.isEmpty()) {
      positive(configRefresh, "EDGE_CONFIG_REFRESH_INTERVAL");
      positive(configTimeout, "EDGE_CONFIG_TIMEOUT");
    }
    if (!classifierUrl.isEmpty()) positive(classifierTimeout, "CLASSIFIER_TIMEOUT");
    if (!llmUrl.isEmpty()) positive(llmTimeout, "LLM_TIMEOUT");
  }

  static void positive(Duration value, String name) {
    if (value.isZero() || value.isNegative())
      throw new IllegalArgumentException(name + " must be positive");
  }

  private static String get(Map<String, String> env, String key, String fallback) {
    String value = Normalizer.trim(env.getOrDefault(key, ""));
    return value.isEmpty() ? fallback : value;
  }

  private static Duration duration(Map<String, String> env, String key, String fallback) {
    try {
      return Env.parseDuration(get(env, key, fallback));
    } catch (RuntimeException ex) {
      throw new IllegalArgumentException("parse " + key + ": " + ex.getMessage(), ex);
    }
  }

  private static long number(Map<String, String> env, String key, long fallback) {
    try {
      return Long.parseLong(get(env, key, Long.toString(fallback)));
    } catch (RuntimeException ex) {
      throw new IllegalArgumentException("parse " + key + ": " + ex.getMessage(), ex);
    }
  }

  private static String hostname() {
    try {
      return InetAddress.getLocalHost().getHostName();
    } catch (Exception ex) {
      return System.getenv().getOrDefault("HOSTNAME", "");
    }
  }
}
