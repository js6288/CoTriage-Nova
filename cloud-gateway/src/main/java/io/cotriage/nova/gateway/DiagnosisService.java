package io.cotriage.nova.gateway;

import com.google.re2j.Pattern;
import java.util.*;

public final class DiagnosisService {
  private DiagnosisService() {}

  static final Pattern IP = Pattern.compile("\\b(?:\\d{1,3}\\.){3}\\d{1,3}\\b"),
      EMAIL = Pattern.compile("[\\w.+-]+@[\\w.-]+\\.[A-Za-z]{2,}"),
      SECRET =
          Pattern.compile("(?i)(password|passwd|token|api[_-]?key|secret)\\s*[:=]\\s*[^\\s,;]+"),
      HASH = Pattern.compile("\\b[A-Fa-f0-9]{32,}\\b");

  public static String redact(String s) {
    s = IP.matcher(s).replaceAll("<IP>");
    s = EMAIL.matcher(s).replaceAll("<EMAIL>");
    s = SECRET.matcher(s).replaceAll("$1=<SECRET>");
    return HASH.matcher(s).replaceAll("<HASH>");
  }

  public static String truncateRunes(String s, int n) {
    int count = s.codePointCount(0, s.length());
    return count > n ? s.substring(0, s.offsetByCodePoints(0, n)) : s;
  }

  public record Prompt(String system, String user) {}

  public static Prompt prompt(Events.Routed in, List<Events.Inferred> related, int maxChars) {
    String raw = truncateRunes(redact(in.input.raw_log), maxChars);
    List<String> lines = new ArrayList<>();
    if (related != null)
      for (var r : related)
        lines.add(
            String.format(
                Locale.ROOT,
                "- %s | %s | %s | severity=%d | %s",
                r.event_ts,
                r.source,
                r.edge_pred,
                r.severity,
                redact(r.raw_log)));
    String system =
        "You are a senior AIOps diagnosis assistant for industrial edge-cloud log triage. Return"
            + " only valid JSON. Do not invent facts. If evidence is insufficient, use UNKNOWN.";
    String evidence =
        in.input.evidence == null ? "[]" : "[" + String.join(" ", in.input.evidence) + "]";
    String user =
        String.format(
            Locale.ROOT,
            "Diagnose this event.\n"
                + "Raw log: %s\n"
                + "Template: %s\n"
                + "Related logs:\n"
                + "%s\n"
                + "Equipment: %s\n"
                + "Source: %s\n"
                + "Edge: pred=%s severity=%d confidence=%.3f uncertainty=%.3f evidence=%s"
                + " advice=%s\n"
                + "Router: reason=%s score=%.3f threshold=%.3f\n"
                + "Return JSON fields pred, root_cause, recommendation, explanation, confidence,"
                + " evidence.",
            raw,
            in.input.template_text,
            String.join("\n", lines),
            in.input.equipment,
            in.input.source,
            in.input.edge_pred,
            in.input.severity,
            in.input.confidence,
            in.input.uncertainty,
            evidence,
            in.input.advice,
            in.decision.reason,
            in.decision.score,
            in.decision.threshold);
    return new Prompt(system, user);
  }

  public static Events.Feedback feedback(
      Events.Inferred in, Events.Diagnosis d, Events.Usage u, int latencySLA, int tokenBudget) {
    boolean disagree = !d.pred.equals(in.edge_pred) && !d.pred.equals("UNKNOWN"),
        actionable = !d.root_cause.isEmpty() && !d.recommendation.isEmpty(),
        high = in.severity >= 75 && d.confidence >= .65;
    double cg = Math.max(0, d.confidence - in.confidence),
        lp = Math.min(1, (double) u.latency_ms / Math.max(latencySLA, 1)),
        tp = Math.min(1, (double) u.total_tokens / Math.max(tokenBudget, 1));
    double reward =
        (disagree ? .35 : 0)
            + (actionable ? .30 : 0)
            + (high ? .20 : 0)
            + cg * .15
            - lp * .10
            - tp * .05;
    reward = Math.max(-1, Math.min(1, reward));
    String reason =
        disagree
            ? "CORRECTED_EDGE"
            : actionable
                ? "ADDED_ACTIONABLE_INFO"
                : high
                    ? "CONFIRMED_HIGH_RISK"
                    : reward < 0 ? "LOW_UTILITY_OR_HIGH_COST" : "LOW_INCREMENTAL_GAIN";
    return new Events.Feedback(disagree, actionable, high, reward, reason);
  }
}
