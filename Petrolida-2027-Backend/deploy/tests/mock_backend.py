"""Synthetic upstream used only by the Docker proxy integration checks."""

import json
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlsplit


class Handler(BaseHTTPRequestHandler):
    counts = {}
    lock = threading.Lock()

    def respond(self):
        query = parse_qs(urlsplit(self.path).query)
        with self.lock:
            count = self.counts[self.path] = self.counts.get(self.path, 0) + 1
        time.sleep(float(query.get("delay", ["0"])[0]))
        if self.headers.get("Content-Length"):
            self.rfile.read(int(self.headers["Content-Length"]))
        body = json.dumps({"path": self.path, "count": count}).encode()
        self.send_response(int(query.get("status", ["200"])[0]))
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("X-Request-ID", self.headers.get("X-Request-ID", "upstream-id"))
        if "policy" in query:
            self.send_header("Cache-Control", query["policy"][0])
        if "set_cookie" in query:
            self.send_header("Set-Cookie", "fixture=only-for-tests; HttpOnly")
        if self.headers.get("Origin") == "https://allowed.example":
            self.send_header("Access-Control-Allow-Origin", "https://allowed.example")
            self.send_header("Vary", "Origin")
        self.end_headers()
        if self.command != "HEAD":
            self.wfile.write(body)

    do_GET = respond
    do_HEAD = respond
    do_POST = respond
    do_OPTIONS = respond
    do_PATCH = respond

    def log_message(self, *_):
        pass


if __name__ == "__main__":
    for port in (8998, 3000):
        server = ThreadingHTTPServer(("0.0.0.0", port), Handler)
        threading.Thread(target=server.serve_forever, daemon=True).start()
    threading.Event().wait()
