package io.cotriage.nova.edgeinfer;

import static io.cotriage.nova.edgeinfer.Models.*;
import static org.junit.jupiter.api.Assertions.*;

import java.io.IOException;
import java.time.Duration;
import java.util.List;
import java.util.Map;
import java.util.concurrent.atomic.AtomicInteger;
import org.junit.jupiter.api.Test;

class EngineContractTest {
  static RawEvent raw(String text) {
    var raw = new RawEvent();
    raw.eventId = "event-1";
    raw.ts = "2026-09-12T00:00:00.123456789Z";
    raw.equipment = "LinuxServer";
    raw.source = "replay";
    raw.log.raw = text;
    return raw;
  }

  static Engine engine(
      FakeRedis redis, ModelClients.Classifier classifier, ModelClients.EdgeLlm llm)
      throws Exception {
    return new Engine(
        redis, RuleEngine.load(""), Map.of(), Duration.ofMinutes(5), 8192, classifier, llm);
  }

  @Test
  void ruleWinsAndPublishesFullContract() throws Exception {
    var redis = new FakeRedis();
    var engine =
        engine(
            redis,
            input -> {
              fail("classifier called");
              return null;
            },
            null);
    var result = engine.infer(raw("2026-09-12T00:00:00Z no space left on device /var/log/app"));
    assertEquals("RULE", result.inferencePath);
    assertEquals("DISK_ERROR", result.edgePred);
    assertEquals(.01, result.uncertainty, 1e-12);
    var json = EdgeJson.MAPPER.valueToTree(result.toMap());
    assertEquals("replay", json.path("source").asText());
    assertEquals("LinuxServer", json.path("equipment").asText());
    assertEquals(0, json.path("stage_latency_ms").path("publish").asInt());
    assertFalse(json.has("model_version"));
    assertTrue(redis.strings.containsKey(Engine.cacheKey(result.templateId, "edge-rules-v1")));
    assertEquals(
        Duration.ofMinutes(5), redis.ttls.get(Engine.cacheKey(result.templateId, "edge-rules-v1")));
  }

  @Test
  void explicitNullModelsFallbackButLocalTypedNilRemainsFatal() throws Exception {
    var result = engine(new FakeRedis(), null, null).infer(raw("unclassified ordinary log"));
    assertEquals("FALLBACK", result.inferencePath);
    assertEquals("UNKNOWN", result.edgePred);
    assertEquals(50, result.severity);
    assertEquals(1, result.uncertainty);
    assertEquals(List.of(), result.evidence);
    var legacy =
        engine(new FakeRedis(), ModelClients.localClassifier("", Duration.ofSeconds(1)), null);
    assertThrows(
        ModelClients.LegacyModelPanic.class, () -> legacy.infer(raw("unclassified ordinary log")));
  }

  @Test
  void templateCacheWinsAndRetainsOriginalModelVersionOmission() throws Exception {
    var redis = new FakeRedis();
    var calls = new AtomicInteger();
    ModelClients.Classifier classifier =
        input -> {
          calls.incrementAndGet();
          var c = new ClassifierResult();
          c.label = "NON_ALERT";
          c.alertProbability = .05;
          c.modelVersion = "model-1";
          return c;
        };
    var engine = engine(redis, classifier, null);
    var first = engine.infer(raw("service completed 123"));
    var second = engine.infer(raw("service completed 456"));
    assertEquals("MODEL", first.inferencePath);
    assertEquals("CACHE", second.inferencePath);
    assertEquals("", second.modelVersion);
    assertEquals(1, calls.get());
  }

  @Test
  void staticTemplateBypassesRedisAndClassifier() throws Exception {
    var value = new TemplateResult();
    value.edgePred = "NORMAL";
    value.confidence = .4;
    value.severity = 9;
    var redis = new FakeRedis();
    redis.fail = op -> true;
    var engine =
        new Engine(
            redis,
            RuleEngine.load(""),
            Map.of(Normalizer.templateId("ordinary"), value),
            Duration.ofMinutes(5),
            8192,
            input -> {
              fail("classifier called");
              return null;
            },
            null);
    assertEquals("CACHE", engine.infer(raw("ordinary")).inferencePath);
    assertTrue(redis.calls.isEmpty());
  }

  @Test
  void classifierFailureFallsBackAndCacheReadEvidenceIsOverwrittenAsBefore() throws Exception {
    var redis = new FakeRedis();
    redis.fail = op -> op.startsWith("get:");
    var result =
        engine(
                redis,
                input -> {
                  throw new IOException("offline");
                },
                null)
            .infer(raw("ordinary"));
    assertEquals("FALLBACK", result.inferencePath);
    assertEquals(List.of("classifier_error=offline"), result.evidence);
  }

  @Test
  void abnormalCallsLlmAndLlmFailureRetainsAbnormal() throws Exception {
    ModelClients.Classifier classifier =
        input -> {
          var c = new ClassifierResult();
          c.label = "ALERT";
          c.alertProbability = .45;
          c.modelVersion = "classifier-v1";
          return c;
        };
    var result =
        engine(
                new FakeRedis(),
                classifier,
                input -> {
                  throw new IOException("timeout");
                })
            .infer(raw("ordinary"));
    assertEquals("ABNORMAL", result.edgePred);
    assertEquals("MODEL", result.inferencePath);
    assertEquals("classifier-v1", result.modelVersion);
    assertTrue(result.evidence.contains("edge_llm_error=timeout"));
    var fine =
        engine(
                new FakeRedis(),
                classifier,
                input -> {
                  var r = new LlmResult();
                  r.edgePred = "NET_ERROR";
                  r.severity = 78;
                  r.confidence = .72;
                  r.modelVersion = "llm-v1";
                  return r;
                })
            .infer(raw("ordinary"));
    assertEquals("EDGE_LLM", fine.inferencePath);
    assertEquals("NET_ERROR", fine.edgePred);
    assertEquals("llm-v1", fine.modelVersion);
  }

  @Test
  void normalDoesNotCallLlmAndCacheWritesAreBestEffort() throws Exception {
    var redis = new FakeRedis();
    redis.fail = op -> op.startsWith("set:");
    var result =
        engine(
                redis,
                input -> {
                  var c = new ClassifierResult();
                  c.label = "NON_ALERT";
                  c.alertProbability = .01;
                  return c;
                },
                input -> {
                  fail("LLM called");
                  return null;
                })
            .infer(raw("ordinary"));
    assertEquals("NORMAL", result.edgePred);
    assertEquals("MODEL", result.inferencePath);
    assertTrue(
        result.evidence.stream()
            .anyMatch(value -> value.startsWith("template_cache_write_error=")));
  }

  @Test
  void legacyNilLlmPanicsForAbnormal() throws Exception {
    var engine =
        engine(
            new FakeRedis(),
            input -> {
              var c = new ClassifierResult();
              c.label = "ALERT";
              c.alertProbability = .2;
              return c;
            },
            ModelClients.localLlm("", Duration.ofSeconds(3)));
    assertThrows(ModelClients.LegacyModelPanic.class, () -> engine.infer(raw("ordinary")));
  }

  @Test
  void invalidRawEventsAreRejected() throws Exception {
    var engine = engine(new FakeRedis(), null, null);
    var missingId = raw("x");
    missingId.eventId = " ";
    assertThrows(IOException.class, () -> engine.infer(missingId));
    assertThrows(IOException.class, () -> engine.infer(raw(" \n")));
  }
}
