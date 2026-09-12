#!/usr/bin/env python3
"""Train a lightweight BGL log-template binary classifier.

The script intentionally uses a small custom PyTorch loop instead of the
Hugging Face Trainer so memory use is explicit on a 10c/16GB Mac.
"""

from __future__ import annotations

import argparse
import json
import math
import random
import shutil
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Dict, List, Optional, Sequence, Tuple

import numpy as np
import torch
from sklearn.metrics import (
    average_precision_score,
    confusion_matrix,
    precision_recall_fscore_support,
    roc_auc_score,
)
from torch.utils.data import DataLoader, Dataset
from tqdm.auto import tqdm
from transformers import AutoModelForSequenceClassification, AutoTokenizer, get_linear_schedule_with_warmup


LABEL_NAMES = {0: "NON_ALERT", 1: "ALERT"}


@dataclass
class TrainConfig:
    model_name: str
    data_dir: str
    output_dir: str
    epochs: int
    batch_size: int
    learning_rate: float
    weight_decay: float
    max_length: int
    warmup_ratio: float
    class_weight: str
    patience: int
    seed: int
    device: str


class LogTemplateDataset(Dataset):
    def __init__(self, records: Sequence[dict], tokenizer, max_length: int):
        self.records = list(records)
        self.tokenizer = tokenizer
        self.max_length = max_length

    def __len__(self) -> int:
        return len(self.records)

    def __getitem__(self, index: int) -> dict:
        item = self.records[index]
        encoded = self.tokenizer(
            item["text"],
            truncation=True,
            max_length=self.max_length,
            padding="max_length",
            return_tensors="pt",
        )
        return {
            "input_ids": encoded["input_ids"].squeeze(0),
            "attention_mask": encoded["attention_mask"].squeeze(0),
            "labels": torch.tensor(int(item["label"]), dtype=torch.long),
        }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Train a BGL log-template binary classifier.")
    parser.add_argument("--model-name", default="sentence-transformers/all-MiniLM-L6-v2")
    parser.add_argument("--data-dir", default="dataset/processed/bgl")
    parser.add_argument("--output-dir", default="dataset/model-training/outputs/bgl-minilm")
    parser.add_argument("--epochs", type=int, default=20)
    parser.add_argument("--batch-size", type=int, default=16)
    parser.add_argument("--learning-rate", type=float, default=2e-5)
    parser.add_argument("--weight-decay", type=float, default=0.01)
    parser.add_argument("--max-length", type=int, default=128)
    parser.add_argument("--warmup-ratio", type=float, default=0.1)
    parser.add_argument("--class-weight", choices=["none", "auto"], default="auto")
    parser.add_argument("--patience", type=int, default=5)
    parser.add_argument("--seed", type=int, default=20260623)
    parser.add_argument("--device", choices=["auto", "cpu", "mps", "cuda"], default="auto")
    parser.add_argument("--overwrite-output-dir", action="store_true")
    return parser.parse_args()


def set_seed(seed: int) -> None:
    random.seed(seed)
    np.random.seed(seed)
    torch.manual_seed(seed)
    if torch.cuda.is_available():
        torch.cuda.manual_seed_all(seed)


def select_device(requested: str) -> torch.device:
    if requested != "auto":
        return torch.device(requested)
    if torch.cuda.is_available():
        return torch.device("cuda")
    if hasattr(torch.backends, "mps") and torch.backends.mps.is_available():
        return torch.device("mps")
    return torch.device("cpu")


def load_jsonl(path: Path) -> List[dict]:
    records: List[dict] = []
    with path.open("r", encoding="utf-8") as f:
        for line_no, line in enumerate(f, start=1):
            if not line.strip():
                continue
            item = json.loads(line)
            if "text" not in item or "label" not in item:
                raise ValueError(f"{path}:{line_no} must contain text and label")
            records.append(item)
    if not records:
        raise ValueError(f"{path} is empty")
    return records


def label_counts(records: Sequence[dict]) -> Dict[int, int]:
    counts = {0: 0, 1: 0}
    for item in records:
        counts[int(item["label"])] += 1
    return counts


def make_class_weights(records: Sequence[dict], mode: str, device: torch.device) -> Optional[torch.Tensor]:
    if mode == "none":
        return None
    counts = label_counts(records)
    total = sum(counts.values())
    weights = [total / (2 * max(counts[label], 1)) for label in [0, 1]]
    return torch.tensor(weights, dtype=torch.float32, device=device)


