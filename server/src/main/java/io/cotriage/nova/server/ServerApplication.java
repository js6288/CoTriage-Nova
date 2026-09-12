package io.cotriage.nova.server;

import io.cotriage.nova.common.Db;
import java.util.*;
import javax.sql.DataSource;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.context.annotation.Bean;
import org.springframework.jdbc.core.JdbcTemplate;

@SpringBootApplication(
    excludeName = "org.springframework.boot.jdbc.autoconfigure.DataSourceAutoConfiguration")
public class ServerApplication {
  public static void main(String[] args) {
    TimeZone.setDefault(TimeZone.getTimeZone("UTC"));
    var listen = Values.env("LISTEN_ADDR", ":8080");
    var app = new SpringApplication(ServerApplication.class);
    var defaults = new HashMap<String, Object>();
    defaults.put("server.port", listen.substring(listen.lastIndexOf(':') + 1));
    if (listen.lastIndexOf(':') > 0)
      defaults.put("server.address", listen.substring(0, listen.lastIndexOf(':')));
    defaults.put("spring.threads.virtual.enabled", true);
    defaults.put("spring.lifecycle.timeout-per-shutdown-phase", "5s");
    defaults.put("server.shutdown", "graceful");
    defaults.put("spring.mvc.async.request-timeout", -1);
    app.setDefaultProperties(defaults);
    app.run(args);
  }

  @Bean(destroyMethod = "close")
  DataSource dataSource() {
    return Db.open(
        Values.env(
            "MYSQL_DSN",
            "cotriage:cotriage-pass@tcp(mysql.cotriage.svc.cluster.local:3306)/cotriage?charset=utf8mb4&parseTime=true&loc=UTC"));
  }

  @Bean
  JdbcTemplate jdbcTemplate(DataSource ds) {
    var jdbc = new JdbcTemplate(ds);
    jdbc.setQueryTimeout(10);
    return jdbc;
  }

  @Bean
  ConfigRepository configRepository(JdbcTemplate jdbc) {
    var repo = new ConfigRepository(jdbc);
    repo.initialize();
    return repo;
  }

  @Bean
  RouterRepository routerRepository(JdbcTemplate jdbc) {
    return new RouterRepository(jdbc);
  }

  @Bean(destroyMethod = "close")
  EventStore eventStore() {
    return new EventStore(Values.env("REDIS_ADDR", "redis:6379"));
  }

  @Bean
  DeviceService deviceService(EventStore events) throws Exception {
    return new DeviceService(events);
  }

  @Bean(destroyMethod = "close")
  MetricsService metricsService(EventStore events, RouterRepository router) {
    var service = new MetricsService(events, router);
    service.start();
    return service;
  }
}
