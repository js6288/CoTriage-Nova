package io.cotriage.nova.edgeinfer;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.WebApplicationType;
import org.springframework.boot.autoconfigure.SpringBootApplication;

@SpringBootApplication(
    excludeName = "org.springframework.boot.jdbc.autoconfigure.DataSourceAutoConfiguration")
public class EdgeInferApplication {
  private static final Logger LOG = LoggerFactory.getLogger(EdgeInferApplication.class);

  public static void main(String[] args) throws Exception {
    SpringApplication app = new SpringApplication(EdgeInferApplication.class);
    app.setWebApplicationType(WebApplicationType.NONE);
    var context = app.run(args);
    Config cfg = Config.load();
    Thread workerThread = Thread.currentThread();
    Thread shutdown = Thread.ofPlatform().unstarted(workerThread::interrupt);
    Runtime.getRuntime().addShutdownHook(shutdown);
    try (var redis = new LettuceRedis(cfg.redisAddr())) {
      var engine =
          new Engine(
              redis,
              RuleEngine.load(cfg.rulesFile()),
              Engine.loadTemplates(cfg.templatesFile()),
              cfg.templateCacheTtl(),
              cfg.logMaxBytes(),
              ModelClients.localClassifier(cfg.classifierUrl(), cfg.classifierTimeout()),
              ModelClients.localLlm(cfg.llmUrl(), cfg.llmTimeout()));
      try (var sync =
          new RemoteConfig(engine, cfg.edgeConfigUrl(), cfg.configRefresh(), cfg.configTimeout())) {
        sync.start();
        LOG.info(
            "[edge-infer] start redis={} raw={} inferred={} group={} consumer={} classifier={}"
                + " edge_llm={} edge_config={}",
            cfg.redisAddr(),
            cfg.rawStream(),
            cfg.inferredStream(),
            cfg.consumerGroup(),
            cfg.consumerName(),
            !cfg.classifierUrl().isEmpty(),
            !cfg.llmUrl().isEmpty(),
            !cfg.edgeConfigUrl().isEmpty());
        try {
          new Worker(redis, cfg, engine).run();
        } catch (InterruptedException ex) {
          Thread.currentThread().interrupt();
        }
      }
    } finally {
      context.close();
      try {
        Runtime.getRuntime().removeShutdownHook(shutdown);
      } catch (IllegalStateException ignored) {
      }
    }
    LOG.info("[edge-infer] exit");
  }
}
