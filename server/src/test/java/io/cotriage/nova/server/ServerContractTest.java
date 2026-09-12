package io.cotriage.nova.server;

import static io.cotriage.nova.server.Values.*;
import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.*;

import com.zaxxer.hikari.HikariDataSource;
import io.cotriage.nova.common.Db;
import io.cotriage.nova.common.Json;
import java.io.*;
import java.nio.charset.StandardCharsets;
import java.time.*;
import java.util.*;
import java.util.concurrent.*;
import org.junit.jupiter.api.*;
import org.springframework.core.annotation.AnnotatedElementUtils;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;
import org.springframework.web.bind.annotation.RequestMapping;
import org.testcontainers.containers.GenericContainer;
import org.testcontainers.containers.MySQLContainer;
import org.testcontainers.junit.jupiter.*;

/**
 * Destructive setup is confined to disposable Testcontainers databases. Docker absence is a
 * failure.
 */
@Testcontainers
class ServerContractTest {
  @Container
  static final MySQLContainer<?> MYSQL =
      new MySQLContainer<>("mysql:8.4")
          .withDatabaseName("server_contract")
          .withUsername("test")
          .withPassword("test");

  @Container
  static final GenericContainer<?> REDIS =
      new GenericContainer<>("redis:7.4-alpine").withExposedPorts(6379);

  static HikariDataSource dataSource;
  static JdbcTemplate jdbc;
  static ConfigRepository repo;
  static RouterRepository router;
  static EventStore events;
  static MetricsService metrics;
  MockMvc mvc;

  @BeforeAll
  static void connect() {
    dataSource =
        Db.open(
            "test:test@tcp("
                + MYSQL.getHost()
                + ":"
                + MYSQL.getMappedPort(3306)
                + ")/server_contract?parseTime=true&loc=UTC");
    jdbc = new JdbcTemplate(dataSource);
    repo = new ConfigRepository(jdbc);
    repo.initialize();
    router = new RouterRepository(jdbc);
    events = new EventStore(REDIS.getHost() + ":" + REDIS.getMappedPort(6379));
    metrics = new MetricsService(events, router);
  }

  @AfterAll
  static void close() {
    if (events != null) events.close();
    if (dataSource != null) dataSource.close();
  }

  @BeforeEach
  void reset() {
    for (var table : ConfigRepository.TABLES.values()) jdbc.update("DELETE FROM " + table);
    for (var table :
        List.of("router_decisions", "router_state_snapshots", "cloud_outbox", "cloud_feedback"))
      jdbc.execute("DROP TABLE IF EXISTS " + table);
    jdbc.update("DELETE FROM competition_metric_snapshots");
    repo.initialize();
    events.redis().flushdb();
    var devices = mock(DeviceService.class);
    when(devices.devices(anyMap()))
        .thenReturn(obj("items", List.of(), "total", 0, "generated_at", Instant.now().toString()));
    mvc =
        MockMvcBuilders.standaloneSetup(
                new ManagementController(repo, router, events, metrics, devices, "secret"))
            .setControllerAdvice(new ApiErrors())
            .build();
  }

  static Map<String, Object> provider(String name) {
    return obj(
        "name",
        name,
        "provider",
        "openai_compat",
        "endpoint",
        "http://127.0.0.1:18081/v1/chat/completions",
        "model",
        "test-model",
        "timeout_ms",
        500,
        "max_tokens",
        80,
        "temperature",
        .1,
        "top_p",
        1,
        "seed",
        7,
        "healthcheck_path",
        "/health",
        "response_path",
        "choices[0].message.content");
  }

  static Map<String, Object> cloud(String name) {
    var p = provider(name);
    p.remove("healthcheck_path");
    p.put("health_path", "/health");
    p.put("api_key_env", "TEST_CLOUD_KEY");
    p.put("api_key", "test-only-key");
    return p;
  }

  Map<String, Object> request(String method, String path, Object body, int expected)
      throws Exception {
    var builder =
        switch (method) {
          case "POST" -> post(path);
          case "PATCH" -> patch(path);
          case "DELETE" -> delete(path);
          default -> get(path);
        };
    if (body != null) builder.contentType("application/json").content(Json.write(body));
    var result = mvc.perform(builder).andReturn().getResponse();
    assertEquals(expected, result.getStatus(), result.getContentAsString());
    return result.getContentAsString().isBlank() ? Map.of() : Json.map(result.getContentAsString());
  }

