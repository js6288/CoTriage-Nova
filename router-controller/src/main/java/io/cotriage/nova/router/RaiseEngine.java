package io.cotriage.nova.router;

import io.cotriage.nova.common.Env;
import java.util.*;

public final class RaiseEngine {
  public static double clamp(double v) {
    return v < 0 ? 0 : v > 1 ? 1 : v;
  }

  static String norm(String v) {
    return v == null ? "" : v.strip().toUpperCase(Locale.ROOT);
  }

  public static class Config {
    public String policyRef = "raise-v1", version = "raise-config-v1";
    public double risk = .30,
        uncertainty = .25,
        novelty = .15,
        disagreement = .15,
        fleet = .10,
        path = .05;
    public double costCloud = .25, costLatency = .25, costNetwork = .30, costPrivacy = .20;
    public double scoreValue = .55, scoreGain = .35, scoreCost = .30;
    public double thresholdBase = .55,
        thresholdQueue = .20,
        thresholdBudget = .15,
        thresholdNetwork = .25,
        thresholdAudit = .10;
    public double networkRTT = .30, networkFail = .50, networkHealth = .20;
    public double temperature = 1.2,
        eta = .75,
        tau = 20,
        ucbAlpha = .15,
        betaAlpha = 1,
        betaBeta = 3,
        estimatedTokens = 900,
        targetLatency = 5000,
        failLimit = .20,
        gainBias = .05;
    public long maxFleet = 20, totalLabels = 8;
    public Map<String, Double> pathRisk =
        new HashMap<>(
            Map.of("RULE", .05, "CACHE", .10, "MODEL", .45, "EDGE_LLM", .65, "FALLBACK", 1.0));

    public static Config fromEnv() {
      Config c = new Config();
      c.policyRef = Env.get("RAISE_POLICY_REF", c.policyRef);
      c.version = Env.get("RAISE_VERSION", c.version);
      c.risk = d("WEIGHT_RISK", c.risk);
      c.uncertainty = d("WEIGHT_UNCERTAINTY", c.uncertainty);
      c.novelty = d("WEIGHT_NOVELTY", c.novelty);
      c.disagreement = d("WEIGHT_DISAGREEMENT", c.disagreement);
      c.fleet = d("WEIGHT_FLEET_IMPACT", c.fleet);
      c.path = d("WEIGHT_PATH_RISK", c.path);
      c.costCloud = d("COST_CLOUD", c.costCloud);
      c.costLatency = d("COST_LATENCY", c.costLatency);
      c.costNetwork = d("COST_NETWORK", c.costNetwork);
      c.costPrivacy = d("COST_PRIVACY", c.costPrivacy);
      c.scoreValue = d("SCORE_VALUE", c.scoreValue);
      c.scoreGain = d("SCORE_GAIN", c.scoreGain);
      c.scoreCost = d("SCORE_COST", c.scoreCost);
      c.thresholdBase = d("THRESHOLD_BASE", c.thresholdBase);
      c.thresholdQueue = d("THRESHOLD_QUEUE", c.thresholdQueue);
      c.thresholdBudget = d("THRESHOLD_BUDGET", c.thresholdBudget);
      c.thresholdNetwork = d("THRESHOLD_NETWORK", c.thresholdNetwork);
      c.thresholdAudit = d("THRESHOLD_AUDIT", c.thresholdAudit);
      c.temperature = d("TEMPERATURE", c.temperature);
      c.eta = d("UNCERTAINTY_ETA", c.eta);
      c.tau = d("TEMPLATE_TAU", c.tau);
      c.ucbAlpha = d("UCB_ALPHA", c.ucbAlpha);
      c.betaAlpha = d("BETA_PRIOR_ALPHA", c.betaAlpha);
      c.betaBeta = d("BETA_PRIOR_BETA", c.betaBeta);
      c.estimatedTokens = d("ESTIMATED_TOKENS", c.estimatedTokens);
      c.targetLatency = d("TARGET_LATENCY_MS", c.targetLatency);
      c.failLimit = d("FAIL_RATE_LIMIT", c.failLimit);
      c.gainBias = d("EXPECTED_GAIN_BIAS", c.gainBias);
      c.maxFleet = Env.longValue("RAISE_MAX_FLEET_DEVICES", c.maxFleet);
      c.totalLabels = Env.longValue("RAISE_TOTAL_LABELS", c.totalLabels);
      c.pathRisk.replaceAll((k, v) -> d("PATH_RISK_" + k, v));
      return c;
    }

