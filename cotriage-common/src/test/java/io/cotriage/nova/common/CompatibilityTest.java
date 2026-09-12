package io.cotriage.nova.common;

import static org.junit.jupiter.api.Assertions.*;

import java.time.Duration;
import java.time.Instant;
import org.junit.jupiter.api.Test;

class CompatibilityTest {
  @Test
  void goDurationsKeepNanoseconds() {
    assertEquals(Duration.ofNanos(5_400_500_000_003L), Env.parseDuration("1h30m0.5s3ns"));
    assertEquals(Duration.ofNanos(-1500), Env.parseDuration("-1.5us"));
    assertEquals(Duration.ZERO, Env.parseDuration("0"));
    assertThrows(IllegalArgumentException.class, () -> Env.parseDuration("10"));
    assertThrows(IllegalArgumentException.class, () -> Env.parseDuration("PT2S"));
  }

  @Test
  void dsnKeepsCredentialSeparatorsAndConvertsTimeUnits() {
    var s =
        Db.parse(
            "user:p@ss:word@tcp([::1]:3306)/cotriage?charset=utf8mb4&parseTime=true&loc=UTC&timeout=1.5s");
    assertEquals("p@ss:word", s.password());
    assertEquals("user", s.username());
    assertTrue(s.url().contains("connectTimeout=1500"));
    assertTrue(s.url().contains("jdbc:mysql://[::1]:3306/cotriage"));
    assertFalse(s.url().contains("parseTime"));
  }

  @Test
  void jsonDoesNotNormalizeWireCasingOrNulls() {
    var value =
        Json.object(
            "Action",
            "CLOUD",
            "features",
            Json.object("risk", .5),
            "absent",
            null,
            "ts",
            Instant.parse("2026-01-01T00:00:00Z"));
    String body = Json.write(value);
    assertTrue(body.contains("\"Action\""));
    assertTrue(body.contains("\"absent\":null"));
    assertEquals("2026-01-01T00:00:00Z", Json.map(body).get("ts"));
  }
}