  @Test
  void exposesExactlyTheOriginal47Routes() {
    var routes = new TreeSet<String>();
    for (var method : ManagementController.class.getDeclaredMethods()) {
      var m = AnnotatedElementUtils.findMergedAnnotation(method, RequestMapping.class);
      if (m != null)
        for (var path : m.path()) for (var verb : m.method()) routes.add(verb + " " + path);
    }
    assertEquals(47, routes.size(), routes.toString());
    assertTrue(routes.contains("POST /edge-templates/from-cluster"));
    assertTrue(routes.contains("GET /router/stream"));
  }

  @Test
  void edgeProviderCrudActivationAndConditionalConfig() throws Exception {
    var first = request("POST", "/api/v1/edge-providers", provider("first"), 201);
    var second = request("POST", "/api/v1/edge-providers", provider("second"), 201);
    long a = num(first, "id"), b = num(second, "id");
    var activeA = request("POST", "/api/v1/edge-providers/" + a + "/activate", null, 200);
    var legacyProvider = (Map<String, Object>) activeA.get("active_provider");
    assertFalse(yes(legacyProvider, "is_active"));
    assertEquals("draft", legacyProvider.get("status"));
    assertEquals(0, num(legacyProvider, "config_version"));
    assertTrue(
        yes(repo.get("edge-providers", a), "is_active"),
        "database commits activation even though response embeds prior snapshot");
    request("DELETE", "/api/v1/edge-providers/" + a, null, 409);
    var activeB = request("POST", "/api/v1/edge-providers/" + b + "/activate", null, 200);
    assertTrue(num(activeB, "config_version") > num(activeA, "config_version"));
    assertFalse(yes(repo.get("edge-providers", a), "is_active"));
    var cfg = mvc.perform(get("/api/v1/edge-config")).andReturn().getResponse();
    assertEquals(200, cfg.getStatus());
    String etag = cfg.getHeader("ETag");
    assertNotNull(etag);
    var cached =
        mvc.perform(get("/api/v1/edge-config").header("If-None-Match", etag))
            .andReturn()
            .getResponse();
    assertEquals(304, cached.getStatus());
    assertEquals("", cached.getContentAsString());
    request("DELETE", "/api/v1/edge-providers/" + a, null, 204);
    assertEquals("deleted", request("GET", "/api/v1/edge-providers/" + a, null, 200).get("status"));
  }

  @Test
  void patchPreservesAbsentAndNullFieldsLikeGoDoublePointers() throws Exception {
    var p = request("POST", "/api/v1/edge-providers", provider("patch"), 201);
    var patched =
        request(
            "PATCH",
            "/api/v1/edge-providers/" + num(p, "id"),
            obj("seed", null, "healthcheck_path", null, "temperature", 0),
            200);
    assertEquals("test-model", patched.get("model"));
    assertEquals(7, num(patched, "seed"));
    assertEquals("/health", patched.get("healthcheck_path"));
    assertEquals(0, dec(patched, "temperature"));
    request("PATCH", "/api/v1/edge-providers/" + num(p, "id"), obj("is_active", true), 400);
    request("GET", "/api/v1/edge-providers/bad", null, 400);
  }

  @Test
  void cloudLifecycleTokenAnd304() throws Exception {
    var p = request("POST", "/api/v1/cloud-providers", cloud("cloud"), 201);
    long id = num(p, "id");
    assertEquals("test-only-key", p.get("api_key"));
    var patched =
        request(
            "PATCH", "/api/v1/cloud-providers/" + id, obj("api_key", "", "temperature", 0), 200);
    assertEquals("test-only-key", patched.get("api_key"));
    request("POST", "/api/v1/cloud-providers/" + id + "/activate", null, 200);
    request("PATCH", "/api/v1/cloud-providers/" + id, obj("model", "other"), 409);
    request("DELETE", "/api/v1/cloud-providers/" + id, null, 409);
    request("GET", "/api/v1/cloud-config", null, 401);
    var response =
        mvc.perform(get("/api/v1/cloud-config").header("Authorization", "Bearer secret"))
            .andReturn()
            .getResponse();
    assertEquals(200, response.getStatus());
    assertEquals(
        304,
        mvc.perform(
                get("/api/v1/cloud-config")
                    .header("Authorization", "Bearer secret")
                    .header("If-None-Match", response.getHeader("ETag")))
            .andReturn()
            .getResponse()
            .getStatus());
  }

