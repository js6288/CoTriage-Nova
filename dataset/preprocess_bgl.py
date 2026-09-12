#!/usr/bin/env python3
"""Build leakage-safe template-level BGL train/validation/test datasets."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sqlite3
import sys
from collections import Counter
from pathlib import Path
from typing import TextIO


NORMALIZERS = (
    (re.compile(r"(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\b"), "<UUID>"),
    (re.compile(r"\b(?:\d{1,3}\.){3}\d{1,3}\b"), "<IP>"),
    (re.compile(r"(?i)\b(?:[0-9a-f]{2}:){5,}[0-9a-f]{2}\b"), "<HEXSEQ>"),
    (re.compile(r"(?i)\b0x[0-9a-f]+\b"), "<HEX>"),
    (re.compile(r"(?i)\b(?=[0-9a-f]*[a-f])[0-9a-f]{8,}\b"), "<HEX>"),
    (re.compile(r"(?:/[A-Za-z0-9._-]+){2,}"), "<PATH>"),
    (re.compile(r"(?i)\b([a-z][a-z_-]*?)\d+\b"), r"\1<NUM>"),
    (re.compile(r"\b\d+\b"), "<NUM>"),
)


def normalize_message(message: str) -> str:
    normalized = message.strip()
    for pattern, replacement in NORMALIZERS:
        normalized = pattern.sub(replacement, normalized)
    return " ".join(normalized.split())


def template_id(template_text: str) -> str:
    return hashlib.sha256(template_text.encode("utf-8")).hexdigest()[:16]


def parse_bgl_line(line: str) -> dict[str, object] | None:
    parts = line.rstrip("\r\n").split(maxsplit=9)
    if len(parts) != 10:
        return None

    raw_label, timestamp, date, node, precise_time, repeated_node, record_type, component, level, message = parts
    normalized = normalize_message(message)
    if not normalized:
        return None

    is_alert = raw_label != "-"
    return {
        "raw_label": raw_label,
        "is_alert": is_alert,
        "timestamp": timestamp,
        "date": date,
        "node": node,
        "precise_time": precise_time,
        "repeated_node": repeated_node,
        "record_type": record_type,
        "component": component,
        "level": level,
        "message": message,
        "template_text": normalized,
        "template_id": template_id(normalized),
    }


def split_for_template(template: str, seed: str, train_ratio: float, validation_ratio: float) -> str:
    digest = hashlib.sha256(f"{seed}:{template}".encode("utf-8")).digest()
    value = int.from_bytes(digest[:8], "big") / float(1 << 64)
    if value < train_ratio:
        return "train"
    if value < train_ratio + validation_ratio:
        return "validation"
    return "test"


def initialize_database(connection: sqlite3.Connection) -> None:
    connection.execute("PRAGMA journal_mode=WAL")
    connection.execute("PRAGMA synchronous=NORMAL")
    connection.execute(
        """
        CREATE TABLE IF NOT EXISTS templates (
            template_id TEXT PRIMARY KEY,
            template_text TEXT NOT NULL,
            component TEXT NOT NULL,
            level TEXT NOT NULL,
            sample_message TEXT NOT NULL,
            first_timestamp TEXT NOT NULL,
            last_timestamp TEXT NOT NULL,
            normal_count INTEGER NOT NULL DEFAULT 0,
            alert_count INTEGER NOT NULL DEFAULT 0,
            first_alert_category TEXT
        )
        """
    )


def aggregate(input_path: Path, database_path: Path, max_lines: int | None, batch_size: int) -> dict[str, int]:
    if database_path.exists():
        database_path.unlink()

    connection = sqlite3.connect(database_path)
    initialize_database(connection)
    sql = """
        INSERT INTO templates (
            template_id, template_text, component, level, sample_message,
            first_timestamp, last_timestamp, normal_count, alert_count,
            first_alert_category
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(template_id) DO UPDATE SET
            last_timestamp = excluded.last_timestamp,
            normal_count = templates.normal_count + excluded.normal_count,
            alert_count = templates.alert_count + excluded.alert_count,
            first_alert_category = COALESCE(templates.first_alert_category, excluded.first_alert_category)
    """

    stats = {
        "total_lines": 0,
        "parsed_lines": 0,
        "malformed_lines": 0,
        "empty_message_lines": 0,
        "normal_lines": 0,
        "alert_lines": 0,
    }
    batch: list[tuple[object, ...]] = []
    try:
        with input_path.open("r", encoding="utf-8", errors="replace") as source:
            for line in source:
                if max_lines is not None and stats["total_lines"] >= max_lines:
                    break
                stats["total_lines"] += 1
                record = parse_bgl_line(line)
                if record is None:
                    if len(line.split()) == 9:
                        stats["empty_message_lines"] += 1
                    else:
                        stats["malformed_lines"] += 1
                    continue

                stats["parsed_lines"] += 1
                is_alert = bool(record["is_alert"])
                stats["alert_lines" if is_alert else "normal_lines"] += 1
                batch.append(
                    (
                        record["template_id"], record["template_text"], record["component"],
                        record["level"], record["message"], record["timestamp"], record["timestamp"],
                        0 if is_alert else 1, 1 if is_alert else 0,
                        record["raw_label"] if is_alert else None,
                    )
                )
                if len(batch) >= batch_size:
                    connection.executemany(sql, batch)
                    connection.commit()
                    batch.clear()
                    if stats["total_lines"] % 500_000 < batch_size:
                        print(f"processed {stats['total_lines']:,} lines", file=sys.stderr)
        if batch:
            connection.executemany(sql, batch)
            connection.commit()
    finally:
        connection.close()
    return stats


def export_datasets(
    database_path: Path,
    output_dir: Path,
    seed: str,
    train_ratio: float,
    validation_ratio: float,
    min_purity: float,
) -> dict[str, object]:
    output_dir.mkdir(parents=True, exist_ok=True)
    paths = {name: output_dir / f"{name}.jsonl" for name in ("train", "validation", "test", "ambiguous")}
    handles: dict[str, TextIO] = {name: path.open("w", encoding="utf-8") for name, path in paths.items()}
    split_counts: dict[str, Counter[str]] = {name: Counter() for name in paths}

    connection = sqlite3.connect(database_path)
    try:
        cursor = connection.execute(
            """
            SELECT template_id, template_text, component, level, sample_message,
                   first_timestamp, last_timestamp, normal_count, alert_count,
                   first_alert_category
            FROM templates ORDER BY template_id
            """
        )
        for row in cursor:
            tid, template, component, level, sample, first_ts, last_ts, normal_count, alert_count, category = row
            total = normal_count + alert_count
            label = 1 if alert_count > normal_count else 0
            purity = max(normal_count, alert_count) / total
            split = split_for_template(tid, seed, train_ratio, validation_ratio)
            destination = split if purity >= min_purity else "ambiguous"
            item = {
                "template_id": tid,
                "template_text": template,
                "text": f"{component} {level} {template}",
                "label": label,
                "label_name": "ALERT" if label else "NON_ALERT",
                "alert_category": category,
                "component": component,
                "level": level,
                "occurrences": total,
                "normal_count": normal_count,
                "alert_count": alert_count,
                "label_purity": round(purity, 6),
                "sample_message": sample,
                "first_timestamp": first_ts,
                "last_timestamp": last_ts,
                "split": split,
            }
            handles[destination].write(json.dumps(item, ensure_ascii=False) + "\n")
            split_counts[destination][item["label_name"]] += 1
            split_counts[destination]["templates"] += 1
            split_counts[destination]["occurrences"] += total
    finally:
        connection.close()
        for handle in handles.values():
            handle.close()

    return {name: dict(counts) for name, counts in split_counts.items()}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", type=Path, default=Path(__file__).with_name("BGL.log"))
    parser.add_argument("--output-dir", type=Path, default=Path(__file__).with_name("processed") / "bgl")
    parser.add_argument("--train-ratio", type=float, default=0.70)
    parser.add_argument("--validation-ratio", type=float, default=0.15)
    parser.add_argument("--min-purity", type=float, default=0.90)
    parser.add_argument("--seed", default="cotriage-bgl-v1")
    parser.add_argument("--batch-size", type=int, default=10_000)
    parser.add_argument("--max-lines", type=int, help="Process only the first N lines for a smoke test")
    parser.add_argument("--keep-database", action="store_true")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    if not args.input.is_file():
        print(f"input file not found: {args.input}", file=sys.stderr)
        return 2
    if not 0 < args.train_ratio < 1 or not 0 < args.validation_ratio < 1:
        print("train and validation ratios must be between 0 and 1", file=sys.stderr)
        return 2
    if args.train_ratio + args.validation_ratio >= 1:
        print("train_ratio + validation_ratio must be less than 1", file=sys.stderr)
        return 2
    if not 0.5 <= args.min_purity <= 1:
        print("min_purity must be between 0.5 and 1", file=sys.stderr)
        return 2

    args.output_dir.mkdir(parents=True, exist_ok=True)
    database_path = args.output_dir / "templates.sqlite3"
    ingest_stats = aggregate(args.input, database_path, args.max_lines, args.batch_size)
    split_stats = export_datasets(
        database_path, args.output_dir, args.seed,
        args.train_ratio, args.validation_ratio, args.min_purity,
    )
    report = {
        "input": str(args.input),
        "seed": args.seed,
        "train_ratio": args.train_ratio,
        "validation_ratio": args.validation_ratio,
        "test_ratio": round(1 - args.train_ratio - args.validation_ratio, 6),
        "min_purity": args.min_purity,
        "ingest": ingest_stats,
        "splits": split_stats,
    }
    (args.output_dir / "dataset_stats.json").write_text(
        json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    if not args.keep_database:
        database_path.unlink(missing_ok=True)
        Path(str(database_path) + "-wal").unlink(missing_ok=True)
        Path(str(database_path) + "-shm").unlink(missing_ok=True)
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