def collate_batch(batch: Sequence[dict]) -> dict:
    return {
        "input_ids": torch.stack([x["input_ids"] for x in batch]),
        "attention_mask": torch.stack([x["attention_mask"] for x in batch]),
        "labels": torch.stack([x["labels"] for x in batch]),
    }


def move_batch(batch: dict, device: torch.device) -> dict:
    return {key: value.to(device) for key, value in batch.items()}


def compute_loss(logits: torch.Tensor, labels: torch.Tensor, class_weights: Optional[torch.Tensor]) -> torch.Tensor:
    return torch.nn.functional.cross_entropy(logits, labels, weight=class_weights)


@torch.no_grad()
def predict(model, loader: DataLoader, device: torch.device, class_weights: Optional[torch.Tensor]) -> Tuple[np.ndarray, np.ndarray, float]:
    model.eval()
    all_labels: List[int] = []
    all_probs: List[float] = []
    total_loss = 0.0
    total_items = 0
    for batch in loader:
        batch = move_batch(batch, device)
        outputs = model(input_ids=batch["input_ids"], attention_mask=batch["attention_mask"])
        loss = compute_loss(outputs.logits, batch["labels"], class_weights)
        probs = torch.softmax(outputs.logits, dim=-1)[:, 1]
        labels = batch["labels"]
        all_probs.extend(probs.detach().cpu().numpy().tolist())
        all_labels.extend(labels.detach().cpu().numpy().tolist())
        total_loss += float(loss.item()) * labels.size(0)
        total_items += labels.size(0)
    return np.array(all_labels), np.array(all_probs), total_loss / max(total_items, 1)


def metrics_at_threshold(labels: np.ndarray, probs: np.ndarray, threshold: float) -> dict:
    preds = (probs >= threshold).astype(int)
    precision, recall, f1, _ = precision_recall_fscore_support(
        labels,
        preds,
        average="binary",
        pos_label=1,
        zero_division=0,
    )
    cm = confusion_matrix(labels, preds, labels=[0, 1])
    result = {
        "threshold": round(float(threshold), 6),
        "precision": round(float(precision), 6),
        "recall": round(float(recall), 6),
        "f1": round(float(f1), 6),
        "confusion_matrix": {
            "labels": ["NON_ALERT", "ALERT"],
            "matrix": cm.astype(int).tolist(),
        },
    }
    if len(set(labels.tolist())) == 2:
        result["pr_auc"] = round(float(average_precision_score(labels, probs)), 6)
        result["roc_auc"] = round(float(roc_auc_score(labels, probs)), 6)
    else:
        result["pr_auc"] = None
        result["roc_auc"] = None
    return result


def choose_threshold(labels: np.ndarray, probs: np.ndarray) -> dict:
    best = None
    for threshold in np.linspace(0.05, 0.95, 181):
        item = metrics_at_threshold(labels, probs, float(threshold))
        if best is None:
            best = item
            continue
        # Prefer F1, then recall, then precision for anomaly screening.
        current_key = (item["f1"], item["recall"], item["precision"])
        best_key = (best["f1"], best["recall"], best["precision"])
        if current_key > best_key:
            best = item
    assert best is not None
    return best


def write_json(path: Path, data: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False, indent=2)
        f.write("\n")


def append_jsonl(path: Path, data: dict) -> None:
    with path.open("a", encoding="utf-8") as f:
        f.write(json.dumps(data, ensure_ascii=False) + "\n")


