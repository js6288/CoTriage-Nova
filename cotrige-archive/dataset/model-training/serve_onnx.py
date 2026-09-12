#!/usr/bin/env python3
"""Serve the ONNX BGL classifier over a tiny HTTP API."""

from __future__ import annotations

import argparse
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any, Dict

import numpy as np
import onnxruntime as ort
from transformers import AutoTokenizer


def softmax(logits: np.ndarray) -> np.ndarray:
    shifted = logits - np.max(logits, axis=-1, keepdims=True)
    exp = np.exp(shifted)
    return exp / np.sum(exp, axis=-1, keepdims=True)


class ClassifierService:
    def __init__(self, onnx_dir: Path):
        with (onnx_dir / "deployment.json").open("r", encoding="utf-8") as f:
            self.deployment = json.load(f)
        self.onnx_dir = onnx_dir
        self.max_length = int(self.deployment["max_length"])
        self.threshold = float(self.deployment["threshold"])
        self.model_version = self.deployment.get("source_model_dir", "bgl-minilm")
        self.tokenizer = AutoTokenizer.from_pretrained(onnx_dir / self.deployment["tokenizer_dir"])
        self.session = ort.InferenceSession(
            str(onnx_dir / self.deployment["onnx_file"]),
            providers=["CPUExecutionProvider"],
        )

    def predict(self, text: str) -> Dict[str, Any]:
        encoded = self.tokenizer(
            text,
            truncation=True,
            max_length=self.max_length,
            padding="max_length",
            return_tensors="np",
        )
        logits = self.session.run(
            ["logits"],
            {
                "input_ids": encoded["input_ids"].astype("int64"),
                "attention_mask": encoded["attention_mask"].astype("int64"),
            },
        )[0]
        probs = softmax(logits)[0]
        alert_probability = float(probs[1])
        label_id = 1 if alert_probability >= self.threshold else 0
        label = "ALERT" if label_id == 1 else "NON_ALERT"
        return {
            "label": label,
            "label_id": label_id,
            "alert_probability": round(alert_probability, 6),
            "threshold": self.threshold,
            "probabilities": {
                "NON_ALERT": round(float(probs[0]), 6),
                "ALERT": round(float(probs[1]), 6),
            },
            "model_version": self.model_version,
        }


def make_handler(service: ClassifierService):
    class Handler(BaseHTTPRequestHandler):
        def do_GET(self) -> None:
            if self.path != "/healthz":
                self.send_error(404)
                return
            self.write_json({"ok": True, "model_version": service.model_version})

        def do_POST(self) -> None:
            if self.path != "/predict":
                self.send_error(404)
                return
            try:
                length = int(self.headers.get("Content-Length", "0"))
                payload = json.loads(self.rfile.read(length).decode("utf-8"))
                text = payload.get("template_text") or payload.get("text")
                if not isinstance(text, str) or not text.strip():
                    self.send_error(400, "template_text or text is required")
                    return
                result = service.predict(text)
                result["template_id"] = payload.get("template_id", "")
                result["event_id"] = payload.get("event_id", "")
                self.write_json(result)
            except Exception as exc:  # pragma: no cover - defensive server boundary
                self.send_error(500, str(exc))

        def log_message(self, fmt: str, *args: Any) -> None:
            return

        def write_json(self, payload: Dict[str, Any]) -> None:
            body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

    return Handler


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Serve MiniLM ONNX classifier for edge-infer.")
    parser.add_argument("--onnx-dir", default="dataset/model-training/outputs/bgl-minilm/onnx")
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=18080)
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    service = ClassifierService(Path(args.onnx_dir))
    server = ThreadingHTTPServer((args.host, args.port), make_handler(service))
    print(f"serving ONNX classifier at http://{args.host}:{args.port}/predict")
    server.serve_forever()


if __name__ == "__main__":
    main()