  @Test
  void seedIsIdempotentAndPreservesActiveUserSet() {
    assertEquals(5, repo.count("SELECT COUNT(*) FROM edge_rules"));
    assertEquals(5, list(repo.activeSet("edge-rule-sets").get("items")).size());
    long version = num(repo.activeSet("edge-rule-sets"), "version");
    repo.initialize();
    assertEquals(5, repo.count("SELECT COUNT(*) FROM edge_rules"));
    assertEquals(version, num(repo.activeSet("edge-rule-sets"), "version"));
    var set = repo.create("edge-rule-sets", obj("name", "user", "item_ids", List.of()));
    repo.activate("edge-rule-sets", num(set, "id"));
    repo.initialize();
    assertEquals(
        "user",
        repo.rows("SELECT name FROM edge_rule_sets WHERE status='active'").getFirst().get("name"));
  }

  @Test
  void rulesTemplatesAndClassifierAreActualRepositories() throws Exception {
    var rule =
        obj(
            "name",
            "custom",
            "match_type",
            "contains",
            "pattern",
            "panic",
            "edge_pred",
            "APP_ERROR",
            "severity",
            80,
            "confidence",
            .9,
            "enabled",
            true,
            "priority",
            1);
    var saved = request("POST", "/api/v1/edge-rules", rule, 201);
    long id = num(saved, "id");
    rule.put("pattern", "fatal");
    request("PATCH", "/api/v1/edge-rules/" + id, rule, 200);
    var listed = request("GET", "/api/v1/edge-rules?q=custom&edge_pred=APP_ERROR", null, 200);
    assertEquals(1, num(listed, "total"));
    var set =
        request(
            "POST",
            "/api/v1/edge-rule-sets",
            obj("name", "selected", "item_ids", List.of(id)),
            201);
    var snapshot =
        request("POST", "/api/v1/edge-rule-sets/" + num(set, "id") + "/activate", null, 200);
    assertEquals(1, list(snapshot.get("items")).size());
    request("DELETE", "/api/v1/edge-rules/" + id, null, 204);
    assertEquals(
        1,
        list(repo.activeSet("edge-rule-sets").get("items")).size(),
        "published sets remain immutable snapshots");
    var template =
        obj(
            "template_text",
            "fatal at <*>",
            "edge_pred",
            "APP_ERROR",
            "max_severity",
            80,
            "avg_confidence",
            .9);
    var cluster = request("POST", "/api/v1/edge-templates/from-cluster", template, 201);
    var duplicate = request("POST", "/api/v1/edge-templates/from-cluster", template, 200);
    assertFalse(yes(duplicate, "created"));
    var ts = request("POST", "/api/v1/edge-template-sets", obj("name", "templates"), 201);
    assertEquals(
        1,
        list(request("POST", "/api/v1/edge-template-sets/" + num(ts, "id") + "/activate", null, 200)
                .get("items"))
            .size());
    var classifier =
        obj(
            "name",
            "classifier",
            "endpoint",
            "http://localhost:18080/predict",
            "model",
            "onnx",
            "timeout_ms",
            100,
            "threshold",
            .8);
    var c = request("POST", "/api/v1/edge-classifiers", classifier, 201);
    request("POST", "/api/v1/edge-classifiers/" + num(c, "id") + "/activate", null, 200);
    request("DELETE", "/api/v1/edge-classifiers/" + num(c, "id"), null, 409);
    assertNotNull(repo.edgeConfig().get("classifier"));
  }

  @Test
  void concurrentActivationsHaveUniqueVersionsAndSingleActive() throws Exception {
    var a = repo.create("edge-providers", provider("a"));
    var b = repo.create("edge-providers", provider("b"));
    var gate = new CountDownLatch(1);
    try (var pool = Executors.newVirtualThreadPerTaskExecutor()) {
      var f1 =
          pool.submit(
              () -> {
                gate.await();
                return repo.activate("edge-providers", num(a, "id"));
              });
      var f2 =
          pool.submit(
              () -> {
                gate.await();
                return repo.activate("edge-providers", num(b, "id"));
              });
      gate.countDown();
      assertNotEquals(
          num(f1.get(10, TimeUnit.SECONDS), "config_version"),
          num(f2.get(10, TimeUnit.SECONDS), "config_version"));
    }
    assertEquals(1, repo.count("SELECT COUNT(*) FROM edge_providers WHERE is_active=true"));
  }

