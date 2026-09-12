package io.cotriage.nova.verification;

import static org.junit.jupiter.api.Assertions.*;

import io.cotriage.nova.common.Json;
import java.io.IOException;
import java.net.URI;
import java.net.http.*;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.*;

/**
 * Tracks actual cross-process HTTP calls; the expected route list comes from the original Go
 * router.
 */
final class ManagementContracts {
  final Map<String, List<Integer>> routes = new TreeMap<>();
  private final HttpClient http;
  private final String base;

  ManagementContracts(HttpClient http, String base) {
    this.http = http;
    this.base = base;
  }

  long exercise(String stub, long activeCloud) throws Exception {
    var provider =
        Json.object(
            "name",
            "edge fixture",
            "provider",
            "openai_compat",
            "endpoint",
            stub + "/chat",
            "model",
            "fixture-edge",
            "timeout_ms",
            5000,
            "max_tokens",
            128,
            "temperature",
            .1,
            "top_p",
            1,
            "response_path",
            "choices[0].message.content",
            "healthcheck_path",
            "/health");
    long edge = id(call("POST", "/edge-providers", provider, 201));
    provider.put("name", "edge spare");
    long edgeSpare = id(call("POST", "/edge-providers", provider, 201));
    var patched =
        call(
            "PATCH",
            "/edge-providers/" + edgeSpare,
            Json.object("temperature", .2, "seed", null),
            200);
    assertEquals("fixture-edge", patched.get("model"));
    assertNull(patched.get("seed"));
    call("GET", "/edge-providers/" + edge, null, 200);
    call("GET", "/edge-providers", null, 200);
    call("POST", "/edge-providers/" + edge + "/activate", Json.object(), 200);
    call("DELETE", "/edge-providers/" + edge, null, 409);
    call("DELETE", "/edge-providers/" + edgeSpare, null, 204);

    var classifier =
        Json.object(
            "name",
            "classifier fixture",
            "endpoint",
            stub + "/predict",
            "model",
            "fixture-v1",
            "timeout_ms",
            1500,
            "threshold",
            .5);
    long classifierId = id(call("POST", "/edge-classifiers", classifier, 201));
    classifier.put("name", "classifier spare");
    long spareClassifier = id(call("POST", "/edge-classifiers", classifier, 201));
    classifier.put("threshold", .6);
    call("PATCH", "/edge-classifiers/" + spareClassifier, classifier, 200);
    call("GET", "/edge-classifiers", null, 200);
    call("POST", "/edge-classifiers/" + classifierId + "/activate", Json.object(), 200);
    call("DELETE", "/edge-classifiers/" + spareClassifier, null, 204);

    var rule =
        Json.object(
            "name",
            "network fixture",
            "match_type",
            "contains",
            "pattern",
            "connection refused",
            "edge_pred",
            "NET_ERROR",
            "severity",
            80,
            "confidence",
            .96,
            "enabled",
            true,
            "priority",
            100,
            "evidence",
            List.of("fixture network"));
    long ruleId = id(call("POST", "/edge-rules", rule, 201));
    rule.put("severity", 81);
    call("PATCH", "/edge-rules/" + ruleId, rule, 200);
    call("GET", "/edge-rules", null, 200);
    rule.put("name", "rule spare");
    long spareRule = id(call("POST", "/edge-rules", rule, 201));
    call("DELETE", "/edge-rules/" + spareRule, null, 204);
    long setId =
        id(
            call(
                "POST",
                "/edge-rule-sets",
                Json.object("name", "fixture rules", "item_ids", List.of(ruleId)),
                201));
    var ruleSet = call("POST", "/edge-rule-sets/" + setId + "/activate", Json.object(), 200);
    assertEquals(1, ((List<?>) ruleSet.get("items")).size());

    var template =
        Json.object(
            "template_text",
            "static verification template",
            "edge_pred",
            "NORMAL",
            "severity",
            10,
            "confidence",
            .95,
            "evidence",
            List.of("fixture template"));
    long templateId = id(call("POST", "/edge-templates", template, 201));
    template.put("advice", "No action");
    call("PATCH", "/edge-templates/" + templateId, template, 200);
    call("GET", "/edge-templates", null, 200);
    template.put("template_text", "spare template");
    long spareTemplate = id(call("POST", "/edge-templates", template, 201));
    call("DELETE", "/edge-templates/" + spareTemplate, null, 204);
    var cluster =
        Json.object(
            "template_text",
            "cluster example <*>",
            "edge_pred",
            "APP_ERROR",
            "max_severity",
            70,
            "avg_confidence",
            .9,
            "evidence",
            List.of("cluster"));
    assertEquals(true, call("POST", "/edge-templates/from-cluster", cluster, 201).get("created"));
    assertEquals(false, call("POST", "/edge-templates/from-cluster", cluster, 200).get("created"));
    long templates =
        id(
            call(
                "POST",
                "/edge-template-sets",
                Json.object("name", "fixture templates", "item_ids", List.of(templateId)),
                201));
    call("POST", "/edge-template-sets/" + templates + "/activate", Json.object(), 200);
    var edgeConfig =
        http.send(
            HttpRequest.newBuilder(URI.create(base + "/api/v1/edge-config"))
                .timeout(Duration.ofSeconds(8))
                .GET()
                .build(),
            HttpResponse.BodyHandlers.ofString());
    record("GET", "/edge-config", edgeConfig.statusCode());
    assertEquals(200, edgeConfig.statusCode());
    var unchanged =
        http.send(
            HttpRequest.newBuilder(URI.create(base + "/api/v1/edge-config"))
                .timeout(Duration.ofSeconds(8))
                .header("If-None-Match", edgeConfig.headers().firstValue("ETag").orElseThrow())
                .GET()
                .build(),
            HttpResponse.BodyHandlers.ofString());
    record("GET", "/edge-config", unchanged.statusCode());
    assertEquals(304, unchanged.statusCode());
    assertTrue(unchanged.body().isEmpty());

    var cloud =
        Json.object(
            "name",
            "cloud spare",
            "provider",
            "openai_compat",
            "endpoint",
            stub + "/chat",
            "model",
            "fixture-cloud",
            "api_key_env",
            "FIXTURE_KEY",
            "api_key",
            "fixture-key",
            "timeout_ms",
            5000,
            "max_tokens",
            128,
            "temperature",
            .1,
            "top_p",
            1);
    long cloudSpare = id(call("POST", "/cloud-providers", cloud, 201));
    call(
        "PATCH",
        "/cloud-providers/" + cloudSpare,
        Json.object("temperature", .2, "api_key", ""),
        200);
    call("GET", "/cloud-providers/" + activeCloud, null, 200);
    call("GET", "/cloud-providers", null, 200);
    call("DELETE", "/cloud-providers/" + cloudSpare, null, 204);
    return ((Number) ruleSet.get("version")).longValue();
  }

