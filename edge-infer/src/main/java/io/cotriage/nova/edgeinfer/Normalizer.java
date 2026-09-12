package io.cotriage.nova.edgeinfer;

import com.google.re2j.Pattern;
import java.io.ByteArrayOutputStream;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.Arrays;
import java.util.HexFormat;

final class Normalizer {
  record Normalized(String text, String id) {}

  private record Replacement(Pattern pattern, String value) {}

  private static final Replacement[] REPLACEMENTS = {
    new Replacement(
        Pattern.compile(
            "(?i)\\b[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\\b"),
        "<UUID>"),
    new Replacement(Pattern.compile("\\b(?:\\d{1,3}\\.){3}\\d{1,3}\\b"), "<IP>"),
    new Replacement(Pattern.compile("(?i)\\b0x[0-9a-f]+\\b"), "<HEX>"),
    new Replacement(Pattern.compile("(?:/[A-Za-z0-9._-]+){2,}"), "<PATH>"),
    new Replacement(Pattern.compile("\\b\\d+\\b"), "<NUM>")
  };
  private static final Pattern PREFIX =
      Pattern.compile(
          "^\\s*(?:\\[?\\d{4}[-/]\\d{2}[-/]\\d{2}[T ][^ ]+\\]?\\s*)?(?:\\[?(?:pid|tid|thread)[=:"
              + " ]?\\d+\\]?\\s*)*");

  private Normalizer() {}

  static String normalize(String raw, int maxBytes) {
    return normalizeResult(raw, maxBytes).text();
  }

  static Normalized normalizeResult(String raw, int maxBytes) {
    byte[] bytes = trim(raw).getBytes(StandardCharsets.UTF_8);
    // Preserve invalid trailing bytes through Go-style substring operations. JSON replaces them,
    // but TemplateID hashes the original bytes before JSON encoding.
    String text = decodeGoUtf8(bytes, Math.min(bytes.length, maxBytes));
    text = PREFIX.matcher(text).replaceAll("");
    for (var replacement : REPLACEMENTS)
      text = replacement.pattern.matcher(text).replaceAll(replacement.value);
    StringBuilder result = new StringBuilder();
    boolean separated = true;
    for (int cp : text.codePoints().toArray()) {
      if (isSpace(cp)) {
        separated = true;
        continue;
      }
      if (separated && !result.isEmpty()) result.append(' ');
      result.appendCodePoint(cp);
      separated = false;
    }
    String internal = result.toString();
    var encoded = new ByteArrayOutputStream();
    var visible = new StringBuilder();
    for (int i = 0; i < internal.length(); ) {
      char ch = internal.charAt(i);
      if (ch >= 0xdc80 && ch <= 0xdcff) {
        encoded.write(ch - 0xdc00);
        visible.append('\ufffd');
        i++;
      } else {
        int cp = internal.codePointAt(i);
        String value = new String(Character.toChars(cp));
        encoded.writeBytes(value.getBytes(StandardCharsets.UTF_8));
        visible.append(value);
        i += Character.charCount(cp);
      }
    }
    return new Normalized(visible.toString(), hash(encoded.toByteArray()));
  }

  static String templateId(String template) {
    return hash(template.getBytes(StandardCharsets.UTF_8));
  }

  private static String hash(byte[] bytes) {
    try {
      return HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(bytes), 0, 8);
    } catch (java.security.NoSuchAlgorithmException ex) {
      throw new IllegalStateException(ex);
    }
  }

  static String trim(String value) {
    if (value == null) return "";
    int start = 0, end = value.length();
    while (start < end && isSpace(value.codePointAt(start)))
      start += Character.charCount(value.codePointAt(start));
    while (end > start && isSpace(value.codePointBefore(end)))
      end -= Character.charCount(value.codePointBefore(end));
    return value.substring(start, end);
  }

  static boolean isSpace(int cp) {
    return cp == 0x85
        || Character.isWhitespace(cp) && cp != 0x1c && cp != 0x1d && cp != 0x1e && cp != 0x1f
        || Character.isSpaceChar(cp);
  }

  static String truncateRawLog(String raw) {
    raw = trim(raw);
    int count = raw.codePointCount(0, raw.length());
    return count <= 2048
        ? raw
        : raw.substring(0, raw.offsetByCodePoints(0, 2048)) + "...[truncated]";
  }

  static String preview(String value, int maxRunes) {
    value = trim(value);
    return value.codePointCount(0, value.length()) <= maxRunes
        ? value
        : value.substring(0, value.offsetByCodePoints(0, maxRunes)) + "...[truncated]";
  }

  private static String decodeGoUtf8(byte[] bytes, int length) {
    if (length == bytes.length) return new String(bytes, StandardCharsets.UTF_8);
    int end = length;
    while (end > 0 && (bytes[end - 1] & 0xc0) == 0x80) end--;
    int lead = end - 1;
    if (lead >= 0) {
      int b = bytes[lead] & 0xff;
      int needed = b < 0x80 ? 1 : b < 0xe0 ? 2 : b < 0xf0 ? 3 : 4;
      if (length - lead < needed) {
        var result =
            new StringBuilder(new String(Arrays.copyOf(bytes, lead), StandardCharsets.UTF_8));
        for (int i = lead; i < length; i++) result.append((char) (0xdc00 + (bytes[i] & 0xff)));
        return result.toString();
      }
    }
    return new String(bytes, 0, length, StandardCharsets.UTF_8);
  }
}