def train() -> None:
    args = parse_args()
    set_seed(args.seed)

    output_dir = Path(args.output_dir)
    if output_dir.exists() and args.overwrite_output_dir:
        shutil.rmtree(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    history_path = output_dir / "history.jsonl"
    if history_path.exists():
        history_path.unlink()

    data_dir = Path(args.data_dir)
    train_records = load_jsonl(data_dir / "train.jsonl")
    val_records = load_jsonl(data_dir / "validation.jsonl")
    test_records = load_jsonl(data_dir / "test.jsonl")

    device = select_device(args.device)
    config = TrainConfig(
        model_name=args.model_name,
        data_dir=str(data_dir),
        output_dir=str(output_dir),
        epochs=args.epochs,
        batch_size=args.batch_size,
        learning_rate=args.learning_rate,
        weight_decay=args.weight_decay,
        max_length=args.max_length,
        warmup_ratio=args.warmup_ratio,
        class_weight=args.class_weight,
        patience=args.patience,
        seed=args.seed,
        device=str(device),
    )

    tokenizer = AutoTokenizer.from_pretrained(args.model_name)
    model = AutoModelForSequenceClassification.from_pretrained(
        args.model_name,
        num_labels=2,
        id2label={0: "NON_ALERT", 1: "ALERT"},
        label2id={"NON_ALERT": 0, "ALERT": 1},
    )
    model.to(device)

    train_dataset = LogTemplateDataset(train_records, tokenizer, args.max_length)
    val_dataset = LogTemplateDataset(val_records, tokenizer, args.max_length)
    test_dataset = LogTemplateDataset(test_records, tokenizer, args.max_length)

    train_loader = DataLoader(train_dataset, batch_size=args.batch_size, shuffle=True, collate_fn=collate_batch)
    val_loader = DataLoader(val_dataset, batch_size=args.batch_size, shuffle=False, collate_fn=collate_batch)
    test_loader = DataLoader(test_dataset, batch_size=args.batch_size, shuffle=False, collate_fn=collate_batch)

    class_weights = make_class_weights(train_records, args.class_weight, device)
    optimizer = torch.optim.AdamW(model.parameters(), lr=args.learning_rate, weight_decay=args.weight_decay)
    total_steps = max(len(train_loader) * args.epochs, 1)
    warmup_steps = int(math.ceil(total_steps * args.warmup_ratio))
    scheduler = get_linear_schedule_with_warmup(optimizer, warmup_steps, total_steps)

    best_val_f1 = -1.0
    best_epoch = 0
    stale_epochs = 0
    best_dir = output_dir / "model"

    for epoch in range(1, args.epochs + 1):
        model.train()
        running_loss = 0.0
        seen = 0
        progress = tqdm(train_loader, desc=f"epoch {epoch}/{args.epochs}", leave=False)
        for batch in progress:
            batch = move_batch(batch, device)
            optimizer.zero_grad(set_to_none=True)
            outputs = model(input_ids=batch["input_ids"], attention_mask=batch["attention_mask"])
            loss = compute_loss(outputs.logits, batch["labels"], class_weights)
            loss.backward()
            torch.nn.utils.clip_grad_norm_(model.parameters(), max_norm=1.0)
            optimizer.step()
            scheduler.step()

            batch_size = batch["labels"].size(0)
            running_loss += float(loss.item()) * batch_size
            seen += batch_size
            progress.set_postfix(loss=running_loss / max(seen, 1))

        val_labels, val_probs, val_loss = predict(model, val_loader, device, class_weights)
        val_best = choose_threshold(val_labels, val_probs)
        epoch_item = {
            "epoch": epoch,
            "train_loss": round(running_loss / max(seen, 1), 6),
            "validation_loss": round(val_loss, 6),
            "validation": val_best,
        }
        append_jsonl(history_path, epoch_item)
        print(json.dumps(epoch_item, ensure_ascii=False))

        if val_best["f1"] > best_val_f1:
            best_val_f1 = val_best["f1"]
            best_epoch = epoch
            stale_epochs = 0
            model.save_pretrained(best_dir)
            tokenizer.save_pretrained(best_dir)
        else:
            stale_epochs += 1
            if stale_epochs >= args.patience:
                print(f"early stopping at epoch {epoch}; best epoch is {best_epoch}")
                break

    model = AutoModelForSequenceClassification.from_pretrained(best_dir)
    model.to(device)
    val_labels, val_probs, _ = predict(model, val_loader, device, class_weights)
    threshold_info = choose_threshold(val_labels, val_probs)
    threshold = float(threshold_info["threshold"])
    test_labels, test_probs, test_loss = predict(model, test_loader, device, class_weights)
    test_metrics = metrics_at_threshold(test_labels, test_probs, threshold)

    metrics = {
        "config": asdict(config),
        "labels": LABEL_NAMES,
        "data": {
            "train": label_counts(train_records),
            "validation": label_counts(val_records),
            "test": label_counts(test_records),
        },
        "best_epoch": best_epoch,
        "validation_best_threshold": threshold_info,
        "test_loss": round(float(test_loss), 6),
        "test": test_metrics,
    }
    write_json(output_dir / "metrics.json", metrics)
    write_json(output_dir / "label_mapping.json", {"id2label": LABEL_NAMES, "label2id": {"NON_ALERT": 0, "ALERT": 1}})
    print(f"saved model to {best_dir}")
    print(f"saved metrics to {output_dir / 'metrics.json'}")


if __name__ == "__main__":
    train()
