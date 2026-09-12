package io.cotriage.nova.gateway;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import java.util.*;

public final class Events {
  private Events() {}

  @JsonIgnoreProperties(ignoreUnknown = true)
  public static class Inferred {
    public String event_id = "",
        source = "",
        equipment = "",
        event_ts = "",
        raw_log = "",
        template_id = "",
        template_text = "",
        edge_pred = "",
        advice_code = "",
        advice = "";
    public long severity;
    public double confidence, uncertainty;
    public List<String> evidence;
  }

  @JsonIgnoreProperties(ignoreUnknown = true)
  public static class Decision {
    public String action = "", reason = "";
    public double score, threshold;
  }

  @JsonIgnoreProperties(ignoreUnknown = true)
  public static class Routed {
    public Inferred input = new Inferred();
    public Decision decision = new Decision();
    public Map<String, Object> system_state;
  }

  public record Request(String eventID, String requestID, String idempotencyKey, Routed routed) {}

  @JsonIgnoreProperties(ignoreUnknown = true)
  public static class Diagnosis {
    public String pred = "", root_cause = "", recommendation = "", explanation = "";
    public double confidence;
    public List<String> evidence;
  }

  public static class Usage {
    public long prompt_tokens, completion_tokens, total_tokens, latency_ms;
    public double cost;
  }

  public record Feedback(
      boolean corrected_edge,
      boolean added_actionable_info,
      boolean confirmed_high_risk,
      double reward,
      String reward_reason) {}

  public record EdgeSummary(String pred, double confidence, long severity, String advice_code) {}

  public static String value(Map<String, ?> row, String key) {
    Object v = row.get(key);
    return v == null ? "" : v.toString();
  }
}
