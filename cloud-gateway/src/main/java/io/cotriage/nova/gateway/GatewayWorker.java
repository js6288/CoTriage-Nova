package io.cotriage.nova.gateway;

import io.cotriage.nova.common.Json;
import io.lettuce.core.*;
import io.lettuce.core.api.sync.RedisCommands;
import java.sql.Timestamp;
import java.time.*;
import java.util.*;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicBoolean;

public class GatewayWorker {
  final GatewayConfig cfg;
  final RedisCommands<String, String> redis;
  final GatewayStore store;
  final Provider.Manager providers;
  final Provider.Client client;
  final ContextAggregator aggregator;
  RedisCommands<String, String> reader;

  public GatewayWorker(
      GatewayConfig cfg,
      RedisCommands<String, String> redis,
      GatewayStore store,
      Provider.Manager providers,
      Provider.Client client,
      ContextAggregator aggregator) {
    this.cfg = cfg;
    this.redis = redis;
    this.store = store;
    this.providers = providers;
    this.client = client;
    this.aggregator = aggregator;
  }

  public void run(AtomicBoolean running) {
    Semaphore sem = new Semaphore(Math.max(cfg.concurrency, 1));
    ExecutorService workers = Executors.newCachedThreadPool();
    boolean groupReady = false;
    try {
      while (running.get() && !Thread.currentThread().isInterrupted()) {
        if (!groupReady)
          try {
            ensureGroup();
            groupReady = true;
          } catch (Exception e) {
            if (!running.get()) return;
            log("ensure consumer group failed", e);
            if (!waitRetry()) return;
            continue;
          }
        List<StreamMessage<String, String>> messages;
        try {
          messages =
              (reader == null ? redis : reader)
                  .xreadgroup(
                      Consumer.from(cfg.group, cfg.consumer),
                      XReadArgs.Builder.count(cfg.batch).block(cfg.block),
                      XReadArgs.StreamOffset.lastConsumed(cfg.request));
        } catch (Exception e) {
          if (!running.get()) return;
          log("read request stream failed", e);
          groupReady = false;
          if (!waitRetry()) return;
          continue;
        }
        if (messages == null) continue;
        for (var message : messages) {
          try {
            sem.acquire();
          } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            return;
          }
          workers.submit(
              () -> {
                try {
                  handle(message);
                } catch (Exception e) {
                  log("message=" + message.getId() + " failed", e);
                } finally {
                  sem.release();
                }
              });
        }
      }
    } finally {
      workers.shutdownNow();
    }
  }

  public boolean waitRetry() {
    Duration delay =
        cfg.retry.isNegative() || cfg.retry.isZero() ? Duration.ofSeconds(3) : cfg.retry;
    try {
      TimeUnit.NANOSECONDS.sleep(delay.toNanos());
      return true;
    } catch (InterruptedException e) {
      Thread.currentThread().interrupt();
      return false;
    }
  }

  public void ensureGroup() {
    try {
      redis.xgroupCreate(
          XReadArgs.StreamOffset.from(cfg.request, "0"),
          cfg.group,
          XGroupCreateArgs.Builder.mkstream());
    } catch (RedisCommandExecutionException e) {
      if (!e.getMessage().toUpperCase(Locale.ROOT).contains("BUSYGROUP")) throw e;
    }
  }

  public static Events.Request decode(Map<String, String> values) {
    String eventID = get(values, "event_id"),
        requestID = get(values, "request_id"),
        key = get(values, "idempotency_key");
    if (eventID.isEmpty() || requestID.isEmpty() || key.isEmpty())
      throw new IllegalArgumentException("required message field missing");
    Events.Routed routed = GoWire.read(get(values, "payload_json"), Events.Routed.class);
    if (routed == null) routed = new Events.Routed();
    if (!routed.input.event_id.equals(eventID))
      throw new IllegalArgumentException("event_id mismatch");
    return new Events.Request(eventID, requestID, key, routed);
  }

  // fmt.Sprint(nil) in the Go decoder returns <nil>, rather than an empty value.
  static String get(Map<String, String> values, String key) {
    return values.containsKey(key) ? values.get(key) : "<nil>";
  }

  public void handle(StreamMessage<String, String> message) {
    Events.Request req;
    try {
      req = decode(message.getBody());
    } catch (Exception e) {
      dlq(message, "BAD_PAYLOAD", e);
      return;
    }
    if (store.exists(req.eventID())) {
      publish(req, Events.value(store.get(req.eventID()), "feedback_json"));
      ack(message);
      return;
    }
    Provider.Config provider;
    try {
      provider = providers.get();
    } catch (Exception e) {
      dlq(message, "PROVIDER_CONFIG_MISSING", e);
      return;
    }
    ContextAggregator.Context context = aggregator.build(req.routed().input);
    DiagnosisService.Prompt prompt =
        DiagnosisService.prompt(req.routed(), context.related_events, cfg.rawMaxChars);
    Provider.Result result;
    try {
      result = client.diagnose(provider, prompt.system(), prompt.user());
    } catch (Provider.Failure e) {
      if (e.retryable()) throw e;
      dlq(message, errorCode(e), e);
      return;
    }
    Events.Inferred in = req.routed().input;
    Events.Diagnosis diagnosis = result.diagnosis();
    Events.Usage usage = result.usage();
    Events.Feedback feedback =
        DiagnosisService.feedback(in, diagnosis, usage, cfg.latencySLA, cfg.tokenBudget);
    Map<String, Object> response =
        Json.object(
            "schema_version",
            "cloud.response.v1",
            "event_id",
            req.eventID(),
            "request_id",
            req.requestID(),
            "idempotency_key",
            req.idempotencyKey(),
            "status",
            "OK",
            "edge",
            new Events.EdgeSummary(in.edge_pred, in.confidence, in.severity, in.advice_code),
            "cloud",
            diagnosis,
            "feedback",
            feedback,
            "usage",
            usage,
            "context",
            context,
            "processed_at",
            Instant.now().toString());
    String body = Json.write(response);
    store.insert(
        Json.object(
            "event_id",
            req.eventID(),
            "request_id",
            req.requestID(),
            "idempotency_key",
            req.idempotencyKey(),
            "source",
            in.source,
            "equipment",
            in.equipment,
            "template_id",
            in.template_id,
            "edge_pred",
            in.edge_pred,
            "edge_confidence",
            in.confidence,
            "route_action",
            req.routed().decision.action,
            "route_score",
            req.routed().decision.score,
            "route_threshold",
            req.routed().decision.threshold,
            "cloud_model",
            provider.model,
            "cloud_pred",
            diagnosis.pred,
            "root_cause",
            diagnosis.root_cause,
            "recommendation",
            diagnosis.recommendation,
            "explanation",
            diagnosis.explanation,
            "cloud_confidence",
            diagnosis.confidence,
            "cloud_latency_ms",
            usage.latency_ms,
            "token_usage",
            usage.total_tokens,
            "cost",
            usage.cost,
            "reward",
            feedback.reward(),
            "reward_reason",
            feedback.reward_reason(),
            "corrected_edge",
            feedback.corrected_edge(),
            "added_actionable_info",
            feedback.added_actionable_info(),
            "confirmed_high_risk",
            feedback.confirmed_high_risk(),
            "feedback_json",
            body,
            "processed_by_router",
            false,
            "created_at",
            Timestamp.from(Instant.now())));
    publish(req, body);
    ack(message);
  }

  void publish(Events.Request req, String body) {
    add(
        cfg.response,
        Map.of(
            "event_id",
            req.eventID(),
            "request_id",
            req.requestID(),
            "status",
            "OK",
            "payload_json",
            body));
  }

  void dlq(StreamMessage<String, String> message, String code, Exception error) {
    Map<String, String> m = message.getBody();
    String event = get(m, "event_id"), request = get(m, "request_id");
    Map<String, Object> failure =
        Json.object(
            "schema_version",
            "cloud.response.v1",
            "event_id",
            event,
            "request_id",
            request,
            "idempotency_key",
            get(m, "idempotency_key"),
            "status",
            "FAILED",
            "error_code",
            code,
            "error_message",
            error.getMessage(),
            "processed_at",
            Instant.now().toString());
    add(
        cfg.response,
        Map.of(
            "event_id",
            event,
            "request_id",
            request,
            "status",
            "FAILED",
            "payload_json",
            Json.write(failure)));
    add(
        cfg.dlq,
        Map.of(
            "source_stream",
            cfg.request,
            "message_id",
            message.getId(),
            "event_id",
            event,
            "request_id",
            request,
            "error_code",
            code,
            "error",
            error.getMessage()));
    ack(message);
  }

  void ack(StreamMessage<String, String> m) {
    redis.xack(cfg.request, cfg.group, m.getId());
  }

  void add(String stream, Map<String, String> values) {
    if (cfg.maxlen > 0)
      redis.xadd(stream, XAddArgs.Builder.maxlen(cfg.maxlen).approximateTrimming(), values);
    else redis.xadd(stream, values);
  }

  static String errorCode(Exception e) {
    int i = e.getMessage().indexOf(':');
    return i > 0 ? e.getMessage().substring(0, i) : e.getMessage();
  }

  static void log(String message, Exception e) {
    System.err.println("[cloud-gateway] " + message + ": " + e.getMessage());
  }
}
