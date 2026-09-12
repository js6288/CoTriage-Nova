# CoTriage experiment bundle

`run_cloud_edge_comparison.py` executes the frozen BGL test split against the deployed ONNX classifier, obtains real RAISE decisions from `router-controller`, and submits selected requests to the deployed `cloud-gateway`.

The runner uses experiment-prefixed event IDs and does not delete Redis or MySQL data. Raw per-sample records are written to `raw/`; aggregate results are written to `summary/`.

The current fixed baseline escalates events whose edge confidence is below `0.80`. Cloud predictions are mapped to the BGL binary task as `NORMAL -> NON_ALERT`, and every other allowed diagnosis class -> `ALERT`. This mapping and the fact that the BGL test split has only nine alert templates must accompany any published result.
