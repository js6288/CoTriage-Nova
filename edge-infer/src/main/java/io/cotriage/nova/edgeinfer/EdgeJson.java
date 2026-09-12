package io.cotriage.nova.edgeinfer;

import com.fasterxml.jackson.annotation.JsonSetter;
import com.fasterxml.jackson.annotation.Nulls;
import com.fasterxml.jackson.core.SerializableString;
import com.fasterxml.jackson.core.io.CharacterEscapes;
import com.fasterxml.jackson.core.io.SerializedString;
import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.MapperFeature;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.SerializationFeature;
import com.fasterxml.jackson.databind.cfg.CoercionAction;
import com.fasterxml.jackson.databind.cfg.CoercionInputShape;
import com.fasterxml.jackson.databind.type.LogicalType;
import java.io.IOException;
import java.util.LinkedHashMap;
import java.util.Map;

/**
 * Isolated codec: Go encoding/json accepts unknown fields and leaves null scalar values at zero.
 */
final class EdgeJson {
  static final ObjectMapper MAPPER = createMapper();

  private static ObjectMapper createMapper() {
    var mapper =
        new ObjectMapper()
            .setPropertyNamingStrategy(PropertyNamingStrategies.SNAKE_CASE)
            .configure(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES, false)
            .configure(DeserializationFeature.ACCEPT_FLOAT_AS_INT, false)
            .configure(MapperFeature.ACCEPT_CASE_INSENSITIVE_PROPERTIES, true)
            .configure(MapperFeature.ALLOW_COERCION_OF_SCALARS, false)
            .configure(DeserializationFeature.FAIL_ON_TRAILING_TOKENS, true)
            .configure(SerializationFeature.ORDER_MAP_ENTRIES_BY_KEYS, true)
            .setDefaultSetterInfo(JsonSetter.Value.forValueNulls(Nulls.SKIP));
    mapper
        .coercionConfigFor(LogicalType.Textual)
        .setCoercion(CoercionInputShape.Integer, CoercionAction.Fail)
        .setCoercion(CoercionInputShape.Float, CoercionAction.Fail)
        .setCoercion(CoercionInputShape.Boolean, CoercionAction.Fail);
    mapper.getFactory().setCharacterEscapes(new GoEscapes());
    return mapper;
  }

  private EdgeJson() {}

  static String write(Object value) {
    try {
      return MAPPER.writeValueAsString(value);
    } catch (IOException ex) {
      throw new IllegalArgumentException("encode JSON: " + ex.getMessage(), ex);
    }
  }

  static <T> T read(String value, Class<T> type) throws IOException {
    return MAPPER.readValue(value, type);
  }

  static <T> T read(String value, TypeReference<T> type) throws IOException {
    return MAPPER.readValue(value, type);
  }

  /** net/http paths use json.Decoder.Decode once rather than json.Unmarshal. */
  static <T> T readOne(String value, Class<T> type) throws IOException {
    return MAPPER
        .readerFor(type)
        .without(DeserializationFeature.FAIL_ON_TRAILING_TOKENS)
        .readValue(value);
  }

  static com.fasterxml.jackson.databind.JsonNode treeOne(String value) throws IOException {
    return MAPPER.reader().without(DeserializationFeature.FAIL_ON_TRAILING_TOKENS).readTree(value);
  }

  static Map<String, Object> object(Object... pairs) {
    Map<String, Object> result = new LinkedHashMap<>();
    for (int i = 0; i < pairs.length; i += 2) result.put((String) pairs[i], pairs[i + 1]);
    return result;
  }

  static void optional(Map<String, Object> target, String key, String value) {
    if (value != null && !value.isEmpty()) target.put(key, value);
  }

  private static class GoEscapes extends CharacterEscapes {
    private final int[] codes = CharacterEscapes.standardAsciiEscapesForJSON();

    GoEscapes() {
      codes['<'] = ESCAPE_CUSTOM;
      codes['>'] = ESCAPE_CUSTOM;
      codes['&'] = ESCAPE_CUSTOM;
    }

    @Override
    public int[] getEscapeCodesForAscii() {
      return codes;
    }

    @Override
    public SerializableString getEscapeSequence(int ch) {
      return switch (ch) {
        case '<' -> new SerializedString("\\u003c");
        case '>' -> new SerializedString("\\u003e");
        case '&' -> new SerializedString("\\u0026");
        case 0x2028 -> new SerializedString("\\u2028");
        case 0x2029 -> new SerializedString("\\u2029");
        default -> null;
      };
    }
  }
}