  @Test
  void routerMissingTablesDegradeAndMetricHistoryPersists() throws Exception {
    var list = request("GET", "/api/v1/router/decisions", null, 200);
    assertEquals(0, num(list, "total"));
    assertEquals(List.of(), list.get("items"));
    request("GET", "/api/v1/router/decisions/missing", null, 404);
    assertFalse(yes(request("GET", "/api/v1/router/state", null, 200), "available"));
    var metric = metrics.competition();
    router.saveSnapshot(Json.write(metric), str(metric, "window"), Instant.now());
    assertEquals(1, num(request("GET", "/api/v1/competition-metrics/history", null, 200), "total"));
  }

  @Test
  void redisDashboardAndSsePreservePayloadContract() throws Exception {
    var event =
        obj(
            "event_id",
            "e1",
            "equipment",
            "dev1",
            "edge_pred",
            "NET_ERROR",
            "template_id",
            "abc",
            "template_text",
            "timeout",
            "inference_path",
            "RULE",
            "latency_ms",
            42,
            "event_ts",
            Instant.now().minusMillis(50).toString(),
            "inferred_at",
            Instant.now().toString(),
            "severity",
            80,
            "confidence",
            .9);
    events.redis().xadd(events.inferred, Map.of("payload", Json.write(event)));
    var list = request("GET", "/api/v1/events", null, 200);
    assertEquals(1, num(list, "total"));
    assertEquals(42, num(request("GET", "/api/v1/dashboard/summary", null, 200), "p95_latency_ms"));
    var buffer = new ByteArrayOutputStream();
    var output =
        new OutputStream() {
          @Override
          public void write(int b) {
            buffer.write(b);
          }

          @Override
          public void flush() throws IOException {
            if (buffer.toString(StandardCharsets.UTF_8).contains("data:"))
              throw new IOException("test subscriber closed");
          }
        };
    assertThrows(IOException.class, () -> events.stream(output, false, "0"));
    String frame = buffer.toString(StandardCharsets.UTF_8);
    assertTrue(frame.contains("event: inferred\n"));
    assertTrue(frame.contains("\"event_id\":\"e1\""));
    buffer.reset();
    events
        .redis()
        .xadd(
            events.routed,
            Map.of(
                "payload",
                Json.write(
                    obj("input", obj("event_id", "r1"), "decision", obj("Action", "CLOUD")))));
    assertThrows(IOException.class, () -> events.stream(output, true, "0"));
    assertTrue(buffer.toString(StandardCharsets.UTF_8).contains("event: routed\n"));
    assertTrue(buffer.toString(StandardCharsets.UTF_8).contains("\"Action\":\"CLOUD\""));
  }

  @Test
  void invalidBodyAndRegexAreRejected() throws Exception {
    assertEquals(
        400,
        mvc.perform(post("/api/v1/edge-rules").contentType("application/json").content("{"))
            .andReturn()
            .getResponse()
            .getStatus());
    request(
        "POST",
        "/api/v1/edge-rules",
        obj("name", "invalid", "match_type", "regex", "pattern", "[", "edge_pred", "ABNORMAL"),
        400);
  }

  @Test
  void requestValidationIgnoresUnknownFieldsButRejectsKnownClusterTypeErrors() throws Exception {
    request(
        "POST",
        "/api/v1/edge-rules",
        obj(
            "name",
            "unknown-metadata",
            "match_type",
            "contains",
            "pattern",
            "panic",
            "edge_pred",
            "APP_ERROR",
            "timeout_ms",
            "metadata"),
        201);
    request(
        "POST",
        "/api/v1/edge-templates/from-cluster",
        obj("template_text", "panic", "edge_pred", "APP_ERROR", "max_severity", "90"),
        400);
    request(
        "POST",
        "/api/v1/edge-templates/from-cluster",
        obj("template_text", "panic", "edge_pred", "APP_ERROR", "max_severity", 90.5),
        400);
    request(
        "POST",
        "/api/v1/edge-templates/from-cluster",
        obj("template_text", "panic", "edge_pred", "APP_ERROR", "avg_confidence", "0.9"),
        400);
  }

