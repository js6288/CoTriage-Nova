package io.cotriage.nova.gateway;

import com.fasterxml.jackson.annotation.JsonSetter;
import com.fasterxml.jackson.annotation.Nulls;
import com.fasterxml.jackson.databind.*;
import com.fasterxml.jackson.databind.cfg.*;
import com.fasterxml.jackson.databind.type.LogicalType;
import io.cotriage.nova.common.Json;

/** Go encoding/json rejects scalar coercion and leaves non-pointer fields unchanged on null. */
final class GoWire {
  private GoWire() {}

  static final ObjectMapper MAPPER = mapper();

  static ObjectMapper mapper() {
    ObjectMapper m =
        Json.MAPPER
            .copy()
            .disable(MapperFeature.ALLOW_COERCION_OF_SCALARS)
            .disable(DeserializationFeature.ACCEPT_FLOAT_AS_INT);
    m.setDefaultSetterInfo(JsonSetter.Value.forValueNulls(Nulls.SKIP));
    for (CoercionInputShape shape :
        new CoercionInputShape[] {
          CoercionInputShape.Integer, CoercionInputShape.Float, CoercionInputShape.Boolean
        }) m.coercionConfigFor(LogicalType.Textual).setCoercion(shape, CoercionAction.Fail);
    return m;
  }

  static <T> T read(String text, Class<T> type) {
    try {
      return MAPPER.readValue(text, type);
    } catch (Exception e) {
      throw new IllegalArgumentException("JSON decode failed: " + e.getMessage(), e);
    }
  }
}
