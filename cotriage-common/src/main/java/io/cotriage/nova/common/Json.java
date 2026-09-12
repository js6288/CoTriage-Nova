package io.cotriage.nova.common;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.*;
import com.fasterxml.jackson.databind.json.JsonMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import java.util.LinkedHashMap;
import java.util.Map;

/** Wire JSON deliberately has no global property naming or null exclusion policy. */
public final class Json {
  public static final ObjectMapper MAPPER =
      JsonMapper.builder()
          .addModule(new JavaTimeModule())
          .disable(SerializationFeature.WRITE_DATES_AS_TIMESTAMPS)
          .disable(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES)
          .enable(MapperFeature.ACCEPT_CASE_INSENSITIVE_PROPERTIES)
          .build();

  private Json() {}

  public static String write(Object value) {
    try {
      return MAPPER.writeValueAsString(value);
    } catch (JsonProcessingException e) {
      throw new IllegalArgumentException("JSON encode failed", e);
    }
  }

  public static <T> T read(String json, Class<T> type) {
    try {
      return MAPPER.readValue(json, type);
    } catch (JsonProcessingException e) {
      throw new IllegalArgumentException("JSON decode failed", e);
    }
  }

  public static Map<String, Object> map(String json) {
    try {
      return MAPPER.readValue(json, new TypeReference<LinkedHashMap<String, Object>>() {});
    } catch (JsonProcessingException e) {
      throw new IllegalArgumentException("JSON decode failed", e);
    }
  }

  public static LinkedHashMap<String, Object> object(Object... entries) {
    if (entries.length % 2 != 0) throw new IllegalArgumentException("Expected key/value pairs");
    var result = new LinkedHashMap<String, Object>();
    for (int i = 0; i < entries.length; i += 2) result.put((String) entries[i], entries[i + 1]);
    return result;
  }
}
