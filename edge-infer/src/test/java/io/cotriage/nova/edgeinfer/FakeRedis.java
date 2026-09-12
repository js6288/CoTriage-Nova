package io.cotriage.nova.edgeinfer;

import java.time.Duration;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.function.Predicate;

class FakeRedis implements RedisPort {
  record Added(String stream, Map<String, String> values) {}

  final Map<String, String> strings = new HashMap<>();
  final Map<String, Long> counts = new HashMap<>();
  final Map<String, Duration> ttls = new HashMap<>();
  final List<Added> added = new ArrayList<>();
  final List<String> calls = new ArrayList<>();
  final List<String> acked = new ArrayList<>();
  List<Message> pending = List.of();
  Predicate<String> fail = operation -> false;

  private void record(String operation) {
    calls.add(operation);
    if (fail.test(operation)) throw new IllegalStateException("injected " + operation);
  }

  @Override
  public void ping() {
    record("ping");
  }

  @Override
  public void createGroup(String stream, String group) {
    record("group:" + stream + ":" + group);
  }

  @Override
  public List<Message> readGroup(
      String stream, String group, String consumer, int count, Duration block) {
    record("read");
    return List.of();
  }

  @Override
  public List<Message> autoClaim(
      String stream, String group, String consumer, Duration idle, int count) {
    record("claim:" + count);
    return pending;
  }

  @Override
  public void trim(String stream, long maxLen) {
    record("trim:" + stream + ":" + maxLen);
  }

  @Override
  public boolean exists(String key) {
    record("exists:" + key);
    return strings.containsKey(key);
  }

  @Override
  public String get(String key) {
    record("get:" + key);
    return strings.get(key);
  }

  @Override
  public void set(String key, String value, Duration ttl) {
    record("set:" + key);
    strings.put(key, value);
    ttls.put(key, ttl);
  }

  @Override
  public void expire(String key, Duration ttl) {
    record("expire:" + key);
    ttls.put(key, ttl);
  }

  @Override
  public long increment(String key, String field) {
    record("increment:" + key + ":" + field);
    return counts.merge(key + ":" + field, 1L, Long::sum);
  }

  @Override
  public void add(String stream, long maxLen, Map<String, String> values) {
    record("add:" + stream);
    added.add(new Added(stream, new HashMap<>(values)));
  }

  @Override
  public void ackAndDeleteAttempt(String stream, String group, String id, String key) {
    record("ack:" + id);
    acked.add(id);
    counts.remove(key + ":" + id);
  }

  @Override
  public void close() {}
}