  private Map<String, Object> call(String method, String path, Object body, int expected)
      throws Exception {
    var request =
        HttpRequest.newBuilder(URI.create(base + "/api/v1" + path))
            .timeout(Duration.ofSeconds(8))
            .header("Content-Type", "application/json")
            .method(
                method,
                body == null
                    ? HttpRequest.BodyPublishers.noBody()
                    : HttpRequest.BodyPublishers.ofString(Json.write(body)))
            .build();
    var response = http.send(request, HttpResponse.BodyHandlers.ofString());
    record(method, path, response.statusCode());
    assertEquals(expected, response.statusCode(), method + " " + path + ": " + response.body());
    return response.body().isBlank() ? Map.of() : Json.map(response.body());
  }

  void record(String method, String path, int status) {
    String route = path.split("\\?", 2)[0].replaceFirst("^/api/v1", "");
    route =
        route
            .replaceAll("/router/decisions/[^/]+", "/router/decisions/{event_id}")
            .replaceAll("/\\d+(?=/|$)", "/{id}");
    routes.computeIfAbsent(method + " " + route, key -> new ArrayList<>()).add(status);
  }

  void assertCoverage() throws IOException {
    try (var input = getClass().getResourceAsStream("/original-routes.txt")) {
      assertNotNull(input);
      var expected =
          new TreeSet<>(
              new String(input.readAllBytes(), StandardCharsets.UTF_8)
                  .lines()
                  .filter(line -> !line.isBlank())
                  .toList());
      assertEquals(47, expected.size());
      assertEquals(
          expected,
          routes.keySet(),
          "Every original HTTP method/route must have actual packaged-server evidence");
    }
  }

  private static long id(Map<String, Object> value) {
    return ((Number) value.get("id")).longValue();
  }
}
