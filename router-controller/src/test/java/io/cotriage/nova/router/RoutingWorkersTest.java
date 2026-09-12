package io.cotriage.nova.router;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;

import io.cotriage.nova.common.Json;
import io.lettuce.core.*;
import io.lettuce.core.api.sync.RedisCommands;
import java.time.*;
import java.util.*;
import org.junit.jupiter.api.Test;
import org.mockito.InOrder;

@SuppressWarnings("unchecked")
class RoutingWorkersTest {
  final Instant now = Instant.parse("2026-06-30T12:00:00Z");

  @Test
  void outboxClaimsPublishesAndMarksSentInOriginalOrder() {
    var cfg = new RouterConfig();
    var store = mock(RouterStore.class);
    var publisher = mock(OutboxReplayer.Publisher.class);
    var row = Json.object("id", 1L, "attempt", 1, "max_attempts", 5);
    when(store.due(cfg.replayerBatch, now)).thenReturn(List.of(row));
    when(store.lock(1, cfg.consumer, now, now.plus(cfg.lockTTL))).thenReturn(Optional.of(row));
    var rep = new OutboxReplayer(cfg, store, publisher);
    rep.clock = () -> now;
    rep.replayOnce();
    InOrder order = inOrder(store, publisher);
    order.verify(store).requeueExpired(now);
    order.verify(store).due(cfg.replayerBatch, now);
    order.verify(store).lock(1, cfg.consumer, now, now.plus(cfg.lockTTL));
    order.verify(publisher).publish(row);
    order.verify(store).sent(1, now);
  }

  @Test
  void outboxRetriesAndMaxAttemptsDlqWithoutThrowingPublicationError() {
    var cfg = new RouterConfig();
    var store = mock(RouterStore.class);
    var publisher = mock(OutboxReplayer.Publisher.class);
    var row = Json.object("id", 1L, "attempt", 2, "max_attempts", 3);
    when(store.due(cfg.replayerBatch, now)).thenReturn(List.of(row));
    when(store.lock(1, cfg.consumer, now, now.plus(cfg.lockTTL))).thenReturn(Optional.of(row));
    doThrow(new IllegalStateException("offline")).when(publisher).publish(row);
    var rep = new OutboxReplayer(cfg, store, publisher);
    rep.clock = () -> now;
    assertDoesNotThrow(rep::replayOnce);
    verify(store).retry(1, now.plusSeconds(10), "offline", now);
    row.put("attempt", 3);
    rep.replayOnce();
    verify(store).dlq(1, "offline", now);
    assertEquals(Duration.ofSeconds(5), rep.backoff(1));
    assertEquals(Duration.ofMinutes(1), rep.backoff(8));
    verify(store, never()).sent(anyLong(), any());
  }

  @Test
  void unclaimedOutboxDoesNotPublish() {
    var cfg = new RouterConfig();
    var store = mock(RouterStore.class);
    var publisher = mock(OutboxReplayer.Publisher.class);
    var row = Json.object("id", 1L);
    when(store.due(cfg.replayerBatch, now)).thenReturn(List.of(row));
    when(store.lock(1, cfg.consumer, now, now.plus(cfg.lockTTL))).thenReturn(Optional.empty());
    var rep = new OutboxReplayer(cfg, store, publisher);
    rep.clock = () -> now;
    rep.replayOnce();
    verifyNoInteractions(publisher);
  }

  @Test
  void feedbackClaimsBeforeLearningAndKeepsTwoPersistenceSteps() {
    var store = mock(RouterStore.class);
    var state = new RouterState(Duration.ofMinutes(10), .1);
    var row =
        Json.object(
            "id",
            1,
            "event_id",
            "e",
            "template_id",
            "tpl",
            "edge_pred",
            "NORMAL",
            "cloud_pred",
            "UNKNOWN",
            "reward",
            -.2,
            "route_score",
            .7);
    when(store.feedback(20)).thenReturn(List.of(row));
    when(store.claimFeedback(1, now)).thenReturn(true);
    var p = new FeedbackProcessor(store, state, 20);
    p.clock = () -> now;
    p.processOnce();
    InOrder order = inOrder(store);
    order.verify(store).feedback(20);
    order.verify(store).claimFeedback(1, now);
    order.verify(store).reward(1, 1, "corrected_edge", now);
    order
        .verify(store)
        .snapshot(
            eq("router_state"),
            eq("default"),
            anyString(),
            eq("raise-feedback-v1"),
            anyString(),
            eq(now));
    assertEquals(1, state.template_stats.get("tpl").correction_count);
  }

  @Test
  void cloudWorkerPreservesDbThenCloudThenRoutedThenAck() {
    var cfg = new RouterConfig();
    cfg.healthURL = "";
    var redis = mock(RedisCommands.class);
    when(redis.xinfoGroups(cfg.inferred)).thenReturn(List.of());
    var store = mock(RouterStore.class);
    var state = new RouterState(Duration.ofMinutes(10), .1);
    var runtime =
        new RouterRuntime(
            cfg,
            store,
            redis,
            new RouterRuntime.HealthProbe("", Duration.ofSeconds(1), 4),
            new RouterRuntime.Buckets(60, 1000, 10000, 60, now));
    var w =
        new RouterWorker(
            cfg, redis, store, new RaiseEngine(new RaiseEngine.Config()), state, runtime);
    w.clock = () -> now;
    w.policy.forceCloudTemplates = Set.of("TPL-1");
    w.handle(
        new StreamMessage<>(
            cfg.inferred, "1-0", Map.of("payload", Json.write(RaiseEngineTest.event()))));
    InOrder order = inOrder(store, redis);
    order.verify(store).pendingCount();
    order.verify(store).decision(any(), argThat(d -> d.Action().equals("CLOUD")), eq(now), eq(""));
    order
        .verify(redis)
        .xadd(
            eq(cfg.cloud),
            any(XAddArgs.class),
            argThat(
                (Map<String, String> m) -> "router-cloud-evt-1".equals(m.get("idempotency_key"))));
    order.verify(redis).xadd(eq(cfg.routed), any(XAddArgs.class), anyMap());
    order
        .verify(store)
        .snapshot(
            eq("system_state"),
            eq("default"),
            anyString(),
            eq("raise-runtime-v1"),
            isNull(),
            eq(now));
    order.verify(redis).xack(cfg.inferred, cfg.group, "1-0");
    assertEquals(1, state.recent_total_actions);
  }

  @Test
  void malformedRouterMessageStillAcksEvenWhenDlqPublicationFails() {
    var cfg = new RouterConfig();
    var redis = mock(RedisCommands.class);
    when(redis.xadd(eq(cfg.dlq), any(XAddArgs.class), anyMap()))
        .thenThrow(new IllegalStateException("offline"));
    var worker = new RouterWorker(cfg, redis, null, null, null, null);
    worker.handle(new StreamMessage<>(cfg.inferred, "1-0", Map.of("payload", "broken")));
    verify(redis).xack(cfg.inferred, cfg.group, "1-0");
  }
}
