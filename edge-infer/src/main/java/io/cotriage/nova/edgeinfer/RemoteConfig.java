package io.cotriage.nova.edgeinfer;

import static io.cotriage.nova.edgeinfer.ModelClients.*;
import static io.cotriage.nova.edgeinfer.Models.*;

import com.fasterxml.jackson.annotation.JsonProperty;
import com.google.re2j.Pattern;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

final class RemoteConfig implements AutoCloseable {
  private static final Logger LOG = LoggerFactory.getLogger(RemoteConfig.class);
  private final Engine engine;
  private final String url;
  private final Duration interval, timeout;
  private final HttpClient client =
      HttpClient.newBuilder().followRedirects(HttpClient.Redirect.ALWAYS).build();
  private String etag = "";
  private Thread thread;

  RemoteConfig(Engine engine, String url, Duration interval, Duration timeout) {
    this.engine = engine;
    this.url = Normalizer.trim(url);
    this.interval = interval;
    this.timeout = timeout;
  }

  void start() {
    if (url.isEmpty()) return;
    thread =
        Thread.ofVirtual()
            .name("edge-config-sync")
            .start(
                () -> {
                  fetchAndApply();
                  while (!Thread.currentThread().isInterrupted()) {
                    try {
                      Thread.sleep(interval);
                    } catch (InterruptedException ex) {
                      Thread.currentThread().interrupt();
                      return;
                    }
                    fetchAndApply();
                  }
                });
  }

  boolean fetchAndApply() {
    try {
      var builder = HttpRequest.newBuilder(URI.create(url)).timeout(timeout).GET();
      if (!etag.isEmpty()) builder.header("If-None-Match", etag);
      var response = client.send(builder.build(), HttpResponse.BodyHandlers.ofString());
      if (response.statusCode() == 304) return false;
      if (response.statusCode() < 200 || response.statusCode() >= 300) {
        LOG.warn(
            "[edge-infer] fetch edge config status={} body={}",
            response.statusCode(),
            Normalizer.preview(response.body(), 2048));
        return false;
      }
      var cfg = EdgeJson.readOne(response.body(), Document.class);
      if (cfg == null) cfg = new Document();
      RuleEngine rules = rulesFromRemote(cfg.rules.version, cfg.rules.items);
      Map<String, TemplateResult> templates = templatesFromRemote(cfg.templates.items);
      Classifier classifier = classifierFromRemote(cfg.classifier);
      EdgeLlm llm = llmFromRemote(cfg.provider);
      engine.updateKnowledge(rules, templates);
      engine.updateModels(classifier, llm);
      etag = Normalizer.trim(response.headers().firstValue("ETag").orElse(""));
      if (etag.isEmpty()) etag = Normalizer.trim(cfg.rules.etag + cfg.templates.etag);
      LOG.info(
          "[edge-infer] edge config applied rules={} templates={} classifier={} edge_llm={}"
              + " etag={}",
          cfg.rules.items.size(),
          cfg.templates.items.size(),
          classifier != null,
          llm != null,
          etag);
      return true;
    } catch (InterruptedException ex) {
      Thread.currentThread().interrupt();
      return false;
    } catch (Exception ex) {
      LOG.warn("[edge-infer] fetch/apply edge config error={}", ex.getMessage());
      return false;
    }
  }

  String etag() {
    return etag;
  }

  @Override
  public void close() {
    if (thread != null) thread.interrupt();
    client.close();
  }

  public static class Document {
    public RemoteClassifier classifier;

    @JsonProperty("active_provider")
    public RemoteProvider provider;

    public Rules rules = new Rules();
    public Templates templates = new Templates();
  }

  public static class Rules {
    public long version;
    public String etag = "";
    public List<RemoteRule> items = new ArrayList<>();
  }

  public static class Templates {
    public long version;
    public String etag = "";
    public List<RemoteTemplate> items = new ArrayList<>();
  }

  public static class RemoteClassifier {
    public String endpoint = "", model = "", status = "";
    public int timeoutMs;
    public double threshold;
    public boolean isActive;
  }

  public static class RemoteProvider {
    public String endpoint = "", model = "", status = "", responsePath = "";
    public int timeoutMs, maxTokens;
    public double temperature, topP;
    public Integer seed;
    public boolean isActive;
  }

