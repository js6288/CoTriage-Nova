#!/usr/bin/env python3
"""Run the CoTriage cloud/edge routing comparison on the frozen BGL test split."""

from __future__ import annotations

import argparse
import csv
import json
import math
import statistics
import subprocess
import time
import uuid
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]
TEST_DATA = ROOT / "dataset/processed/bgl/test.jsonl"
RAW_DIR = Path(__file__).resolve().parent / "raw"
SUMMARY_DIR = Path(__file__).resolve().parent / "summary"
REDIS = ["redis-cli", "-h", "127.0.0.1", "-p", "16379", "--json"]


def redis(*args: str):
    out = subprocess.check_output([*REDIS, *args], text=True, timeout=30)
    return json.loads(out)


def xlast(stream: str) -> str:
    rows = redis("XREVRANGE", stream, "+", "-", "COUNT", "1")
    return rows[0][0] if rows else "0-0"


def xadd(stream: str, values: dict[str, str]) -> str:
    args = ["XADD", stream, "*"]
    for key, value in values.items():
        args.extend([key, value])
    return str(redis(*args))


def read_after(stream: str, start: str) -> list[tuple[str, dict[str, str]]]:
    rows = redis("XRANGE", stream, f"({start}", "+")
    result = []
    for message_id, flat in rows:
        result.append((message_id, dict(zip(flat[0::2], flat[1::2]))))
    return result


def percentile(values: list[float], q: float) -> float:
    if not values:
        return 0.0
    ordered = sorted(values)
    index = (len(ordered) - 1) * q
    lo, hi = math.floor(index), math.ceil(index)
    if lo == hi:
        return ordered[lo]
    return ordered[lo] * (hi - index) + ordered[hi] * (index - lo)


def binary_metrics(truth: list[int], prediction: list[int]) -> dict:
    tp = sum(t == 1 and p == 1 for t, p in zip(truth, prediction))
    tn = sum(t == 0 and p == 0 for t, p in zip(truth, prediction))
    fp = sum(t == 0 and p == 1 for t, p in zip(truth, prediction))
    fn = sum(t == 1 and p == 0 for t, p in zip(truth, prediction))
    precision = tp / (tp + fp) if tp + fp else 0
    recall = tp / (tp + fn) if tp + fn else 0
    f1 = 2 * precision * recall / (precision + recall) if precision + recall else 0
    specificity = tn / (tn + fp) if tn + fp else 0
    f1_negative = 2 * specificity * (tn / (tn + fn) if tn + fn else 0) / (specificity + (tn / (tn + fn) if tn + fn else 0)) if specificity else 0
    return {
        "accuracy": (tp + tn) / len(truth), "precision": precision, "recall": recall,
        "f1": f1, "macro_f1": (f1 + f1_negative) / 2,
        "tp": tp, "tn": tn, "fp": fp, "fn": fn,
    }


