package io.cotriage.nova.router;

import io.cotriage.nova.common.Db;
import io.lettuce.core.*;
import io.lettuce.core.api.StatefulRedisConnection;
import java.time.Duration;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicBoolean;
import org.springframework.boot.*;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.context.annotation.Bean;
import org.springframework.jdbc.core.JdbcTemplate;

@SpringBootApplication(
    excludeName = "org.springframework.boot.jdbc.autoconfigure.DataSourceAutoConfiguration")
public class RouterApplication {
  public static void main(String[] args) {
    SpringApplication app = new SpringApplication(RouterApplication.class);
    app.setWebApplicationType(WebApplicationType.NONE);
    app.run(args);
  }

  @Bean
  ApplicationRunner runner() {
    return args -> {
      RouterConfig cfg = new RouterConfig();
      var ds = Db.open(cfg.dsn);
      RouterStore store = new RouterStore(new JdbcTemplate(ds));
      if (cfg.autoMigrate) store.ensureSchema();
      RedisClient client =
          RedisClient.create(cfg.redis.contains("://") ? cfg.redis : "redis://" + cfg.redis);
      StatefulRedisConnection<String, String> connection = client.connect();
      connection.setTimeout(cfg.redisPingTimeout);
      connection.sync().ping();
      connection.setTimeout(Duration.ofSeconds(60));
      RouterState state = new RouterState(cfg.stateWindow, cfg.gainAlpha);
      try {
        var snapshot = store.latestState();
        if (snapshot.isPresent()) state = RouterState.restore(snapshot.get());
      } catch (Exception e) {
        RouterRuntime.log("restore state snapshot", e);
      }
      RouterRuntime runtime = new RouterRuntime(cfg, store, connection.sync());
      RouterWorker worker =
          new RouterWorker(
              cfg,
              connection.sync(),
              store,
              new RaiseEngine(RaiseEngine.Config.fromEnv()),
              state,
              runtime);
      var readConnection = client.connect();
      worker.reader = readConnection.sync();
      AtomicBoolean running = new AtomicBoolean(true);
      ExecutorService threads = Executors.newCachedThreadPool();
      Runtime.getRuntime()
          .addShutdownHook(
              new Thread(
                  () -> {
                    running.set(false);
                    threads.shutdownNow();
                    readConnection.close();
                    connection.close();
                    client.shutdown();
                    ds.close();
                  }));
      threads.submit(
          () -> {
            try {
              worker.ensureGroup();
            } catch (Exception e) {
              RouterRuntime.log("ensure consumer group", e);
            }
            while (running.get() && !Thread.currentThread().isInterrupted())
              try {
                worker.readOnce();
              } catch (Exception e) {
                if (running.get()) RouterRuntime.log("main worker tick", e);
              }
          });
      if (cfg.replayerEnabled) {
        var replayer =
            new OutboxReplayer(cfg, store, OutboxReplayer.redis(connection.sync(), cfg.cloud));
        threads.submit(() -> loop(running, cfg.replayerInterval, replayer::replayOnce));
      }
      if (cfg.feedbackEnabled) {
        var feedback = new FeedbackProcessor(store, state, cfg.feedbackBatch);
        threads.submit(() -> loop(running, cfg.feedbackInterval, feedback::processOnce));
      }
      System.out.printf(
          "[router-controller] ready redis=%s inferred=%s routed=%s cloud_req=%s group=%s"
              + " consumer=%s%n",
          cfg.redis, cfg.inferred, cfg.routed, cfg.cloud, cfg.group, cfg.consumer);
    };
  }

  static void loop(AtomicBoolean running, Duration period, Runnable task) {
    long next = System.nanoTime();
    while (running.get() && !Thread.currentThread().isInterrupted()) {
      try {
        task.run();
      } catch (Exception e) {
        RouterRuntime.log("worker tick", e);
      }
      next += period.toNanos();
      long wait = next - System.nanoTime();
      if (wait <= 0) {
        next = System.nanoTime();
        continue;
      }
      try {
        TimeUnit.NANOSECONDS.sleep(wait);
      } catch (InterruptedException e) {
        Thread.currentThread().interrupt();
        break;
      }
    }
  }
}
