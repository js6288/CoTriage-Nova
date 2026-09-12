package io.cotriage.nova.collector;

import static org.junit.jupiter.api.Assertions.*;

import io.cotriage.nova.common.Json;
import java.nio.file.*;
import java.time.*;
import java.util.*;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicInteger;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

class TailPublisherTest {
  @TempDir Path temp;

  @Test
  void eventMatchesRawWireSchemaAndNanosecondTime() {
    var tail =
        new TailPublisher(
            Clock.fixed(Instant.parse("2026-01-01T00:00:00.100Z"), ZoneOffset.UTC), x -> {});
    var fields = tail.event("日志😀", "node-a", "fixed-id");
    assertEquals(Set.of("event_id", "payload"), fields.keySet());
    var payload = Json.map(fields.get("payload"));
    assertEquals("2026-01-01T00:00:00.1Z", payload.get("ts"));
    assertEquals(Map.of("raw", "日志😀"), payload.get("log"));
  }

  @Test
  void skipsExistingLinesAndBlankLinesButKeepsCrLfAndUnicode() throws Exception {
    Path file = temp.resolve("current.log");
    Files.writeString(file, "old\n");
    var running = new AtomicBoolean(true);
    var results = new ArrayList<Map<String, String>>();
    var tail = new TailPublisher(Clock.systemUTC(), x -> running.set(false));
    tail.run(
        file,
        "edge",
        fields -> results.add(fields),
        running::get,
        () -> {
          try {
            Files.writeString(file, "\r\n\u0085\n设备😀\r\n", StandardOpenOption.APPEND);
          } catch (Exception e) {
            throw new RuntimeException(e);
          }
        });
    assertEquals(1, results.size());
    assertEquals(Map.of("raw", "设备😀"), Json.map(results.getFirst().get("payload")).get("log"));
  }

  @Test
  void eofPartialLineIsDiscardedAsInOriginalCollector() throws Exception {
    Path file = temp.resolve("partial.log");
    Files.createFile(file);
    var running = new AtomicBoolean(true);
    var results = new ArrayList<Map<String, String>>();
    var pauses = new AtomicInteger();
    var tail =
        new TailPublisher(
            Clock.systemUTC(),
            x -> {
              try {
                if (pauses.getAndIncrement() == 0)
                  Files.writeString(file, "suffix\n", StandardOpenOption.APPEND);
                else running.set(false);
              } catch (Exception e) {
                throw new RuntimeException(e);
              }
            });
    tail.run(
        file,
        "edge",
        results::add,
        running::get,
        () -> {
          try {
            Files.writeString(file, "discarded", StandardOpenOption.APPEND);
          } catch (Exception e) {
            throw new RuntimeException(e);
          }
        });
    assertEquals(1, results.size());
    assertEquals(Map.of("raw", "suffix"), Json.map(results.getFirst().get("payload")).get("log"));
  }

  @Test
  void failedPublishSkipsLineRatherThanRetrying() throws Exception {
    Path file = temp.resolve("failure.log");
    Files.createFile(file);
    var running = new AtomicBoolean(true);
    var attempted = new ArrayList<String>();
    var tail =
        new TailPublisher(
            Clock.systemUTC(),
            x -> {
              if (x == 200) running.set(false);
            });
    tail.run(
        file,
        "edge",
        fields -> {
          String raw = (String) ((Map<?, ?>) Json.map(fields.get("payload")).get("log")).get("raw");
          attempted.add(raw);
          if (raw.equals("lost")) throw new Exception("redis unavailable");
        },
        running::get,
        () -> {
          try {
            Files.writeString(file, "lost\nnext\n", StandardOpenOption.APPEND);
          } catch (Exception e) {
            throw new RuntimeException(e);
          }
        });
    assertEquals(List.of("lost", "next"), attempted);
  }
}
