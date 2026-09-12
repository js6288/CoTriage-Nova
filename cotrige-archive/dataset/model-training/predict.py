#!/usr/bin/env python3
"""Run local inference with the trained BGL classifier."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Optional, Tuple

import numpy as np
from transformers import AutoTokenizer


LABELS = {0: "NON_ALERT", 1: "ALERT"}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Predict NON_ALERT/ALERT for one log-template text.")
    parser.add_argument("--text", required=True)
    parser.add_argument("--backend", choices=["torch", "onnx"], default="torch")
    parser.add_argument("--model-dir", default="dataset/model-training/outputs/bgl-minilm/model")
    parser.add_argument("--onnx-dir", default="dataset/model-training/outputs/bgl-minilm/onnx")
    parser.add_argument("--metrics-file", default="dataset/model-training/outputs/bgl-minilm/metrics.json")
    parser.add_argument("--max-length", type=int, default=None)
    parser.add_argument("--threshold", type=float, default=None)
    return parser.parse_args()


def softmax(logits: np.ndarray) -> np.ndarray:
    shifted = logits - np.max(logits, axis=-1, keepdims=True)
    exp = np.exp(shifted)
    return exp / np.sum(exp, axis=-1, keepdims=True)


def load_torch_logits(model_dir: Path, text: str, max_length: int) -> Tuple[np.ndarray, dict]:
    try:
        import torch
        from transformers import AutoModelForSequenceClassification
    except ModuleNotFoundError as exc:
        missing = exc.name or "torch/transformers"
        raise SystemExit(
            f"missing dependency: {missing}. Install training dependencies with "
            "`pip install -r dataset/model-training/requirements.txt`."
        ) from exc

    tokenizer = AutoTokenizer.from_pretrained(model_dir)
    model = AutoModelForSequenceClassification.from_pretrained(model_dir)
    model.eval()
    encoded = tokenizer(
        text,
        truncation=True,
        max_length=max_length,
        padding="max_length",
        return_tensors="pt",
    )
    with torch.no_grad():
        outputs = model(input_ids=encoded["input_ids"], attention_mask=encoded["attention_mask"])
    return outputs.logits.detach().cpu().numpy(), encoded


def load_onnx_logits(onnx_dir: Path, text: str, max_length: int) -> Tuple[np.ndarray, dict]:
    try:
        import onnxruntime as ort
    except ModuleNotFoundError as exc:
        raise SystemExit(
            "missing dependency: onnxruntime. Install it with `pip install onnxruntime`, "
            "or install all model-training dependencies with "
            "`pip install -r dataset/model-training/requirements.txt`."
        ) from exc

    with (onnx_dir / "deployment.json").open("r", encoding="utf-8") as f:
        deployment = json.load(f)
    tokenizer = AutoTokenizer.from_pretrained(onnx_dir / deployment["tokenizer_dir"])
    encoded = tokenizer(
        text,
        truncation=True,
        max_length=max_length,
        padding="max_length",
        return_tensors="np",
    )
    session = ort.InferenceSession(str(onnx_dir / deployment["onnx_file"]), providers=["CPUExecutionProvider"])
    logits = session.run(
        ["logits"],
        {
            "input_ids": encoded["input_ids"].astype("int64"),
            "attention_mask": encoded["attention_mask"].astype("int64"),
        },
    )[0]
    return logits, encoded


def load_threshold(metrics_file: Path, onnx_dir: Path, override: Optional[float]) -> float:
    if override is not None:
        return override
    deployment_file = onnx_dir / "deployment.json"
    if deployment_file.exists():
        with deployment_file.open("r", encoding="utf-8") as f:
            return float(json.load(f)["threshold"])
    with metrics_file.open("r", encoding="utf-8") as f:
        return float(json.load(f)["validation_best_threshold"]["threshold"])


def load_max_length(onnx_dir: Path, override: Optional[int]) -> int:
    if override is not None:
        return override
    deployment_file = onnx_dir / "deployment.json"
    if deployment_file.exists():
        with deployment_file.open("r", encoding="utf-8") as f:
            return int(json.load(f)["max_length"])
    return 128


def main() -> None:
    args = parse_args()
    onnx_dir = Path(args.onnx_dir)
    threshold = load_threshold(Path(args.metrics_file), onnx_dir, args.threshold)
    max_length = load_max_length(onnx_dir, args.max_length)

    if args.backend == "torch":
        logits, _ = load_torch_logits(Path(args.model_dir), args.text, max_length)
    else:
        logits, _ = load_onnx_logits(onnx_dir, args.text, max_length)

    probs = softmax(logits)[0]
    alert_prob = float(probs[1])
    label_id = 1 if alert_prob >= threshold else 0
    result = {
        "backend": args.backend,
        "text": args.text,
        "label": LABELS[label_id],
        "label_id": label_id,
        "alert_probability": round(alert_prob, 6),
        "threshold": threshold,
        "probabilities": {
            "NON_ALERT": round(float(probs[0]), 6),
            "ALERT": round(float(probs[1]), 6),
        },
    }
    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
