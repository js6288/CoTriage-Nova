#!/usr/bin/env python3
"""Export a trained BGL classifier to an ONNX deployment package."""

from __future__ import annotations

import argparse
import json
import shutil
from pathlib import Path

import torch
from transformers import AutoModelForSequenceClassification, AutoTokenizer


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Export a trained log classifier to ONNX.")
    parser.add_argument("--model-dir", default="dataset/model-training/outputs/bgl-minilm/model")
    parser.add_argument("--metrics-file", default="dataset/model-training/outputs/bgl-minilm/metrics.json")
    parser.add_argument("--output-dir", default="dataset/model-training/outputs/bgl-minilm/onnx")
    parser.add_argument("--opset", type=int, default=17)
    parser.add_argument("--max-length", type=int, default=128)
    parser.add_argument(
        "--dynamo",
        action="store_true",
        help="Use PyTorch's new dynamo ONNX exporter. Disabled by default to avoid extra exporter dependencies.",
    )
    parser.add_argument(
        "--sample-text",
        default="KERNEL FATAL rts panic! - stopping execution",
        help="Text used to trace ONNX input shapes.",
    )
    parser.add_argument("--overwrite-output-dir", action="store_true")
    return parser.parse_args()


def load_threshold(metrics_file: Path) -> float:
    with metrics_file.open("r", encoding="utf-8") as f:
        metrics = json.load(f)
    return float(metrics["validation_best_threshold"]["threshold"])


def write_json(path: Path, data: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False, indent=2)
        f.write("\n")


def main() -> None:
    args = parse_args()
    model_dir = Path(args.model_dir)
    metrics_file = Path(args.metrics_file)
    output_dir = Path(args.output_dir)

    if output_dir.exists() and args.overwrite_output_dir:
        shutil.rmtree(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    tokenizer = AutoTokenizer.from_pretrained(model_dir)
    model = AutoModelForSequenceClassification.from_pretrained(model_dir)
    model.eval()
    model.cpu()

    encoded = tokenizer(
        args.sample_text,
        truncation=True,
        max_length=args.max_length,
        padding="max_length",
        return_tensors="pt",
    )
    input_ids = encoded["input_ids"].to(dtype=torch.long)
    attention_mask = encoded["attention_mask"].to(dtype=torch.long)

    onnx_path = output_dir / "model.onnx"
    torch.onnx.export(
        model,
        (input_ids, attention_mask),
        onnx_path,
        input_names=["input_ids", "attention_mask"],
        output_names=["logits"],
        dynamic_axes={
            "input_ids": {0: "batch_size", 1: "sequence_length"},
            "attention_mask": {0: "batch_size", 1: "sequence_length"},
            "logits": {0: "batch_size"},
        },
        opset_version=args.opset,
        do_constant_folding=True,
        dynamo=args.dynamo,
    )

    tokenizer.save_pretrained(output_dir / "tokenizer")
    deployment = {
        "model_type": "bgl-template-binary-classifier",
        "backend": "onnxruntime",
        "source_model_dir": str(model_dir),
        "onnx_file": "model.onnx",
        "tokenizer_dir": "tokenizer",
        "max_length": args.max_length,
        "threshold": load_threshold(metrics_file),
        "labels": {"0": "NON_ALERT", "1": "ALERT"},
        "positive_label": "ALERT",
        "opset": args.opset,
    }
    write_json(output_dir / "deployment.json", deployment)

    if metrics_file.exists():
        shutil.copy2(metrics_file, output_dir / "training_metrics.json")

    print(f"saved ONNX model to {onnx_path}")
    print(f"saved deployment manifest to {output_dir / 'deployment.json'}")


if __name__ == "__main__":
    main()