    private static double d(String k, double v) {
      return Env.decimal("RAISE_" + k, v);
    }
  }

  public static class Stats {
    public long templateFrequency,
        historicalCloudCount,
        historicalCorrectionCount,
        affectedDevices,
        conformalSetSize;
    public double ucbUncertainty;
  }

  public static class Policy {
    public Set<String> denyPreds = Set.of(),
        forceLocalPreds = Set.of(),
        forceCloudPreds = Set.of(),
        denySources = Set.of(),
        forceCloudTemplates = Set.of();
  }

  public static class SystemState {
    public String CloudHealth = "";
    public double RTTMS,
        FailRate,
        MinuteBudgetRemaining,
        HourBudgetRemaining,
        DayBudgetRemaining,
        RecentCloudRatio,
        MinCloudRatioForAudit,
        EstimatedLatencyMS,
        PrivacyCost;
    public long RateLimitRemaining, CloudQueueLag, OutboxLag, QueueMax;
  }

  public record Features(
      double risk,
      double calibrated_uncertainty,
      long conformal_set_size,
      double novelty,
      double historical_disagreement,
      double fleet_impact,
      double path_risk,
      double expected_gain,
      double ucb_bonus,
      double cloud_value) {}

  public record Costs(
      double cloud_cost,
      double latency_cost,
      double network_cost,
      double privacy_cost,
      double total_cost) {}

  public record Pressures(
      double queue_pressure,
      double budget_pressure,
      double network_pressure,
      double audit_pressure) {}

  public record Decision(
      String EventID,
      String Action,
      String Reason,
      String PolicyRef,
      double Score,
      double Threshold,
      Features Features,
      Costs Costs,
      Pressures Pressures,
      String CloudHealth,
      String RaiseVersion,
      boolean Sendable) {}

  final Config c;

  public RaiseEngine(Config config) {
    c = config;
    if (c.policyRef.isEmpty()) c.policyRef = "raise-v1";
    if (c.version.isEmpty()) c.version = "raise-config-v1";
    if (c.pathRisk == null) c.pathRisk = new Config().pathRisk;
    if (c.totalLabels <= 0) c.totalLabels = 8;
    if (c.tau <= 0) c.tau = 20;
    if (c.maxFleet <= 0) c.maxFleet = 20;
    if (c.targetLatency <= 0) c.targetLatency = 5000;
    if (c.failLimit <= 0) c.failLimit = .20;
    if (c.temperature <= 0) c.temperature = 1.2;
  }

