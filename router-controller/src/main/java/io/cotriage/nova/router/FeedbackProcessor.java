package io.cotriage.nova.router;

import static io.cotriage.nova.router.Events.*;

import io.cotriage.nova.common.Json;
import java.time.*;
import java.util.*;
import java.util.function.Supplier;

public class FeedbackProcessor {
  public record Reward(double reward, String reason, boolean corrected) {}

  final RouterStore store;
  final RouterState state;
  final int batch;
  Supplier<Instant> clock = Instant::now;

  public FeedbackProcessor(RouterStore store, RouterState state, int batch) {
    this.store = store;
    this.state = state;
    this.batch = batch <= 0 ? 20 : batch;
  }

  public static Reward classify(Map<String, ?> f) {
    String edge = str(f, "edge_pred").strip().toLowerCase(Locale.ROOT),
        cloud = str(f, "cloud_pred").strip().toLowerCase(Locale.ROOT);
    if (flag(f, "corrected_edge") || (!edge.isEmpty() && !cloud.isEmpty() && !edge.equals(cloud)))
      return new Reward(1, "corrected_edge", true);
    if (flag(f, "added_actionable_info")
        || !str(f, "root_cause").isBlank()
        || !str(f, "recommendation").isBlank()) return new Reward(.7, "actionable_info", false);
    if (flag(f, "confirmed_high_risk")) return new Reward(.3, "confirmed_high_risk", false);
    return new Reward(0, "no_additional_signal", false);
  }

  public static Events.Inferred inferred(Map<String, ?> f) {
    Events.Inferred in = new Events.Inferred();
    in.event_id = str(f, "event_id");
    in.source = str(f, "source");
    in.equipment = str(f, "equipment");
    in.template_id = str(f, "template_id");
    in.edge_pred = str(f, "edge_pred");
    in.confidence = num(f, "edge_confidence");
    long severity = (long) (num(f, "route_score") * 100);
    if (severity <= 0) severity = (long) (num(f, "route_threshold") * 100);
    if (severity <= 0) severity = 50;
    in.severity = Math.min(severity, 100);
    return in;
  }

  public static RaiseEngine.Features features(Map<String, ?> f, Reward r) {
    return new RaiseEngine.Features(
        0,
        RaiseEngine.clamp(1 - num(f, "edge_confidence")),
        0,
        0,
        r.corrected() ? 1 : 0,
        RaiseEngine.clamp(num(f, "route_score")),
        RaiseEngine.clamp(num(f, "route_threshold")),
        0,
        0,
        0);
  }

  public void processOnce() {
    RuntimeException first = null;
    for (Map<String, Object> f : store.feedback(batch))
      try {
        long id = (long) num(f, "id");
        if (!store.claimFeedback(id, clock.get())) continue;
        Reward r = classify(f);
        state.feedback(inferred(f), r.corrected(), r.reward(), features(f, r));
        store.reward(id, r.reward(), r.reason(), clock.get());
        store.snapshot(
            "router_state",
            "default",
            state.snapshot(),
            "raise-feedback-v1",
            Json.write(
                Json.object("component", "feedback_processor", "version", "raise-feedback-v1")),
            clock.get());
      } catch (RuntimeException e) {
        if (first == null) first = e;
      }
    if (first != null) throw first;
  }
}
