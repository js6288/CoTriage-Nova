package io.cotriage.nova.server;

import static io.cotriage.nova.server.Values.*;

import io.cotriage.nova.common.Env;
import io.cotriage.nova.common.Json;
import java.nio.charset.StandardCharsets;
import java.time.*;
import java.time.temporal.ChronoUnit;
import java.util.*;
import java.util.concurrent.*;

public class MetricsService implements AutoCloseable {
  final EventStore events;
  final RouterRepository router;
  final Clock clock;
  private ScheduledExecutorService collector;

  public MetricsService(EventStore events, RouterRepository router) {
    this(events, router, Clock.systemUTC());
  }

  MetricsService(EventStore events, RouterRepository router, Clock clock) {
    this.events = events;
    this.router = router;
    this.clock = clock;
  }

  public void start() {
    if (!router.available()) return;
    collector =
        Executors.newSingleThreadScheduledExecutor(
            Thread.ofPlatform().daemon(true).name("competition-metrics").factory());
    long interval = Env.duration("COMPETITION_METRICS_INTERVAL", Duration.ofSeconds(30)).toMillis();
    if (interval <= 0) interval = 30_000;
    collector.scheduleWithFixedDelay(
        () -> {
          try {
            var metrics = competition();
            router.saveSnapshot(Json.write(metrics), str(metrics, "window"), clock.instant());
          } catch (Exception e) {
            System.getLogger(getClass().getName())
                .log(
                    System.Logger.Level.WARNING,
                    "competition snapshot unavailable: " + e.getMessage());
          }
        },
        0,
        interval,
        TimeUnit.MILLISECONDS);
  }

  public Map<String, Object> dashboard() {
    var recent = events.recent(500);
    var now = clock.instant();
    var classification = new TreeMap<String, Integer>();
    var paths = new TreeMap<String, Integer>();
    var latencies = new ArrayList<Long>();
    int abnormal = 0, fallback = 0, rate = 0;
    for (var e : recent) {
      classification.merge(str(e, "edge_pred"), 1, Integer::sum);
      paths.merge(str(e, "inference_path"), 1, Integer::sum);
      latencies.add(num(e, "latency_ms"));
      if (!str(e, "edge_pred").equals("NORMAL")) abnormal++;
      if (str(e, "inference_path").equals("FALLBACK")) fallback++;
      var ts = instant(e.get("inferred_at"));
      if (ts != null && Duration.between(ts, now).compareTo(Duration.ofMinutes(1)) <= 0) rate++;
    }
    return obj(
        "raw_total",
        events.length(events.raw),
        "inferred_total",
        events.length(events.inferred),
        "pending",
        events.pending(),
        "dlq_total",
        events.length(events.dlq),
        "ingest_rate_1m",
        events.recentCount(now.minusSeconds(60)),
        "infer_rate_1m",
        rate,
        "p95_latency_ms",
        percentile(latencies, 95),
        "abnormal_rate",
        recent.isEmpty() ? 0 : (double) abnormal / recent.size(),
        "fallback_rate",
        recent.isEmpty() ? 0 : (double) fallback / recent.size(),
        "classification",
        classification,
        "paths",
        paths,
        "generated_at",
        now.toString());
  }

  public Map<String, Object> timeseries() {
    var recent = events.recent(1000);
    var now = clock.instant().truncatedTo(ChronoUnit.MINUTES);
    var points = new LinkedHashMap<Instant, Map<String, Object>>();
    for (int i = -9; i <= 0; i++) {
      var ts = now.plusSeconds(i * 60L);
      points.put(ts, obj("time", ts.toString(), "inferred", 0, "abnormal", 0));
    }
    for (var e : recent) {
      var ts = instant(e.get("inferred_at"));
      if (ts == null) continue;
      var point = points.get(ts.truncatedTo(ChronoUnit.MINUTES));
      if (point == null) continue;
      point.put("inferred", num(point, "inferred") + 1);
      if (!str(e, "edge_pred").equals("NORMAL")) point.put("abnormal", num(point, "abnormal") + 1);
    }
    return obj("items", new ArrayList<>(points.values()));
  }

  public Map<String, Object> abnormal() {
    return analyze(events.recent(2000), clock.instant());
  }

  static class Cluster {
    final Map<String, Object> value;
    final Set<String> equipments = new TreeSet<>();
    final Map<String, Integer> paths = new HashMap<>();
    double confidence;
    Instant first, last;

    Cluster(String key, Map<String, Object> e) {
      value =
          obj(
              "cluster_id",
              key,
              "edge_pred",
              str(e, "edge_pred"),
              "template_id",
              str(e, "template_id"),
              "template_text",
              str(e, "template_text"),
              "sample_raw_log",
              str(e, "raw_log"),
              "count",
              0,
              "max_severity",
              0,
              "advice",
              str(e, "advice"),
              "advice_code",
              str(e, "advice_code"),
              "evidence",
              e.get("evidence"));
    }

