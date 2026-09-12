package io.cotriage.nova.edgeinfer;

import static io.cotriage.nova.edgeinfer.ModelClients.*;
import static io.cotriage.nova.edgeinfer.Models.*;

import com.fasterxml.jackson.core.type.TypeReference;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

final class Engine {
  private record Knowledge(RuleEngine rules, Map<String, TemplateResult> templates) {}

  private record Clients(Classifier classifier, EdgeLlm llm) {}

  private final RedisPort redis;
  private final Duration cacheTtl;
  private final int maxLogBytes;
  private Knowledge knowledge;
  private Clients clients;

  Engine(
      RedisPort redis,
      RuleEngine rules,
      Map<String, TemplateResult> templates,
      Duration cacheTtl,
      int maxLogBytes,
      Classifier classifier,
      EdgeLlm llm) {
    this.redis = redis;
    this.knowledge = new Knowledge(rules, templates);
    this.cacheTtl = cacheTtl;
    this.maxLogBytes = maxLogBytes;
    this.clients = new Clients(classifier, llm);
  }

  InferredEvent infer(RawEvent raw) throws Exception {
    long start = System.nanoTime();
    if (Normalizer.trim(raw.eventId).isEmpty()) throw new IOException("event_id is empty");
    if (Normalizer.trim(raw.log.raw).isEmpty()) throw new IOException("log.raw is empty");
    long normalizeStart = System.nanoTime();
    var normalized = Normalizer.normalizeResult(raw.log.raw, maxLogBytes);
    String templateText = normalized.text(), templateId = normalized.id();
    Knowledge data = knowledgeSnapshot();
    Clients models = modelSnapshot();
    String ruleVersion = data.rules.version();
    var result = baseResult(raw, templateId, templateText, ruleVersion);
    result.normalizeMs = elapsed(normalizeStart);
    var matched = data.rules.match(templateText);
    if (matched != null) {
      apply(result, matched, "RULE");
      result.finish(start);
      cacheBestEffort(result, templateId, matched, ruleVersion);
      return result;
    }
    long cacheStart = System.nanoTime();
    var cached = data.templates.get(templateId);
    if (cached != null) {
      apply(result, cached, "CACHE");
      result.cacheMs = elapsed(cacheStart);
      result.finish(start);
      return result;
    }
    try {
      cached = loadCache(templateId, ruleVersion);
    } catch (Exception ex) {
      result.evidence.add("template_cache_error=" + ex.getMessage());
    }
    if (cached != null) {
      apply(result, cached, "CACHE");
      result.cacheMs = elapsed(cacheStart);
      result.finish(start);
      return result;
    }
    result.cacheMs = elapsed(cacheStart);
    if (models.classifier != null) {
      long modelStart = System.nanoTime();
      ClassifierResult classified;
      try {
        classified =
            models.classifier.predict(new ClassifierInput(raw.eventId, templateId, templateText));
      } catch (Exception ex) {
        result.modelMs = elapsed(modelStart);
        fallback(result, "classifier_error=" + ex.getMessage());
        result.finish(start);
        return result;
      }
      result.modelMs = elapsed(modelStart);
      var modelResult = resultFromClassifier(classified);
      apply(result, modelResult, "MODEL");
      result.modelVersion = classified.modelVersion;
      if (models.llm != null && modelResult.edgePred.equals("ABNORMAL")) {
        long llmStart = System.nanoTime();
        try {
          var diagnosis = models.llm.diagnose(new LlmInput(raw, result, classified));
          result.edgeLlmMs = elapsed(llmStart);
          var llmResult = resultFromLlm(diagnosis);
          apply(result, llmResult, "EDGE_LLM");
          result.modelVersion = diagnosis.modelVersion;
          result.finish(start);
          cacheBestEffort(result, templateId, llmResult, ruleVersion);
          return result;
        } catch (Exception ex) {
          result.edgeLlmMs = elapsed(llmStart);
          result.evidence.add("edge_llm_error=" + ex.getMessage());
        }
      }
      result.finish(start);
      cacheBestEffort(result, templateId, modelResult, ruleVersion);
      return result;
    }
    fallback(result);
    result.finish(start);
    return result;
  }

