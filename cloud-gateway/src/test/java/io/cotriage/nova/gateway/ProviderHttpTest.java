package io.cotriage.nova.gateway;

import static org.junit.jupiter.api.Assertions.*;

import com.sun.net.httpserver.HttpServer;
import io.cotriage.nova.common.Json;
import java.net.*;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.*;
import java.util.concurrent.atomic.*;
import org.junit.jupiter.api.Test;

class ProviderHttpTest {
  @Test
  void providerRequestAndUsageMatchChatContract() throws Exception {
    HttpServer server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
    AtomicReference<Map<String, Object>> request = new AtomicReference<>();
    AtomicReference<String> auth = new AtomicReference<>();
    server.createContext(
        "/chat",
        e -> {
          request.set(
              Json.map(new String(e.getRequestBody().readAllBytes(), StandardCharsets.UTF_8)));
          auth.set(e.getRequestHeaders().getFirst("Authorization"));
          byte[] body =
              Json.write(
                      Json.object(
                          "choices",
                          List.of(
                              Map.of(
                                  "message",
                                  Map.of("content", "{\"pred\":\"NORMAL\",\"confidence\":0.9}"))),
                          "usage",
                          Map.of("prompt_tokens", 10, "completion_tokens", 5, "total_tokens", 15)))
                  .getBytes(StandardCharsets.UTF_8);
          e.sendResponseHeaders(200, body.length);
          e.getResponseBody().write(body);
          e.close();
        });
    server.start();
    try {
      var c = new Provider.Config();
      c.endpoint = "http://127.0.0.1:" + server.getAddress().getPort() + "/chat";
      c.api_key = " local-token ";
      c.model = "test-model";
      c.max_tokens = 100;
      c.seed = 4L;
      var result = new Provider.Client().diagnose(c, "system", "user");
      assertEquals("Bearer local-token", auth.get());
      assertEquals("test-model", request.get().get("model"));
      assertEquals(Map.of("type", "json_object"), request.get().get("response_format"));
      assertEquals(4, request.get().get("seed"));
      assertEquals(15, result.usage().total_tokens);
      assertEquals("NORMAL", result.diagnosis().pred);
    } finally {
      server.stop(0);
    }
  }

  @Test
  void configEtagAuthorizationDisableAndFailedRefresh() throws Exception {
    HttpServer server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
    AtomicInteger mode = new AtomicInteger();
    AtomicReference<String> tag = new AtomicReference<>();
    server.createContext(
        "/config",
        e -> {
          assertEquals("Bearer config-token", e.getRequestHeaders().getFirst("Authorization"));
          tag.set(e.getRequestHeaders().getFirst("If-None-Match"));
          int m = mode.get();
          if (m == 1 || m == 3) {
            e.sendResponseHeaders(m == 1 ? 304 : 503, -1);
            e.close();
            return;
          }
          String json =
              m == 2
                  ? "{\"enabled\":false,\"etag\":\"v2\"}"
                  : "{\"enabled\":true,\"etag\":\"v1\",\"api_key\":\"top-key\",\"provider\":{\"model\":\"one\",\"api_key\":\"nested-key\"}}";
          byte[] bytes = json.getBytes(StandardCharsets.UTF_8);
          e.sendResponseHeaders(200, bytes.length);
          e.getResponseBody().write(bytes);
          e.close();
        });
    server.start();
    try {
      var m =
          new Provider.Manager(
              "http://127.0.0.1:" + server.getAddress().getPort() + "/config",
              Duration.ofSeconds(1),
              "config-token");
      m.refresh();
      assertEquals("top-key", m.get().api_key);
      mode.set(1);
      m.refresh();
      assertEquals("v1", tag.get());
      assertEquals("one", m.get().model);
      mode.set(3);
      assertThrows(Exception.class, m::refresh);
      assertEquals("one", m.get().model);
      mode.set(2);
      m.refresh();
      assertThrows(Provider.Failure.class, m::get);
    } finally {
      server.stop(0);
    }
  }

  @Test
  void errorClassificationKeepsRetryBoundaries() throws Exception {
    for (int status : new int[] {429, 503, 401}) {
      HttpServer server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
      server.createContext(
          "/",
          e -> {
            e.sendResponseHeaders(status, -1);
            e.close();
          });
      server.start();
      try {
        var c = new Provider.Config();
        c.endpoint = "http://127.0.0.1:" + server.getAddress().getPort();
        c.api_key = "fake";
        var error =
            assertThrows(Provider.Failure.class, () -> new Provider.Client().diagnose(c, "s", "u"));
        assertEquals(status != 401, error.retryable());
      } finally {
        server.stop(0);
      }
    }
  }

  @Test
  void providerTimeoutAlsoBoundsSlowResponseBody() throws Exception {
    HttpServer server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
    server.createContext(
        "/",
        e -> {
          e.sendResponseHeaders(200, 100);
          e.getResponseBody().write('{');
          e.getResponseBody().flush();
          try {
            Thread.sleep(1000);
          } catch (InterruptedException interrupted) {
            Thread.currentThread().interrupt();
          }
          e.close();
        });
    server.start();
    try {
      var c = new Provider.Config();
      c.endpoint = "http://127.0.0.1:" + server.getAddress().getPort();
      c.api_key = "fake";
      c.timeout_ms = 150;
      assertTimeoutPreemptively(
          Duration.ofMillis(800),
          () -> {
            var error =
                assertThrows(
                    Provider.Failure.class, () -> new Provider.Client().diagnose(c, "s", "u"));
            assertEquals("JSON_SCHEMA_INVALID: provider response", error.getMessage());
          });
    } finally {
      server.stop(0);
    }
  }
}