    void add(Map<String, Object> e, Instant ts) {
      value.put("count", num(value, "count") + 1);
      if (!str(e, "equipment").isEmpty()) equipments.add(str(e, "equipment"));
      if (!str(e, "inference_path").isEmpty())
        paths.merge(str(e, "inference_path"), 1, Integer::sum);
      value.put("max_severity", Math.max(num(value, "max_severity"), num(e, "severity")));
      confidence += dec(e, "confidence");
      if (str(value, "sample_raw_log").isEmpty() && !str(e, "raw_log").isEmpty())
        value.put("sample_raw_log", e.get("raw_log"));
      for (var key : List.of("advice", "advice_code"))
        if (str(value, key).isEmpty() && !str(e, key).isEmpty()) value.put(key, e.get(key));
      if (list(value.get("evidence")).isEmpty() && !list(e.get("evidence")).isEmpty())
        value.put("evidence", e.get("evidence"));
      if (first == null || ts.isBefore(first)) first = ts;
      if (last == null || ts.isAfter(last)) last = ts;
    }

    Map<String, Object> finish() {
      var sorted =
          paths.keySet().stream()
              .sorted(
                  Comparator.<String>comparingInt(paths::get)
                      .reversed()
                      .thenComparing(Comparator.naturalOrder()))
              .toList();
      double avg = round2(confidence / num(value, "count"));
      value.putAll(
          obj(
              "affected_equipments",
              equipments.size(),
              "equipments",
              new ArrayList<>(equipments),
              "inference_paths",
              sorted,
              "primary_inference_path",
              sorted.isEmpty() ? "" : sorted.getFirst(),
              "avg_confidence",
              avg,
              "first_seen",
              first.toString(),
              "last_seen",
              last.toString(),
              "priority_score",
              round2(
                  num(value, "max_severity") * .5
                      + num(value, "count") * 2
                      + equipments.size() * 8
                      + avg * 20)));
      for (var k : List.of("advice", "advice_code")) if (str(value, k).isEmpty()) value.remove(k);
      if (list(value.get("evidence")).isEmpty()) value.remove("evidence");
      return value;
    }
  }

  static Map<String, Object> analyze(List<Map<String, Object>> events, Instant now) {
    var clusters = new TreeMap<String, Cluster>();
    var types = new TreeMap<String, Integer>();
    int one = 0, five = 0, previous = 0;
    for (var e : events) {
      if (str(e, "edge_pred").equals("NORMAL")) continue;
      var ts = instant(e.get("inferred_at"));
      if (ts == null) continue;
      long age = Math.max(0, Duration.between(ts, now).toMillis());
      if (age > 600_000) continue;
      if (age <= 60_000) one++;
      if (age <= 300_000) five++;
      else previous++;
      types.merge(str(e, "edge_pred"), 1, Integer::sum);
      var key = str(e, "template_id") + ":" + str(e, "edge_pred");
      clusters.computeIfAbsent(key, k -> new Cluster(k, e)).add(e, ts);
    }
    double growth = previous == 0 ? five : (double) five / previous;
    var items = new ArrayList<>(clusters.values().stream().map(Cluster::finish).toList());
    items.sort(
        Comparator.<Map<String, Object>>comparingLong(m -> num(m, "count"))
            .reversed()
            .thenComparing(
                Comparator.<Map<String, Object>>comparingLong(m -> num(m, "max_severity"))
                    .reversed()));
    var priority = new ArrayList<>(items);
    priority.sort(
        Comparator.<Map<String, Object>>comparingDouble(m -> dec(m, "priority_score")).reversed());
    String risk = "LOW";
    if (one >= 50 || growth >= 3 || priority.stream().anyMatch(m -> num(m, "max_severity") >= 90))
      risk = "HIGH";
    else if (one >= 10
        || five >= 30
        || growth >= 1.5
        || priority.stream().anyMatch(m -> num(m, "max_severity") >= 70)) risk = "MEDIUM";
    return obj(
        "risk_level",
        risk,
        "abnormal_1m",
        one,
        "abnormal_5m",
        five,
        "growth_ratio",
        growth,
        "top_types",
        types,
        "top_clusters",
        items.subList(0, Math.min(8, items.size())),
        "priority_queue",
        priority.subList(0, Math.min(5, priority.size())),
        "generated_at",
        now.toString());
  }

  public Map<String, Object> routerSummary() {
    var summary = router.summary();
    long routed = events.length(events.routed);
    if (num(summary, "routed_total") == 0 && routed > 0) summary.put("routed_total", routed);
    long req = events.length(events.cloudReq), resp = events.length(events.cloudResp);
    if (num(summary, "outbox_pending") == 0 && req > resp)
      summary.put("outbox_pending", req - resp);
    return obj("degraded", !router.available(), "summary", summary);
  }

