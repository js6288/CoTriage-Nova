package io.cotriage.nova.server;

import static io.cotriage.nova.server.Values.*;

import io.cotriage.nova.common.Json;
import io.cotriage.nova.common.RedisSupport;
import io.lettuce.core.*;
import io.lettuce.core.api.StatefulRedisConnection;
import io.lettuce.core.api.sync.RedisCommands;
import java.io.*;
import java.nio.charset.StandardCharsets;
import java.time.*;
import java.util.*;

public class EventStore implements AutoCloseable {
  final RedisClient client;
  private StatefulRedisConnection<String, String> connection;
  public final String raw = env("REDIS_STREAM", "cotriage.events.raw"),
      inferred = env("INFERRED_STREAM", "cotriage.events.inferred"),
      routed = env("ROUTED_STREAM", "cotriage.events.routed"),
      cloudReq = env("CLOUD_REQ_STREAM", "cotriage.events.cloud.req"),
      cloudResp = env("CLOUD_RESP_STREAM", "cotriage.events.cloud.resp"),
      dlq = env("DLQ_STREAM", "cotriage.events.dlq");

  public EventStore(String addr) {
    var uri = RedisSupport.uri(addr);
    uri.setTimeout(Duration.ofSeconds(3));
    client = RedisClient.create(uri);
  }

  protected synchronized RedisCommands<String, String> redis() {
    if (connection == null || !connection.isOpen()) connection = client.connect();
    return connection.sync();
  }

  public long length(String stream) {
    try {
      return redis().xlen(stream);
    } catch (Exception e) {
      return 0;
    }
  }

  public long pending() {
    try {
      return redis().xpending(raw, "edge-infer-g").getCount();
    } catch (Exception e) {
      return 0;
    }
  }

  public long recentCount(Instant since) {
    try {
      return redis()
          .xrange(raw, Range.create(since.toEpochMilli() + "-0", "+"), Limit.from(2000))
          .size();
    } catch (Exception e) {
      return 0;
    }
  }

  public List<Map<String, Object>> recent(int count) {
    try {
      var result = new ArrayList<Map<String, Object>>();
      for (var m : redis().xrevrange(inferred, Range.unbounded(), Limit.from(count))) {
        var payload = decode(m.getBody());
        if (payload != null) result.add(payload);
      }
      return result;
    } catch (Exception e) {
      throw new ApiFailure(503, "inferred stream unavailable: " + e.getMessage());
    }
  }

  static Map<String, Object> decode(Map<String, String> fields) {
    var text = fields.get("payload");
    if (text == null) return null;
    return InferredPayload.decode(text);
  }

  public Map<String, Object> deviceStats(String equipment) {
    var result = obj("last_event_time", null, "ingest_rate_1m", 0);
    if (equipment.isBlank()) return result;
    try {
      var latest = redis().xrevrange(raw, Range.unbounded(), Limit.from(1));
      if (latest.isEmpty()) return result;
      var id = latest.getFirst().getId();
      long now = Long.parseLong(id.split("-")[0]);
      var lookback =
          io.cotriage.nova.common.Env.duration("EVENTS_LOOKBACK", Duration.ofSeconds(60));
      int count = 0;
      for (var msg : redis().xrange(raw, Range.create((now - lookback.toMillis()) + "-0", id))) {
        var body = msg.getBody().get("payload");
        if (body == null) continue;
        var p = json(body);
        if (p instanceof Map<?, ?> map
            && equipment.equals(Objects.toString(map.get("equipment"), "").trim())) {
          count++;
          var ts = Objects.toString(map.get("ts"), "").trim();
          if (!ts.isEmpty()) result.put("last_event_time", ts);
        }
      }
      result.put("ingest_rate_1m", count);
    } catch (Exception ignored) {
    }
    return result;
  }

  /** Each blocking reader owns a connection, so one SSE subscriber cannot block REST reads. */
  public void stream(OutputStream out, boolean router, String lastId) throws IOException {
    try (var dedicated = client.connect()) {
      dedicated.setTimeout(Duration.ofSeconds(20));
      var commands = dedicated.sync();
      var stream = router ? routed : inferred;
      String cursor = lastId;
      out.flush();
      while (!Thread.currentThread().isInterrupted()) {
        List<StreamMessage<String, String>> messages;
        try {
          messages =
              commands.xread(
                  new XReadArgs().count(20).block(Duration.ofSeconds(15)),
                  XReadArgs.StreamOffset.from(stream, cursor));
        } catch (Exception e) {
          if (router)
            write(out, "event: degraded\ndata: {\"error\":\"router stream unavailable\"}\n\n");
          return;
        }
        if (messages == null || messages.isEmpty()) {
          write(out, ": keepalive\n\n");
          continue;
        }
        for (var message : messages) {
          Object data;
          if (router) {
            String p = message.getBody().get("payload");
            data = p == null ? message.getBody() : validJson(p) ? json(p) : obj("payload", p);
          } else {
            data = decode(message.getBody());
            if (data == null) continue;
          }
          cursor = message.getId();
          write(
              out,
              "id: "
                  + cursor
                  + "\nevent: "
                  + (router ? "routed" : "inferred")
                  + "\ndata: "
                  + Json.write(data)
                  + "\n\n");
        }
      }
    } catch (IOException e) {
      throw e;
    } catch (Exception e) {
      if (router)
        write(out, "event: degraded\ndata: {\"error\":\"router stream unavailable\"}\n\n");
    }
  }

  static void write(OutputStream out, String text) throws IOException {
    out.write(text.getBytes(StandardCharsets.UTF_8));
    out.flush();
  }

  @Override
  public synchronized void close() {
    if (connection != null) connection.close();
    client.shutdown();
  }
}
