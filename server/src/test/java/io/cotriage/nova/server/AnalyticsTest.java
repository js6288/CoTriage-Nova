package io.cotriage.nova.server;

import static io.cotriage.nova.server.Values.*;
import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;

import java.time.*;
import java.util.*;
import org.junit.jupiter.api.Test;

class AnalyticsTest {
  @Test
  void nearestRankPercentilesAndGrowthKeepOriginalDefinitions() {
    assertEquals(
        19, percentile(java.util.stream.LongStream.rangeClosed(1, 20).boxed().toList(), 95));
    assertEquals(
        10, percentile(java.util.stream.LongStream.rangeClosed(1, 20).boxed().toList(), 50));
    var now = Instant.parse("2026-09-12T00:10:00Z");
    List<Map<String, Object>> events =
        List.of(
            obj(
                "edge_pred",
                "NET_ERROR",
                "template_id",
                "a",
                "equipment",
                "node2",
                "severity",
                80,
                "confidence",
                .9,
                "inference_path",
                "RULE",
                "inferred_at",
                now.minusSeconds(20).toString()),
            obj(
                "edge_pred",
                "NET_ERROR",
                "template_id",
                "a",
                "equipment",
                "node1",
                "severity",
                90,
                "confidence",
                .8,
                "inference_path",
                "ONNX",
                "inferred_at",
                now.minusSeconds(90).toString()),
            obj("edge_pred", "NORMAL", "template_id", "b", "inferred_at", now.toString()));
    var analysis = MetricsService.analyze(events, now);
    assertEquals("HIGH", analysis.get("risk_level"));
    assertEquals(1, num(analysis, "abnormal_1m"));
    assertEquals(2, num(analysis, "abnormal_5m"));
    assertEquals(2, dec(analysis, "growth_ratio"));
    var cluster = (Map<String, Object>) list(analysis.get("top_clusters")).getFirst();
    assertEquals(List.of("node1", "node2"), cluster.get("equipments"));
    assertEquals(.85, dec(cluster, "avg_confidence"));
    assertEquals(82, dec(cluster, "priority_score"));
  }

  @Test
  void competitionDistinguishesProxyMetricsFromGroundTruth() {
    var events = mock(EventStore.class);
    var router = mock(RouterRepository.class);
    when(events.recent(2000)).thenReturn(List.of());
    when(router.competition())
        .thenReturn(
            obj(
                "routed_total",
                4,
                "local_total",
                3,
                "cloud_total",
                1,
                "cloud_samples",
                2,
                "edge_cloud_agree",
                1,
                "edge_cloud_conflict",
                1));
    var metrics =
        new MetricsService(events, router, Clock.fixed(Instant.EPOCH, ZoneOffset.UTC))
            .competition();
    assertEquals(.75, dec((Map<String, Object>) metrics.get("efficiency"), "cloud_offload_rate"));
    var effect = (Map<String, Object>) metrics.get("effect");
    assertEquals(
        "not_available", ((Map<?, ?>) effect.get("ground_truth_evaluation")).get("status"));
    assertEquals("proxy", ((Map<?, ?>) effect.get("cloud_comparison")).get("status"));
  }

  @Test
  void invalidEventsAreSkippedAndMissingFieldsUseGoDefaults() {
    assertNull(EventStore.decode(Map.of("payload", "bad json")));
    assertNull(EventStore.decode(Map.of()));
    var decoded = EventStore.decode(Map.of("payload", "{\"event_id\":\"x\"}"));
    assertEquals("", decoded.get("edge_pred"));
    assertEquals(0, num(decoded, "latency_ms"));
    assertFalse(decoded.containsKey("raw_log"));
  }

  @Test
  void inferredPayloadRejectsGoTypeErrorsAndAcceptsCaseInsensitiveNames() {
    for (var invalid :
        List.of(
            "{\"event_id\":123}",
            "{\"latency_ms\":\"10\"}",
            "{\"latency_ms\":1.5}",
            "{\"confidence\":\"0.5\"}",
            "{\"evidence\":[1]}",
            "{\"stage_latency_ms\":{\"rule\":\"3\"}}",
            "{\"stage_latency_ms\":{\"rule\":2.5}}",
            "{\"latency_ms\":9223372036854775808}",
            "{\"confidence\":1e400}",
            "{} {}")) assertNull(InferredPayload.decode(invalid), invalid);
    var valid =
        InferredPayload.decode(
            "{\"EVENT_ID\":\"case-insensitive\",\"Latency_MS\":3,\"latency_ms\":null,\"equipment\":null,\"evidence\":[null,\"a\"],\"stage_latency_ms\":{\"rule\":null}}");
    assertNotNull(valid);
    assertEquals("case-insensitive", valid.get("event_id"));
    assertEquals(3, num(valid, "latency_ms"));
    assertEquals("", valid.get("equipment"));
    assertEquals(List.of("", "a"), valid.get("evidence"));
    assertEquals(0, num((Map<String, Object>) valid.get("stage_latency_ms"), "rule"));
    assertNotNull(InferredPayload.decode("null"));
  }
}
