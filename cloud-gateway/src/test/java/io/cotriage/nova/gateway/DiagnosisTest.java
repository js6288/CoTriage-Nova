package io.cotriage.nova.gateway;

import static org.junit.jupiter.api.Assertions.*;

import io.cotriage.nova.common.Json;
import io.lettuce.core.StreamMessage;
import java.time.Duration;
import java.util.*;
import org.junit.jupiter.api.Test;

class DiagnosisTest {
  @Test
  void redactsAllExistingPatternsAndPreservesUnicode() {
    String text =
        DiagnosisService.redact(
            "主机 10.0.0.2 a@example.com token=abcdef API-key:1234 0123456789abcdef0123456789abcdef");
    assertEquals("主机 <IP> <EMAIL> token=<SECRET> API-key=<SECRET> <HASH>", text);
    assertEquals("😀中", DiagnosisService.truncateRunes("😀中文", 2));
  }

  @Test
  void feedbackMatchesGoFormula() {
    var in = new Events.Inferred();
    in.edge_pred = "SERVICE_ERROR";
    in.severity = 90;
    in.confidence = .4;
    var d = new Events.Diagnosis();
    d.pred = "NET_ERROR";
    d.root_cause = "closed";
    d.recommendation = "restart";
    d.confidence = .9;
    var u = new Events.Usage();
    u.latency_ms = 1000;
    u.total_tokens = 500;
    var f = DiagnosisService.feedback(in, d, u, 5000, 2000);
    assertTrue(f.corrected_edge());
    assertTrue(f.added_actionable_info());
    assertTrue(f.confirmed_high_risk());
    assertEquals("CORRECTED_EDGE", f.reward_reason());
    assertEquals(.8925, f.reward(), 1e-12);
    d.pred = "UNKNOWN";
    d.root_cause = "";
    d.recommendation = "";
    d.confidence = .3;
    in.severity = 10;
    var low = DiagnosisService.feedback(in, d, u, 5000, 2000);
    assertEquals(-.0325, low.reward(), 1e-12);
    assertEquals("LOW_UTILITY_OR_HIGH_COST", low.reward_reason());
  }

  @Test
  void repairsWrappedJsonAndUsesConfidenceFallback() {
    var p =
        Provider.parse(
            "result:\n"
                + "{\"pred\":\"net_error\",\"root_cause\":\"port"
                + " closed\",\"recommendation\":\"check"
                + " service\",\"explanation\":\"refused\",\"evidence\":[\"connection refused\"]}\n"
                + "thanks");
    assertTrue(p.structured());
    assertEquals("NET_ERROR", p.diagnosis().pred);
    assertEquals(.95, p.diagnosis().confidence);
    assertEquals(.3, Provider.parse("not json").diagnosis().confidence);
    assertEquals(
        "UNKNOWN",
        Provider.parse("{\"pred\":\"unsupported\",\"confidence\":1.4}").diagnosis().pred);
    assertEquals(
        1,
        Provider.parse("```json\n{\"pred\":\"NORMAL\",\"confidence\":2}\n```")
            .diagnosis()
            .confidence);
  }

  @Test
  void nullAndWrongScalarTypesFollowGoDecodeRules() {
    assertEquals(.4, Provider.parse("null").diagnosis().confidence);
    assertTrue(Provider.parse("null").structured());
    assertFalse(Provider.parse("{\"confidence\":\"0.9\"}").structured());
    assertEquals("UNKNOWN", Provider.parse("{\"pred\":null}").diagnosis().pred);
    assertThrows(
        IllegalArgumentException.class,
        () -> GoWire.read("{\"severity\":1.2}", Events.Inferred.class));
  }

  @Test
  void routesDecodePascalCaseAndKeepGoMissingStringQuirk() {
    var req =
        GatewayWorker.decode(
            Map.of(
                "event_id",
                "a",
                "payload_json",
                "{\"input\":{\"event_id\":\"a\"},\"decision\":{\"Action\":\"CLOUD\",\"Score\":0.5}}"));
    assertEquals("CLOUD", req.routed().decision.action);
    assertEquals(.5, req.routed().decision.score);
    assertEquals("<nil>", req.requestID());
    assertThrows(
        IllegalArgumentException.class,
        () ->
            GatewayWorker.decode(
                Map.of(
                    "event_id",
                    "a",
                    "request_id",
                    "",
                    "idempotency_key",
                    "x",
                    "payload_json",
                    "{}")));
  }

  @Test
  void promptKeepsFormattingAndRedactsRelatedLogs() {
    var in = new Events.Routed();
    in.input.raw_log = "😀中 token=x";
    in.input.evidence = List.of("one", "two");
    in.decision.reason = "HIGH_SCORE_SENDABLE";
    var related = new Events.Inferred();
    related.raw_log = "10.0.0.1";
    String prompt = DiagnosisService.prompt(in, List.of(related), 2).user();
    assertTrue(prompt.contains("Raw log: 😀中\n"));
    assertTrue(prompt.contains("evidence=[one two]"));
    assertTrue(prompt.contains("<IP>"));
    assertFalse(prompt.contains("10.0.0.1"));
  }

  @Test
  void contextSelectsByScoreFiltersWindowAndUsesByteBudget() {
    var center = new Events.Inferred();
    center.event_id = "center";
    center.event_ts = "2026-06-30T12:00:00Z";
    center.source = "host";
    center.template_id = "tpl";
    var agg = new ContextAggregator(null, "s", Duration.ofMinutes(5), 2, 1);
    var messages =
        List.of(
            message("a", "2026-06-30T12:00:00Z", "host", "tpl", "中文😀", 80),
            message("b", "2026-06-30T11:00:00Z", "host", "tpl", "outside", 99),
            message("c", "2026-06-30T12:01:00Z", "other", "other", "second", 10));
    var out = agg.select(center, messages);
    assertEquals(1, out.related_events.size());
    assertEquals("a", out.related_events.getFirst().event_id);
    assertEquals("中�", out.related_events.getFirst().raw_log);
    assertEquals(List.of("tpl"), out.aggregate.top_templates);
    assertEquals(80, out.aggregate.max_severity);
  }

  static StreamMessage<String, String> message(
      String id, String ts, String source, String template, String raw, long severity) {
    return new StreamMessage<>(
        "s",
        "1-0",
        Map.of(
            "payload",
            Json.write(
                Json.object(
                    "event_id",
                    id,
                    "event_ts",
                    ts,
                    "source",
                    source,
                    "template_id",
                    template,
                    "raw_log",
                    raw,
                    "severity",
                    severity))));
  }
}
