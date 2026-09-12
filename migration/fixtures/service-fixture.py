from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json

class Fixture(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path.startswith("/api/v1/nodes"):
            node = {"apiVersion": "v1", "kind": "Node", "metadata": {"name": "fixture-node", "labels": {"cotriage.io/equipment": "image-smoke"}}, "status": {"conditions": [{"type": "Ready", "status": "True"}]}}
            result = {"apiVersion": "v1", "kind": "NodeList", "metadata": {}, "items": [node]} if self.path == "/api/v1/nodes" or self.path.startswith("/api/v1/nodes?") else node
        elif "/pods" in self.path:
            result = {"apiVersion": "v1", "kind": "PodList", "metadata": {}, "items": []}
        else:
            result = {}
        self.reply(result)
    def do_POST(self):
        self.rfile.read(int(self.headers.get("Content-Length", "0")))
        if self.path == "/predict":
            result = {"label": "NON_ALERT", "label_id": 0, "alert_probability": 0.01, "threshold": 0.5, "probabilities": {"NON_ALERT": 0.99, "ALERT": 0.01}, "model_version": "image-fixture"}
        else:
            diagnosis = {"pred": "NET_ERROR", "root_cause": "fixture", "recommendation": "check network", "confidence": 0.9, "evidence": ["fixture"]}
            result = {"choices": [{"message": {"content": json.dumps(diagnosis)}}], "usage": {"total_tokens": 20}}
        self.reply(result)
    def reply(self, obj):
        body = json.dumps(obj).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)
    def log_message(self, *_): pass

ThreadingHTTPServer(("0.0.0.0", 18080), Fixture).serve_forever()
