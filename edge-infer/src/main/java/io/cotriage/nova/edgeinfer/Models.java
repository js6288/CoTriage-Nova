package io.cotriage.nova.edgeinfer;

import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Set;

final class Models {
  static final Set<String> PREDICTIONS =
      Set.of(
          "NORMAL",
          "APP_ERROR",
          "NET_ERROR",
          "DISK_ERROR",
          "INFRA_ERROR",
          "SECURITY",
          "ABNORMAL",
          "UNKNOWN");
  static final Set<String> LLM_PREDICTIONS =
      Set.of("APP_ERROR", "NET_ERROR", "DISK_ERROR", "INFRA_ERROR", "SECURITY", "UNKNOWN");
  static final String DEFAULT_RULE_VERSION = "edge-rules-v1";

  private Models() {}

  public static class RawEvent {
    public String eventId = "", ts = "", equipment = "", source = "";
    public RawLog log = new RawLog();
  }

  public static class RawLog {
    public String raw = "";
  }

  public static class Rule {
    public String id = "", pattern = "", edgePred = "", adviceCode = "", advice = "";
    public int severity, priority;
    public double confidence;
    public List<String> evidence;
    public boolean enabled;
  }

  public static class TemplateResult {
    public String templateId = "", templateText = "", edgePred = "", adviceCode = "", advice = "";
    public int severity;
    public double confidence;
    public List<String> evidence;

    Map<String, Object> toMap() {
      var result =
          EdgeJson.object("edge_pred", edgePred, "severity", severity, "confidence", confidence);
      EdgeJson.optional(result, "template_id", templateId);
      EdgeJson.optional(result, "template_text", templateText);
      if (evidence != null && !evidence.isEmpty()) result.put("evidence", evidence);
      EdgeJson.optional(result, "advice_code", adviceCode);
      EdgeJson.optional(result, "advice", advice);
      return result;
    }
  }

  public static class ClassifierInput {
    public String eventId = "", templateId = "", templateText = "";

    ClassifierInput(String eventId, String templateId, String templateText) {
      this.eventId = eventId;
      this.templateId = templateId;
      this.templateText = templateText;
    }

    Map<String, Object> toMap() {
      var result = EdgeJson.object("template_id", templateId, "template_text", templateText);
      EdgeJson.optional(result, "event_id", eventId);
      return result;
    }
  }

  public static class ClassifierResult {
    public String label = "", modelVersion = "";
    public int labelId;
    public double alertProbability, threshold;
    public Map<String, Double> probabilities;
  }

  public static class LlmInput {
    public String eventId, equipment, rawLog, templateId, templateText;
    public ClassifierResult classifier;

    LlmInput(RawEvent raw, InferredEvent result, ClassifierResult classifier) {
      eventId = raw.eventId;
      equipment = raw.equipment;
      rawLog = result.rawLog;
      templateId = result.templateId;
      templateText = result.templateText;
      this.classifier = classifier;
    }
  }

  public static class LlmResult {
    public String edgePred = "", adviceCode = "", advice = "", modelVersion = "";
    public int severity;
    public double confidence;
    public List<String> evidence;
  }

  public static class InferredEvent {
    public String eventId = "",
        source = "",
        equipment = "",
        eventTs = "",
        inferredAt = "",
        rawLog = "";
    public String templateId = "",
        templateText = "",
        edgePred = "",
        module = "",
        adviceCode = "",
        advice = "";
    public String inferencePath = "", modelVersion = "", ruleVersion = "";
    public int severity;
    public double confidence, uncertainty;
    public List<String> evidence = new ArrayList<>();
    public long latencyMs, normalizeMs, cacheMs, modelMs, edgeLlmMs, publishMs;

    void finish(long start) {
      inferredAt = Instant.now().toString();
      latencyMs = elapsed(start);
    }

    Map<String, Object> toMap() {
      var result =
          EdgeJson.object(
              "event_id",
              eventId,
              "equipment",
              equipment,
              "event_ts",
              eventTs,
              "inferred_at",
              inferredAt,
              "template_id",
              templateId,
              "template_text",
              templateText,
              "edge_pred",
              edgePred,
              "severity",
              severity,
              "confidence",
              confidence,
              "uncertainty",
              uncertainty,
              "evidence",
              evidence,
              "inference_path",
              inferencePath,
              "latency_ms",
              latencyMs,
              "stage_latency_ms",
              EdgeJson.object(
                  "normalize",
                  normalizeMs,
                  "cache",
                  cacheMs,
                  "model",
                  modelMs,
                  "edge_llm",
                  edgeLlmMs,
                  "publish",
                  publishMs));
      EdgeJson.optional(result, "source", source);
      EdgeJson.optional(result, "raw_log", rawLog);
      EdgeJson.optional(result, "module", module);
      EdgeJson.optional(result, "advice_code", adviceCode);
      EdgeJson.optional(result, "advice", advice);
      EdgeJson.optional(result, "model_version", modelVersion);
      EdgeJson.optional(result, "rule_version", ruleVersion);
      return result;
    }
  }

  static long elapsed(long start) {
    return (System.nanoTime() - start) / 1_000_000L;
  }
}
