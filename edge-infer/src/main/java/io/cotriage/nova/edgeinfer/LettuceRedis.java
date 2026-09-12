package io.cotriage.nova.edgeinfer;

import io.cotriage.nova.common.RedisSupport;
import io.lettuce.core.Consumer;
import io.lettuce.core.RedisClient;
import io.lettuce.core.SetArgs;
import io.lettuce.core.XAddArgs;
import io.lettuce.core.XAutoClaimArgs;
import io.lettuce.core.XGroupCreateArgs;
import io.lettuce.core.XReadArgs;
import io.lettuce.core.XTrimArgs;
import io.lettuce.core.api.StatefulRedisConnection;
import io.lettuce.core.api.sync.RedisCommands;
import java.time.Duration;
import java.util.List;
import java.util.Map;

final class LettuceRedis implements RedisPort {
  private final RedisClient client;
  private StatefulRedisConnection<String, String> connection;

  LettuceRedis(String address) {
    client = RedisSupport.client(address);
  }

  private synchronized RedisCommands<String, String> commands() {
    // Connection must be lazy: the worker is allowed to start before Redis.
    if (connection == null) connection = client.connect();
    return connection.sync();
  }

  @Override
  public void ping() {
    commands().ping();
  }

  @Override
  public void createGroup(String stream, String group) {
    commands()
        .xgroupCreate(
            XReadArgs.StreamOffset.from(stream, "0"), group, XGroupCreateArgs.Builder.mkstream());
  }

  @Override
  public List<Message> readGroup(
      String stream, String group, String consumer, int count, Duration block) {
    var args = XReadArgs.Builder.count(count);
    if (!block.isNegative()) args.block(block);
    var messages =
        commands()
            .xreadgroup(
                Consumer.from(group, consumer), args, XReadArgs.StreamOffset.lastConsumed(stream));
    return messages == null
        ? List.of()
        : messages.stream()
            .map(message -> new Message(message.getId(), message.getBody()))
            .toList();
  }

  @Override
  public List<Message> autoClaim(
      String stream, String group, String consumer, Duration idle, int count) {
    var args =
        XAutoClaimArgs.Builder.xautoclaim(Consumer.from(group, consumer), idle.toMillis(), "0-0")
            .count(count);
    // Deliberately discard the next cursor, as the Go worker always restarts at 0-0.
    return commands().xautoclaim(stream, args).getMessages().stream()
        .map(message -> new Message(message.getId(), message.getBody()))
        .toList();
  }

  @Override
  public void trim(String stream, long maxLen) {
    commands().xtrim(stream, XTrimArgs.Builder.maxlen(maxLen).approximateTrimming());
  }

  @Override
  public boolean exists(String key) {
    return commands().exists(key) > 0;
  }

  @Override
  public String get(String key) {
    return commands().get(key);
  }

  @Override
  public void set(String key, String value, Duration ttl) {
    if (ttl.equals(Duration.ofNanos(-1))) commands().set(key, value, SetArgs.Builder.keepttl());
    else if (ttl.isZero() || ttl.isNegative()) commands().set(key, value);
    else commands().set(key, value, SetArgs.Builder.px(Math.max(1, ttl.toMillis())));
  }

  @Override
  public void expire(String key, Duration ttl) {
    commands().expire(key, ttl.isPositive() ? Math.max(1, ttl.toSeconds()) : ttl.toSeconds());
  }

  @Override
  public long increment(String key, String field) {
    return commands().hincrby(key, field, 1);
  }

  @Override
  public void add(String stream, long maxLen, Map<String, String> values) {
    commands().xadd(stream, XAddArgs.Builder.maxlen(maxLen).approximateTrimming(), values);
  }

  @Override
  public void ackAndDeleteAttempt(String stream, String group, String id, String attemptKey) {
    var commands = commands();
    commands.multi();
    try {
      commands.xack(stream, group, id);
      commands.hdel(attemptKey, id);
      commands.exec();
    } catch (RuntimeException ex) {
      try {
        commands.discard();
      } catch (RuntimeException ignored) {
      }
      throw ex;
    }
  }

  @Override
  public synchronized void close() {
    if (connection != null) connection.close();
    client.shutdown();
  }
}
