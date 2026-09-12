package io.cotriage.nova.router;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import io.cotriage.nova.common.Json;
import java.time.*;
import java.util.*;

/** Snapshot shapes, including the Go nanosecond duration, are persisted verbatim. */
@JsonIgnoreProperties(ignoreUnknown = true)
public class RouterState {
  public Window window = new Window();
  public TemplateStats template_stats = new TemplateStats();
  public GainModel gain_model = new GainModel();
  public long conformal_set_size = 1, recent_cloud_actions, recent_total_actions;

  public RouterState() {}

  public RouterState(Duration span, double alpha) {
    window.duration =
        (span.isNegative() || span.isZero() ? Duration.ofMinutes(10) : span).toNanos();
    gain_model = new GainModel(7, alpha);
  }

  static String norm(String s) {
    return s == null ? "" : s.strip().toLowerCase(Locale.ROOT);
  }

  public static class Observation {
    public String template_id = "", source = "", edge_pred = "", seen_at = "";

    public Observation() {}

    public Observation(Events.Inferred in, Instant now) {
      template_id = norm(in.template_id);
      source = in.source.strip();
      edge_pred = RaiseEngine.norm(in.edge_pred);
      seen_at = now.toString();
    }
  }

  public static class Window {
    public long duration = Duration.ofMinutes(10).toNanos();
    public List<Observation> observations;

    public void add(Events.Inferred in, Instant now) {
      if (observations == null) observations = new ArrayList<>();
      observations.add(new Observation(in, now));
      prune(now);
    }

    void prune(Instant now) {
      if (observations != null) {
        Instant cutoff = now.minusNanos(duration);
        observations.removeIf(o -> Instant.parse(o.seen_at).isBefore(cutoff));
      }
    }

    public long frequency(String template, Instant now) {
      prune(now);
      return observations == null
          ? 0
          : observations.stream().filter(o -> o.template_id.equals(norm(template))).count();
    }

    public long devices(String template, Instant now) {
      prune(now);
      return observations == null
          ? 0
          : observations.stream()
              .filter(o -> o.template_id.equals(norm(template)) && !o.source.isEmpty())
              .map(o -> o.source)
              .distinct()
              .count();
    }
  }

  public static class TemplateStat {
    public long cloud_count, correction_count;
  }

  public static class TemplateStats {
    public Map<String, TemplateStat> items = new HashMap<>();

    void ensure() {
      if (items == null) items = new HashMap<>();
    }

    public TemplateStat get(String template) {
      ensure();
      return items.getOrDefault(norm(template), new TemplateStat());
    }

    public void record(String template, boolean corrected) {
      ensure();
      TemplateStat s = items.computeIfAbsent(norm(template), k -> new TemplateStat());
      s.cloud_count++;
      if (corrected) s.correction_count++;
    }
  }

  public static class GainModel {
    public double[] theta, a, b;
    public double alpha;

    public GainModel() {}

    public GainModel(int dim, double alpha) {
      this.alpha = alpha;
      theta = new double[Math.max(dim, 1)];
      a = new double[theta.length];
      b = new double[theta.length];
      Arrays.fill(a, 1);
    }

    void ensure(int dim) {
      dim = Math.max(dim, 1);
      if (theta != null
          && a != null
          && b != null
          && theta.length == dim
          && a.length == dim
          && b.length == dim) {
        for (int i = 0; i < dim; i++) if (a[i] <= 0) a[i] = 1;
        return;
      }
      GainModel n = new GainModel(dim, alpha);
      if (theta != null) System.arraycopy(theta, 0, n.theta, 0, Math.min(dim, theta.length));
      if (a != null) System.arraycopy(a, 0, n.a, 0, Math.min(dim, a.length));
      if (b != null) System.arraycopy(b, 0, n.b, 0, Math.min(dim, b.length));
      theta = n.theta;
      a = n.a;
      b = n.b;
    }

    public double[] predict(double[] x) {
      ensure(x.length);
      double dot = 0, u = 0;
      for (int i = 0; i < x.length; i++) {
        dot += theta[i] * x[i];
        u += x[i] * x[i] / a[i];
      }
      double bonus = alpha * Math.sqrt(Math.max(u, 0));
      return new double[] {
        RaiseEngine.clamp(1 / (1 + Math.exp(-dot)) + bonus), RaiseEngine.clamp(bonus)
      };
    }

    public void update(double[] x, double reward) {
      ensure(x.length);
      reward = RaiseEngine.clamp(reward);
      for (int i = 0; i < x.length; i++) {
        a[i] += x[i] * x[i];
        b[i] += reward * x[i];
        theta[i] = b[i] / a[i];
      }
    }
  }

  public void observe(Events.Inferred in, String action, Instant now) {
    window.add(in, now);
    recent_total_actions++;
    if (action.equals("CLOUD")) recent_cloud_actions++;
  }

  public double ratio() {
    return recent_total_actions <= 0 ? 0 : (double) recent_cloud_actions / recent_total_actions;
  }

  public RaiseEngine.Stats stats(Events.Inferred in, Instant now) {
    TemplateStat t = template_stats.get(in.template_id);
    RaiseEngine.Stats s = new RaiseEngine.Stats();
    s.templateFrequency = window.frequency(in.template_id, now);
    s.historicalCloudCount = t.cloud_count;
    s.historicalCorrectionCount = t.correction_count;
    s.affectedDevices = window.devices(in.template_id, now);
    s.conformalSetSize = conformal_set_size <= 0 ? 1 : conformal_set_size;
    s.ucbUncertainty = gain_model.predict(features(in, null))[1];
    return s;
  }

  public void feedback(
      Events.Inferred in, boolean corrected, double reward, RaiseEngine.Features f) {
    template_stats.record(in.template_id, corrected);
    gain_model.update(features(in, f), reward);
  }

  public static double[] features(Events.Inferred in, RaiseEngine.Features f) {
    return new double[] {
      in.severity / 100.0,
      f == null ? 0 : f.calibrated_uncertainty(),
      f == null ? 0 : f.novelty(),
      f == null ? 0 : f.historical_disagreement(),
      f == null ? 0 : f.fleet_impact(),
      f == null ? 0 : f.path_risk(),
      1
    };
  }

  public String snapshot() {
    return Json.write(this);
  }

  public static RouterState restore(String json) {
    RouterState s = GoWire.read(json, RouterState.class);
    if (s.template_stats == null) s.template_stats = new TemplateStats();
    s.template_stats.ensure();
    if (s.window == null) s.window = new Window();
    if (s.window.duration <= 0) s.window.duration = Duration.ofMinutes(10).toNanos();
    if (s.gain_model == null) s.gain_model = new GainModel();
    s.gain_model.ensure(7);
    if (s.conformal_set_size <= 0) s.conformal_set_size = 1;
    return s;
  }
}
