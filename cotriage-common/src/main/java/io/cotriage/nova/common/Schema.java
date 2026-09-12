package io.cotriage.nova.common;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.sql.*;
import java.util.*;
import java.util.regex.*;
import org.springframework.core.io.Resource;
import org.springframework.jdbc.core.JdbcTemplate;

/**
 * Add missing columns and named indexes using the fixed, source-derived DDL; never drop user data.
 */
public final class Schema {
  private static final Pattern TABLE =
      Pattern.compile(
          "(?is)^CREATE\\s+TABLE\\s+(?:IF\\s+NOT\\s+EXISTS\\s+)?`?([a-zA-Z0-9_]+)`?\\s*\\(");
  private static final Pattern INDEX =
      Pattern.compile("(?is)^(?:UNIQUE\\s+)?(?:KEY|INDEX)\\s+`?([a-zA-Z0-9_]+)`?.*");

  private Schema() {}

  public static void ensure(JdbcTemplate jdbc, Resource resource) {
    String script;
    try {
      script = resource.getContentAsString(StandardCharsets.UTF_8);
    } catch (IOException e) {
      throw new IllegalStateException("Read schema", e);
    }
    for (String statement : split(script, ';')) {
      String sql = statement.trim();
      if (sql.isEmpty()) continue;
      var match = TABLE.matcher(sql);
      if (!match.find()) {
        jdbc.execute(sql);
        continue;
      }
      String table = match.group(1);
      jdbc.execute(sql);
      try (Connection c = Objects.requireNonNull(jdbc.getDataSource()).getConnection()) {
        var metadata = c.getMetaData();
        var columns = new HashSet<String>();
        var indexes = new HashSet<String>();
        try (ResultSet rows = metadata.getColumns(c.getCatalog(), null, table, null)) {
          while (rows.next()) columns.add(rows.getString("COLUMN_NAME").toLowerCase(Locale.ROOT));
        }
        try (ResultSet rows = metadata.getIndexInfo(c.getCatalog(), null, table, false, false)) {
          while (rows.next()) {
            String name = rows.getString("INDEX_NAME");
            if (name != null) indexes.add(name.toLowerCase(Locale.ROOT));
          }
        }
        String definitions = sql.substring(match.end(), sql.lastIndexOf(')'));
        for (String raw : split(definitions, ',')) {
          String definition = raw.trim();
          String upper = definition.toUpperCase(Locale.ROOT);
          if (upper.startsWith("PRIMARY KEY")) {
            if (!indexes.contains("primary"))
              jdbc.execute("ALTER TABLE `" + table + "` ADD " + definition);
            continue;
          }
          var index = INDEX.matcher(definition);
          if (index.matches()) {
            if (!indexes.contains(index.group(1).toLowerCase(Locale.ROOT)))
              jdbc.execute("ALTER TABLE `" + table + "` ADD " + definition);
            continue;
          }
          if (upper.startsWith("CONSTRAINT")
              || upper.startsWith("FOREIGN KEY")
              || upper.startsWith("CHECK")) continue;
          String name = definition.split("\\s+", 2)[0].replace("`", "");
          if (!columns.contains(name.toLowerCase(Locale.ROOT)))
            jdbc.execute("ALTER TABLE `" + table + "` ADD COLUMN " + definition);
        }
      } catch (SQLException e) {
        throw new IllegalStateException("Inspect schema for " + table, e);
      }
    }
  }

  static List<String> split(String text, char delimiter) {
    var result = new ArrayList<String>();
    int begin = 0, depth = 0;
    char quote = 0;
    for (int i = 0; i < text.length(); i++) {
      char c = text.charAt(i);
      if (quote != 0) {
        if (c == '\\') {
          i++;
          continue;
        }
        if (c == quote) {
          if (i + 1 < text.length() && text.charAt(i + 1) == quote) {
            i++;
            continue;
          }
          quote = 0;
        }
        continue;
      }
      if (c == '\'' || c == '"' || c == '`') {
        quote = c;
        continue;
      }
      if (c == '(') depth++;
      else if (c == ')') depth--;
      else if (c == delimiter && depth == 0) {
        result.add(text.substring(begin, i));
        begin = i + 1;
      }
    }
    result.add(text.substring(begin));
    return result;
  }
}
