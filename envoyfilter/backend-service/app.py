"""
Mock backend service.

Receives requests forwarded by Envoy, validates the injected OAuth token,
and returns a detailed JSON response showing all request headers and token claims.

GET|POST /api/*
GET      /health
"""

import json
import time
import hashlib
import hmac
import base64
import os
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse

SECRET_KEY = os.environ.get("JWT_SECRET", "super-secret-key-change-in-prod")


def _b64url_decode(s: str) -> bytes:
    pad = 4 - len(s) % 4
    return base64.urlsafe_b64decode(s + "=" * pad)


def verify_jwt(token: str) -> dict:
    parts = token.split(".")
    if len(parts) != 3:
        raise ValueError("Invalid token format")
    header_enc, payload_enc, sig_enc = parts
    signing_input = f"{header_enc}.{payload_enc}".encode()
    expected_sig = hmac.new(SECRET_KEY.encode(), signing_input, hashlib.sha256).digest()
    actual_sig = _b64url_decode(sig_enc)
    if not hmac.compare_digest(expected_sig, actual_sig):
        raise ValueError("Invalid signature")
    payload = json.loads(_b64url_decode(payload_enc))
    if payload["exp"] < int(time.time()):
        raise ValueError("Token expired")
    return payload


class BackendHandler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        print(f"[backend-service] {fmt % args}", flush=True)

    def _send_json(self, status: int, data: dict):
        body = json.dumps(data, indent=2).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _handle(self):
        path = urlparse(self.path).path

        if path == "/health":
            self._send_json(200, {"status": "ok", "service": "backend-service"})
            return

        # Collect all incoming headers
        headers_dict = {k.lower(): v for k, v in self.headers.items()}

        # Validate the Authorization token injected by Envoy
        auth = headers_dict.get("authorization", "")
        token_status = {}
        if auth.startswith("Bearer "):
            token = auth[7:]
            try:
                claims = verify_jwt(token)
                token_status = {
                    "valid": True,
                    "claims": claims,
                    "injected_by_envoy": headers_dict.get("x-envoy-injected-token") == "true",
                }
                print(f"[backend-service] Valid token — sub={claims.get('sub')} path={path}", flush=True)
            except ValueError as e:
                token_status = {"valid": False, "error": str(e)}
                print(f"[backend-service] Invalid token: {e}", flush=True)
        else:
            token_status = {"valid": False, "error": "No Bearer token present"}
            print(f"[backend-service] No token on request to {path}", flush=True)

        # Read body if present
        length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(length).decode() if length else None

        self._send_json(200, {
            "service": "backend-service",
            "request": {
                "method": self.command,
                "path": path,
                "headers": headers_dict,
                "body": body,
            },
            "token_validation": token_status,
            "message": (
                "Request authenticated via Envoy-injected OAuth token"
                if token_status.get("valid")
                else "WARNING: Token validation failed"
            ),
        })

    def do_GET(self):
        self._handle()

    def do_POST(self):
        self._handle()

    def do_PUT(self):
        self._handle()

    def do_DELETE(self):
        self._handle()


if __name__ == "__main__":
    port = int(os.environ.get("PORT", 8081))
    server = HTTPServer(("0.0.0.0", port), BackendHandler)
    print(f"[backend-service] Listening on port {port}", flush=True)
    server.serve_forever()
