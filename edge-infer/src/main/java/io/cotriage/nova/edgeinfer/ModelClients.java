package io.cotriage.nova.edgeinfer;

import static io.cotriage.nova.edgeinfer.Models.*;

import com.google.re2j.Pattern;
import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Locale;

final class ModelClients {
  @FunctionalInterface
  interface Classifier {
    ClassifierResult predict(ClassifierInput input) throws Exception;
  }

  @FunctionalInterface
  interface EdgeLlm {
    LlmResult diagnose(LlmInput input) throws Exception;
  }

  /**
   * The Go entrypoint passes a typed nil pointer into a non-nil interface. Preserve that failure
   * separately from a request error.
   */
  static final class LegacyModelPanic extends Error {
    LegacyModelPanic(String model) {
      super("legacy " + model + " nil-pointer invocation (local URL is empty)");
    }
  }

  private ModelClients() {}

  static Classifier localClassifier(String url, Duration timeout) {
    Classifier client = classifier(url, timeout);
    return client == null
        ? input -> {
          throw new LegacyModelPanic("classifier");
        }
        : client;
  }

  static EdgeLlm localLlm(String url, Duration timeout) {
    EdgeLlm client = llm(url, "qwen1.5-1.8b-chat", timeout, 384, .1, 1, null);
    return client == null
        ? input -> {
          throw new LegacyModelPanic("edge llm");
        }
        : client;
  }

  static Classifier classifier(String url, Duration timeout) {
    url = Normalizer.trim(url);
    if (url.isEmpty()) return null;
    Config.positive(timeout, "classifier timeout");
    String endpoint = url;
    HttpClient client = HttpClient.newBuilder().followRedirects(HttpClient.Redirect.ALWAYS).build();
    return input -> {
      String body = post(client, endpoint, timeout, input.toMap(), "classifier");
      ClassifierResult result;
      try {
        result = EdgeJson.readOne(body, ClassifierResult.class);
      } catch (IOException ex) {
        throw new IOException("decode classifier response: " + ex.getMessage(), ex);
      }
      if (result == null) result = new ClassifierResult();
      result.label = Normalizer.trim(result.label);
      if (!result.label.equals("ALERT") && !result.label.equals("NON_ALERT"))
        throw new IOException("classifier returned invalid label \"" + result.label + "\"");
      if (result.alertProbability < 0 || result.alertProbability > 1)
        throw new IOException(
            "classifier returned invalid alert_probability " + result.alertProbability);
      return result;
    };
  }

  static EdgeLlm llm(
      String url,
      String model,
      Duration timeout,
      int maxTokens,
      double temperature,
      double topP,
      Integer seed) {
    url = Normalizer.trim(url);
    if (url.isEmpty()) return null;
    model = Normalizer.trim(model);
    if (model.isEmpty()) model = "default";
    Config.positive(timeout, "llm timeout");
    String endpoint = url, modelName = model;
    int tokenLimit = maxTokens <= 0 ? 384 : maxTokens;
    HttpClient client = HttpClient.newBuilder().followRedirects(HttpClient.Redirect.ALWAYS).build();
    return input -> {
      var request =
          EdgeJson.object(
              "model",
              modelName,
              "messages",
              List.of(
                  EdgeJson.object("role", "system", "content", systemPrompt()),
                  EdgeJson.object("role", "user", "content", userPrompt(input))),
              "temperature",
              temperature,
              "top_p",
              topP,
              "max_tokens",
              tokenLimit,
              "stream",
              false);
      if (seed != null) request.put("seed", seed);
      String body = post(client, endpoint, timeout, request, "edge llm");
      var root = EdgeJson.treeOne(body);
      var choices = root == null ? null : root.get("choices");
      if (choices == null || !choices.isArray() || choices.isEmpty())
        throw new IOException("edge llm returned no choices");
      var content = choices.get(0).path("message").path("content");
      if (!content.isMissingNode() && !content.isNull() && !content.isTextual())
        throw new IOException("decode edge llm response: content must be string");
      var result = parseLlmJson(content.asText(""));
      if (result.modelVersion.isEmpty()) result.modelVersion = modelName;
      return result;
    };
  }

  private static String post(
      HttpClient client, String endpoint, Duration timeout, Object value, String name)
      throws Exception {
    var request =
        HttpRequest.newBuilder(URI.create(endpoint))
            .timeout(timeout)
            .header("Content-Type", "application/json")
            .POST(HttpRequest.BodyPublishers.ofString(EdgeJson.write(value)))
            .build();
    HttpResponse<String> response;
    try {
      response = client.send(request, HttpResponse.BodyHandlers.ofString());
    } catch (InterruptedException ex) {
      Thread.currentThread().interrupt();
      throw ex;
    } catch (Exception ex) {
      throw new IOException("call " + name + ": " + ex.getMessage(), ex);
    }
    if (response.statusCode() < 200 || response.statusCode() >= 300)
      throw new IOException(name + " status " + response.statusCode());
    return response.body();
  }

  static TemplateResult resultFromClassifier(ClassifierResult input) {
    var result = new TemplateResult();
    result.confidence = input.alertProbability;
    result.edgePred = "ABNORMAL";
    result.severity = 75;
    result.adviceCode = "CHECK_ALERT_CONTEXT";
    result.advice = "轻量分类模型判断该日志模板疑似异常，建议进入边缘 LLM 或云端进一步诊断。";
    if (input.label.equals("NON_ALERT")) {
      result.confidence = 1 - input.alertProbability;
      result.edgePred = "NORMAL";
      result.severity = 10;
      result.adviceCode = "NO_ACTION";
      result.advice = "轻量分类模型判断该日志模板为正常。";
    }
    result.confidence = Math.max(0, Math.min(1, result.confidence));
    result.evidence =
        List.of(
            "classifier_label=" + input.label,
            String.format(Locale.ROOT, "alert_probability=%.6f", input.alertProbability),
            String.format(Locale.ROOT, "threshold=%.6f", input.threshold));
    return result;
  }

