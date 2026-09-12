package io.cotriage.nova.gateway;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;

import io.lettuce.core.*;
import io.lettuce.core.api.sync.RedisCommands;
import java.util.*;
import org.junit.jupiter.api.Test;
import org.mockito.InOrder;

@SuppressWarnings("unchecked")
class GatewayWorkerTest {
  final GatewayConfig cfg = new GatewayConfig();
  final RedisCommands<String, String> redis = mock(RedisCommands.class);
  final GatewayStore store = mock(GatewayStore.class);
  final Provider.Manager manager = mock(Provider.Manager.class);
  final Provider.Client client = mock(Provider.Client.class);
  final ContextAggregator context = mock(ContextAggregator.class);
  final GatewayWorker worker = new GatewayWorker(cfg, redis, store, manager, client, context);

  StreamMessage<String, String> message() {
    return new StreamMessage<>(
        cfg.request,
        "1-0",
        Map.of(
            "event_id",
            "a",
            "request_id",
            "cloud-a",
            "idempotency_key",
            "router-cloud-a",
            "payload_json",
            "{\"input\":{\"event_id\":\"a\"},\"decision\":{\"Action\":\"CLOUD\"}}"));
  }

  void provider() {
    when(manager.get()).thenReturn(new Provider.Config());
    when(context.build(any())).thenReturn(new ContextAggregator.Context(new Events.Inferred()));
  }

  @Test
  void cachedFeedbackSkipsProviderAndPublishesBeforeAck() {
    when(store.exists("a")).thenReturn(true);
    when(store.get("a")).thenReturn(Map.of("feedback_json", "saved"));
    worker.handle(message());
    InOrder order = inOrder(store, redis);
    order.verify(store).exists("a");
    order.verify(store).get("a");
    order
        .verify(redis)
        .xadd(
            eq(cfg.response),
            any(XAddArgs.class),
            argThat((Map<String, String> m) -> "saved".equals(m.get("payload_json"))));
    order.verify(redis).xack(cfg.request, cfg.group, "1-0");
    verifyNoInteractions(manager, client, context);
  }

  @Test
  void successCommitsFeedbackBeforeResponseAndAck() {
    provider();
    var d = new Events.Diagnosis();
    d.pred = "NORMAL";
    when(client.diagnose(any(), anyString(), anyString()))
        .thenReturn(new Provider.Result(d, new Events.Usage()));
    worker.handle(message());
    InOrder order = inOrder(store, redis);
    order.verify(store).exists("a");
    order
        .verify(store)
        .insert(
            argThat(
                m ->
                    "a".equals(m.get("event_id"))
                        && Boolean.FALSE.equals(m.get("processed_by_router"))));
    order.verify(redis).xadd(eq(cfg.response), any(XAddArgs.class), anyMap());
    order.verify(redis).xack(cfg.request, cfg.group, "1-0");
  }

  @Test
  void retryableErrorRemainsPendingWithoutAddedRetryMechanism() {
    provider();
    when(client.diagnose(any(), anyString(), anyString()))
        .thenThrow(new Provider.Failure("RATE_LIMIT"));
    assertThrows(Provider.Failure.class, () -> worker.handle(message()));
    verify(store, never()).insert(anyMap());
    verifyNoInteractions(redis);
  }

  @Test
  void missingProviderPublishesFailureAndDlqBeforeAck() {
    when(manager.get()).thenThrow(new Provider.Failure("PROVIDER_CONFIG_MISSING"));
    worker.handle(message());
    InOrder order = inOrder(redis);
    order
        .verify(redis)
        .xadd(
            eq(cfg.response),
            any(XAddArgs.class),
            argThat((Map<String, String> m) -> "FAILED".equals(m.get("status"))));
    order.verify(redis).xadd(eq(cfg.dlq), any(XAddArgs.class), anyMap());
    order.verify(redis).xack(cfg.request, cfg.group, "1-0");
  }

  @Test
  void failedResponsePublicationNeverAcks() {
    when(store.exists("a")).thenReturn(true);
    when(store.get("a")).thenReturn(Map.of("feedback_json", "saved"));
    when(redis.xadd(eq(cfg.response), any(XAddArgs.class), anyMap()))
        .thenThrow(new IllegalStateException("offline"));
    assertThrows(IllegalStateException.class, () -> worker.handle(message()));
    verify(redis, never()).xack(anyString(), anyString(), anyString());
  }

  @Test
  void retryDelayStopsOnInterruption() {
    Thread.currentThread().interrupt();
    try {
      assertFalse(worker.waitRetry());
      assertTrue(Thread.currentThread().isInterrupted());
    } finally {
      Thread.interrupted();
    }
  }
}
