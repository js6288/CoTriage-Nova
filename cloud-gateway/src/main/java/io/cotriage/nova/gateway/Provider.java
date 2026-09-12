package io.cotriage.nova.gateway;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import io.cotriage.nova.common.Json;
import java.io.*;
import java.net.*;
import java.net.http.*;
import java.time.*;
import java.util.*;
import java.util.concurrent.*;

public final class Provider {
  private Provider() {}

  @JsonIgnoreProperties(ignoreUnknown = true)
  public static class Config {
    public long id;
    public String provider = "",
        endpoint = "",
        model = "",
        api_key_env = "",
        api_key = "",
        response_path = "";
    public int timeout_ms, max_tokens;
    public double temperature, top_p;
    public Long seed;
  }

  @JsonIgnoreProperties(ignoreUnknown = true)
  public static class RemoteConfig {
    public boolean enabled;
    public String reason = "", etag = "", api_key = "";
    public long config_version;
    public Config provider;
  }

  public static class Manager {
    final String url, token;
    final Duration timeout;
    final HttpClient http;
    private Config config;
    private String etag = "";

    public Manager(String url, Duration timeout, String token) {
      this.url = url;
      this.timeout = timeout;
      this.token = token;
      http = HttpClient.newBuilder().followRedirects(HttpClient.Redirect.NORMAL).build();
    }

    public void refresh() throws IOException, InterruptedException {
      HttpRequest.Builder b = HttpRequest.newBuilder(URI.create(url)).GET();
      if (timeout.isPositive()) b.timeout(timeout);
      synchronized (this) {
        if (!etag.isEmpty()) b.header("If-None-Match", etag);
      }
      if (!token.isEmpty()) b.header("Authorization", "Bearer " + token);
      HttpResponse<String> response = http.send(b.build(), HttpResponse.BodyHandlers.ofString());
      if (response.statusCode() == 304) return;
      if (response.statusCode() != 200)
        throw new IOException("cloud config status " + response.statusCode());
      RemoteConfig r = GoWire.read(response.body(), RemoteConfig.class);
      synchronized (this) {
        etag = r.etag;
        if (r.enabled) {
          if (r.provider != null && !r.api_key.isEmpty()) r.provider.api_key = r.api_key;
          config = r.provider;
        } else config = null;
      }
    }

    public synchronized Config get() {
      if (config == null) throw new Failure("PROVIDER_CONFIG_MISSING");
      return GoWire.read(Json.write(config), Config.class);
    }
  }

  public static class Failure extends RuntimeException {
    public Failure(String text) {
      super(text);
    }

    public Failure(String text, Throwable cause) {
      super(text, cause);
    }

    public boolean retryable() {
      return getMessage().contains("LLM_TIMEOUT")
          || getMessage().contains("RATE_LIMIT")
          || getMessage().contains("PROVIDER_5XX");
    }
  }

  public record Result(Events.Diagnosis diagnosis, Events.Usage usage) {}

  public static class ProviderResponse {
    public List<Choice> choices;
    public Events.Usage usage = new Events.Usage();
  }

  public static class Choice {
    public Message message = new Message();
  }

  public static class Message {
    public String content = "";
  }

  public static class Client {
    private static final ScheduledExecutorService BODY_TIMEOUTS =
        Executors.newSingleThreadScheduledExecutor(
            Thread.ofPlatform().daemon().name("provider-body-timeout").factory());
    final HttpClient http;

    public Client() {
      this(HttpClient.newBuilder().followRedirects(HttpClient.Redirect.NORMAL).build());
    }

    public Client(HttpClient http) {
      this.http = http;
    }

