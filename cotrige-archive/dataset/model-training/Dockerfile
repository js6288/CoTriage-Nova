FROM docker.xuanyuan.run/library/python:3.11-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    HF_HUB_OFFLINE=1 \
    TRANSFORMERS_OFFLINE=1 \
    TOKENIZERS_PARALLELISM=false

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY requirements-serve.txt /app/requirements-serve.txt
RUN pip install --no-cache-dir -r /app/requirements-serve.txt

COPY serve_onnx.py /app/serve_onnx.py
COPY outputs/bgl-minilm/onnx /app/onnx

EXPOSE 18080

CMD ["python", "/app/serve_onnx.py", "--onnx-dir", "/app/onnx", "--host", "0.0.0.0", "--port", "18080"]
