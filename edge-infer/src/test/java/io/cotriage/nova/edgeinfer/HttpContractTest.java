package io.cotriage.nova.edgeinfer;

import static io.cotriage.nova.edgeinfer.Models.*;
import static org.junit.jupiter.api.Assertions.*;

import com.sun.net.httpserver.HttpServer;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.List;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class HttpContractTest {
  private HttpServer server;
  private String base;

  @BeforeEach
  void start() throws Exception {
    server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
    server.start();
    base = "http://127.0.0.1:" + server.getAddress().getPort();
  }

  @AfterEach
  void stop() {
    server.stop(0);
  }

  @Test
  void classifierPostsOriginalShapeAndChecksResponse() throws Exception {
    var requestBody = new AtomicReference<String>();
    var contentType = new AtomicReference<String>();
    var method = new AtomicReference<String>();
    var responseBody =
        new AtomicReference<>(
            "{\"label\":\"ALERT\",\"label_id\":1,\"alert_probability\":0.451605,\"threshold\":0.25,\"model_version\":\"bgl-distilbert\"}");
    server.createContext(
        "/predict",
        exchange -> {
          requestBody.set(
              new String(exchange.getRequestBody().readAllBytes(), StandardCharsets.UTF_8));
          contentType.set(exchange.getRequestHeaders().getFirst("Content-Type"));
          method.set(exchange.getRequestMethod());
          byte[] bytes = responseBody.get().getBytes(StandardCharsets.UTF_8);
          exchange.sendResponseHeaders(200, bytes.length);
          exchange.getResponseBody().write(bytes);
          exchange.close();
        });
    var classifier = ModelClients.classifier(base + "/predict", Duration.ofSeconds(2));
    var result = classifier.predict(new ClassifierInput("event-1", "t1", "KERNEL FATAL rts panic"));
    assertEquals("POST", method.get());
    assertEquals("application/json", contentType.get());
    assertEquals("event-1", EdgeJson.MAPPER.readTree(requestBody.get()).path("event_id").asText());
    assertEquals(.451605, result.alertProbability);
    assertEquals("bgl-distilbert", result.modelVersion);
    responseBody.set("{\"label\":\"WRONG\",\"alert_probability\":0.1}");
    assertThrows(Exception.class, () -> classifier.predict(new ClassifierInput("", "t", "text")));
    responseBody.set("{\"label\":\"ALERT\",\"alert_probability\":2}");
    assertThrows(Exception.class, () -> classifier.predict(new ClassifierInput("", "t", "text")));
  }

  @Test
  void llmUsesOpenAiShapeAndParsesChoices() throws Exception {
    var requestBody = new AtomicReference<String>();
    var responseBody =
        new AtomicReference<>(
            EdgeJson.write(
                EdgeJson.object(
                    "choices",
                    List.of(
                        EdgeJson.object(
                            "message",
                            EdgeJson.object(
                                "content",
                                "{\"edge_pred\":\"NET_ERROR\",\"severity\":78,\"confidence\":0.72}"))))));
    server.createContext(
        "/v1/chat/completions",
        exchange -> {
          requestBody.set(
              new String(exchange.getRequestBody().readAllBytes(), StandardCharsets.UTF_8));
          byte[] body = responseBody.get().getBytes(StandardCharsets.UTF_8);
          exchange.sendResponseHeaders(200, body.length);
          exchange.getResponseBody().write(body);
          exchange.close();
        });
    var client =
        ModelClients.llm(
            base + "/v1/chat/completions", "test-llm", Duration.ofSeconds(2), 512, .2, .8, 42);
    var raw = EngineContractTest.raw("ordinary");
    var event = new InferredEvent();
    event.rawLog = raw.log.raw;
    event.templateText = "ordinary";
    var classification = new ClassifierResult();
    classification.label = "ALERT";
    classification.alertProbability = .7;
    var result = client.diagnose(new LlmInput(raw, event, classification));
    var body = EdgeJson.MAPPER.readTree(requestBody.get());
    assertEquals("test-llm", body.path("model").asText());
    assertEquals(512, body.path("max_tokens").asInt());
    assertFalse(body.path("stream").asBoolean());
    assertEquals(42, body.path("seed").asInt());
    assertEquals(
        ModelClients.systemPrompt(), body.path("messages").get(0).path("content").asText());
    assertTrue(body.path("messages").get(1).path("content").asText().contains("LinuxServer"));
    assertEquals("NET_ERROR", result.edgePred);
    assertEquals("test-llm", result.modelVersion);
    responseBody.set("{\"choices\":[]}");
    assertThrows(Exception.class, () -> client.diagnose(new LlmInput(raw, event, classification)));
  }

  @Test
  void httpErrorAndTimeoutAreModelErrors() throws Exception {
    server.createContext(
        "/bad",
        exchange -> {
          exchange.sendResponseHeaders(503, -1);
          exchange.close();
        });
    server.createContext(
        "/slow",
        exchange -> {
          try {
            Thread.sleep(150);
          } catch (InterruptedException ignored) {
          }
          exchange.sendResponseHeaders(503, -1);
          exchange.close();
        });
    assertThrows(
        Exception.class,
        () ->
            ModelClients.classifier(base + "/bad", Duration.ofSeconds(1))
                .predict(new ClassifierInput("", "t", "text")));
    assertThrows(
        Exception.class,
        () ->
            ModelClients.classifier(base + "/slow", Duration.ofMillis(20))
                .predict(new ClassifierInput("", "t", "text")));
  }

  @Test
  void remoteEtagBadConfigAndExplicitModelDisablePreserveContract() throws Exception {
    var response =
        new AtomicReference<>(
            "{\"rules\":{\"version\":9,\"items\":[]},\"templates\":{\"items\":[]}}");
    var tag = new AtomicReference<>("\"v9\"");
    var seen = new AtomicReference<String>();
    var status = new java.util.concurrent.atomic.AtomicInteger(200);
    server.createContext(
        "/config",
        exchange -> {
          seen.set(exchange.getRequestHeaders().getFirst("If-None-Match"));
          exchange.getResponseHeaders().add("ETag", tag.get());
          if (status.get() == 304) {
            exchange.sendResponseHeaders(304, -1);
          } else {
            byte[] bytes = response.get().getBytes(StandardCharsets.UTF_8);
            exchange.sendResponseHeaders(status.get(), bytes.length);
            exchange.getResponseBody().write(bytes);
          }
          exchange.close();
        });
    var engine =
        EngineContractTest.engine(
            new FakeRedis(), ModelClients.localClassifier("", Duration.ofSeconds(1)), null);
    try (var sync =
        new RemoteConfig(engine, base + "/config", Duration.ofSeconds(1), Duration.ofSeconds(2))) {
      assertTrue(sync.fetchAndApply());
      assertEquals("edge-rules-9", engine.ruleVersion());
      assertEquals("\"v9\"", sync.etag());
      assertNull(seen.get());
      assertEquals("FALLBACK", engine.infer(EngineContractTest.raw("ordinary")).inferencePath);
      status.set(304);
      assertFalse(sync.fetchAndApply());
      assertEquals("\"v9\"", seen.get());
      status.set(200);
      tag.set("\"v10\"");
      response.set(
          "{\"rules\":{\"version\":10,\"items\":[{\"id\":1,\"match_type\":\"all_keywords\",\"keywords\":[\"one\",\"two\"],\"edge_pred\":\"NET_ERROR\",\"enabled\":true}]}}");
      assertFalse(sync.fetchAndApply());
      assertEquals("edge-rules-9", engine.ruleVersion());
      assertEquals("\"v9\"", sync.etag());
      status.set(500);
      assertFalse(sync.fetchAndApply());
      assertEquals("edge-rules-9", engine.ruleVersion());
    }
  }

  @Test
  void remoteRulesPublishedModelsAndIgnoredFieldsStayCompatible() throws Exception {
    var rule = new RemoteConfig.RemoteRule();
    rule.id = 9;
    rule.matchType = "contains";
    rule.pattern = "(disk)";
    rule.edgePred = "DISK_ERROR";
    rule.enabled = true;
    rule.status = "draft";
    rule.targetField = "equipment";
    rule.severity = 80;
    rule.confidence = .9;
    var rules = RemoteConfig.rulesFromRemote(4, List.of(rule));
    assertNotNull(rules.match("(disk)"));
    assertNull(rules.match("disk"));
    rule.matchType = "any_keywords";
    rule.keywords = List.of(" one ", "two");
    assertNotNull(RemoteConfig.rulesFromRemote(5, List.of(rule)).match("TWO"));
    rule.matchType = " all_keywords ";
    assertThrows(
        IllegalArgumentException.class, () -> RemoteConfig.rulesFromRemote(6, List.of(rule)));
    var classifier =
        EdgeJson.read(
            "{\"is_active\":true,\"status\":\"published\",\"endpoint\":\""
                + base
                + "/predict\",\"timeout_ms\":1000}",
            RemoteConfig.RemoteClassifier.class);
    assertNotNull(RemoteConfig.classifierFromRemote(classifier));
    classifier.status = "draft";
    assertNull(RemoteConfig.classifierFromRemote(classifier));
    var template = new RemoteConfig.RemoteTemplate();
    template.edgePred = "UNKNOWN";
    template.templateText = "";
    assertTrue(
        RemoteConfig.templatesFromRemote(List.of(template)).containsKey(Normalizer.templateId("")));
  }
}
