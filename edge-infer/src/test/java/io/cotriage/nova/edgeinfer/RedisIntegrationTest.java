package io.cotriage.nova.edgeinfer;

import static org.junit.jupiter.api.Assertions.*;

import io.cotriage.nova.common.RedisSupport;
import io.lettuce.core.Consumer;
import io.lettuce.core.XReadArgs;
import java.time.Duration;
import java.util.Map;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.testcontainers.containers.GenericContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;

@Testcontainers(disabledWithoutDocker = true)
class RedisIntegrationTest {
  @Container
  static final GenericContainer<?> REDIS =
      new GenericContainer<>("redis:7.4-alpine").withExposedPorts(6379);

  private String address() {
    return REDIS.getHost() + ":" + REDIS.getMappedPort(6379);
  }

  @BeforeEach
  void reset() {
    try (var client = RedisSupport.client(address());
        var connection = client.connect()) {
      connection.sync().flushall();
    }
  }

  @Test
  void realRedisPublishAckDedupeAndCacheTtl() throws Exception {
    var cfg = Config.load(Map.of("REDIS_ADDR", address()), "java-consumer");
    try (var redis = new LettuceRedis(address());
        var client = RedisSupport.client(address());
        var connection = client.connect()) {
      var engine =
          new Engine(
              redis,
              RuleEngine.load(""),
              Map.of(),
              cfg.templateCacheTtl(),
              cfg.logMaxBytes(),
              null,
              null);
      var worker = new Worker(redis, cfg, engine);
      worker.ensureGroup();
      worker.ensureGroup();
      var input = WorkerContractTest.message("unused", "a");
      redis.add(cfg.rawStream(), 100000, input.values());
      var messages =
          redis.readGroup(
              cfg.rawStream(), cfg.consumerGroup(), cfg.consumerName(), 16, Duration.ofMillis(50));
      assertEquals(1, messages.size());
      worker.handle(messages.getFirst());
      assertEquals(0, connection.sync().xpending(cfg.rawStream(), cfg.consumerGroup()).getCount());
      assertEquals(1, connection.sync().xlen(cfg.inferredStream()));
      assertEquals("1", connection.sync().get(Worker.processedKey("a")));
      assertTrue(connection.sync().ttl(Worker.processedKey("a")) > 86000);
      var result =
          connection
              .sync()
              .xrange(cfg.inferredStream(), io.lettuce.core.Range.unbounded())
              .getFirst();
      var payload = EdgeJson.MAPPER.readTree(result.getBody().get("payload"));
      assertEquals("a", payload.path("event_id").asText());
      assertEquals("DISK_ERROR", payload.path("edge_pred").asText());
      assertTrue(
          connection
                  .sync()
                  .pttl(Engine.cacheKey(payload.path("template_id").asText(), "edge-rules-v1"))
              > 0);
      redis.add(cfg.rawStream(), 100000, input.values());
      worker.handle(
          redis
              .readGroup(
                  cfg.rawStream(),
                  cfg.consumerGroup(),
                  cfg.consumerName(),
                  16,
                  Duration.ofMillis(50))
              .getFirst());
      assertEquals(1, connection.sync().xlen(cfg.inferredStream()));
    }
  }

  @Test
  void realPendingClaimRetriesToDlqAndClearsAttempts() throws Exception {
    var cfg =
        Config.load(Map.of("REDIS_ADDR", address(), "CLAIM_IDLE_TIME", "10ms"), "new-consumer");
    try (var redis = new LettuceRedis(address());
        var client = RedisSupport.client(address());
        var connection = client.connect()) {
      var worker =
          new Worker(
              redis,
              cfg,
              new Engine(
                  redis,
                  RuleEngine.load(""),
                  Map.of(),
                  cfg.templateCacheTtl(),
                  cfg.logMaxBytes(),
                  null,
                  null));
      worker.ensureGroup();
      redis.add(cfg.rawStream(), 100000, Map.of("event_id", "a", "payload", "{}"));
      var original =
          connection
              .sync()
              .xreadgroup(
                  Consumer.from(cfg.consumerGroup(), "dead-consumer"),
                  XReadArgs.StreamOffset.lastConsumed(cfg.rawStream()))
              .getFirst();
      for (int i = 0; i < 3; i++) {
        Thread.sleep(20);
        worker.claimPending();
      }
      assertEquals(0, connection.sync().xpending(cfg.rawStream(), cfg.consumerGroup()).getCount());
      assertEquals(1, connection.sync().xlen(cfg.dlqStream()));
      var dlq =
          connection
              .sync()
              .xrange(cfg.dlqStream(), io.lettuce.core.Range.unbounded())
              .getFirst()
              .getBody();
      assertEquals(original.getId(), dlq.get("source_message_id"));
      assertEquals("3", dlq.get("attempt"));
      assertNull(connection.sync().hget(worker.attemptKey(), original.getId()));
      worker.cleanup();
    }
  }
}
