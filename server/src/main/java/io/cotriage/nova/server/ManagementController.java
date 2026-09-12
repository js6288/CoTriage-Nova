package io.cotriage.nova.server;

import static io.cotriage.nova.server.Values.*;

import io.cotriage.nova.common.Json;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.Instant;
import java.util.*;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.*;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.servlet.mvc.method.annotation.StreamingResponseBody;

/** Explicit mappings preserve all 47 management API routes and the browser's named SSE events. */
@RestController
@RequestMapping("/api/v1")
public class ManagementController {
  final ConfigRepository configs;
  final RouterRepository router;
  final EventStore events;
  final MetricsService metrics;
  final DeviceService devices;
  final String configToken;

  @Autowired
  public ManagementController(
      ConfigRepository configs,
      RouterRepository router,
      EventStore events,
      MetricsService metrics,
      DeviceService devices) {
    this(configs, router, events, metrics, devices, env("CLOUD_GATEWAY_CONFIG_TOKEN", ""));
  }

  ManagementController(
      ConfigRepository configs,
      RouterRepository router,
      EventStore events,
      MetricsService metrics,
      DeviceService devices,
      String token) {
    this.configs = configs;
    this.router = router;
    this.events = events;
    this.metrics = metrics;
    this.devices = devices;
    this.configToken = token;
  }

  static ResponseEntity<String> response(int status, Object data) {
    return ResponseEntity.status(status)
        .contentType(MediaType.APPLICATION_JSON)
        .body(Json.write(data));
  }

  static ResponseEntity<String> ok(Object data) {
    return response(200, data);
  }

  static String kind(HttpServletRequest request) {
    String[] segments = request.getRequestURI().split("/");
    return segments[3];
  }

  static long id(String raw) {
    try {
      long value = Long.parseLong(raw.trim());
      if (value > 0) return value;
    } catch (Exception ignored) {
    }
    throw new ApiFailure(400, "invalid id");
  }

  @GetMapping("/devices")
  public ResponseEntity<String> devices(@RequestParam Map<String, String> q) {
    return ok(devices.devices(q));
  }

  @GetMapping("/dashboard/summary")
  public ResponseEntity<String> dashboard() {
    return ok(metrics.dashboard());
  }

  @GetMapping("/dashboard/timeseries")
  public ResponseEntity<String> timeseries() {
    return ok(metrics.timeseries());
  }

  @GetMapping("/dashboard/abnormal-analysis")
  public ResponseEntity<String> abnormal() {
    return ok(metrics.abnormal());
  }

  @GetMapping("/competition-metrics")
  public ResponseEntity<String> competition() {
    return ok(metrics.competition());
  }

  @GetMapping("/competition-metrics/history")
  public ResponseEntity<String> history(@RequestParam Map<String, String> q) {
    return ok(router.history(integer(q, "limit", 120, 1, 1000)));
  }

  @GetMapping("/events")
  public ResponseEntity<String> events(@RequestParam Map<String, String> q) {
    int limit = integer(q, "limit", 50, Integer.MIN_VALUE, Integer.MAX_VALUE);
    if (limit <= 0 || limit > 200) limit = 50;
    var items = events.recent(limit);
    return ok(obj("items", items, "total", items.size()));
  }

  @GetMapping(
      value = {"/events/stream", "/router/stream"},
      produces = MediaType.TEXT_EVENT_STREAM_VALUE)
  public StreamingResponseBody stream(
      HttpServletRequest request,
      HttpServletResponse response,
      @RequestParam(name = "last_id", defaultValue = "$") String lastId) {
    response.setContentType("text/event-stream");
    response.setHeader("Cache-Control", "no-cache");
    response.setHeader("Connection", "keep-alive");
    response.setHeader("X-Accel-Buffering", "no");
    boolean routed = request.getRequestURI().endsWith("/router/stream");
    return output -> {
      response.flushBuffer();
      events.stream(
          new java.io.FilterOutputStream(output) {
            @Override
            public void flush() throws java.io.IOException {
              super.flush();
              response.flushBuffer();
            }
          },
          routed,
          lastId);
    };
  }

  @GetMapping("/router/summary")
  public ResponseEntity<String> routerSummary() {
    return ok(metrics.routerSummary());
  }

  @GetMapping({"/router/decisions", "/router/outbox", "/router/cloud-feedback"})
  public ResponseEntity<String> routerList(
      HttpServletRequest request, @RequestParam Map<String, String> q) {
    String path = request.getRequestURI();
    return ok(router.list(path.substring(path.lastIndexOf('/') + 1), q));
  }

  @GetMapping("/router/decisions/{event_id}")
  public ResponseEntity<String> routerDecision(@PathVariable("event_id") String eventId) {
    if (eventId.isBlank()) throw new ApiFailure(400, "event_id is required");
    return ok(router.decision(eventId.trim()));
  }

  @GetMapping("/router/state")
  public ResponseEntity<String> routerState(
      @RequestParam(name = "type", defaultValue = "") String type) {
    return ok(router.state(type.trim()));
  }

  @GetMapping({
    "/edge-providers",
    "/edge-classifiers",
    "/edge-rules",
    "/edge-templates",
    "/cloud-providers"
  })
  public ResponseEntity<String> configList(
      HttpServletRequest request, @RequestParam Map<String, String> q) {
    return ok(configs.list(kind(request), q));
  }

