package io.cotriage.nova.edgeinfer;

import static org.junit.jupiter.api.Assertions.*;

import java.time.Duration;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;

class WorkerContractTest {
  static Config config() {
    return Config.load(Map.of(), "test-consumer");
  }

  static RedisPort.Message message(String id, String eventId) {
    var raw = EngineContractTest.raw("no space left on device");
    raw.eventId = eventId;
    return new RedisPort.Message(id, Map.of("event_id", eventId, "payload", EdgeJson.write(raw)));
  }

  static Worker worker(FakeRedis redis) throws Exception {
    return new Worker(redis, config(), EngineContractTest.engine(redis, null, null));
  }

  @Test
  void successPublishesBeforeProcessedAndAckAndDeduplicatesByEventId() throws Exception {
    var redis = new FakeRedis();
    var worker = worker(redis);
    worker.handle(message("1-0", "a"));
    worker.handle(message("2-0", "a"));
    assertEquals(1, redis.added.size());
    assertEquals(List.of("1-0", "2-0"), redis.acked);
    assertTrue(
        redis.calls.indexOf("add:cotriage.events.inferred")
            < redis.calls.indexOf("set:" + Worker.processedKey("a")));
    assertTrue(
        redis.calls.indexOf("set:" + Worker.processedKey("a")) < redis.calls.indexOf("ack:1-0"));
    assertEquals(Duration.ofHours(24), redis.ttls.get(Worker.processedKey("a")));
    var payload = EdgeJson.MAPPER.readTree(redis.added.getFirst().values().get("payload"));
    assertEquals("a", payload.path("event_id").asText());
    assertEquals("DISK_ERROR", payload.path("edge_pred").asText());
  }

  @Test
  void malformedPayloadRetriesThenDlqAndAck() throws Exception {
    var redis = new FakeRedis();
    var worker = worker(redis);
    var invalid = new RedisPort.Message("1-0", Map.of("event_id", "a", "payload", "{}"));
    worker.handle(invalid);
    worker.handle(invalid);
    assertTrue(redis.acked.isEmpty());
    worker.handle(invalid);
    assertEquals(1, redis.added.size());
    var dlq = redis.added.getFirst();
    assertEquals(config().dlqStream(), dlq.stream());
    assertEquals("3", dlq.values().get("attempt"));
    assertEquals("1-0", dlq.values().get("source_message_id"));
    assertTrue(dlq.values().get("error").contains("event_id mismatch"));
    assertEquals(List.of("1-0"), redis.acked);
    assertTrue(redis.counts.isEmpty());
  }

  @Test
  void dlqFailureLeavesPendingAndAttemptCount() throws Exception {
    var redis = new FakeRedis();
    redis.fail = op -> op.equals("add:cotriage.events.dlq");
    var worker = worker(redis);
    var invalid = new RedisPort.Message("2-0", Map.of());
    for (int i = 0; i < 3; i++) worker.handle(invalid);
    assertTrue(redis.acked.isEmpty());
    assertEquals(3L, redis.counts.get(worker.attemptKey() + ":2-0"));
  }

  @Test
  void failedProcessedSetKeepsOriginalDuplicatePublicationWindow() throws Exception {
    var redis = new FakeRedis();
    redis.fail = op -> op.equals("set:" + Worker.processedKey("a"));
    var worker = worker(redis);
    worker.handle(message("1-0", "a"));
    worker.handle(message("1-0", "a"));
    assertEquals(2, redis.added.size());
    assertTrue(redis.acked.isEmpty());
    assertFalse(redis.strings.containsKey(Worker.processedKey("a")));
  }

  @Test
  void ackFailureDoesNotRepublishAfterProcessedMarker() throws Exception {
    var redis = new FakeRedis();
    redis.fail = op -> op.startsWith("ack:");
    var worker = worker(redis);
    worker.handle(message("1-0", "a"));
    redis.fail = op -> false;
    worker.handle(message("1-0", "a"));
    assertEquals(1, redis.added.size());
    assertEquals(List.of("1-0"), redis.acked);
  }

  @Test
  void pendingClaimProcessesBatchAndCleanupPreservesStreamLimits() throws Exception {
    var redis = new FakeRedis();
    var worker = worker(redis);
    redis.pending = List.of(message("1-0", "a"));
    worker.claimPending();
    worker.cleanup();
    assertTrue(redis.calls.contains("claim:16"));
    assertTrue(redis.calls.contains("trim:cotriage.events.raw:100000"));
    assertTrue(redis.calls.contains("trim:cotriage.events.inferred:100000"));
    assertTrue(redis.calls.contains("trim:cotriage.events.dlq:10000"));
    assertEquals(List.of("1-0"), redis.acked);
  }

  @Test
  void emptyEventIdAndPayloadAreRejected() throws Exception {
    var worker = worker(new FakeRedis());
    assertThrows(Exception.class, () -> worker.process(new RedisPort.Message("1", Map.of())));
    assertThrows(
        Exception.class, () -> worker.process(new RedisPort.Message("1", Map.of("event_id", "a"))));
  }

  @Test
  void jsonUnmarshalContractsRejectTrailingAndCoercionButAcceptNullScalars() throws Exception {
    assertThrows(Exception.class, () -> EdgeJson.read("{\"event_id\":3}", Models.RawEvent.class));
    assertThrows(
        Exception.class, () -> EdgeJson.read("{\"event_id\":\"a\"} {}", Models.RawEvent.class));
    assertThrows(
        Exception.class, () -> EdgeJson.read("{\"severity\":\"78\"}", Models.LlmResult.class));
    var raw = EdgeJson.read("{\"event_id\":null,\"log\":null,\"extra\":3}", Models.RawEvent.class);
    assertEquals("", raw.eventId);
    assertNotNull(raw.log);
    assertEquals("", raw.log.raw);
    assertEquals(
        "ALERT",
        EdgeJson.readOne("{\"label\":\"ALERT\"} ignored", Models.ClassifierResult.class).label);
  }

  @Test
  void invalidPositiveSettingsFailButZeroClaimIntervalKeepsLegacyStartupFailure() throws Exception {
    assertThrows(
        IllegalArgumentException.class, () -> Config.load(Map.of("MAX_ATTEMPTS", "0"), "host"));
    assertThrows(
        IllegalArgumentException.class,
        () -> Config.load(Map.of("REDIS_RETRY_DELAY", "0s"), "host"));
    assertThrows(
        IllegalArgumentException.class, () -> Config.load(Map.of("LOG_MAX_BYTES", "127"), "host"));
    var cfg = Config.load(Map.of("CLAIM_IDLE_TIME", "0s"), "host");
    assertThrows(
        IllegalArgumentException.class,
        () ->
            new Worker(new FakeRedis(), cfg, EngineContractTest.engine(new FakeRedis(), null, null))
                .run());
    assertEquals(
        Duration.ofMillis(1500),
        Config.load(Map.of("CLASSIFIER_TIMEOUT", "1.5s"), "host").classifierTimeout());
  }
}
