package io.cotriage.nova.collector;

import io.cotriage.nova.common.*;
import io.lettuce.core.XAddArgs;
import java.util.concurrent.atomic.AtomicBoolean;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.boot.*;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.context.annotation.Bean;

@SpringBootApplication(
    excludeName = "org.springframework.boot.jdbc.autoconfigure.DataSourceAutoConfiguration")
public class CollectorApplication {
  private static final Logger LOG = LoggerFactory.getLogger(CollectorApplication.class);

  public static void main(String[] args) {
    var app = new SpringApplication(CollectorApplication.class);
    app.setWebApplicationType(WebApplicationType.NONE);
    app.run(args);
  }

  static String equipment(CollectorConfig cfg) {
    String result = cfg.equipment();
    try {
      if (cfg.nodeName().isBlank()) throw new IllegalArgumentException("NODE_NAME is empty");
      var client = Kube.client();
      client.setReadTimeout(0);
      var node =
          new io.kubernetes.client.openapi.apis.CoreV1Api(client)
              .readNode(cfg.nodeName())
              .execute();
      var labels = node.getMetadata() == null ? null : node.getMetadata().getLabels();
      String label = labels == null ? null : labels.get("cotriage.io/equipment");
      if (label == null || label.isBlank())
        throw new IllegalArgumentException("equipment label missing");
      result = label.trim();
    } catch (Exception e) {
      LOG.warn("get equipment from node label failed: {}", e.toString());
    }
    return result.isBlank() ? Env.get("HOSTNAME", "unknown") : result;
  }

  @Bean
  CommandLineRunner collector() {
    return args -> {
      CollectorConfig cfg = CollectorConfig.load();
      String equipment = equipment(cfg);
      var running = new AtomicBoolean(true);
      Thread hook = new Thread(() -> running.set(false), "collector-cancel");
      Runtime.getRuntime().addShutdownHook(hook);
      try (var sink = new RedisPublisher(cfg)) {
        try {
          sink.ping();
        } catch (Exception e) {
          LOG.warn("redis ping failed: {}", e.toString());
        }
        LOG.info(
            "start LOG_PATH={} STREAM={} EQUIPMENT={}", cfg.logPath(), cfg.stream(), equipment);
        new TailPublisher().run(cfg.logPath(), equipment, sink::publish, running::get);
      } finally {
        try {
          Runtime.getRuntime().removeShutdownHook(hook);
        } catch (IllegalStateException ignored) {
        }
      }
    };
  }

  static final class RedisPublisher implements AutoCloseable {
    private final CollectorConfig cfg;
    private final io.lettuce.core.RedisClient client;
    private io.lettuce.core.api.StatefulRedisConnection<String, String> connection;

    RedisPublisher(CollectorConfig cfg) {
      this.cfg = cfg;
      client = RedisSupport.client(cfg.redisAddress());
    }

    private io.lettuce.core.api.sync.RedisCommands<String, String> commands() {
      if (connection == null) {
        connection = client.connect();
        connection.setTimeout(java.time.Duration.ofSeconds(3));
      }
      return connection.sync();
    }

    void ping() {
      commands().ping();
    }

    void publish(java.util.Map<String, String> fields) {
      commands()
          .xadd(
              cfg.stream(),
              XAddArgs.Builder.maxlen(cfg.streamMaxLen()).approximateTrimming(),
              fields);
    }

    public void close() {
      if (connection != null) connection.close();
      client.shutdown();
    }
  }
}
