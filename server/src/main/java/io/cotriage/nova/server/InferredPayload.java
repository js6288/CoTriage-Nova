package io.cotriage.nova.server;

import com.fasterxml.jackson.annotation.JsonSetter;
import com.fasterxml.jackson.annotation.Nulls;
import com.fasterxml.jackson.databind.*;
import com.fasterxml.jackson.databind.cfg.*;
import com.fasterxml.jackson.databind.type.LogicalType;
import io.cotriage.nova.common.Json;
import java.util.*;

/** Go's inferredEvent wire type: unknown fields ignored, known fields strictly typed. */
final class InferredPayload {
  private static final ObjectMapper DECODER = decoder();

  @JsonSetter(nulls = Nulls.SKIP)
  public String event_id = "";

  @JsonSetter(nulls = Nulls.SKIP)
  public String source = "";

  @JsonSetter(nulls = Nulls.SKIP)
  public String equipment = "";

  @JsonSetter(nulls = Nulls.SKIP)
  public String event_ts = "";

  @JsonSetter(nulls = Nulls.SKIP)
  public String inferred_at = "";

  @JsonSetter(nulls = Nulls.SKIP)
  public String raw_log = "";

  @JsonSetter(nulls = Nulls.SKIP)
  public String template_id = "";

  @JsonSetter(nulls = Nulls.SKIP)
  public String template_text = "";

  @JsonSetter(nulls = Nulls.SKIP)
  public String edge_pred = "";

  @JsonSetter(nulls = Nulls.SKIP)
  public long severity;

  @JsonSetter(nulls = Nulls.SKIP)
  public double confidence;

  @JsonSetter(nulls = Nulls.SKIP)
  public double uncertainty;

  public List<String> evidence;

  @JsonSetter(nulls = Nulls.SKIP)
  public String advice_code = "";

  @JsonSetter(nulls = Nulls.SKIP)
  public String advice = "";

  @JsonSetter(nulls = Nulls.SKIP)
  public String inference_path = "";

  @JsonSetter(nulls = Nulls.SKIP)
  public String model_version = "";

  @JsonSetter(nulls = Nulls.SKIP)
  public String rule_version = "";

  @JsonSetter(nulls = Nulls.SKIP)
  public long latency_ms;

  public Map<String, Long> stage_latency_ms;

  @JsonSetter("evidence")
  public void evidence(List<String> value) {
    evidence = value == null ? null : value.stream().map(v -> v == null ? "" : v).toList();
  }

  @JsonSetter("stage_latency_ms")
  public void stages(Map<String, Long> value) {
    if (value == null) {
      stage_latency_ms = null;
      return;
    }
    stage_latency_ms = new LinkedHashMap<>();
    value.forEach((k, v) -> stage_latency_ms.put(k, v == null ? 0L : v));
  }

  static ObjectMapper decoder() {
    var mapper =
        Json.MAPPER
            .copy()
            .enable(MapperFeature.ACCEPT_CASE_INSENSITIVE_PROPERTIES)
            .enable(DeserializationFeature.FAIL_ON_TRAILING_TOKENS)
            .disable(DeserializationFeature.ACCEPT_FLOAT_AS_INT)
            .disable(MapperFeature.ALLOW_COERCION_OF_SCALARS);
    for (var shape :
        List.of(CoercionInputShape.Integer, CoercionInputShape.Float, CoercionInputShape.Boolean))
      mapper.coercionConfigFor(LogicalType.Textual).setCoercion(shape, CoercionAction.Fail);
    for (var type : List.of(LogicalType.Integer, LogicalType.Float))
      for (var shape :
          List.of(
              CoercionInputShape.String,
              CoercionInputShape.EmptyString,
              CoercionInputShape.Boolean))
        mapper.coercionConfigFor(type).setCoercion(shape, CoercionAction.Fail);
    mapper
        .coercionConfigFor(LogicalType.Integer)
        .setCoercion(CoercionInputShape.Float, CoercionAction.Fail);
    return mapper;
  }

  @SuppressWarnings("unchecked")
  static Map<String, Object> decode(String text) {
    try {
      var value = DECODER.readValue(text, InferredPayload.class);
      if (value == null) value = new InferredPayload();
      if (!Double.isFinite(value.confidence) || !Double.isFinite(value.uncertainty)) return null;
      var out = (Map<String, Object>) Json.MAPPER.convertValue(value, Map.class);
      for (var key :
          List.of("source", "raw_log", "advice_code", "advice", "model_version", "rule_version"))
        if (Objects.toString(out.get(key), "").isEmpty()) out.remove(key);
      return out;
    } catch (Exception e) {
      return null;
    }
  }
}