  @Test
  void allRestFamiliesExecuteThroughHttpWithOriginalShapes() throws Exception {
    for (var path :
        List.of(
            "/devices",
            "/dashboard/summary",
            "/dashboard/timeseries",
            "/dashboard/abnormal-analysis",
            "/competition-metrics",
            "/competition-metrics/history",
            "/events",
            "/router/summary",
            "/router/decisions",
            "/router/outbox",
            "/router/cloud-feedback",
            "/router/state",
            "/edge-providers",
            "/edge-classifiers",
            "/edge-rules",
            "/edge-templates",
            "/cloud-providers",
            "/edge-config")) request("GET", "/api/v1" + path, null, 200);
    var cloud = request("POST", "/api/v1/cloud-providers", cloud("http-cloud"), 201);
    request("GET", "/api/v1/cloud-providers/" + num(cloud, "id"), null, 200);
    request("DELETE", "/api/v1/cloud-providers/" + num(cloud, "id"), null, 204);
    var template =
        obj(
            "template_text",
            "failure <*>",
            "edge_pred",
            "APP_ERROR",
            "severity",
            60,
            "confidence",
            .8);
    var saved = request("POST", "/api/v1/edge-templates", template, 201);
    template.put("template_text", "updated failure <*>");
    var patched = request("PATCH", "/api/v1/edge-templates/" + num(saved, "id"), template, 200);
    assertEquals(hash("updated failure <*>").substring(0, 16), patched.get("template_id"));
    request("DELETE", "/api/v1/edge-templates/" + num(saved, "id"), null, 204);
    var classifier =
        obj(
            "name",
            "http-classifier",
            "endpoint",
            "http://localhost:18080/predict",
            "model",
            "classifier",
            "timeout_ms",
            500,
            "threshold",
            .9);
    var c = request("POST", "/api/v1/edge-classifiers", classifier, 201);
    classifier.put("status", "disabled");
    request("PATCH", "/api/v1/edge-classifiers/" + num(c, "id"), classifier, 200);
    request("DELETE", "/api/v1/edge-classifiers/" + num(c, "id"), null, 204);
    var disabled =
        mvc.perform(get("/api/v1/cloud-config").header("Authorization", "Bearer secret"))
            .andReturn()
            .getResponse();
    assertEquals(200, disabled.getStatus());
    assertFalse(yes(Json.map(disabled.getContentAsString()), "enabled"));
  }

  @Test
  void sseHttpMappingsPassCursorAndSetNoBufferHeaders() throws Exception {
    var streamStore = mock(EventStore.class);
    doAnswer(
        call -> {
          var out = (OutputStream) call.getArgument(0);
          out.write("event: test\ndata: {}\n\n".getBytes(StandardCharsets.UTF_8));
          return null;
        })
        .when(streamStore)
        .stream(any(), anyBoolean(), anyString());
    var streamingMvc =
        MockMvcBuilders.standaloneSetup(
                new ManagementController(
                    repo, router, streamStore, metrics, mock(DeviceService.class), ""))
            .setControllerAdvice(new ApiErrors())
            .build();
    for (var path : List.of("/api/v1/events/stream", "/api/v1/router/stream")) {
      var pending = streamingMvc.perform(get(path).param("last_id", "1-2")).andReturn();
      assertTrue(pending.getRequest().isAsyncStarted());
      var response = streamingMvc.perform(asyncDispatch(pending)).andReturn().getResponse();
      assertEquals(200, response.getStatus());
      assertTrue(response.getContentType().startsWith("text/event-stream"));
      assertEquals("no", response.getHeader("X-Accel-Buffering"));
      assertEquals("no-cache", response.getHeader("Cache-Control"));
    }
    verify(streamStore).stream(any(), eq(false), eq("1-2"));
    verify(streamStore).stream(any(), eq(true), eq("1-2"));
  }