def cloud_request(run_id: str, method: str, sample: dict, event: dict, decision: dict) -> tuple[str, int]:
    event_id = f"exp-{run_id}-{method.lower()}-{sample['template_id']}"
    cloud_input = dict(event)
    cloud_input["event_id"] = event_id
    if method == "ALL_CLOUD":
        cloud_input.update({"edge_pred": "UNKNOWN", "severity": 0, "confidence": 0.0, "uncertainty": 1.0, "evidence": [], "advice": ""})
    routed = {"input": cloud_input, "decision": decision, "system_state": {"experiment_id": run_id, "method": method}}
    payload = json.dumps(routed, ensure_ascii=False, separators=(",", ":"))
    xadd("cotriage.events.cloud.req", {
        "event_id": event_id, "request_id": f"cloud-{event_id}",
        "idempotency_key": f"experiment-{event_id}", "payload_json": payload,
        "source": "experiment-runner",
    })
    return event_id, len(payload.encode())


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--limit", type=int, default=0, help="0 uses the complete test split")
    parser.add_argument("--fixed-confidence", type=float, default=0.80)
    parser.add_argument("--cloud-timeout", type=int, default=900)
    args = parser.parse_args()
    run_id = time.strftime("%Y%m%d%H%M%S") + "-" + uuid.uuid4().hex[:6]
    RAW_DIR.mkdir(parents=True, exist_ok=True)
    SUMMARY_DIR.mkdir(parents=True, exist_ok=True)

    samples = [json.loads(line) for line in TEST_DATA.read_text().splitlines() if line.strip()]
    if args.limit:
        alerts = [x for x in samples if x["label"] == 1]
        normal = [x for x in samples if x["label"] == 0]
        alert_count = min(len(alerts), max(1, args.limit // 2))
        samples = alerts[:alert_count] + normal[:args.limit-alert_count]

    inferred_start = xlast("cotriage.events.inferred")
    raw_ts = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    for sample in samples:
        event_id = f"exp-{run_id}-raise-{sample['template_id']}"
        raw = {"event_id": event_id, "ts": raw_ts, "equipment": "bgl-node", "source": "experiment-bgl-test", "log": {"raw": sample["text"]}}
        xadd("cotriage.events.raw", {"event_id": event_id, "payload": json.dumps(raw, ensure_ascii=False)})
    events = {}
    deadline = time.time() + 300
    while time.time() < deadline and len(events) < len(samples):
        for _, values in read_after("cotriage.events.inferred", inferred_start):
            event_id = values.get("event_id", "")
            if event_id.startswith(f"exp-{run_id}-raise-"):
                event = json.loads(values["payload"])
                events[event["template_id"]] = event
        if len(events) < len(samples):
            time.sleep(2)
    if len(events) != len(samples):
        raise RuntimeError(f"edge-infer completed {len(events)}/{len(samples)} samples")

    routed_start = xlast("cotriage.events.routed")
    for event in events.values():
        xadd("cotriage.events.inferred", {"event_id": event["event_id"], "payload": json.dumps(event, ensure_ascii=False)})
    deadline = time.time() + 180
    raise_decisions = {}
    while time.time() < deadline and len(raise_decisions) < len(samples):
        for _, values in read_after("cotriage.events.routed", routed_start):
            if values.get("event_id", "").startswith(f"exp-{run_id}-raise-"):
                payload = json.loads(values["payload"])
                template_id = payload["input"]["template_id"]
                decision = payload["decision"]
                raise_decisions[template_id] = {str(k).lower(): v for k, v in decision.items()}
        if len(raise_decisions) < len(samples):
            time.sleep(2)
    if len(raise_decisions) != len(samples):
        raise RuntimeError(f"RAISE routed {len(raise_decisions)}/{len(samples)} samples")

    response_start = xlast("cotriage.events.cloud.resp")
    expected = {}
    upload_bytes = {m: 0 for m in ["ALL_CLOUD", "EDGE_ONLY", "FIXED_THRESHOLD", "RAISE"]}
    for sample in samples:
        tid = sample["template_id"]
        event = events[tid]
        selected = {
            "ALL_CLOUD": True,
            "EDGE_ONLY": False,
            "FIXED_THRESHOLD": event["confidence"] < args.fixed_confidence,
            "RAISE": raise_decisions[tid]["action"] == "CLOUD",
        }
        decisions = {
            "ALL_CLOUD": {"action": "CLOUD", "reason": "EXPERIMENT_ALL_CLOUD", "score": 1.0, "threshold": 0.0},
            "FIXED_THRESHOLD": {"action": "CLOUD", "reason": "EXPERIMENT_FIXED_CONFIDENCE", "score": 1-event["confidence"], "threshold": 1-args.fixed_confidence},
            "RAISE": raise_decisions[tid],
        }
        for method in ["ALL_CLOUD", "FIXED_THRESHOLD", "RAISE"]:
            if selected[method]:
                event_id, size = cloud_request(run_id, method, sample, event, decisions[method])
                expected[event_id] = (method, tid)
                upload_bytes[method] += size

    responses = {}
    deadline = time.time() + args.cloud_timeout
    while time.time() < deadline and len(responses) < len(expected):
        for _, values in read_after("cotriage.events.cloud.resp", response_start):
            event_id = values.get("event_id", "")
            if event_id in expected and event_id not in responses:
                responses[event_id] = json.loads(values["payload_json"])
        if len(responses) < len(expected):
            time.sleep(3)

    rows = []
    for sample in samples:
        tid, truth = sample["template_id"], int(sample["label"])
        event = events[tid]
        edge_prediction = 1 if event["edge_pred"] == "ABNORMAL" else 0
        for method in ["ALL_CLOUD", "EDGE_ONLY", "FIXED_THRESHOLD", "RAISE"]:
            event_id = f"exp-{run_id}-{method.lower()}-{tid}"
            response = responses.get(event_id)
            selected = method == "ALL_CLOUD" or (method == "FIXED_THRESHOLD" and event["confidence"] < args.fixed_confidence) or (method == "RAISE" and raise_decisions[tid]["action"] == "CLOUD")
            cloud_pred = response.get("cloud", {}).get("pred", "") if response else ""
            final_prediction = edge_prediction
            if response:
                final_prediction = 0 if cloud_pred == "NORMAL" else 1
            usage = response.get("usage", {}) if response else {}
            rows.append({
                "experiment_id": run_id, "method": method, "template_id": tid, "truth": truth,
                "edge_prediction": edge_prediction, "edge_confidence": event["confidence"],
                "selected_cloud": int(selected), "raise_action": raise_decisions[tid]["action"] if method == "RAISE" else "",
                "final_prediction": final_prediction, "cloud_prediction": cloud_pred,
                "edge_latency_ms": event["latency_ms"], "cloud_latency_ms": usage.get("latency_ms", 0),
                "first_decision_ms": usage.get("latency_ms", 0) if method == "ALL_CLOUD" else event["latency_ms"],
                "complete_diagnosis_ms": event["latency_ms"] + usage.get("latency_ms", 0) if response else event["latency_ms"],
                "token_usage": usage.get("total_tokens", 0), "cloud_status": response.get("status", "MISSING") if response else "NOT_REQUESTED",
            })

    raw_path = RAW_DIR / f"method_predictions_{run_id}.csv"
    with raw_path.open("w", newline="") as f:
        writer = csv.DictWriter(f, fieldnames=rows[0].keys()); writer.writeheader(); writer.writerows(rows)

    summaries = []
    for method in ["ALL_CLOUD", "EDGE_ONLY", "FIXED_THRESHOLD", "RAISE"]:
        group = [r for r in rows if r["method"] == method]
        metrics = binary_metrics([r["truth"] for r in group], [r["final_prediction"] for r in group])
        requested = [r for r in group if r["selected_cloud"]]
        summaries.append({
            "experiment_id": run_id, "method": method, "samples": len(group), **metrics,
            "cloud_requests": len(requested), "cloud_completion": sum(r["cloud_status"] == "OK" for r in requested),
            "cloud_ratio": len(requested)/len(group), "upload_bytes": upload_bytes[method],
            "token_total": sum(int(r["token_usage"]) for r in group),
            "first_decision_p50_ms": percentile([float(r["first_decision_ms"]) for r in group], .5),
            "first_decision_p95_ms": percentile([float(r["first_decision_ms"]) for r in group], .95),
            "complete_p95_ms": percentile([float(r["complete_diagnosis_ms"]) for r in group], .95),
        })
    summary_path = SUMMARY_DIR / f"cloud_edge_comparison_{run_id}.csv"
    with summary_path.open("w", newline="") as f:
        writer = csv.DictWriter(f, fieldnames=summaries[0].keys()); writer.writeheader(); writer.writerows(summaries)
    (SUMMARY_DIR / f"cloud_edge_comparison_{run_id}.json").write_text(json.dumps(summaries, ensure_ascii=False, indent=2))
    print(json.dumps({"experiment_id": run_id, "raw": str(raw_path), "summary": str(summary_path), "expected_cloud": len(expected), "received_cloud": len(responses), "results": summaries}, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
