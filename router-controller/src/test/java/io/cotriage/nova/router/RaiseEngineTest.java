package io.cotriage.nova.router;

import static org.junit.jupiter.api.Assertions.*;

import com.fasterxml.jackson.databind.JsonNode;
import io.cotriage.nova.common.Json;
import java.util.*;
import org.junit.jupiter.api.Test;

class RaiseEngineTest {
  static Events.Inferred event() {
    Events.Inferred in = new Events.Inferred();
    in.event_id = "evt-1";
    in.source = "host-1";
    in.template_id = "tpl-1";
    in.edge_pred = "NET_ERROR";
    in.severity = 80;
    in.confidence = .6;
    in.uncertainty = .4;
    in.inference_path = "MODEL";
    return in;
  }

  static RaiseEngine.SystemState healthy() {
    RaiseEngine.SystemState s = new RaiseEngine.SystemState();
    s.CloudHealth = "UP";
    s.RTTMS = 80;
    s.FailRate = .01;
    s.RateLimitRemaining = 10;
    s.MinuteBudgetRemaining = 10;
    s.DayBudgetRemaining = 10000;
    s.QueueMax = 1000;
    s.MinCloudRatioForAudit = .1;
    s.RecentCloudRatio = .1;
    s.EstimatedLatencyMS = 1000;
    return s;
  }

  static RaiseEngine.Stats high() {
    RaiseEngine.Stats s = new RaiseEngine.Stats();
    s.templateFrequency = 1;
    s.historicalCloudCount = 10;
    s.historicalCorrectionCount = 6;
    s.affectedDevices = 8;
    s.conformalSetSize = 4;
    s.ucbUncertainty = .25;
    return s;
  }

  @Test
  void allDecisionFieldsMatchGo122Golden() throws Exception {
    RaiseEngine e = new RaiseEngine(new RaiseEngine.Config());
    var in = event();
    var s = healthy();
    var st = high();
    List<RaiseEngine.Decision> cases = new ArrayList<>();
    cases.add(e.decide(in, s, st, new RaiseEngine.Policy()));
    in = event();
    in.edge_pred = "NORMAL";
    in.severity = 5;
    in.confidence = .99;
    in.uncertainty = .01;
    in.inference_path = "RULE";
    st = new RaiseEngine.Stats();
    st.templateFrequency = 100;
    st.conformalSetSize = 1;
    cases.add(e.decide(in, s, st, new RaiseEngine.Policy()));
    in = event();
    in.severity = 99;
    in.confidence = .2;
    in.uncertainty = .8;
    in.inference_path = "FALLBACK";
    s = healthy();
    s.CloudHealth = "DOWN";
    st = high();
    st.templateFrequency = 0;
    st.historicalCorrectionCount = 10;
    st.affectedDevices = 20;
    st.conformalSetSize = 8;
    st.ucbUncertainty = 1;
    cases.add(e.decide(in, s, st, new RaiseEngine.Policy()));
    JsonNode expected =
        Json.MAPPER.readTree(getClass().getResourceAsStream("/raise-go-golden.json"));
    assertTree(expected, Json.MAPPER.valueToTree(cases));
  }

  static void assertTree(JsonNode expected, JsonNode actual) {
    if (expected.isNumber()) {
      assertEquals(expected.asDouble(), actual.asDouble(), 1e-12);
      return;
    }
    if (expected.isObject()) {
      assertEquals(expected.size(), actual.size());
      expected
          .fields()
          .forEachRemaining(
              e -> {
                assertTrue(actual.has(e.getKey()), e.getKey());
                assertTree(e.getValue(), actual.get(e.getKey()));
              });
      return;
    }
    if (expected.isArray()) {
      assertEquals(expected.size(), actual.size());
      for (int i = 0; i < expected.size(); i++) assertTree(expected.get(i), actual.get(i));
      return;
    }
    assertEquals(expected, actual);
  }

  @Test
  void policiesPreservePriorityAndNormalizedLookup() {
    var e = new RaiseEngine(new RaiseEngine.Config());
    var p = new RaiseEngine.Policy();
    p.denyPreds = Set.of("NET_ERROR");
    p.forceCloudTemplates = Set.of("TPL-1");
    assertEquals("DENY_CLOUD_POLICY", e.decide(event(), healthy(), high(), p).Reason());
    p.denyPreds = Set.of();
    p.forceLocalPreds = Set.of("NET_ERROR");
    assertEquals("FORCE_LOCAL_POLICY", e.decide(event(), healthy(), high(), p).Reason());
    p.forceLocalPreds = Set.of();
    var s = healthy();
    s.RateLimitRemaining = 0;
    assertEquals("FORCE_CLOUD_DEFER", e.decide(event(), s, new RaiseEngine.Stats(), p).Reason());
    s.RateLimitRemaining = 1;
    assertEquals("FORCE_CLOUD_SENDABLE", e.decide(event(), s, new RaiseEngine.Stats(), p).Reason());
  }

  @Test
  void minuteDayFailureHealthGatesAndThresholdEquality() {
    var c = new RaiseEngine.Config();
    c.scoreValue = 0;
    c.scoreGain = 0;
    c.scoreCost = 0;
    c.thresholdBase = 0;
    c.thresholdQueue = 0;
    c.thresholdBudget = 0;
    c.thresholdNetwork = 0;
    c.thresholdAudit = 0;
    var e = new RaiseEngine(c);
    var s = healthy();
    assertEquals("CLOUD", e.decide(event(), s, high(), new RaiseEngine.Policy()).Action());
    s.MinuteBudgetRemaining = 0;
    assertEquals("DEFER", e.decide(event(), s, high(), new RaiseEngine.Policy()).Action());
    s.MinuteBudgetRemaining = 1;
    s.DayBudgetRemaining = 0;
    assertFalse(e.decide(event(), s, high(), new RaiseEngine.Policy()).Sendable());
    s.DayBudgetRemaining = 10;
    s.FailRate = .21;
    assertFalse(e.decide(event(), s, high(), new RaiseEngine.Policy()).Sendable());
  }

  @Test
  void inferredWireOmissionAndStageZero() {
    var in = Events.decode("{\"EVENT_ID\":\"e\",\"ignored\":1}");
    JsonNode n = Json.MAPPER.valueToTree(in);
    assertEquals("e", n.path("event_id").asText());
    assertFalse(n.has("trace_id"));
    assertFalse(n.has("evidence"));
    assertEquals(5, n.path("stage_latency_ms").size());
    assertTrue(n.has("severity"));
  }

  @Test
  void goDecoderRejectsWrongScalarTypesAndPreservesZeroOnNull() {
    assertThrows(IllegalArgumentException.class, () -> Events.decode("{\"severity\":\"4\"}"));
    assertThrows(IllegalArgumentException.class, () -> Events.decode("{\"severity\":1.5}"));
    assertThrows(IllegalArgumentException.class, () -> Events.decode("{\"event_id\":42}"));
    var in = Events.decode("{\"event_id\":null,\"stage_latency_ms\":null,\"confidence\":null}");
    assertEquals("", in.event_id);
    assertNotNull(in.stage_latency_ms);
    assertEquals(0, in.confidence);
  }
}