  @Test
  void realRedisIdleSseProducesHeartbeat() throws Exception {
    var buffer = new ByteArrayOutputStream();
    var output =
        new OutputStream() {
          @Override
          public void write(int b) {
            buffer.write(b);
          }

          @Override
          public void flush() throws IOException {
            if (buffer.toString(StandardCharsets.UTF_8).contains(": keepalive\n\n"))
              throw new IOException("test subscriber closed");
          }
        };
    long started = System.nanoTime();
    assertThrows(IOException.class, () -> events.stream(output, false, "0"));
    assertTrue(Duration.ofNanos(System.nanoTime() - started).toSeconds() >= 14);
    assertTrue(buffer.toString(StandardCharsets.UTF_8).contains(": keepalive\n\n"));
  }

  @Test
  void schemaAddsMissingColumnsAndNamedIndexesWithoutDeletingRows() {
    var p = repo.create("cloud-providers", cloud("schema-preserved"));
    jdbc.execute("ALTER TABLE cloud_providers DROP COLUMN health_path");
    jdbc.execute("ALTER TABLE cloud_providers DROP INDEX idx_cloud_providers_status");
    repo.initialize();
    assertEquals("schema-preserved", repo.get("cloud-providers", num(p, "id")).get("name"));
    assertTrue(repo.get("cloud-providers", num(p, "id")).containsKey("health_path"));
    assertEquals(
        1,
        repo.count(
            "SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND"
                + " table_name='cloud_providers' AND index_name='idx_cloud_providers_status'"));
  }

  @Test
  void routerHttpViewsReadOriginalTablesAndDoNotExposeInternalFeedbackColumns() throws Exception {
    io.cotriage.nova.common.Schema.ensure(
        jdbc, new org.springframework.core.io.ClassPathResource("router-fixture.sql"));
    jdbc.update(
        "INSERT INTO"
            + " router_decisions(event_id,action,score,threshold,latency_ms,routed_at,source,equipment)"
            + " VALUES ('r1','LOCAL',0.2,0.5,30,UTC_TIMESTAMP(3),'source1','machine1')");
    jdbc.update(
        "INSERT INTO"
            + " cloud_outbox(event_id,request_id,idempotency_key,payload_json,status,priority_score,created_at)"
            + " VALUES ('r2','req2','idem2','{}','RETRY',0.8,UTC_TIMESTAMP(3))");
    jdbc.update(
        "INSERT INTO"
            + " cloud_feedback(event_id,edge_pred,cloud_pred,cloud_latency_ms,token_usage,processed_by_router,idempotency_key,feedback_json)"
            + " VALUES ('r1','NORMAL','APP_ERROR',150,20,true,'internal','{}')");
    jdbc.update(
        "INSERT INTO"
            + " router_state_snapshots(snapshot_type,snapshot_key,payload_json,is_active,created_at)"
            + " VALUES ('router_state','default','{\"theta\":0.5}',true,UTC_TIMESTAMP(3))");
    var decision = request("GET", "/api/v1/router/decisions/r1", null, 200);
    assertEquals("LOCAL", decision.get("action"));
    assertEquals("", decision.get("trace_id"));
    assertEquals(0, dec(decision, "confidence"));
    assertNull(decision.get("event_ts"));
    assertEquals(
        1,
        num(
            request("GET", "/api/v1/router/decisions?action=LOCAL&source=source1", null, 200),
            "total"));
    assertEquals(
        0, num(request("GET", "/api/v1/router/decisions?action=CLOUD", null, 200), "total"));
    assertEquals(1, num(request("GET", "/api/v1/router/outbox?status=RETRY", null, 200), "total"));
    var feedback =
        (Map<String, Object>)
            list(request("GET", "/api/v1/router/cloud-feedback", null, 200).get("items"))
                .getFirst();
    assertFalse(feedback.containsKey("feedback_json"));
    assertFalse(feedback.containsKey("idempotency_key"));
    assertEquals(false, feedback.get("corrected_edge"));
    var summary =
        (Map<String, Object>) request("GET", "/api/v1/router/summary", null, 200).get("summary");
    assertEquals(1, num(summary, "local_total"));
    assertEquals(30, num(summary, "p95_route_latency_ms"));
    assertEquals(1, num(summary, "outbox_retry"));
    assertTrue(
        yes(request("GET", "/api/v1/router/state?type=router_state", null, 200), "available"));
    var competition = request("GET", "/api/v1/competition-metrics", null, 200);
    assertEquals(1, num((Map<String, Object>) competition.get("consistency"), "conflict_count"));
    assertEquals(
        1, dec((Map<String, Object>) competition.get("consistency"), "resolution_success_rate"));
  }
}