  @PostMapping({
    "/edge-providers",
    "/edge-classifiers",
    "/edge-rules",
    "/edge-templates",
    "/edge-rule-sets",
    "/edge-template-sets",
    "/cloud-providers"
  })
  public ResponseEntity<String> create(HttpServletRequest request, @RequestBody String text) {
    return response(201, configs.create(kind(request), body(text)));
  }

  @GetMapping({"/edge-providers/{id}", "/cloud-providers/{id}"})
  public ResponseEntity<String> get(HttpServletRequest request, @PathVariable("id") String id) {
    return ok(configs.get(kind(request), id(id)));
  }

  @PatchMapping({
    "/edge-providers/{id}",
    "/edge-classifiers/{id}",
    "/edge-rules/{id}",
    "/edge-templates/{id}",
    "/cloud-providers/{id}"
  })
  public ResponseEntity<String> patch(
      HttpServletRequest request, @PathVariable("id") String id, @RequestBody String text) {
    return ok(configs.patch(kind(request), id(id), body(text)));
  }

  @DeleteMapping({
    "/edge-providers/{id}",
    "/edge-classifiers/{id}",
    "/edge-rules/{id}",
    "/edge-templates/{id}",
    "/cloud-providers/{id}"
  })
  public ResponseEntity<String> delete(HttpServletRequest request, @PathVariable("id") String id) {
    configs.delete(kind(request), id(id));
    return ResponseEntity.noContent().build();
  }

  @PostMapping({
    "/edge-providers/{id}/activate",
    "/edge-classifiers/{id}/activate",
    "/edge-rule-sets/{id}/activate",
    "/edge-template-sets/{id}/activate",
    "/cloud-providers/{id}/activate"
  })
  public ResponseEntity<String> activate(
      HttpServletRequest request, @PathVariable("id") String id) {
    String kind = kind(request);
    var config = configs.activate(kind, id(id));
    var builder = ResponseEntity.ok().contentType(MediaType.APPLICATION_JSON);
    if (kind.endsWith("providers")) builder.eTag(str(config, "etag"));
    if (kind.equals("edge-providers")) builder.header("Cache-Control", "no-store");
    return builder.body(Json.write(config));
  }

  @PostMapping("/edge-templates/from-cluster")
  public ResponseEntity<String> fromCluster(@RequestBody String text) {
    var item = configs.fromCluster(body(text));
    return response(yes(item, "created") ? 201 : 200, item);
  }

  @GetMapping("/edge-config")
  public ResponseEntity<String> edgeConfig(
      @RequestHeader(name = "If-None-Match", defaultValue = "") String etag) {
    var cfg = configs.edgeConfig();
    var tag = str(cfg, "etag");
    if (!etag.trim().isEmpty() && etag.trim().equals(tag))
      return ResponseEntity.status(304).eTag(tag).header("Cache-Control", "no-store").build();
    return ResponseEntity.ok()
        .eTag(tag)
        .header("Cache-Control", "no-store")
        .contentType(MediaType.APPLICATION_JSON)
        .body(Json.write(cfg));
  }

  @GetMapping("/cloud-config")
  public ResponseEntity<String> cloudConfig(
      @RequestHeader(name = "Authorization", defaultValue = "") String authorization,
      @RequestHeader(name = "If-None-Match", defaultValue = "") String etag) {
    if (!configToken.isBlank()) {
      var provided =
          (authorization.startsWith("Bearer ") ? authorization.substring(7) : authorization).trim();
      if (!MessageDigest.isEqual(
          configToken.getBytes(StandardCharsets.UTF_8), provided.getBytes(StandardCharsets.UTF_8)))
        throw new ApiFailure(401, "cloud gateway authorization required");
    }
    var provider = configs.active("cloud-providers");
    var now = Instant.now().toString();
    if (provider == null)
      return ok(
          obj(
              "enabled",
              false,
              "reason",
              "NO_ACTIVE_CLOUD_PROVIDER",
              "prompt",
              obj(
                  "version",
                  "",
                  "raw_log_max_chars",
                  0,
                  "context_window",
                  "",
                  "context_max_events",
                  0,
                  "context_max_tokens",
                  0),
              "reward",
              obj("latency_sla_ms", 0, "token_budget_per_req", 0),
              "generated_at",
              now));
    var tag = str(provider, "etag");
    if (etag.equals(tag)) return ResponseEntity.status(304).eTag(tag).build();
    var config =
        obj(
            "enabled",
            true,
            "config_version",
            provider.get("config_version"),
            "etag",
            tag,
            "provider",
            provider,
            "prompt",
            obj(
                "version",
                "cloud-diag-prompt-v1",
                "raw_log_max_chars",
                2000,
                "context_window",
                "5m",
                "context_max_events",
                20,
                "context_max_tokens",
                3000),
            "reward",
            obj("latency_sla_ms", 5000, "token_budget_per_req", 2000),
            "generated_at",
            now);
    return ResponseEntity.ok()
        .eTag(tag)
        .header("Cache-Control", "no-store")
        .contentType(MediaType.APPLICATION_JSON)
        .body(Json.write(config));
  }
}
