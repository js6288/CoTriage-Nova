package io.cotriage.nova.common;

import java.math.BigDecimal;
import java.time.Duration;
import java.util.Map;
import java.util.regex.Pattern;

public final class Env {
  private static final Pattern DURATION =
      Pattern.compile("(\\d+(?:\\.\\d*)?|\\.\\d+)(ns|us|µs|μs|ms|s|m|h)");
  private static final Map<String, BigDecimal> NANOS =
      Map.of(
          "ns",
          BigDecimal.ONE,
          "us",
          new BigDecimal("1000"),
          "µs",
          new BigDecimal("1000"),
          "μs",
          new BigDecimal("1000"),
          "ms",
          new BigDecimal("1000000"),
          "s",
          new BigDecimal("1000000000"),
          "m",
          new BigDecimal("60000000000"),
          "h",
          new BigDecimal("3600000000000"));

  private Env() {}

  public static String get(String key, String fallback) {
    String value = System.getenv(key);
    return value == null || value.trim().isEmpty() ? fallback : value.trim();
  }

  public static int integer(String key, int fallback) {
    try {
      return Integer.parseInt(get(key, ""));
    } catch (NumberFormatException e) {
      return fallback;
    }
  }

  public static long longValue(String key, long fallback) {
    try {
      return Long.parseLong(get(key, ""));
    } catch (NumberFormatException e) {
      return fallback;
    }
  }

  public static double decimal(String key, double fallback) {
    try {
      return Double.parseDouble(get(key, ""));
    } catch (NumberFormatException e) {
      return fallback;
    }
  }

  public static boolean bool(String key, boolean fallback) {
    return switch (get(key, "")) {
      case "1", "t", "T", "TRUE", "true", "True" -> true;
      case "0", "f", "F", "FALSE", "false", "False" -> false;
      default -> fallback;
    };
  }

  public static Duration duration(String key, Duration fallback) {
    try {
      return parseDuration(get(key, ""));
    } catch (IllegalArgumentException | ArithmeticException e) {
      return fallback;
    }
  }

  /** Go time.ParseDuration units; an integer alone is valid only for zero. */
  public static Duration parseDuration(String value) {
    if (value == null || value.isEmpty()) throw new IllegalArgumentException("empty duration");
    boolean negative = value.startsWith("-");
    String s = (negative || value.startsWith("+")) ? value.substring(1) : value;
    if (s.equals("0")) return Duration.ZERO;
    var matcher = DURATION.matcher(s);
    int pos = 0;
    BigDecimal nanos = BigDecimal.ZERO;
    while (matcher.find()) {
      if (matcher.start() != pos)
        throw new IllegalArgumentException("invalid Go duration: " + value);
      nanos = nanos.add(new BigDecimal(matcher.group(1)).multiply(NANOS.get(matcher.group(2))));
      pos = matcher.end();
    }
    if (pos == 0 || pos != s.length())
      throw new IllegalArgumentException("invalid Go duration: " + value);
    long n = (negative ? nanos.negate() : nanos).toBigInteger().longValueExact();
    return Duration.ofNanos(n);
  }
}
