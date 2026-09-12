Linux logs are usually located at /var/log/. The dataset was collected from /var/log/messages on a Linux server over a period of 260+ days, as part of the Public Security Log Sharing Site project.

Apache HTTP Server (https://httpd.apache.org) is one of the most popular web servers. Apache servers usually generate two types of logs: access logs and error logs. We provide an error log for the purpose of research on anomaly detection and diagnosis. The log file was collected from a Linux system running Apache Web server.

ZooKeeper (https://zookeeper.apache.org) is a centralized service for maintaining configuration information, naming, providing distributed synchronization, and providing group services. The log set was collected by aggregating logs from the ZooKeeper service in our lab at CUHK, which comprises a total of 32 machines. The logs cover a time period of more than 26 days.

BGL is an open dataset of logs collected from a BlueGene/L supercomputer system at Lawrence Livermore National Labs (LLNL) in Livermore, California, with 131,072 processors and 32,768GB memory. The log contains alert and non-alert messages identified by alert category tags. In the first column of the log, "-" indicates non-alert messages while others are alert messages. The label information is amenable to alert detection and prediction research. It has been used in several studies on log parsing, anomaly detection, and failure prediction.

For more detailed information, please visit the project page: https://www.usenix.org/cfdr-data#hpc4.

Citation:
Jieming Zhu, Shilin He, Pinjia He, Jinyang Liu, Michael R. Lyu. Loghub: A Large Collection of System Log Datasets for AI-driven Log Analytics. IEEE International Symposium on Software Reliability Engineering (ISSRE), 2023.

## BGL preprocessing for Cotriage

`preprocess_bgl.py` converts the raw BGL file into leakage-safe, template-level binary classification datasets.

The first BGL column is used only as the target:

- `-` -> `NON_ALERT` (`label=0`)
- any alert category -> `ALERT` (`label=1`)

The label column, timestamps, and node IDs are not included in model text. The model input has this form:

```text
<component> <level> <normalized message template>
```

Run the full preprocessing job from the repository root:

```bash
python3 dataset/preprocess_bgl.py
```

Run a smoke test on the first 10,000 lines:

```bash
python3 dataset/preprocess_bgl.py \
  --max-lines 10000 \
  --output-dir dataset/processed/bgl-smoke
```

Default outputs under `dataset/processed/bgl/`:

```text
train.jsonl
validation.jsonl
test.jsonl
ambiguous.jsonl
dataset_stats.json
```

The program aggregates repeated logs by normalized `template_id`. A template is assigned to exactly one split using a deterministic hash, preventing the same template from appearing in both training and evaluation data. Templates with label purity below `0.90` are written to `ambiguous.jsonl` instead of the training files.

Records containing metadata but no message body are counted as `empty_message_lines` and excluded because they provide no semantic input for a text classifier.

Useful options:

```text
--train-ratio 0.70
--validation-ratio 0.15
--min-purity 0.90
--seed cotriage-bgl-v1
--keep-database
```

Run tests:

```bash
python3 -m unittest discover -s dataset/tests -v
```

Raw datasets and generated training artifacts are intentionally ignored by Git. Only this README, preprocessing programs, tests, and example configuration files should be committed.
