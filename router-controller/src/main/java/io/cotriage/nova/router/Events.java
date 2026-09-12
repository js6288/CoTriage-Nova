package io.cotriage.nova.router;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonInclude;
import java.util.*;

/** Wire names intentionally match encoding/json, including mixed-case routing fields. */
public final class Events {
  private Events() {}

  @JsonIgnoreProperties(ignoreUnknown = true)
  public static class Inferred {
    public String event_id = "", edge_pred = "", inference_path = "";
    @JsonInclude(JsonInclude.Include.NON_EMPTY)
    public String trace_id = "",
        schema_version = "",
        source = "",
        equipment = "",
        event_ts = "",
        inferred_at = "",
        raw_log = "",
        template_id = "",
        template_text = "",
        advice_code = "",
        advice = "",
        model_version = "",
        rule_version = "";
    public long severity, latency_ms;
    public double confidence, uncertainty;

    @JsonInclude(JsonInclude.Include.NON_EMPTY)
    public List<String> evidence;

    public StageLatency stage_latency_ms = new StageLatency();
  }

  public static class StageLatency {
    public long normalize, cache, model, edge_llm, publish;
  }

  public record Routed(
      Inferred input,
      RaiseEngine.Decision decision,
      RaiseEngine.SystemState system_state,
      String routed_at,
      String router,
      String schema_version) {}

  public static Inferred decode(String json) {
    return GoWire.read(json, Inferred.class);
  }

  public static String str(Map<String, ?> row, String key) {
    Object v = row.get(key);
    return v == null ? "" : v.toString();
  }

  public static double num(Map<String, ?> row, String key) {
    Object v = row.get(key);
    return v == null
        ? 0
        : v instanceof Number n ? n.doubleValue() : Double.parseDouble(v.toString());
  }

  public static boolean flag(Map<String, ?> row, String key) {
    Object v = row.get(key);
    return Boolean.TRUE.equals(v)
        || (v instanceof Number n && n.intValue() != 0)
        || "true".equalsIgnoreCase(String.valueOf(v));
  }
}
