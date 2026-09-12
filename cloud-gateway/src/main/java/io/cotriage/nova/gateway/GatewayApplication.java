package io.cotriage.nova.gateway;

import io.cotriage.nova.common.Db;
import io.lettuce.core.*;
import io.lettuce.core.api.StatefulRedisConnection;
import java.util.concurrent.*;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.concurrent.atomic.AtomicBoolean;
import org.springframework.boot.*;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.context.annotation.Bean;
import org.springframework.jdbc.core.JdbcTemplate;

@SpringBootApplication(
    excludeName = "org.springframework.boot.jdbc.autoconfigure.DataSourceAutoConfiguration")
public class GatewayApplication {
  public static void main(String[] args) {
    SpringApplication app = new SpringApplication(GatewayApplication.class);
    app.setWebApplicationType(WebApplicationType.NONE);
    app.run(args);
  }

  @Bean
  ApplicationRunner runner() {
    return args -> {
      GatewayConfig cfg = new GatewayConfig();
      if (!cfg.enabled) {
        System.out.println("[cloud-gateway] disabled");
        return;
      }
      var ds = Db.open(cfg.dsn);
      RedisClient client =
          RedisClient.create(cfg.redis.contains("://") ? cfg.redis : "redis://" + cfg.redis);
      var connections = new CopyOnWriteArrayList<StatefulRedisConnection<String, String>>();
      var manager =
          new Provider.Manager(cfg.server + cfg.endpoint, cfg.configTimeout, cfg.configToken);
      try {
        manager.refresh();
      } catch (Exception e) {
        GatewayWorker.log("initial config", e);
      }
      AtomicBoolean running = new AtomicBoolean(true);
      ExecutorService threads = Executors.newCachedThreadPool();
      Runtime.getRuntime()
          .addShutdownHook(
              new Thread(
                  () -> {
                    running.set(false);
                    threads.shutdownNow();
                    connections.forEach(StatefulRedisConnection::close);
                    client.shutdown();
                    ds.close();
                  }));
      threads.submit(
          () -> {
            while (running.get() && !Thread.currentThread().isInterrupted()) {
              try {
                TimeUnit.NANOSECONDS.sleep(cfg.poll.toNanos());
                if (running.get()) manager.refresh();
              } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                return;
              } catch (Exception e) {
                GatewayWorker.log("refresh config", e);
              }
            }
          });
      threads.submit(
          () -> {
            while (running.get() && !Thread.currentThread().isInterrupted()) {
              StatefulRedisConnection<String, String> connection = null, readConnection = null;
              try {
                connection = client.connect();
                connections.add(connection);
                connection.setTimeout(cfg.readTimeout);
                readConnection = client.connect();
                connections.add(readConnection);
                readConnection.setTimeout(cfg.readTimeout);
                var aggregator =
                    new ContextAggregator(
                        connection.sync(), cfg.context, cfg.window, cfg.maxEvents, cfg.maxTokens);
                var worker =
                    new GatewayWorker(
                        cfg,
                        connection.sync(),
                        new GatewayStore(new JdbcTemplate(ds)),
                        manager,
                        new Provider.Client(),
                        aggregator);
                worker.reader = readConnection.sync();
                worker.run(running);
                return;
              } catch (Exception e) {
                if (running.get()) GatewayWorker.log("redis connection failed", e);
              } finally {
                if (readConnection != null) {
                  connections.remove(readConnection);
                  readConnection.close();
                }
                if (connection != null) {
                  connections.remove(connection);
                  connection.close();
                }
              }
              try {
                TimeUnit.NANOSECONDS.sleep(
                    (cfg.retry.isPositive() ? cfg.retry : java.time.Duration.ofSeconds(3))
                        .toNanos());
              } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                return;
              }
            }
          });
      System.out.printf(
          "[cloud-gateway] start stream=%s group=%s consumer=%s%n",
          cfg.request, cfg.group, cfg.consumer);
    };
  }
}
