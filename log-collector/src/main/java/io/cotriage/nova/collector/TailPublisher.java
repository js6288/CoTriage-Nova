package io.cotriage.nova.collector;

import io.cotriage.nova.common.Json;
import java.io.*;
import java.nio.channels.Channels;
import java.nio.channels.FileChannel;
import java.nio.charset.StandardCharsets;
import java.nio.file.*;
import java.time.Clock;
import java.time.format.DateTimeFormatterBuilder;
import java.util.Map;
import java.util.UUID;
import java.util.function.BooleanSupplier;
import java.util.function.LongConsumer;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Compatibility tailer: starts at EOF and deliberately retains the old partial-line and failed-XADD
 * loss windows.
 */
public final class TailPublisher {
  private static final Logger LOG = LoggerFactory.getLogger(TailPublisher.class);

  @FunctionalInterface
  public interface Publisher {
    void publish(Map<String, String> fields) throws Exception;
  }

  private final Clock clock;
  private final LongConsumer pause;

  public TailPublisher() {
    this(
        Clock.systemUTC(),
        millis -> {
          try {
            Thread.sleep(millis);
          } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
          }
        });
  }

  TailPublisher(Clock clock, LongConsumer pause) {
    this.clock = clock;
    this.pause = pause;
  }

  public void run(Path file, String equipment, Publisher publisher, BooleanSupplier running)
      throws IOException {
    run(file, equipment, publisher, running, () -> {});
  }

  void run(
      Path file, String equipment, Publisher publisher, BooleanSupplier running, Runnable opened)
      throws IOException {
    while (running.getAsBoolean()) {
      try {
        Files.readAttributes(file, java.nio.file.attribute.BasicFileAttributes.class);
        break;
      } catch (NoSuchFileException e) {
        pause.accept(1000);
      }
    }
    if (!running.getAsBoolean()) return;
    try (FileChannel channel = FileChannel.open(file, StandardOpenOption.READ)) {
      channel.position(channel.size());
      InputStream reader = new BufferedInputStream(Channels.newInputStream(channel), 256 * 1024);
      opened.run();
      while (running.getAsBoolean() && !Thread.currentThread().isInterrupted()) {
        ByteArrayOutputStream bytes = new ByteArrayOutputStream();
        int next;
        while ((next = reader.read()) != -1 && next != '\n') bytes.write(next);
        // bufio.ReadString returns partial content plus EOF; the Go caller discards it.
        if (next == -1) {
          pause.accept(200);
          continue;
        }
        String line = bytes.toString(StandardCharsets.UTF_8);
        int end = line.length();
        while (end > 0 && (line.charAt(end - 1) == '\r' || line.charAt(end - 1) == '\n')) end--;
        line = line.substring(0, end);
        if (line.codePoints().allMatch(TailPublisher::goSpace)) continue;
        try {
          publisher.publish(event(line, equipment, UUID.randomUUID().toString()));
        } catch (Exception e) {
          LOG.warn("XADD failed; current line skipped: {}", e.toString());
          pause.accept(500);
        }
      }
    }
  }

  private static boolean goSpace(int c) {
    return Character.isWhitespace(c) || Character.isSpaceChar(c) || c == 0x85;
  }

  Map<String, String> event(String raw, String equipment, String eventId) {
    String ts =
        new DateTimeFormatterBuilder().appendInstant(-1).toFormatter().format(clock.instant());
    return Map.of(
        "event_id",
        eventId,
        "payload",
        Json.write(
            Json.object(
                "event_id",
                eventId,
                "ts",
                ts,
                "equipment",
                equipment,
                "log",
                Json.object("raw", raw))));
  }
}
