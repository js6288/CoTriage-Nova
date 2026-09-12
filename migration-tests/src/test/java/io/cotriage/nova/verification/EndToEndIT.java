package io.cotriage.nova.verification;

import static org.junit.jupiter.api.Assertions.*;

import com.sun.net.httpserver.HttpServer;
import io.cotriage.nova.common.*;
import io.lettuce.core.*;
import java.net.*;
import java.net.http.*;
import java.nio.charset.StandardCharsets;
import java.nio.file.*;
import java.time.*;
import java.util.*;
import java.util.concurrent.*;
import java.util.concurrent.atomic.*;
import java.util.function.*;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.testcontainers.containers.*;

/**
 * Runs the five packaged JVM services against disposable dependencies and deterministic HTTP
 * fixtures.
 */
public class EndToEndIT {
  private final Path root = projectRoot();
  private final HttpClient http =
      HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(3)).build();
  private final List<Process> processes = new ArrayList<>();
  private final AtomicBoolean health = new AtomicBoolean(true);
  private final AtomicInteger modelCalls = new AtomicInteger();
  private Path logs;
  private int port;
  private Map<String, String> env;
  private ManagementContracts management;

  static Path projectRoot() {
    String value = System.getProperty("maven.multiModuleProjectDirectory");
    if (value == null || value.isBlank())
      throw new IllegalStateException("Failsafe must pass maven.multiModuleProjectDirectory");
    Path root = Path.of(value).toAbsolutePath().normalize();
    if (!Files.isDirectory(root.resolve("cotriage-common")))
      throw new IllegalStateException("Not reactor root: " + root);
    return root;
  }

  @Test
  void managementSseAndAllRoutingPaths() throws Exception {
    logs =
        Files.createDirectories(
            root.resolve(
                "migration-tests/target/e2e/" + Instant.now().toString().replace(':', '-')));
    try (var mysql =
            new MySQLContainer<>("mysql:8.4")
                .withDatabaseName("cotriage")
                .withUsername("nova")
                .withPassword("test-only");
        var redis = new GenericContainer<>("redis:7.4-alpine").withExposedPorts(6379)) {
      mysql.start();
      redis.start();
      HttpServer stub = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
      stub.createContext(
          "/",
          exchange -> {
            String p = exchange.getRequestURI().getPath();
            String body;
            int status = 200;
            if (p.equals("/health")) {
              status = health.get() ? 200 : 503;
              body = "{}";
            } else if (p.equals("/api/v1/nodes"))
              body =
                  Json.write(
                      Json.object(
                          "apiVersion",
                          "v1",
                          "kind",
                          "NodeList",
                          "metadata",
                          Json.object(),
                          "items",
                          List.of(
                              Json.object(
                                  "apiVersion",
                                  "v1",
                                  "kind",
                                  "Node",
                                  "metadata",
                                  Json.object(
                                      "name",
                                      "test-node",
                                      "labels",
                                      Json.object("cotriage.io/equipment", "test-equipment")),
                                  "status",
                                  Json.object(
                                      "conditions",
                                      List.of(Json.object("type", "Ready", "status", "True")))))));
            else if (p.startsWith("/api/v1/nodes/"))
              body =
                  Json.write(
                      Json.object(
                          "apiVersion",
                          "v1",
                          "kind",
                          "Node",
                          "metadata",
                          Json.object(
                              "name",
                              "test-node",
                              "labels",
                              Json.object("cotriage.io/equipment", "test-equipment"))));
            else if (p.contains("/pods"))
              body =
                  Json.write(
                      Json.object(
                          "apiVersion",
                          "v1",
                          "kind",
                          "PodList",
                          "metadata",
                          Json.object(),
                          "items",
                          List.of()));
            else if (p.equals("/predict"))
              body =
                  Json.write(
                      Json.object(
                          "label",
                          "NON_ALERT",
                          "label_id",
                          0,
                          "alert_probability",
                          .01,
                          "threshold",
                          .5,
                          "probabilities",
                          Map.of("NON_ALERT", .99, "ALERT", .01),
                          "model_version",
                          "fixture-v1"));
            else if (p.equals("/chat")) {
              modelCalls.incrementAndGet();
              body =
                  Json.write(
                      Json.object(
                          "choices",
                          List.of(
                              Json.object(
                                  "message",
                                  Json.object(
                                      "content",
                                      Json.write(
                                          Json.object(
                                              "pred",
                                              "NET_ERROR",
                                              "edge_pred",
                                              "NET_ERROR",
                                              "root_cause",
                                              "network unavailable",
                                              "recommendation",
                                              "check network",
                                              "explanation",
                                              "fixture diagnosis",
                                              "confidence",
                                              .9,
                                              "severity",
                                              90,
                                              "evidence",
                                              List.of("connection refused"),
                                              "advice",
                                              "check network",
                                              "advice_code",
                                              "CHECK_NETWORK"))))),
                          "usage",
                          Json.object(
                              "prompt_tokens", 20, "completion_tokens", 15, "total_tokens", 35)));
            } else {
              status = 404;
              body = "{}";
            }
            byte[] bytes = body.getBytes(StandardCharsets.UTF_8);
            exchange.getResponseHeaders().set("Content-Type", "application/json");
            exchange.sendResponseHeaders(status, bytes.length);
            exchange.getResponseBody().write(bytes);
            exchange.close();
          });
      stub.start();
      String stubUrl = "http://127.0.0.1:" + stub.getAddress().getPort();
      Path kube = logs.resolve("kubeconfig.yaml");
      Files.writeString(
          kube,
          "apiVersion: v1\nkind: Config\nclusters:\n- name: test\n  cluster:\n    server: "
              + stubUrl
              + "\n"
              + "contexts:\n"
              + "- name: test\n"
              + "  context:\n"
              + "    cluster: test\n"
              + "    user: test\n"
              + "current-context: test\n"
              + "users:\n"
              + "- name: test\n"
              + "  user:\n"
              + "    token: fixture\n");
      try (var socket = new java.net.ServerSocket(0)) {
        port = socket.getLocalPort();
      }
      String dsn =
          "nova:test-only@tcp("
              + mysql.getHost()
              + ":"
              + mysql.getMappedPort(3306)
              + ")/cotriage?charset=utf8mb4&parseTime=true&loc=UTC";
      env =
          new HashMap<>(
              Map.of(
                  "MYSQL_DSN",
                  dsn,
                  "REDIS_ADDR",
                  redis.getHost() + ":" + redis.getMappedPort(6379),
                  "KUBECONFIG",
                  kube.toString(),
                  "LISTEN_ADDR",
                  ":" + port,
                  "COTRIAGE_NAMESPACE",
                  "cotriage",
                  "CLOUD_GATEWAY_CONFIG_TOKEN",
                  "fixture-config-token",
                  "SERVER_BASE_URL",
                  "http://127.0.0.1:" + port));
      management = new ManagementContracts(http, "http://127.0.0.1:" + port);
      try {
        start("server", Map.of());
        await(
            () -> status("/api/v1/edge-config") == 200,
            Duration.ofSeconds(60),
            "server API readiness");
        assertEquals(200, status("/api/v1/devices"));
        assertEquals(200, status("/api/v1/dashboard/summary"));
        assertEquals(401, status("/api/v1/cloud-config"));
        var cloud =
            call(
                "POST",
                "/api/v1/cloud-providers",
                Json.object(
                    "name",
                    "fixture",
                    "provider",
                    "openai_compat",
                    "endpoint",
                    stubUrl + "/chat",
                    "model",
                    "fixture",
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
                    1));
        assertEquals(201, cloud.statusCode(), cloud.body());
        long provider = ((Number) Json.map(cloud.body()).get("id")).longValue();
        assertEquals(
            200,
            call("POST", "/api/v1/cloud-providers/" + provider + "/activate", Json.object())
                .statusCode());
        var config =
            http.send(
                HttpRequest.newBuilder(
                        URI.create("http://127.0.0.1:" + port + "/api/v1/cloud-config"))
                    .header("Authorization", "Bearer fixture-config-token")
                    .GET()
                    .build(),
                HttpResponse.BodyHandlers.ofString());
        assertEquals(200, config.statusCode(), config.body());
        String tag = config.headers().firstValue("ETag").orElseThrow();
        var unchanged =
            http.send(
                HttpRequest.newBuilder(
                        URI.create("http://127.0.0.1:" + port + "/api/v1/cloud-config"))
                    .header("Authorization", "Bearer fixture-config-token")
                    .header("If-None-Match", tag)
                    .GET()
                    .build(),
                HttpResponse.BodyHandlers.ofString());
        assertEquals(304, unchanged.statusCode());
        assertEquals(409, call("DELETE", "/api/v1/cloud-providers/" + provider, null).statusCode());
        long ruleVersion = management.exercise(stubUrl, provider);

        var routerEnv = new HashMap<String, String>();
        routerEnv.put("CLOUD_HEALTH_URL", stubUrl + "/health");
        routerEnv.put("CLOUD_HEALTH_WINDOW", "1");
        routerEnv.put("RAISE_WEIGHT_RISK", "1");
        for (String k :
            List.of("UNCERTAINTY", "NOVELTY", "DISAGREEMENT", "FLEET_IMPACT", "PATH_RISK"))
          routerEnv.put("RAISE_WEIGHT_" + k, "0");
        routerEnv.put("RAISE_SCORE_VALUE", "1");
        routerEnv.put("RAISE_SCORE_GAIN", "0");
        routerEnv.put("RAISE_SCORE_COST", "0");
        routerEnv.put("RAISE_THRESHOLD_BASE", "0.5");
        for (String k : List.of("QUEUE", "BUDGET", "NETWORK", "AUDIT"))
          routerEnv.put("RAISE_THRESHOLD_" + k, "0");
        routerEnv.put("OUTBOX_REPLAYER_INTERVAL", "1s");
        routerEnv.put("FEEDBACK_PROCESSOR_INTERVAL", "200ms");
        routerEnv.put("STATE_SNAPSHOT_INTERVAL", "1s");
        start("router-controller", routerEnv);
        try (var ds = Db.open(dsn)) {
          var jdbc = new JdbcTemplate(ds);
          await(
              () -> {
                try {
                  return jdbc.queryForObject("SELECT COUNT(*) FROM router_decisions", Long.class)
                      != null;
                } catch (Exception e) {
                  return false;
                }
              },
              Duration.ofSeconds(30),
              "router schema");
          start("cloud-gateway", Map.of("CLOUD_CONFIG_POLL_INTERVAL", "200ms"));
          start(
              "edge-infer",
              Map.of(
                  "CLASSIFIER_URL",
                  stubUrl + "/predict",
                  "LLM_URL",
                  stubUrl + "/chat",
                  "EDGE_CONFIG_URL",
                  "http://127.0.0.1:" + port + "/api/v1/edge-config",
                  "EDGE_CONFIG_REFRESH_INTERVAL",
                  "200ms"));
          Path log = logs.resolve("current.log");
          Files.writeString(log, "old ignored log\n");
          start("log-collector", Map.of("LOG_PATH", log.toString(), "NODE_NAME", "test-node"));
          var redisClient = RedisSupport.client(env.get("REDIS_ADDR"));
          try (var connection = redisClient.connect()) {
            var commands = connection.sync();
            await(
                () -> {
                  try {
                    return commands
                        .xinfoGroups("cotriage.events.raw")
                        .toString()
                        .contains("edge-infer-g");
                  } catch (Exception e) {
                    return false;
                  }
                },
                Duration.ofSeconds(30),
                "edge consumer");
            await(
                () -> readLog("log-collector").contains("start LOG_PATH"),
                Duration.ofSeconds(30),
                "collector startup");
            await(
                () -> readLog("edge-infer").contains("edge config applied"),
                Duration.ofSeconds(30),
                "edge remote config applied");
            // Collector seeks EOF immediately after its start log.
            Thread.sleep(300);
            var inferredSse =
                http.sendAsync(
                    HttpRequest.newBuilder(
                            URI.create(
                                "http://127.0.0.1:" + port + "/api/v1/events/stream?last_id=0-0"))
                        .GET()
                        .build(),
                    HttpResponse.BodyHandlers.ofInputStream());
            var routedSse =
                http.sendAsync(
                    HttpRequest.newBuilder(
                            URI.create(
                                "http://127.0.0.1:" + port + "/api/v1/router/stream?last_id=0-0"))
                        .GET()
                        .build(),
                    HttpResponse.BodyHandlers.ofInputStream());
            Files.writeString(log, "normal verification event\n", StandardOpenOption.APPEND);
            await(
                () ->
                    count(jdbc, "SELECT COUNT(*) FROM router_decisions WHERE action='LOCAL'") >= 1,
                Duration.ofSeconds(30),
                "raw to LOCAL");
            Files.writeString(
                log,
                "connection refused mysql 3306 verification cloud\n",
                StandardOpenOption.APPEND);
            await(
                () ->
                    count(jdbc, "SELECT COUNT(*) FROM router_decisions WHERE action='CLOUD'") >= 1,
                Duration.ofSeconds(30),
                "raw to CLOUD");
            await(
                () -> count(jdbc, "SELECT COUNT(*) FROM cloud_feedback") >= 1,
                Duration.ofSeconds(30),
                "cloud feedback");
            health.set(false);
            Files.writeString(
                log,
                "connection refused mysql 3306 verification deferred\n",
                StandardOpenOption.APPEND);
            await(
                () ->
                    count(jdbc, "SELECT COUNT(*) FROM router_decisions WHERE action='DEFER'") >= 1,
                Duration.ofSeconds(30),
                "raw to DEFER");
            health.set(true);
            await(
                () ->
                    count(jdbc, "SELECT COUNT(*) FROM cloud_outbox WHERE status IN ('SENT','DONE')")
                        >= 1,
                Duration.ofSeconds(30),
                "outbox replay");
            await(
                () -> count(jdbc, "SELECT COUNT(*) FROM cloud_feedback") >= 2,
                Duration.ofSeconds(30),
                "replayed feedback");
            await(
                () ->
                    count(jdbc, "SELECT COUNT(*) FROM cloud_feedback WHERE processed_by_router=1")
                        >= 2,
                Duration.ofSeconds(30),
                "feedback processor");
            assertTrue(count(jdbc, "SELECT COUNT(*) FROM router_state_snapshots") > 0);
            assertTrue(modelCalls.get() >= 2);
            assertEquals(
                2L,
                count(jdbc, "SELECT COUNT(*) FROM cloud_feedback WHERE cloud_pred='NET_ERROR'"));
            assertEquals(0L, commands.xlen("cotriage.events.dlq"));
            assertSse(sseResponse(inferredSse), "inferred");
            assertSse(sseResponse(routedSse), "routed");
            for (String path :
                List.of(
                    "/events",
                    "/router/summary",
                    "/router/decisions",
                    "/router/outbox",
                    "/router/cloud-feedback",
                    "/router/state",
                    "/dashboard/timeseries",
                    "/dashboard/abnormal-analysis",
                    "/competition-metrics",
                    "/competition-metrics/history"))
              assertEquals(200, status("/api/v1" + path), path);
            String cloudId =
                jdbc.queryForObject(
                    "SELECT event_id FROM router_decisions WHERE action='CLOUD' LIMIT 1",
                    String.class);
            var detail = call("GET", "/api/v1/router/decisions/" + cloudId, null);
            assertEquals(200, detail.statusCode(), detail.body());
            assertEquals("CLOUD", Json.map(detail.body()).get("action"));
            var inferred = commands.xrange("cotriage.events.inferred", Range.unbounded());
            assertTrue(
                inferred.stream()
                    .map(m -> Json.map(m.getBody().get("payload")))
                    .anyMatch(
                        p ->
                            "MODEL".equals(p.get("inference_path"))
                                && "NORMAL".equals(p.get("edge_pred"))));
            assertTrue(
                inferred.stream()
                    .map(m -> Json.map(m.getBody().get("payload")))
                    .anyMatch(
                        p ->
                            ("edge-rules-" + ruleVersion).equals(p.get("rule_version"))
                                && "RULE".equals(p.get("inference_path"))));
            assertTrue(
                inferred.stream()
                    .map(m -> Json.map(m.getBody().get("payload")))
                    .allMatch(p -> "test-equipment".equals(p.get("equipment"))));
            assertFalse(
                commands.xrange("cotriage.events.raw", Range.unbounded()).stream()
                    .anyMatch(m -> m.getBody().get("payload").contains("old ignored log")));
            management.assertCoverage();
            String result =
                Json.write(
                    Json.object(
                        "routes",
                        jdbc.queryForList(
                            "SELECT action,COUNT(*) AS count FROM router_decisions GROUP BY"
                                + " action"),
                        "feedback",
                        jdbc.queryForList(
                            "SELECT event_id,cloud_pred,processed_by_router FROM cloud_feedback"),
                        "model_calls",
                        modelCalls.get(),
                        "sse",
                        List.of("inferred", "routed"),
                        "http_route_count",
                        management.routes.size(),
                        "http_routes",
                        management.routes,
                        "service_pids",
                        processes.stream().map(Process::pid).toList(),
                        "logs",
                        logs.toString()));
            Files.writeString(logs.resolve("result.json"), result);
            Files.writeString(
                root.resolve("migration-tests/target/e2e/latest-result.json"), result);
          } finally {
            redisClient.shutdown();
          }
        }
      } finally {
        for (Process p : processes.reversed()) {
          p.destroy();
          if (!p.waitFor(10, TimeUnit.SECONDS)) p.destroyForcibly();
        }
        stub.stop(0);
      }
    }
  }

  private long count(JdbcTemplate jdbc, String sql) {
    try {
      return jdbc.queryForObject(sql, Long.class);
    } catch (Exception e) {
      return -1;
    }
  }

  private void start(String module, Map<String, String> extra) throws Exception {
    Path jar =
        root.resolve(
            module
                + "/target/"
                + module
                + "-"
                + System.getProperty("cotriage.artifact.version", "0.1.0-SNAPSHOT")
                + ".jar");
    assertTrue(Files.isRegularFile(jar), "missing executable jar: " + jar);
    String java =
        Path.of(
                System.getProperty("java.home"),
                "bin",
                System.getProperty("os.name").startsWith("Windows") ? "java.exe" : "java")
            .toString();
    var builder =
        new ProcessBuilder(java, "-Xms64m", "-Xmx256m", "-jar", jar.toString())
            .directory(root.toFile())
            .redirectErrorStream(true)
            .redirectOutput(logs.resolve(module + ".log").toFile());
    builder
        .environment()
        .keySet()
        .removeIf(
            k ->
                k.matches(
                        "(?:RAISE|CLOUD|EDGE|REDIS|MYSQL|CONSUMER|RULES|TEMPLATES|LOG|OUTBOX|FEEDBACK|STATE|WORKER)_.*")
                    || Set.of("NODE_NAME", "EQUIPMENT", "SERVER_BASE_URL", "AUTO_MIGRATE")
                        .contains(k));
    builder.environment().putAll(env);
    builder.environment().putAll(extra);
    processes.add(builder.start());
  }

  private String readLog(String module) {
    try {
      return Files.readString(logs.resolve(module + ".log"));
    } catch (Exception e) {
      return "";
    }
  }

  private HttpResponse<java.io.InputStream> sseResponse(
      CompletableFuture<HttpResponse<java.io.InputStream>> future) throws Exception {
    try {
      return future.get(20, TimeUnit.SECONDS);
    } catch (Exception failure) {
      try (var socket = new java.net.Socket("127.0.0.1", port)) {
        socket.setSoTimeout(3000);
        socket
            .getOutputStream()
            .write(
                ("GET /api/v1/events/stream?last_id=0-0 HTTP/1.1\r\nHost: 127.0.0.1:"
                        + port
                        + "\r\nAccept: text/event-stream\r\nConnection: close\r\n\r\n")
                    .getBytes(StandardCharsets.US_ASCII));
        socket.getOutputStream().flush();
        byte[] bytes = new byte[8192];
        int n = socket.getInputStream().read(bytes);
        Files.writeString(
            logs.resolve("sse-wire.txt"),
            new String(bytes, 0, Math.max(0, n), StandardCharsets.UTF_8));
      } catch (Exception e) {
        Files.writeString(logs.resolve("sse-wire-error.txt"), e.toString());
      }
      try {
        String exe =
            Path.of(
                    System.getProperty("java.home"),
                    "bin",
                    System.getProperty("os.name").startsWith("Windows") ? "jcmd.exe" : "jcmd")
                .toString();
        new ProcessBuilder(
                exe,
                Long.toString(processes.getFirst().pid()),
                "Thread.dump_to_file",
                logs.resolve("server-threads.json").toString())
            .redirectErrorStream(true)
            .redirectOutput(logs.resolve("jcmd.log").toFile())
            .start()
            .waitFor(5, TimeUnit.SECONDS);
      } catch (Exception ignored) {
      }
      throw failure;
    }
  }

  private HttpResponse<String> call(String method, String path, Object body) throws Exception {
    var b =
        HttpRequest.newBuilder(URI.create("http://127.0.0.1:" + port + path))
            .timeout(Duration.ofSeconds(8))
            .header("Content-Type", "application/json");
    var response =
        http.send(
            b.method(
                    method,
                    body == null
                        ? HttpRequest.BodyPublishers.noBody()
                        : HttpRequest.BodyPublishers.ofString(Json.write(body)))
                .build(),
            HttpResponse.BodyHandlers.ofString());
    management.record(method, path, response.statusCode());
    return response;
  }

  private int status(String path) {
    try {
      return call("GET", path, null).statusCode();
    } catch (Exception e) {
      return -1;
    }
  }

  private void await(BooleanSupplier condition, Duration duration, String message)
      throws Exception {
    long end = System.nanoTime() + duration.toNanos();
    while (System.nanoTime() < end) {
      if (condition.getAsBoolean()) return;
      for (Process p : processes)
        assertTrue(p.isAlive(), message + ": service process exited; logs in " + logs);
      Thread.sleep(100);
    }
    StringBuilder diagnostics = new StringBuilder();
    for (String module :
        List.of("server", "router-controller", "cloud-gateway", "edge-infer", "log-collector")) {
      String log = readLog(module);
      diagnostics
          .append("\n--- ")
          .append(module)
          .append(" ---\n")
          .append(log.substring(Math.max(0, log.length() - 5000)));
    }
    fail(message + " timed out; logs in " + logs + diagnostics);
  }

  private void assertSse(HttpResponse<java.io.InputStream> response, String name) throws Exception {
    assertEquals(200, response.statusCode());
    management.record(
        "GET",
        name.equals("inferred") ? "/api/v1/events/stream" : "/api/v1/router/stream",
        response.statusCode());
    assertTrue(
        response.headers().firstValue("Content-Type").orElse("").startsWith("text/event-stream"));
    try (var stream = response.body();
        var executor = Executors.newVirtualThreadPerTaskExecutor()) {
      var frame =
          executor.submit(
              () -> {
                var reader =
                    new java.io.BufferedReader(
                        new java.io.InputStreamReader(stream, StandardCharsets.UTF_8));
                String event = "", id = "", data = "", line;
                while ((line = reader.readLine()) != null) {
                  if (line.isEmpty()) {
                    if (event.equals(name) && !data.isEmpty())
                      return Map.of("id", id, "data", data);
                    event = "";
                    id = "";
                    data = "";
                  } else if (line.startsWith("event:")) event = line.substring(6).trim();
                  else if (line.startsWith("id:")) id = line.substring(3).trim();
                  else if (line.startsWith("data:")) data = line.substring(5).trim();
                }
                throw new java.io.IOException("SSE closed without " + name);
              });
      try {
        var result = frame.get(10, TimeUnit.SECONDS);
        assertFalse(result.get("id").isBlank());
        var data = Json.map(result.get("data"));
        assertNotNull(data.get(name.equals("inferred") ? "event_id" : "decision"));
      } finally {
        stream.close();
        frame.cancel(true);
      }
    }
  }
}