  public static class RemoteRule {
    public long id;
    public String name = "",
        matchType = "",
        pattern = "",
        targetField = "",
        edgePred = "",
        adviceCode = "",
        advice = "",
        status = "";
    public List<String> keywords = new ArrayList<>(), evidence = new ArrayList<>();
    public int severity, priority;
    public double confidence;
    public boolean enabled;
  }

  public static class RemoteTemplate extends TemplateResult {
    public String status = "";
  }

  static Classifier classifierFromRemote(RemoteClassifier cfg) {
    if (cfg == null || !cfg.isActive || !cfg.status.equals("published")) return null;
    return classifier(cfg.endpoint, Duration.ofMillis(cfg.timeoutMs));
  }

  static EdgeLlm llmFromRemote(RemoteProvider cfg) {
    if (cfg == null || !cfg.isActive || !cfg.status.equals("published")) return null;
    return llm(
        cfg.endpoint,
        cfg.model,
        Duration.ofMillis(cfg.timeoutMs),
        cfg.maxTokens,
        cfg.temperature,
        cfg.topP,
        cfg.seed);
  }

  static RuleEngine rulesFromRemote(long version, List<RemoteRule> items) {
    var rules = new ArrayList<Rule>();
    for (var item : items) {
      if (!item.enabled || item.status.equals("disabled") || item.status.equals("deleted"))
        continue;
      var rule = new Rule();
      rule.pattern = remoteRulePattern(item);
      rule.id = Normalizer.trim(item.name);
      if (rule.id.isEmpty()) rule.id = Long.toString(item.id);
      rule.edgePred = item.edgePred;
      rule.severity = item.severity;
      rule.confidence = item.confidence;
      rule.evidence = item.evidence;
      rule.adviceCode = item.adviceCode;
      rule.advice = item.advice;
      rule.priority = item.priority;
      rule.enabled = true;
      rules.add(rule);
    }
    return new RuleEngine("edge-rules-" + version, rules);
  }

  static String remoteRulePattern(RemoteRule item) {
    String matchType = Normalizer.trim(item.matchType);
    return switch (matchType) {
      case "contains" -> {
        if (Normalizer.trim(item.pattern).isEmpty())
          throw new IllegalArgumentException("rule " + item.id + " contains pattern is empty");
        yield Pattern.quote(Normalizer.trim(item.pattern));
      }
      case "regex" -> Normalizer.trim(item.pattern);
      case "all_keywords", "any_keywords" -> {
        var parts = new ArrayList<String>();
        for (String keyword : item.keywords) {
          keyword = Normalizer.trim(keyword);
          if (!keyword.isEmpty())
            parts.add(
                matchType.equals("all_keywords")
                    ? "(?=.*" + Pattern.quote(keyword) + ")"
                    : Pattern.quote(keyword));
        }
        if (parts.isEmpty())
          throw new IllegalArgumentException("rule " + item.id + " " + matchType + " is empty");
        // Do not silently repair the original unsupported lookahead. RE2J must reject all_keywords.
        yield matchType.equals("all_keywords")
            ? String.join("", parts) + ".*"
            : "(" + String.join("|", parts) + ")";
      }
      default ->
          throw new IllegalArgumentException(
              "rule " + item.id + " has invalid match_type \"" + item.matchType + "\"");
    };
  }

  static Map<String, TemplateResult> templatesFromRemote(List<RemoteTemplate> items) {
    var result = new HashMap<String, TemplateResult>();
    for (var item : items) {
      if (item.status.equals("disabled") || item.status.equals("deleted")) continue;
      String id = Normalizer.trim(item.templateId);
      if (id.isEmpty()) id = Normalizer.templateId(item.templateText);
      if (!PREDICTIONS.contains(item.edgePred))
        throw new IllegalArgumentException(
            "template \"" + id + "\" has invalid edge_pred \"" + item.edgePred + "\"");
      var value = new TemplateResult();
      value.templateId = id;
      value.templateText = item.templateText;
      value.edgePred = item.edgePred;
      value.severity = item.severity;
      value.confidence = item.confidence;
      value.evidence = item.evidence;
      value.adviceCode = item.adviceCode;
      value.advice = item.advice;
      result.put(id, value);
    }
    return result;
  }
}
