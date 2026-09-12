package io.cotriage.nova.router;

import static org.junit.jupiter.api.Assertions.*;

import io.cotriage.nova.common.Json;
import java.time.*;
import java.util.*;
import org.junit.jupiter.api.Test;

class RouterStateTest {
  final Instant now = Instant.parse("2026-06-30T12:00:00Z");

  @Test
  void windowPruningAndSourceBasedDeviceCounts() {
    var state = new RouterState(Duration.ofMinutes(1), .1);
    var in = RaiseEngineTest.event();
    in.source = "a";
    state.observe(in, "LOCAL", now.minusSeconds(120));
    in.source = "b";
    state.observe(in, "CLOUD", now);
    in.equipment = "another-equipment";
    state.observe(in, "LOCAL", now);
    assertEquals(2, state.window.frequency("TPL-1", now));
    assertEquals(1, state.window.devices("tpl-1", now));
    assertEquals(1.0 / 3, state.ratio(), 1e-12);
  }

  @Test
  void gainUpdatesAndOldSnapshotRoundTrip() {
    var state = new RouterState(Duration.ofMinutes(10), .1);
    var in = RaiseEngineTest.event();
    var f = new RaiseEngine.Features(0, .7, 0, .8, .2, .3, .4, 0, 0, 0);
    double before = state.gain_model.predict(RouterState.features(in, f))[0];
    state.feedback(in, true, 1, f);
    assertTrue(state.gain_model.predict(RouterState.features(in, f))[0] > before);
    state.observe(in, "CLOUD", now);
    String json = state.snapshot();
    assertEquals(
        600_000_000_000L, Json.MAPPER.valueToTree(state).path("window").path("duration").asLong());
    var restored = RouterState.restore(json);
    assertEquals(1, restored.template_stats.get("TPL-1").correction_count);
    assertEquals(1, restored.window.frequency("tpl-1", now));
    assertEquals(1, restored.ratio());
    assertEquals(Json.map(json), Json.map(restored.snapshot()));
  }

  @Test
  void emptySnapshotsUseNullObservationSlice() {
    var state = new RouterState(Duration.ZERO, .1);
    assertTrue(Json.MAPPER.valueToTree(state).path("window").path("observations").isNull());
    var restored =
        RouterState.restore(
            "{\"window\":{\"duration\":0},\"gain_model\":{\"a\":[],\"alpha\":0.1}}");
    assertEquals(600_000_000_000L, restored.window.duration);
    assertEquals(7, restored.gain_model.a.length);
    assertEquals(1, restored.conformal_set_size);
  }

  @Test
  void readsActualGoSnapshotAndReproducesLearnedState() throws Exception {
    String json =
        new String(
            getClass().getResourceAsStream("/router-state-go-golden.json").readAllBytes(),
            java.nio.charset.StandardCharsets.UTF_8);
    var restored = RouterState.restore(json);
    var in = RaiseEngineTest.event();
    var actual = new RouterState(Duration.ofMinutes(10), .1);
    actual.feedback(in, true, 1, new RaiseEngine.Features(0, .7, 0, .8, .2, .3, .4, 0, 0, 0));
    actual.observe(in, "CLOUD", Instant.parse("2026-06-30T12:00:00.123456789Z"));
    RaiseEngineTest.assertTree(Json.MAPPER.readTree(json), Json.MAPPER.valueToTree(actual));
    RaiseEngineTest.assertTree(Json.MAPPER.readTree(json), Json.MAPPER.valueToTree(restored));
  }

  @Test
  void bucketsRefillAndRejectWithoutPartialSpend() {
    var b = new RouterRuntime.Buckets(2, 10, 100, 2, now);
    assertTrue(b.allow(1, now));
    assertTrue(b.allow(1, now));
    double day = b.day.tokens;
    assertFalse(b.allow(1, now));
    assertEquals(day, b.day.tokens);
    b.refillTo(now.plusSeconds(30));
    assertEquals(1, b.minute.tokens, 1e-12);
    assertTrue(b.allow(1, now.plusSeconds(30)));
  }

  @Test
  void healthSmoothingAndDefaultHealthyRuntime() {
    var health = new RouterRuntime.HealthProbe("", Duration.ofSeconds(1), 4);
    health.probe();
    health.probe();
    assertEquals("UP", health.snapshot().health());
    health.add(new RouterRuntime.HealthSample(false, 0, ""));
    health.add(new RouterRuntime.HealthSample(false, 0, ""));
    assertEquals("DOWN", health.snapshot().health());
    assertEquals(.5, health.snapshot().failRate());
    var cfg = new RouterConfig();
    cfg.minuteBudget = 10;
    cfg.hourBudget = 100;
    cfg.dayBudget = 1000;
    cfg.rateLimit = 10;
    cfg.queueMax = 100;
    cfg.auditRatio = .1;
    var runtime =
        new RouterRuntime(
            cfg,
            null,
            null,
            new RouterRuntime.HealthProbe("", Duration.ofSeconds(1), 4),
            new RouterRuntime.Buckets(10, 100, 1000, 10, now));
    var s = runtime.build(.05, .2, now);
    assertEquals("UP", s.CloudHealth);
    assertEquals(1000, s.DayBudgetRemaining);
    assertEquals(.2, s.PrivacyCost);
  }

  @Test
  void secondStageRewardKeepsOriginalCategoricalRules() {
    assertEquals(
        1,
        FeedbackProcessor.classify(Map.of("edge_pred", "NORMAL", "cloud_pred", "UNKNOWN"))
            .reward());
    assertEquals(
        .7,
        FeedbackProcessor.classify(Map.of("root_cause", "text", "confirmed_high_risk", true))
            .reward());
    assertEquals(.3, FeedbackProcessor.classify(Map.of("confirmed_high_risk", true)).reward());
    assertEquals(0, FeedbackProcessor.classify(Map.of()).reward());
  }
}
