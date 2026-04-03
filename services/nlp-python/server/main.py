from http.server import BaseHTTPRequestHandler, HTTPServer
import json
import os


EMBEDDING_MODEL = os.getenv("EMBEDDING_MODEL", "all-MiniLM-L6-v2")
EMBEDDING_DIM = int(os.getenv("EMBEDDING_DIM", "384"))
PIPELINE_VERSION = os.getenv("PIPELINE_VERSION", "v0")


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path != "/health":
            self.send_response(404)
            self.end_headers()
            return

        payload = {
            "service": "nlp-python",
            "status": "ok",
            "pipelineVersion": PIPELINE_VERSION,
            "embeddingModel": EMBEDDING_MODEL,
            "embeddingDim": EMBEDDING_DIM,
        }

        body = json.dumps(payload).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, format, *_args):
        return


def main():
    port = int(os.getenv("NLP_HTTP_PORT", "8081"))
    server = HTTPServer(("127.0.0.1", port), Handler)
    print(f"nlp-python listening on :{port}")
    server.serve_forever()


if __name__ == "__main__":
    main()