    public Result diagnose(Config c, String system, String user) {
      String key = c.api_key.strip();
      if (key.isEmpty()) key = System.getenv().getOrDefault(c.api_key_env, "");
      if (key.isEmpty()) throw new Failure("PROVIDER_CONFIG_MISSING: api key env is empty");
      Map<String, Object> body =
          Json.object(
              "model",
              c.model,
              "messages",
              List.of(
                  Map.of("role", "system", "content", system),
                  Map.of("role", "user", "content", user)),
              "max_tokens",
              c.max_tokens,
              "temperature",
              c.temperature,
              "top_p",
              c.top_p,
              "response_format",
              Map.of("type", "json_object"));
      if (c.seed != null) body.put("seed", c.seed);
      HttpRequest req =
          HttpRequest.newBuilder(URI.create(c.endpoint))
              .timeout(Duration.ofMillis(c.timeout_ms <= 0 ? 20000 : c.timeout_ms))
              .header("Content-Type", "application/json")
              .header("Authorization", "Bearer " + key)
              .POST(HttpRequest.BodyPublishers.ofString(Json.write(body)))
              .build();
      long start = System.nanoTime();
      HttpResponse<InputStream> response;
      try {
        response = http.send(req, HttpResponse.BodyHandlers.ofInputStream());
      } catch (Exception e) {
        if (e instanceof InterruptedException) Thread.currentThread().interrupt();
        throw new Failure("LLM_TIMEOUT: " + e, e);
      }
      long latency = (System.nanoTime() - start) / 1_000_000;
      // Go's request context also bounds body reading; latency_ms measures time to headers.
      InputStream responseBody = response.body();
      long remaining =
          Duration.ofMillis(c.timeout_ms <= 0 ? 20000 : c.timeout_ms).toNanos()
              - (System.nanoTime() - start);
      ScheduledFuture<?> bodyTimeout =
          BODY_TIMEOUTS.schedule(
              () -> {
                try {
                  responseBody.close();
                } catch (IOException ignored) {
                }
              },
              Math.max(remaining, 0),
              TimeUnit.NANOSECONDS);
      ByteArrayOutputStream collected = new ByteArrayOutputStream();
      try (InputStream stream = responseBody) {
        byte[] buffer = new byte[8192];
        while (collected.size() < (2 << 20)) {
          int n = stream.read(buffer, 0, Math.min(buffer.length, (2 << 20) - collected.size()));
          if (n < 0) break;
          collected.write(buffer, 0, n);
        }
      } catch (IOException ignored) {
      } finally {
        bodyTimeout.cancel(false);
      }
      byte[] raw = collected.toByteArray();
      if (response.statusCode() == 429) throw new Failure("RATE_LIMIT");
      if (response.statusCode() >= 500)
        throw new Failure("PROVIDER_5XX: status " + response.statusCode());
      if (response.statusCode() >= 300)
        throw new Failure(
            "PROVIDER_ERROR: status "
                + response.statusCode()
                + ": "
                + new String(raw, java.nio.charset.StandardCharsets.UTF_8));
      ProviderResponse decoded;
      try {
        decoded =
            GoWire.read(
                new String(raw, java.nio.charset.StandardCharsets.UTF_8), ProviderResponse.class);
        if (decoded == null || decoded.choices == null || decoded.choices.isEmpty())
          throw new IllegalArgumentException();
      } catch (Exception e) {
        throw new Failure("JSON_SCHEMA_INVALID: provider response");
      }
      Choice choice = decoded.choices.getFirst();
      Parse parsed = parse(choice == null ? "" : choice.message.content);
      Events.Usage usage = decoded.usage;
      usage.latency_ms = latency;
      usage.cost = 0;
      return new Result(parsed.diagnosis(), usage);
    }
  }

  public record Parse(Events.Diagnosis diagnosis, boolean structured) {}

  static final Set<String> ALLOWED =
      Set.of(
          "NORMAL",
          "NET_ERROR",
          "DISK_ERROR",
          "CPU_HIGH",
          "MEMORY_HIGH",
          "SERVICE_ERROR",
          "CONFIG_ERROR",
          "SECURITY_RISK",
          "UNKNOWN");

  public static Parse parse(String raw) {
    String clean = raw.strip();
    if (clean.startsWith("```json")) clean = clean.substring(7);
    if (clean.endsWith("```")) clean = clean.substring(0, clean.length() - 3);
    clean = clean.strip();
    Events.Diagnosis d = null;
    try {
      d = GoWire.read(clean, Events.Diagnosis.class);
      if (d == null) d = new Events.Diagnosis();
    } catch (Exception ignored) {
    }
    if (d == null) {
      int a = raw.indexOf('{'), b = raw.lastIndexOf('}');
      if (a >= 0 && b > a)
        try {
          d = GoWire.read(raw.substring(a, b + 1), Events.Diagnosis.class);
        } catch (Exception ignored) {
        }
    }
    if (d == null) {
      d = new Events.Diagnosis();
      d.pred = "UNKNOWN";
      d.explanation = DiagnosisService.truncateRunes(raw, 4000);
      d.confidence = .3;
      return new Parse(d, false);
    }
    d.pred = d.pred.toUpperCase(Locale.ROOT);
    if (!ALLOWED.contains(d.pred)) d.pred = "UNKNOWN";
    if (d.confidence <= 0) {
      d.confidence = .4;
      if (!d.root_cause.isEmpty()) d.confidence += .2;
      if (!d.recommendation.isEmpty()) d.confidence += .2;
      if (d.evidence != null && !d.evidence.isEmpty()) d.confidence += .1;
      if (!d.pred.equals("UNKNOWN")) d.confidence += .1;
      if (d.confidence > .95) d.confidence = .95;
    }
    if (d.confidence > 1) d.confidence = 1;
    return new Parse(d, true);
  }
}