  static TemplateResult resultFromLlm(LlmResult input) {
    var result = new TemplateResult();
    result.edgePred = input.edgePred;
    result.severity = input.severity;
    result.confidence = input.confidence;
    result.evidence = input.evidence;
    result.adviceCode = input.adviceCode;
    result.advice = input.advice;
    return result;
  }

  static String systemPrompt() {
    return String.join(
        "\n",
        "你是 CoTriage 的边缘日志故障诊断助手。",
        "你只能输出一个单行 JSON 对象，不要输出 Markdown、解释或代码块。",
        "edge_pred 只能是 APP_ERROR、NET_ERROR、DISK_ERROR、INFRA_ERROR、SECURITY、UNKNOWN 之一。",
        "severity 必须是 0 到 100 的整数。",
        "confidence 必须是 0 到 1 的数字。",
        "evidence 最多 2 条，每条不超过 20 个字符。",
        "advice 不超过 30 个汉字。",
        "输出必须形如"
            + " {\"edge_pred\":\"NET_ERROR\",\"severity\":78,\"confidence\":0.72,\"evidence\":[\"connection"
            + " refused\"],\"advice_code\":\"CHECK_NETWORK\",\"advice\":\"检查服务端口\"}");
  }

  static String userPrompt(LlmInput input) {
    var payload =
        EdgeJson.object(
            "classifier",
            EdgeJson.object(
                "alert_probability",
                input.classifier.alertProbability,
                "label",
                input.classifier.label,
                "threshold",
                input.classifier.threshold),
            "equipment",
            input.equipment,
            "raw_log",
            input.rawLog,
            "template_text",
            input.templateText);
    return "请基于以下 JSON 诊断日志，并只返回一个紧凑 JSON 对象：\n" + EdgeJson.write(payload);
  }

  static LlmResult parseLlmJson(String content) throws IOException {
    content = Normalizer.trim(content);
    int start = content.indexOf('{'), end = content.lastIndexOf('}');
    if (start < 0) throw new IOException("edge llm response does not contain JSON object");
    if (end < start) {
      try {
        return parsePartialLlmJson(content.substring(start));
      } catch (Exception ignored) {
        throw new IOException(
            "edge llm response contains incomplete JSON; content_preview="
                + EdgeJson.write(Normalizer.preview(content, 240)));
      }
    }
    LlmResult result;
    try {
      result = EdgeJson.read(content.substring(start, end + 1), LlmResult.class);
    } catch (IOException ex) {
      try {
        return parsePartialLlmJson(content.substring(start));
      } catch (Exception ignored) {
        throw new IOException(
            "decode edge llm JSON: "
                + ex.getMessage()
                + "; content_preview="
                + EdgeJson.write(Normalizer.preview(content, 240)),
            ex);
      }
    }
    validateLlmResult(result);
    return result;
  }

  static void validateLlmResult(LlmResult result) throws IOException {
    result.edgePred = Normalizer.trim(result.edgePred);
    if (!LLM_PREDICTIONS.contains(result.edgePred))
      throw new IOException("edge llm returned invalid edge_pred \"" + result.edgePred + "\"");
    if (result.severity < 0 || result.severity > 100)
      throw new IOException("edge llm returned invalid severity " + result.severity);
    if (result.confidence < 0 || result.confidence > 1)
      throw new IOException("edge llm returned invalid confidence " + result.confidence);
    result.evidence =
        result.evidence == null
            ? new ArrayList<>()
            : new ArrayList<>(result.evidence.subList(0, Math.min(3, result.evidence.size())));
  }

  static LlmResult parsePartialLlmJson(String content) throws IOException {
    var result = new LlmResult();
    result.edgePred = regexString(content, "\"edge_pred\"\\s*:\\s*\"([^\"]+)\"");
    result.adviceCode = regexString(content, "\"advice_code\"\\s*:\\s*\"([^\"]*)\"");
    result.advice = regexString(content, "\"advice\"\\s*:\\s*\"([^\"]*)\"");
    String severity = regexString(content, "\"severity\"\\s*:\\s*([0-9]+)");
    try {
      result.severity = Integer.parseInt(severity);
    } catch (NumberFormatException ex) {
      throw new IOException("partial edge llm JSON missing severity", ex);
    }
    String confidence = regexString(content, "\"confidence\"\\s*:\\s*([0-9]+(?:\\.[0-9]+)?)");
    try {
      result.confidence = Double.parseDouble(confidence);
    } catch (NumberFormatException ex) {
      throw new IOException("partial edge llm JSON missing confidence", ex);
    }
    String evidence = regexString(content, "\"evidence\"\\s*:\\s*\\[([^\\]]*)");
    var match = Pattern.compile("\"([^\"]+)\"").matcher(evidence);
    result.evidence = new ArrayList<>();
    while (match.find() && result.evidence.size() < 3)
      result.evidence.add(Normalizer.trim(match.group(1)));
    if (result.evidence.isEmpty()) result.evidence.add("partial_llm_json");
    validateLlmResult(result);
    result.evidence.add("llm_json_repaired");
    return result;
  }

  private static String regexString(String content, String regex) {
    var match = Pattern.compile(regex).matcher(content);
    return match.find() ? Normalizer.trim(match.group(1)) : "";
  }
}
