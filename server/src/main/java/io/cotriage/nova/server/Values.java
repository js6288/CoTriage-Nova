package io.cotriage.nova.server;

import io.cotriage.nova.common.Json;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.*;
import java.util.*;

final class Values {
  private Values() {}

  static LinkedHashMap<String, Object> obj(Object... pairs) {
    var out = new LinkedHashMap<String, Object>();
    for (int i = 0; i < pairs.length; i += 2) out.put(pairs[i].toString(), pairs[i + 1]);
    return out;
  }

  static String str(Map<String, ?> m, String k) {
    return Objects.toString(m.get(k), "");
  }

  static String trim(Map<String, ?> m, String k) {
    return str(m, k).trim();
  }

  static long num(Map<String, ?> m, String k) {
    return number(m.get(k)).longValue();
  }

  static double dec(Map<String, ?> m, String k) {
    return number(m.get(k)).doubleValue();
  }

  static Number number(Object v) {
    if (v instanceof Number n) return n;
    try {
      return Double.parseDouble(Objects.toString(v, "0"));
    } catch (Exception e) {
      return 0;
    }
  }

  static boolean yes(Map<String, ?> m, String k) {
    var v = m.get(k);
    return Boolean.TRUE.equals(v)
        || "true".equalsIgnoreCase(Objects.toString(v, ""))
        || v instanceof Number n && n.intValue() != 0;
  }

  static String def(String v, String fallback) {
    return v == null || v.isBlank() ? fallback : v.trim();
  }

  static int integer(Map<String, ?> q, String k, int fallback, int min, int max) {
    try {
      return Math.clamp(Integer.parseInt(str(q, k).trim()), min, max);
    } catch (Exception e) {
      return fallback;
    }
  }

  static String env(String key, String fallback) {
    return def(System.getenv(key), fallback);
  }

  static Instant instant(Object value) {
    try {
      return Instant.parse(Objects.toString(value));
    } catch (Exception e) {
      return null;
    }
  }

  static String hash(String s) {
    try {
      return HexFormat.of()
          .formatHex(
              MessageDigest.getInstance("SHA-256").digest(s.getBytes(StandardCharsets.UTF_8)));
    } catch (Exception e) {
      throw new IllegalStateException(e);
    }
  }

  static Map<String, Object> body(String s) {
    try {
      var map = Json.map(s);
      if (map == null) throw new IllegalArgumentException("null body");
      var normalized = new LinkedHashMap<String, Object>();
      map.forEach((k, v) -> normalized.put(k.toLowerCase(Locale.ROOT), v));
      return normalized;
    } catch (Exception e) {
      throw new ApiFailure(400, "invalid JSON body");
    }
  }

  @SuppressWarnings("unchecked")
  static List<Object> list(Object o) {
    return o instanceof List<?> l ? (List<Object>) l : List.of();
  }

  static Object json(String s) {
    try {
      return Json.MAPPER.readValue(s, Object.class);
    } catch (Exception e) {
      return null;
    }
  }

  static boolean validJson(String s) {
    try {
      Json.MAPPER.readValue(s, Object.class);
      return true;
    } catch (Exception e) {
      return false;
    }
  }

  static double round2(double n) {
    return Math.round(n * 100) / 100d;
  }

  static double ratio(double a, double b) {
    return b <= 0 ? 0 : Math.round(a / b * 1_000_000) / 1_000_000d;
  }

  static long percentile(List<Long> values, int p) {
    if (values.isEmpty()) return 0;
    var sorted = new ArrayList<>(values);
    Collections.sort(sorted);
    return sorted.get(Math.clamp((p * sorted.size() + 99) / 100 - 1, 0, sorted.size() - 1));
  }

  static Map<String, Object> latency(List<Long> values) {
    return obj(
        "count",
        values.size(),
        "avg_ms",
        values.isEmpty()
            ? 0
            : round2(values.stream().mapToLong(Long::longValue).average().orElse(0)),
        "p50_ms",
        percentile(values, 50),
        "p95_ms",
        percentile(values, 95));
  }
}