  public Decision decide(Events.Inferred in, SystemState s, Stats st, Policy p) {
    double risk = clamp(in.severity / 100.0),
        conf = in.confidence <= 0 ? .000001 : in.confidence >= 1 ? .999999 : in.confidence;
    double model = 1 - 1 / (1 + Math.exp(-Math.log(conf / (1 - conf)) / c.temperature));
    if (in.uncertainty > 0) model = (model + clamp(in.uncertainty)) / 2;
    double calibrated =
        clamp(
            clamp(c.eta) * model
                + (1 - clamp(c.eta))
                    * clamp(
                        (double) (st.conformalSetSize <= 0 ? 1 : st.conformalSetSize)
                            / c.totalLabels));
    double novelty = Math.exp(-Math.max(st.templateFrequency, 0) / c.tau);
    double disagreement =
        clamp(
            (Math.max(st.historicalCorrectionCount, 0) + (c.betaAlpha <= 0 ? 1 : c.betaAlpha))
                / (Math.max(st.historicalCloudCount, 0)
                    + (c.betaAlpha <= 0 ? 1 : c.betaAlpha)
                    + (c.betaBeta <= 0 ? 3 : c.betaBeta)));
    double fleet = clamp((double) Math.max(st.affectedDevices, 0) / c.maxFleet),
        path = clamp(c.pathRisk.getOrDefault(norm(in.inference_path), .5));
    double value =
        clamp(
            c.risk * risk
                + c.uncertainty * calibrated
                + c.novelty * novelty
                + c.disagreement * disagreement
                + c.fleet * fleet
                + c.path * path);
    double ucb = clamp(c.ucbAlpha * Math.sqrt(Math.max(st.ucbUncertainty, 0))),
        gain = clamp(c.gainBias + .45 * calibrated + .35 * disagreement + .20 * novelty + ucb);
    Features features =
        new Features(
            risk,
            calibrated,
            st.conformalSetSize,
            novelty,
            disagreement,
            fleet,
            path,
            gain,
            ucb,
            value);
    double network =
        clamp(
            c.networkRTT * clamp(s.RTTMS / 1000)
                + c.networkFail * clamp(s.FailRate)
                + c.networkHealth * (norm(s.CloudHealth).equals("UP") ? 0 : 1));
    double cloud = clamp(c.estimatedTokens / Math.max(s.DayBudgetRemaining, 1)),
        latency = clamp(Math.max(s.EstimatedLatencyMS - c.targetLatency, 0) / c.targetLatency),
        privacy = clamp(s.PrivacyCost);
    double total =
        clamp(
            c.costCloud * cloud
                + c.costLatency * latency
                + c.costNetwork * network
                + c.costPrivacy * privacy);
    Costs costs = new Costs(cloud, latency, network, privacy, total);
    double queue =
        clamp(
            (double) (Math.max(s.CloudQueueLag, 0) + Math.max(s.OutboxLag, 0))
                / (s.QueueMax <= 0 ? 1000 : s.QueueMax));
    double budget = s.DayBudgetRemaining <= 0 ? 1 : clamp(1 / Math.sqrt(s.DayBudgetRemaining)),
        audit = clamp(Math.max(s.MinCloudRatioForAudit - s.RecentCloudRatio, 0));
    Pressures pressures = new Pressures(queue, budget, network, audit);
    double
        threshold =
            clamp(
                c.thresholdBase
                    + c.thresholdQueue * queue
                    + c.thresholdBudget * budget
                    + c.thresholdNetwork * network
                    - c.thresholdAudit * audit),
        score = clamp(c.scoreValue * value + c.scoreGain * gain - c.scoreCost * total);
    boolean sendable =
        norm(s.CloudHealth).equals("UP")
            && s.RateLimitRemaining > 0
            && s.MinuteBudgetRemaining > 0
            && s.DayBudgetRemaining > 0
            && s.FailRate <= c.failLimit;
    String action, reason;
    if (p.denyPreds.contains(norm(in.edge_pred)) || p.denySources.contains(norm(in.source))) {
      action = "LOCAL";
      reason = "DENY_CLOUD_POLICY";
    } else if (p.forceLocalPreds.contains(norm(in.edge_pred))) {
      action = "LOCAL";
      reason = "FORCE_LOCAL_POLICY";
    } else if (p.forceCloudPreds.contains(norm(in.edge_pred))
        || p.forceCloudTemplates.contains(norm(in.template_id))) {
      action = sendable ? "CLOUD" : "DEFER";
      reason = sendable ? "FORCE_CLOUD_SENDABLE" : "FORCE_CLOUD_DEFER";
    } else if (score < threshold) {
      action = "LOCAL";
      reason = "LOW_SCORE";
    } else {
      action = sendable ? "CLOUD" : "DEFER";
      reason = sendable ? "HIGH_SCORE_SENDABLE" : "HIGH_SCORE_DEFER";
    }
    return new Decision(
        in.event_id,
        action,
        reason,
        c.policyRef,
        score,
        threshold,
        features,
        costs,
        pressures,
        s.CloudHealth,
        c.version,
        sendable);
  }
}
