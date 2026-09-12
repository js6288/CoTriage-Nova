package io.cotriage.nova.router;

import io.cotriage.nova.common.Json;
import io.lettuce.core.*;
import io.lettuce.core.api.sync.RedisCommands;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.Instant;
import java.util.*;
import java.util.function.Supplier;

public class RouterWorker {
  final RouterConfig cfg;
  final RedisCommands<String, String> redis;
  final RouterStore store;
  final RaiseEngine engine;
  final RouterState state;
  final RouterRuntime runtime;
  public RaiseEngine.Policy policy = new RaiseEngine.Policy();
  Supplier<Instant> clock = Instant::now;
  RedisCommands<String, String> reader;

  public RouterWorker(
      RouterConfig cfg,
      RedisCommands<String, String> redis,
      RouterStore store,
      RaiseEngine engine,
      RouterState state,
      RouterRuntime runtime) {
    this.cfg = cfg;
    this.redis = redis;
    this.store = store;
    this.engine = engine;
    this.state = state;
    this.runtime = runtime;
  }

  public void ensureGroup() {
    try {
      redis.xgroupCreate(
          XReadArgs.StreamOffset.from(cfg.inferred, "0"),
          cfg.group,
          XGroupCreateArgs.Builder.mkstream());
    } catch (RedisCommandExecutionException e) {
      if (!e.getMessage().toUpperCase(Locale.ROOT).contains("BUSYGROUP")
          && !e.getMessage().toUpperCase(Locale.ROOT).contains("ALREADY EXISTS")) throw e;
    }
  }

  public void readOnce() {
    List<StreamMessage<String, String>> msgs =
        (reader == null ? redis : reader)
            .xreadgroup(
                Consumer.from(cfg.group, cfg.consumer),
                XReadArgs.Builder.count(cfg.batch).block(cfg.block),
                XReadArgs.StreamOffset.lastConsumed(cfg.inferred));
    if (msgs == null) return;
    for (var m : msgs)
      try {
        handle(m);
      } catch (Exception e) {
        RouterRuntime.log("handle inferred message id=" + m.getId(), e);
      }
  }

  public void handle(StreamMessage<String, String> message) {
    Instant now = clock.get();
    String raw = message.getBody().getOrDefault("payload", "");
    if (raw.isEmpty()) raw = Json.write(message.getBody());
    Events.Inferred in;
    try {
      in = Events.decode(raw);
      if (in == null) in = new Events.Inferred();
    } catch (Exception e) {
      try {
        add(
            cfg.dlq,
            Map.of(
                "source_stream",
                cfg.inferred,
                "message_id",
                message.getId(),
                "payload",
                raw,
                "error",
                e.getMessage()));
      } catch (Exception ignored) {
      }
      redis.xack(cfg.inferred, cfg.group, message.getId());
      return;
    }
    if (in.event_id.isEmpty()) in.event_id = "redis-" + message.getId();
    RaiseEngine.Stats stats = state.stats(in, now);
    RaiseEngine.SystemState system = runtime.build(state.ratio(), 0, now);
    RaiseEngine.Decision d = engine.decide(in, system, stats, policy);
    if (d.Action().equals("CLOUD")) runtime.buckets.allow(1, now);
    Events.Routed routed =
        new Events.Routed(in, d, system, now.toString(), cfg.consumer, "router.routed.v1");
    store.decision(in, d, now, hash(in.raw_log));
    if (d.Action().equals("DEFER")) store.outbox(routed, cfg.maxAttempts, now);
    if (d.Action().equals("CLOUD"))
      add(
          cfg.cloud,
          Map.of(
              "event_id",
              in.event_id,
              "request_id",
              "cloud-" + in.event_id,
              "idempotency_key",
              "router-cloud-" + in.event_id,
              "payload_json",
              Json.write(routed),
              "source",
              "router_controller"));
    add(
        cfg.routed,
        Map.of("event_id", in.event_id, "action", d.Action(), "payload", Json.write(routed)));
    state.observe(in, d.Action(), now);
    try {
      runtime.snapshot(system, now);
    } catch (Exception ignored) {
    }
    redis.xack(cfg.inferred, cfg.group, message.getId());
  }

  void add(String stream, Map<String, String> values) {
    if (cfg.maxlen > 0)
      redis.xadd(stream, XAddArgs.Builder.maxlen(cfg.maxlen).approximateTrimming(), values);
    else redis.xadd(stream, values);
  }

  static String hash(String raw) {
    if (raw == null || raw.isEmpty()) return "";
    try {
      return HexFormat.of()
          .formatHex(
              MessageDigest.getInstance("SHA-256").digest(raw.getBytes(StandardCharsets.UTF_8)));
    } catch (Exception e) {
      throw new IllegalStateException(e);
    }
  }
}
