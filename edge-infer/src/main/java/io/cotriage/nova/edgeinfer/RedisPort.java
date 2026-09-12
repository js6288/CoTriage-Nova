package io.cotriage.nova.edgeinfer;

import java.time.Duration;
import java.util.List;
import java.util.Map;

/** The order of these operations is part of the original worker's at-least-once behavior. */
interface RedisPort extends AutoCloseable {
  record Message(String id, Map<String, String> values) {}

  void ping();

  void createGroup(String stream, String group);

  List<Message> readGroup(String stream, String group, String consumer, int count, Duration block);

  List<Message> autoClaim(String stream, String group, String consumer, Duration idle, int count);

  void trim(String stream, long maxLen);

  boolean exists(String key);

  String get(String key);

  void set(String key, String value, Duration ttl);

  void expire(String key, Duration ttl);

  long increment(String key, String field);

  void add(String stream, long maxLen, Map<String, String> values);

  void ackAndDeleteAttempt(String stream, String group, String id, String attemptKey);

  @Override
  void close();
}