  // Keep the two original update/snapshot steps; do not merge them into a stronger atomic
  // configuration contract.
  synchronized void updateKnowledge(RuleEngine rules, Map<String, TemplateResult> templates) {
    knowledge =
        new Knowledge(
            rules == null ? knowledge.rules : rules,
            templates == null ? knowledge.templates : templates);
  }

  synchronized void updateModels(Classifier classifier, EdgeLlm llm) {
    clients = new Clients(classifier, llm);
  }

  private synchronized Knowledge knowledgeSnapshot() {
    return knowledge;
  }

  private synchronized Clients modelSnapshot() {
    return clients;
  }

  String ruleVersion() {
    return knowledgeSnapshot().rules.version();
  }

  private static InferredEvent baseResult(RawEvent raw, String id, String text, String version) {
    var result = new InferredEvent();
    result.eventId = raw.eventId;
    result.source = raw.source;
    result.equipment = raw.equipment;
    result.eventTs = raw.ts;
    result.rawLog = Normalizer.truncateRawLog(raw.log.raw);
    result.templateId = id;
    result.templateText = text;
    result.ruleVersion = version;
    return result;
  }

  static void apply(InferredEvent output, TemplateResult input, String path) {
    output.edgePred = input.edgePred;
    output.severity = input.severity;
    output.confidence = input.confidence;
    output.uncertainty = 1 - input.confidence;
    output.evidence = input.evidence == null ? new ArrayList<>() : new ArrayList<>(input.evidence);
    output.adviceCode = input.adviceCode;
    output.advice = input.advice;
    output.inferencePath = path;
  }

  static void fallback(InferredEvent output, String... evidence) {
    output.edgePred = "UNKNOWN";
    output.severity = 50;
    output.confidence = 0;
    output.uncertainty = 1;
    output.inferencePath = "FALLBACK";
    output.evidence = new ArrayList<>(List.of(evidence));
  }

  private void cacheBestEffort(
      InferredEvent result, String id, TemplateResult value, String version) {
    try {
      cache(id, value, version);
    } catch (Exception ex) {
      result.evidence.add("template_cache_write_error=" + ex.getMessage());
    }
  }

  private void cache(String id, TemplateResult result, String version) {
    if (result.confidence < .85 || result.edgePred.equals("UNKNOWN")) return;
    redis.set(cacheKey(id, version), EdgeJson.write(result.toMap()), cacheTtl);
  }

  private TemplateResult loadCache(String id, String version) throws IOException {
    String body;
    try {
      body = redis.get(cacheKey(id, version));
    } catch (Exception ex) {
      throw new IOException("read template cache: " + ex.getMessage(), ex);
    }
    if (body == null) return null;
    try {
      var result = EdgeJson.read(body, TemplateResult.class);
      return result == null ? new TemplateResult() : result;
    } catch (IOException ex) {
      throw new IOException("decode template cache: " + ex.getMessage(), ex);
    }
  }

  static String cacheKey(String id, String version) {
    return "cotriage:edge-infer:template:" + id + ":none:" + version;
  }

  static Map<String, TemplateResult> loadTemplates(String path) throws IOException {
    var result = new HashMap<String, TemplateResult>();
    if (Normalizer.trim(path).isEmpty()) return result;
    List<TemplateResult> templates =
        EdgeJson.read(Files.readString(Path.of(path)), new TypeReference<>() {});
    if (templates == null) return result;
    for (var item : templates) {
      String id = Normalizer.trim(item.templateId);
      if (id.isEmpty() && !item.templateText.isEmpty())
        id = Normalizer.templateId(item.templateText);
      if (id.isEmpty())
        throw new IOException("template entry has neither template_id nor template_text");
      if (!PREDICTIONS.contains(item.edgePred))
        throw new IOException(
            "template \"" + id + "\" has invalid edge_pred \"" + item.edgePred + "\"");
      result.put(id, item);
    }
    return result;
  }
}