  public Map<String, Object> competition() {
    var recent = events.recent(2000);
    var db = router.competition();
    var processing = new ArrayList<Long>();
    var endToEnd = new ArrayList<Long>();
    var byPath = new TreeMap<String, List<Long>>();
    long bytes = 0, fallback = 0;
    for (var e : recent) {
      long ms = num(e, "latency_ms");
      processing.add(ms);
      byPath.computeIfAbsent(str(e, "inference_path"), k -> new ArrayList<>()).add(ms);
      bytes += str(e, "raw_log").getBytes(StandardCharsets.UTF_8).length;
      if (str(e, "inference_path").equals("FALLBACK")) fallback++;
      var start = instant(e.get("event_ts"));
      var end = instant(e.get("inferred_at"));
      if (start != null && end != null && !end.isBefore(start))
        endToEnd.add(Duration.between(start, end).toMillis());
    }
    var pathStats = new LinkedHashMap<String, Object>();
    byPath.forEach((k, v) -> pathStats.put(k, latency(v)));
    var proc = latency(processing);
    var realtime =
        obj(
            "end_to_end",
            latency(endToEnd),
            "edge_processing",
            proc,
            "by_path",
            pathStats,
            "cloud_average_ms",
            dec(db, "cloud_latency_avg_ms"));
    if (dec(db, "cloud_latency_avg_ms") > 0 && !processing.isEmpty())
      realtime.put(
          "improvement_vs_cloud_pct",
          (dec(db, "cloud_latency_avg_ms") - dec(proc, "avg_ms"))
              / dec(db, "cloud_latency_avg_ms")
              * 100);
    long samples = num(db, "cloud_samples"),
        routed = num(db, "routed_total"),
        local = num(db, "local_total");
    double offload = ratio(local, routed);
    var effect =
        obj(
            "ground_truth_evaluation",
            status("not_available", "运行事件没有人工真值标签；Accuracy/Recall/F1 必须由标注评测集计算", 0),
            "cloud_comparison",
            status("proxy", "以云端诊断作为参照，不等同于真实准确率", samples),
            "edge_cloud_agreement_rate",
            ratio(num(db, "edge_cloud_agree"), samples),
            "edge_cloud_conflict_rate",
            ratio(num(db, "edge_cloud_conflict"), samples),
            "edge_correction_rate",
            ratio(num(db, "corrected_total"), samples),
            "actionable_gain_rate",
            ratio(num(db, "actionable_total"), samples),
            "average_reward",
            dec(db, "reward_average"));
    var efficiency =
        obj(
            "routed_total",
            routed,
            "local_total",
            local,
            "cloud_total",
            num(db, "cloud_total"),
            "cloud_offload_rate",
            offload,
            "estimated_upload_reduction_rate",
            offload,
            "recent_input_bytes",
            bytes,
            "estimated_avoided_upload_bytes",
            (long) (bytes * offload),
            "cloud_token_total",
            num(db, "cloud_token_total"),
            "cloud_cost_total",
            dec(db, "cloud_cost_total"));
    var scalability =
        obj(
            "distinct_sources",
            num(db, "distinct_sources"),
            "distinct_equipment",
            num(db, "distinct_equipment"),
            "observed_inference_paths",
            byPath.size(),
            "scenario_evidence",
            status("partial", "设备/日志源多样性已采集，但两类独立应用场景需单独部署或回放实验", 0));
    var stability =
        obj(
            "completion_rate",
            ratio(events.length(events.inferred), events.length(events.raw)),
            "fallback_rate",
            recent.isEmpty() ? 0 : (double) fallback / recent.size(),
            "dlq_total",
            events.length(events.dlq),
            "outbox_success_rate",
            ratio(num(db, "outbox_success"), num(db, "outbox_total")),
            "outbox_retry",
            num(db, "outbox_retry"),
            "outbox_dlq",
            num(db, "outbox_dlq"),
            "network_experiment",
            status("not_available", "尚未标记网络故障实验区间；普通运行数据不能证明弱网保持率", 0));
    var consistency =
        obj(
            "paired_samples",
            samples,
            "conflict_count",
            num(db, "edge_cloud_conflict"),
            "conflict_rate",
            ratio(num(db, "edge_cloud_conflict"), samples),
            "resolved_count",
            num(db, "conflict_resolved"),
            "resolution_success_rate",
            ratio(num(db, "conflict_resolved"), num(db, "edge_cloud_conflict")),
            "multi_node_experiment",
            status("partial", "当前指标衡量边云决策分歧；多边缘节点同任务冲突需要实验 task_id/node_id", 0));
    return obj(
        "schema_version",
        "competition-metrics-v1",
        "window",
        "recent-2000-events+cumulative-db",
        "collected_at",
        clock.instant().toString(),
        "realtime",
        realtime,
        "effect",
        effect,
        "efficiency",
        efficiency,
        "scalability",
        scalability,
        "stability",
        stability,
        "consistency",
        consistency,
        "evidence",
        obj(
            "mechanisms",
            List.of("规则/缓存/ONNX/边缘LLM级联", "RAISE动态云边路由", "Outbox弱网补偿", "云端反馈闭环"),
            "application_signals",
            List.of("日志异常检测", "多设备日志采集", "结构化根因诊断")));
  }

  static Map<String, Object> status(String status, String reason, long samples) {
    return obj("status", status, "reason", reason, "sample_count", samples);
  }

  @Override
  public void close() {
    if (collector != null) collector.shutdownNow();
  }
}
