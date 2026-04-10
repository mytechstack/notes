"""
Mock OAuth 2.0 server — client_credentials grant only.

POST /token
  body: grant_type=client_credentials&client_id=<id>&client_secret=<secret>
  returns: {"access_token": "<jwt>", "token_type": "Bearer", "expires_in": 3600}

GET /verify
  header: Authorization: Bearer <jwt>
  returns: decoded claims (used by backend to verify the token)

GET /health
  returns: {"status": "ok"}
"""

import time
import json
import hashlib
import hmac
import base64
import os
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import parse_qs, urlparse

# ── JWT helpers (no external deps) ──────────────────────────────────────────

SECRET_KEY = os.environ.get("JWT_SECRET", "super-secret-key-change-in-prod")

VALID_CLIENTS = {
    "envoy-proxy": "envoy-secret",
}


def _b64url_encode(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode()


def _b64url_decode(s: str) -> bytes:
    pad = 4 - len(s) % 4
    return base64.urlsafe_b64decode(s + "=" * pad)


def create_jwt(client_id: str, expires_in: int = 3600) -> str:
    now = int(time.time())
    header = {"alg": "HS256", "typ": "JWT"}
    payload = {
        "iss": "mock-oauth-server",
        "sub": client_id,
        "aud": "backend-service",
        "iat": now,
        "exp": now + expires_in,
        "client_id": client_id,
        "scope": "read write",
    }
    header_enc = _b64url_encode(json.dumps(header, separators=(",", ":")).encode())
    payload_enc = _b64url_encode(json.dumps(payload, separators=(",", ":")).encode())
    signing_input = f"{header_enc}.{payload_enc}".encode()
    sig = hmac.new(SECRET_KEY.encode(), signing_input, hashlib.sha256).digest()
    return f"{header_enc}.{payload_enc}.{_b64url_encode(sig)}"


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


# ── Request handler ──────────────────────────────────────────────────────────

class OAuthHandler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        print(f"[oauth-server] {fmt % args}", flush=True)

    def _send_json(self, status: int, data: dict):
        body = json.dumps(data).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        path = urlparse(self.path).path

        if path == "/health":
            self._send_json(200, {"status": "ok", "service": "oauth-server"})

        elif path == "/verify":
            auth = self.headers.get("Authorization", "")
            if not auth.startswith("Bearer "):
                self._send_json(401, {"error": "missing_token"})
                return
            token = auth[7:]
            try:
                claims = verify_jwt(token)
                self._send_json(200, {"valid": True, "claims": claims})
            except ValueError as e:
                self._send_json(401, {"valid": False, "error": str(e)})
        else:
            self._send_json(404, {"error": "not_found"})

    def do_POST(self):
        path = urlparse(self.path).path

        if path == "/token":
            length = int(self.headers.get("Content-Length", 0))
            raw = self.rfile.read(length).decode()
            params = {k: v[0] for k, v in parse_qs(raw).items()}

            grant_type = params.get("grant_type", "")
            client_id = params.get("client_id", "")
            client_secret = params.get("client_secret", "")

            if grant_type != "client_credentials":
                self._send_json(400, {"error": "unsupported_grant_type"})
                return

            expected_secret = VALID_CLIENTS.get(client_id)
            if expected_secret is None or expected_secret != client_secret:
                self._send_json(401, {"error": "invalid_client"})
                return

            token = create_jwt(client_id)
            print(f"[oauth-server] Issued token for client_id={client_id}", flush=True)
            self._send_json(200, {
                "access_token": token,
                "token_type": "Bearer",
                "expires_in": 3600,
                "scope": "read write",
            })
        else:
            self._send_json(404, {"error": "not_found"})


if __name__ == "__main__":
    port = int(os.environ.get("PORT", 8082))
    server = HTTPServer(("0.0.0.0", port), OAuthHandler)
    print(f"[oauth-server] Listening on port {port}", flush=True)
    server.serve_forever()
