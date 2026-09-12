package io.cotriage.nova.gateway;

import io.lettuce.core.*;
import io.lettuce.core.api.sync.RedisCommands;
import java.nio.charset.StandardCharsets;
import java.time.*;
import java.util.*;

public class ContextAggregator {
  public static class Summary {
    public long event_count, max_severity, time_span_ms;
    public List<String> top_templates, top_sources, top_preds;
  }

  public static class Context {
    public Events.Inferred center_event;
    public List<Events.Inferred> related_events;
    public Summary aggregate = new Summary();

    public Context(Events.Inferred center) {
      center_event = center;
    }
  }

  record Scored(Events.Inferred event, double score) {}

  final RedisCommands<String, String> redis;
  final String stream;
  final Duration window;
  final int maxEvents, maxTokens;

  public ContextAggregator(
      RedisCommands<String, String> redis,
      String stream,
      Duration window,
      int maxEvents,
      int maxTokens) {
    this.redis = redis;
    this.stream = stream;
    this.window = window;
    this.maxEvents = maxEvents;
    this.maxTokens = maxTokens;
  }

  public Context build(Events.Inferred center) {
    Context out = new Context(center);
    if (redis == null) return out;
    Instant ct = parse(center.event_ts);
    String start = ct == null ? "-" : (ct.minus(window).toEpochMilli() + "-0");
    List<StreamMessage<String, String>> messages;
    try {
      messages =
          redis.xrange(stream, Range.create(start, "+"), Limit.from(Math.max(maxEvents * 20, 100)));
    } catch (Exception e) {
      return out;
    }
    return select(center, messages);
  }

  public Context select(Events.Inferred center, List<StreamMessage<String, String>> messages) {
    Context out = new Context(center);
    Instant ct = parse(center.event_ts);
    List<Scored> candidates = new ArrayList<>();
    for (var m : messages) {
      String raw = m.getBody().getOrDefault("payload", "");
      if (raw.isEmpty()) raw = m.getBody().getOrDefault("payload_json", "");
      Events.Inferred v;
      try {
        v = GoWire.read(raw, Events.Inferred.class);
        if (v == null) v = new Events.Inferred();
      } catch (Exception e) {
        continue;
      }
      if (v.event_id.equals(center.event_id)) continue;
      Instant ts = parse(v.event_ts);
      if (ct != null
          && (ts == null || ts.isBefore(ct.minus(window)) || ts.isAfter(ct.plus(window)))) continue;
      double delta = Math.abs(millisecondsBetween(ts, ct)),
          timeScore = Math.exp(-delta / Math.max(window.toMillis(), 1));
      double score =
          .25 * timeScore
              + .2 * same(v.template_id, center.template_id)
              + .2 * same(v.source, center.source)
              + .15 * same(v.equipment, center.equipment)
              + .1 * same(v.edge_pred, center.edge_pred)
              + .1 * v.severity / 100.0;
      candidates.add(new Scored(v, score));
    }
    candidates.sort(Comparator.comparingDouble(Scored::score).reversed());
    int limit = Math.min(maxEvents, candidates.size()), budget = maxTokens * 4;
    for (var candidate : candidates.subList(0, limit)) {
      if (budget <= 0) break;
      Events.Inferred v = candidate.event();
      byte[] bytes = v.raw_log.getBytes(StandardCharsets.UTF_8);
      if (bytes.length > budget)
        v.raw_log = new String(Arrays.copyOf(bytes, budget), StandardCharsets.UTF_8);
      budget -= Math.min(bytes.length, budget);
      if (out.related_events == null) out.related_events = new ArrayList<>();
      out.related_events.add(v);
    }
    out.aggregate = summarize(out.related_events);
    return out;
  }

  static Instant parse(String s) {
    try {
      return OffsetDateTime.parse(s).toInstant();
    } catch (Exception e) {
      return null;
    }
  }

  static double millisecondsBetween(Instant a, Instant b) {
    if (a == null) a = Instant.parse("0001-01-01T00:00:00Z");
    if (b == null) b = Instant.parse("0001-01-01T00:00:00Z");
    Duration d = Duration.between(b, a);
    try {
      return d.toNanos() / 1_000_000;
    } catch (ArithmeticException e) {
      return d.isNegative() ? Long.MIN_VALUE / 1_000_000 : Long.MAX_VALUE / 1_000_000;
    }
  }

  static double same(String a, String b) {
    return !a.isEmpty() && a.equals(b) ? 1 : 0;
  }

  static Summary summarize(List<Events.Inferred> events) {
    Summary s = new Summary();
    Map<String, Integer> templates = new HashMap<>(),
        sources = new HashMap<>(),
        preds = new HashMap<>();
    Instant low = null, high = null;
    boolean lowZero = true, highZero = true;
    if (events != null)
      for (var e : events) {
        s.event_count++;
        templates.merge(e.template_id, 1, Integer::sum);
        sources.merge(e.source, 1, Integer::sum);
        preds.merge(e.edge_pred, 1, Integer::sum);
        s.max_severity = Math.max(s.max_severity, e.severity);
        Instant t = parse(e.event_ts);
        if (lowZero || t == null || (low != null && t.isBefore(low))) {
          low = t;
          lowZero = t == null;
        }
        if (highZero || (t != null && (high == null || t.isAfter(high)))) {
          high = t;
          highZero = t == null;
        }
      }
    s.top_templates = top(templates);
    s.top_sources = top(sources);
    s.top_preds = top(preds);
    if (!lowZero) s.time_span_ms = (long) millisecondsBetween(high, low);
    return s;
  }

  static List<String> top(Map<String, Integer> counts) {
    return counts.keySet().stream()
        .filter(k -> !k.isEmpty())
        .sorted((a, b) -> Integer.compare(counts.get(b), counts.get(a)))
        .limit(3)
        .toList();
  }
}
