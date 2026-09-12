package io.cotriage.nova.edgeinfer;

import static io.cotriage.nova.edgeinfer.Models.*;

import com.fasterxml.jackson.core.type.TypeReference;
import com.google.re2j.Pattern;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.List;

final class RuleEngine {
  private record Compiled(Rule rule, Pattern pattern) {}

  private final String version;
  private final List<Compiled> rules = new ArrayList<>();

  RuleEngine(String version, List<Rule> input) {
    this.version = version;
    var sorted = new ArrayList<>(input);
    sorted.sort(Comparator.comparingInt((Rule rule) -> rule.priority).reversed());
    for (Rule rule : sorted) {
      if (!rule.enabled) continue;
      if (!PREDICTIONS.contains(rule.edgePred))
        throw new IllegalArgumentException(
            "rule \"" + rule.id + "\" has invalid edge_pred \"" + rule.edgePred + "\"");
      if (rule.severity < 0 || rule.severity > 100 || rule.confidence < 0 || rule.confidence > 1)
        throw new IllegalArgumentException("rule \"" + rule.id + "\" has out-of-range score");
      try {
        rules.add(new Compiled(rule, Pattern.compile("(?i)" + rule.pattern)));
      } catch (RuntimeException ex) {
        throw new IllegalArgumentException(
            "compile rule \"" + rule.id + "\": " + ex.getMessage(), ex);
      }
    }
  }

  static RuleEngine load(String path) throws IOException {
    List<Rule> rules =
        Normalizer.trim(path).isEmpty()
            ? defaults()
            : EdgeJson.read(Files.readString(Path.of(path)), new TypeReference<List<Rule>>() {});
    return new RuleEngine(DEFAULT_RULE_VERSION, rules == null ? List.of() : rules);
  }

  String version() {
    return version;
  }

  TemplateResult match(String text) {
    for (var compiled : rules)
      if (compiled.pattern.matcher(text).find()) {
        Rule rule = compiled.rule;
        var result = new TemplateResult();
        result.edgePred = rule.edgePred;
        result.severity = rule.severity;
        result.confidence = rule.confidence;
        result.evidence =
            rule.evidence == null || rule.evidence.isEmpty() ? List.of(rule.id) : rule.evidence;
        result.adviceCode = rule.adviceCode;
        result.advice = rule.advice;
        return result;
      }
    return null;
  }

  private static List<Rule> defaults() {
    return List.of(
        rule(
            "oom",
            "out of memory|oom[- ]?killed|cannot allocate memory",
            "INFRA_ERROR",
            95,
            .99,
            "out of memory",
            "CHECK_MEMORY",
            "检查内存使用、限制配置和进程状态",
            100),
        rule(
            "disk-full",
            "no space left on device|disk (?:is )?full",
            "DISK_ERROR",
            92,
            .99,
            "no space left on device",
            "CHECK_DISK_SPACE",
            "检查磁盘空间、inode和日志清理策略",
            100),
        rule(
            "network-refused",
            "connection refused|network is unreachable|no route to host",
            "NET_ERROR",
            78,
            .95,
            "connection failure",
            "CHECK_NETWORK_AND_SERVICE",
            "检查目标服务状态、端口监听和网络策略",
            80),
        rule(
            "auth-failed",
            "authentication failed|access denied|unauthorized",
            "SECURITY",
            75,
            .92,
            "authentication failure",
            "CHECK_AUTH",
            "检查访问凭据、账号权限和异常登录来源",
            70));
  }

  static Rule rule(
      String id,
      String pattern,
      String pred,
      int severity,
      double confidence,
      String evidence,
      String adviceCode,
      String advice,
      int priority) {
    var rule = new Rule();
    rule.id = id;
    rule.pattern = pattern;
    rule.edgePred = pred;
    rule.severity = severity;
    rule.confidence = confidence;
    rule.evidence = List.of(evidence);
    rule.adviceCode = adviceCode;
    rule.advice = advice;
    rule.priority = priority;
    rule.enabled = true;
    return rule;
  }
}
