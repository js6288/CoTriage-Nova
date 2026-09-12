package io.cotriage.nova.collector;

import io.cotriage.nova.common.Env;
import java.nio.file.Path;

public record CollectorConfig(
    Path logPath,
    String redisAddress,
    String stream,
    String nodeName,
    String equipment,
    long streamMaxLen) {
  public static CollectorConfig load() {
    long max = Env.longValue("REDIS_STREAM_MAXLEN", 100000);
    return new CollectorConfig(
        Path.of(Env.get("LOG_PATH", "/var/log/cotriage/input/current.log")),
        Env.get("REDIS_ADDR", "redis:6379"),
        Env.get("REDIS_STREAM", "cotriage.events.raw"),
        Env.get("NODE_NAME", ""),
        Env.get("EQUIPMENT", ""),
        max > 0 ? max : 100000);
  }
}
