package io.cotriage.nova.edgeinfer;

import static io.cotriage.nova.edgeinfer.Models.*;
import static org.junit.jupiter.api.Assertions.*;

import com.fasterxml.jackson.databind.JsonNode;
import java.io.IOException;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.stream.Stream;
import org.junit.jupiter.api.DynamicTest;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestFactory;

class GoldenParityTest {
  @TestFactory
  Stream<DynamicTest> capturedGoResults() throws Exception {
    JsonNode root;
    try (var input = getClass().getResourceAsStream("/go-inference-golden.json")) {
      assertNotNull(input);
      root = EdgeJson.MAPPER.readTree(input);
    }
    var tests = new ArrayList<DynamicTest>();
    int index = 0;
    for (var item : root.path("normalization")) {
      tests.add(
          DynamicTest.dynamicTest(
              "Go normalization " + index++,
              () -> {
                var result =
                    Normalizer.normalizeResult(
                        item.path("raw").asText(), item.path("max_bytes").asInt());
                assertEquals(item.path("template_text").asText(), result.text());
                assertEquals(item.path("template_id").asText(), result.id());
              }));
    }
    for (var item : root.path("classifier"))
      tests.add(
          DynamicTest.dynamicTest(
              "Go classifier " + index++,
              () -> {
                var input = EdgeJson.MAPPER.treeToValue(item.path("input"), ClassifierResult.class);
                assertEquals(
                    item.path("result"),
                    EdgeJson.MAPPER.valueToTree(ModelClients.resultFromClassifier(input).toMap()));
              }));
    for (var item : root.path("llm"))
      tests.add(
          DynamicTest.dynamicTest(
              "Go LLM " + index++,
              () -> {
                if (item.path("error").asBoolean())
                  assertThrows(
                      IOException.class,
                      () -> ModelClients.parseLlmJson(item.path("content").asText()));
                else
                  assertEquals(
                      item.path("result"),
                      EdgeJson.MAPPER.valueToTree(
                          llmMap(ModelClients.parseLlmJson(item.path("content").asText()))));
              }));
    for (var item : root.path("rules"))
      tests.add(
          DynamicTest.dynamicTest(
              "Go default rules " + index++,
              () -> {
                var result = RuleEngine.load("").match(item.path("text").asText());
                if (!item.path("matched").asBoolean()) assertNull(result);
                else {
                  assertNotNull(result);
                  assertEquals(item.path("result"), EdgeJson.MAPPER.valueToTree(result.toMap()));
                }
              }));
    return tests.stream();
  }

  static Map<String, Object> llmMap(LlmResult result) {
    var map =
        EdgeJson.object(
            "edge_pred",
            result.edgePred,
            "severity",
            result.severity,
            "confidence",
            result.confidence);
    if (result.evidence != null && !result.evidence.isEmpty()) map.put("evidence", result.evidence);
    EdgeJson.optional(map, "advice_code", result.adviceCode);
    EdgeJson.optional(map, "advice", result.advice);
    EdgeJson.optional(map, "model_version", result.modelVersion);
    return map;
  }

  @Test
  void highestPriorityAndStableTiesArePreserved() {
    var low = RuleEngine.rule("low", "failed", "APP_ERROR", 50, .8, "low", "", "", 1);
    var high =
        RuleEngine.rule("high", "authentication failed", "SECURITY", 80, .95, "high", "", "", 10);
    assertEquals(
        "SECURITY",
        new RuleEngine("v1", List.of(low, high))
            .match("authentication failed for user admin")
            .edgePred);
    low.priority = 10;
    assertEquals(
        "APP_ERROR",
        new RuleEngine("v1", List.of(low, high)).match("authentication failed").edgePred);
  }

  @Test
  void invalidRuleIsRejectedAndDisabledRuleIsSkipped() {
    var rule = RuleEngine.rule("bad", "(", "UNKNOWN", 50, .5, "", "", "", 0);
    assertThrows(IllegalArgumentException.class, () -> new RuleEngine("v1", List.of(rule)));
    rule.enabled = false;
    assertNull(new RuleEngine("v1", List.of(rule)).match("anything"));
  }

  @Test
  void rawLogUsesCodepointsNotUtf16Length() {
    String text = "😀".repeat(2049);
    assertEquals("😀".repeat(2048) + "...[truncated]", Normalizer.truncateRawLog(text));
  }

  @Test
  void llmResultMappingPreservesAdvice() {
    var input = new LlmResult();
    input.edgePred = "DISK_ERROR";
    input.severity = 91;
    input.confidence = .8;
    input.evidence = List.of("no space");
    input.adviceCode = "CHECK_DISK";
    input.advice = "检查磁盘";
    var result = ModelClients.resultFromLlm(input);
    assertEquals(91, result.severity);
    assertEquals(.8, result.confidence);
    assertEquals("CHECK_DISK", result.adviceCode);
    assertEquals("检查磁盘", result.advice);
  }
}
