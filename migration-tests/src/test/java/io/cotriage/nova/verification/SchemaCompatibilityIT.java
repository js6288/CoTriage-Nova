package io.cotriage.nova.verification;

import static org.junit.jupiter.api.Assertions.*;

import io.cotriage.nova.common.Db;
import io.cotriage.nova.common.Json;
import io.cotriage.nova.common.Schema;
import java.nio.file.Files;
import java.util.List;
import java.util.Set;
import java.util.TreeSet;
import org.junit.jupiter.api.Test;
import org.springframework.core.io.FileSystemResource;
import org.springframework.jdbc.core.JdbcTemplate;
import org.testcontainers.containers.MySQLContainer;

/** Real Go DDL and existing records exercise additive recovery; Docker absence is a failure. */
class SchemaCompatibilityIT {
  @Test
  void restoresMissingNullableColumnAndIndexWithoutReplacingOldData() throws Exception {
    var root = EndToEndIT.projectRoot();
    try (var mysql =
        new MySQLContainer<>("mysql:8.4")
            .withDatabaseName("legacy_schema")
            .withUsername("nova")
            .withPassword("test-only")) {
      mysql.start();
      try (var source =
          Db.open(
              "nova:test-only@tcp("
                  + mysql.getHost()
                  + ":"
                  + mysql.getMappedPort(3306)
                  + ")/legacy_schema?charset=utf8mb4&parseTime=true&loc=UTC")) {
        var jdbc = new JdbcTemplate(source);
        var legacy = new FileSystemResource(root.resolve("migration/fixtures/go-schema.sql"));
        Schema.ensure(jdbc, legacy);
        Set<String> expected =
            Set.of(
                "classifier_configs",
                "cloud_feedback",
                "cloud_outbox",
                "cloud_providers",
                "competition_metric_snapshots",
                "edge_providers",
                "edge_rule_sets",
                "edge_rules",
                "edge_template_sets",
                "edge_templates",
                "router_decisions",
                "router_state_snapshots");
        assertEquals(expected, tables(jdbc));
        jdbc.update(
            "INSERT INTO"
                + " cloud_feedback(event_id,request_id,template_id,root_cause,cloud_confidence)"
                + " VALUES (?,?,?,?,?)",
            "legacy-event",
            "legacy-request",
            "legacy-template",
            "保留既有记录",
            .91);
        jdbc.update(
            "INSERT INTO router_decisions(event_id,action,reason,routed_at) VALUES (?,?,?,?)",
            "legacy-routed",
            "LOCAL",
            "existing decision",
            java.sql.Timestamp.from(java.time.Instant.parse("2026-01-01T00:00:00Z")));
        var feedback =
            jdbc.queryForList(
                "SELECT id,event_id,request_id,template_id,root_cause,cloud_confidence FROM"
                    + " cloud_feedback");
        var decisions = jdbc.queryForList("SELECT id,event_id,action,reason FROM router_decisions");
        jdbc.execute("ALTER TABLE cloud_feedback DROP COLUMN recommendation");
        jdbc.execute("ALTER TABLE cloud_feedback DROP INDEX idx_cloud_feedback_template_id");
        assertEquals(
            0,
            number(
                jdbc,
                "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND"
                    + " table_name='cloud_feedback' AND column_name='recommendation'"));
        assertEquals(
            0,
            number(
                jdbc,
                "SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE()"
                    + " AND table_name='cloud_feedback' AND"
                    + " index_name='idx_cloud_feedback_template_id'"));
        Schema.ensure(jdbc, legacy);
        Schema.ensure(
            jdbc, new FileSystemResource(root.resolve("server/src/main/resources/schema.sql")));
        Schema.ensure(
            jdbc,
            new FileSystemResource(
                root.resolve("router-controller/src/main/resources/router-schema.sql")));
        assertEquals(
            1,
            number(
                jdbc,
                "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND"
                    + " table_name='cloud_feedback' AND column_name='recommendation' AND"
                    + " is_nullable='YES'"));
        assertEquals(
            1,
            number(
                jdbc,
                "SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE()"
                    + " AND table_name='cloud_feedback' AND"
                    + " index_name='idx_cloud_feedback_template_id'"));
        assertEquals(
            feedback,
            jdbc.queryForList(
                "SELECT id,event_id,request_id,template_id,root_cause,cloud_confidence FROM"
                    + " cloud_feedback"));
        assertEquals(
            decisions, jdbc.queryForList("SELECT id,event_id,action,reason FROM router_decisions"));
        assertNull(
            jdbc.queryForObject(
                "SELECT recommendation FROM cloud_feedback WHERE event_id='legacy-event'",
                String.class));
        assertEquals(expected, tables(jdbc));
        Schema.ensure(jdbc, legacy);
        assertEquals(1, number(jdbc, "SELECT COUNT(*) FROM cloud_feedback"));
        assertEquals(1, number(jdbc, "SELECT COUNT(*) FROM router_decisions"));
        var directory =
            Files.createDirectories(root.resolve("migration-tests/target/schema-compatibility"));
        Files.writeString(
            directory.resolve("result.json"),
            Json.write(
                Json.object(
                    "legacy_tables",
                    new TreeSet<>(expected),
                    "restored_column",
                    "cloud_feedback.recommendation",
                    "restored_index",
                    "idx_cloud_feedback_template_id",
                    "retained_feedback",
                    feedback,
                    "retained_decisions",
                    decisions,
                    "extra_tables",
                    List.of())));
      }
    }
  }

  private static Set<String> tables(JdbcTemplate jdbc) {
    return new TreeSet<>(
        jdbc.queryForList(
            "SELECT table_name FROM information_schema.tables WHERE table_schema=DATABASE()",
            String.class));
  }

  private static long number(JdbcTemplate jdbc, String sql) {
    return jdbc.queryForObject(sql, Long.class);
  }
}
