package io.cotriage.nova.router;

import io.cotriage.nova.common.Json;
import io.lettuce.core.api.sync.RedisCommands;
import java.net.*;
import java.net.http.*;
import java.time.*;
import java.util.*;

public class RouterRuntime {
  public static class Bucket {
    public double capacity, tokens, refill;
    public String last;

    public Bucket() {}

    public Bucket(double capacity, double refill, Instant now) {
      this.capacity = Math.max(capacity, 0);
      tokens = this.capacity;
      this.refill = Math.max(refill, 0);
      last = now.toString();
    }

    public void refillTo(Instant now) {
      Instant before = last == null ? now : Instant.parse(last);
      if (now.isBefore(before)) return;
      tokens = Math.min(capacity, tokens + Duration.between(before, now).toNanos() / 1e9 * refill);
      last = now.toString();
    }

    public boolean allow(double cost, Instant now) {
      if (cost <= 0) cost = 1;
      refillTo(now);
      if (tokens < cost) return false;
      tokens -= cost;
      return true;
    }
  }

  public static class Buckets {
    public Bucket minute, hour, day, rate;

    public Buckets(double m, double h, double d, long r, Instant now) {
      minute = new Bucket(m, m / 60, now);
      hour = new Bucket(h, h / 3600, now);
      day = new Bucket(d, d / 86400, now);
      rate = new Bucket(r, r / 60.0, now);
    }

    public void refillTo(Instant now) {
      minute.refillTo(now);
      hour.refillTo(now);
      day.refillTo(now);
      rate.refillTo(now);
    }

    public boolean allow(double cost, Instant now) {
      refillTo(now);
      if (minute.tokens < cost || hour.tokens < cost || day.tokens < cost || rate.tokens < cost)
        return false;
      minute.tokens -= cost;
      hour.tokens -= cost;
      day.tokens -= cost;
      rate.tokens -= cost;
      return true;
    }
  }

  public record HealthSample(boolean ok, long rttNanos, String error) {}

  public record Health(String health, double rttMS, double failRate) {}

  public static class HealthProbe {
    final String url;
    final int window;
    final Duration timeout;
    final HttpClient http;
    final List<HealthSample> samples = new ArrayList<>();

    public HealthProbe(String url, Duration timeout, int window) {
      this.url = url;
      this.timeout = RouterConfig.positive(timeout, 2);
      this.window = window <= 0 ? 20 : window;
      http =
          HttpClient.newBuilder()
              .followRedirects(HttpClient.Redirect.NORMAL)
              .connectTimeout(this.timeout)
              .build();
    }

    public HealthSample probe() {
      long start = System.nanoTime();
      HealthSample sample;
      if (url.isEmpty()) sample = new HealthSample(true, 0, "");
      else
        try {
          HttpResponse<Void> response =
              http.send(
                  HttpRequest.newBuilder(URI.create(url)).timeout(timeout).GET().build(),
                  HttpResponse.BodyHandlers.discarding());
          sample =
              new HealthSample(
                  response.statusCode() >= 200 && response.statusCode() < 500,
                  System.nanoTime() - start,
                  "");
        } catch (Exception e) {
          sample = new HealthSample(false, System.nanoTime() - start, e.toString());
          if (e instanceof InterruptedException) Thread.currentThread().interrupt();
        }
      add(sample);
      return sample;
    }

    public void add(HealthSample s) {
      samples.add(s);
      if (samples.size() > window) samples.removeFirst();
    }

    public Health snapshot() {
      if (samples.isEmpty()) return new Health("UP", 0, 0);
      long fail = samples.stream().filter(s -> !s.ok()).count(),
          rtt = samples.stream().mapToLong(HealthSample::rttNanos).sum();
      double rate = (double) fail / samples.size();
      return new Health(
          rate >= .5 ? "DOWN" : "UP", (double) (rtt / 1_000_000) / samples.size(), rate);
    }
  }

  final RouterConfig cfg;
  final RouterStore store;
  final RedisCommands<String, String> redis;
  final HealthProbe health;
  final Buckets buckets;

  public RouterRuntime(RouterConfig cfg, RouterStore store, RedisCommands<String, String> redis) {
    this(
        cfg,
        store,
        redis,
        new HealthProbe(cfg.healthURL, cfg.healthTimeout, cfg.healthWindow),
        new Buckets(cfg.minuteBudget, cfg.hourBudget, cfg.dayBudget, cfg.rateLimit, Instant.now()));
  }

  public RouterRuntime(
      RouterConfig cfg,
      RouterStore store,
      RedisCommands<String, String> redis,
      HealthProbe health,
      Buckets buckets) {
    this.cfg = cfg;
    this.store = store;
    this.redis = redis;
    this.health = health;
    this.buckets = buckets;
  }

  public RaiseEngine.SystemState build(double ratio, double privacy, Instant now) {
    buckets.refillTo(now);
    health.probe();
    Health h = health.snapshot();
    long inferredPending = 0, cloudLength = 0, outbox = 0;
    if (redis != null) {
      try {
        List<Object> groups = redis.xinfoGroups(cfg.inferred);
        for (Object group : groups) {
          Map<String, Object> values = infoMap(group);
          if (cfg.group.equals(String.valueOf(values.get("name"))))
            inferredPending = Long.parseLong(String.valueOf(values.get("pending")));
        }
      } catch (Exception e) {
        log("xinfo groups inferred", e);
      }
      try {
        cloudLength = redis.xlen(cfg.cloud);
      } catch (Exception e) {
        log("xlen cloud req", e);
      }
    }
    if (store != null)
      try {
        outbox = store.pendingCount();
      } catch (Exception e) {
        log("count pending outbox", e);
      }
    RaiseEngine.SystemState s = new RaiseEngine.SystemState();
    s.CloudHealth = h.health();
    s.RTTMS = h.rttMS();
    s.FailRate = h.failRate();
    s.RateLimitRemaining = (long) buckets.rate.tokens;
    s.MinuteBudgetRemaining = buckets.minute.tokens;
    s.HourBudgetRemaining = buckets.hour.tokens;
    s.DayBudgetRemaining = buckets.day.tokens;
    s.CloudQueueLag = cloudLength + inferredPending;
    s.OutboxLag = outbox;
    s.QueueMax = cfg.queueMax;
    s.RecentCloudRatio = ratio;
    s.MinCloudRatioForAudit = cfg.auditRatio;
    s.EstimatedLatencyMS = cfg.estimatedLatency.toMillis();
    s.PrivacyCost = privacy;
    return s;
  }

  @SuppressWarnings("unchecked")
  static Map<String, Object> infoMap(Object entry) {
    if (entry instanceof Map<?, ?> m) return (Map<String, Object>) m;
    Map<String, Object> result = new HashMap<>();
    if (entry instanceof List<?> xs)
      for (int i = 0; i + 1 < xs.size(); i += 2)
        result.put(String.valueOf(xs.get(i)), xs.get(i + 1));
    return result;
  }

  public void snapshot(RaiseEngine.SystemState s, Instant now) {
    store.snapshot(
        "system_state",
        "default",
        Json.write(Json.object("state", s, "buckets", buckets, "created_at", now.toString())),
        "raise-runtime-v1",
        null,
        now);
  }

  static void log(String context, Exception e) {
    System.err.println("[router-controller] " + context + ": " + e.getMessage());
  }
}
