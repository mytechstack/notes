"""
Token Manager — Envoy ext_authz sidecar.

Envoy calls this service on every request via the ext_authz HTTP filter.
This service returns 200 + a cached Authorization header so Envoy can
inject it into the upstream request.

Token caching strategy:
  - Fetch once from the OAuth server
  - Reuse until REFRESH_BUFFER_SECS before JWT expiry
  - threading.Lock protects the cache under concurrent Envoy workers

Endpoints:
  GET  /health  — health + cache stats
  *    /*        — ext_authz check (any path/method Envoy sends)
"""

import os
import time
import json
import threading
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.request import urlopen, Request as URLRequest
from urllib.parse import urlencode
from urllib.error import URLError

OAUTH_SERVER_URL    = os.environ.get("OAUTH_SERVER_URL", "http://oauth-server:8082")
CLIENT_ID           = os.environ.get("CLIENT_ID", "envoy-proxy")
CLIENT_SECRET       = os.environ.get("CLIENT_SECRET", "envoy-secret")
REFRESH_BUFFER_SECS = int(os.environ.get("REFRESH_BUFFER_SECS", "60"))


class TokenCache:
    """Thread-safe token cache — fetches a new JWT only when the cached one is near expiry."""

    def __init__(self):
        self._lock        = threading.Lock()
        self._token       = None
        self._expires_at  = 0.0
        self._fetch_count = 0
        self._cache_hits  = 0

    def get(self) -> str:
        with self._lock:
            now = time.time()
            if self._token and now < self._expires_at - REFRESH_BUFFER_SECS:
                self._cache_hits += 1
                return self._token
            # Cache miss or approaching expiry — fetch fresh token
            self._token, expires_in = self._fetch_from_oauth()
            self._expires_at = now + expires_in
            self._fetch_count += 1
            print(
                f"[token-manager] Token refreshed "
                f"(fetch #{self._fetch_count}, expires in {expires_in}s, "
                f"cache hits so far: {self._cache_hits})",
                flush=True,
            )
            return self._token

    def stats(self) -> dict:
        with self._lock:
            remaining = max(0.0, self._expires_at - time.time())
            return {
                "token_cached":      self._token is not None,
                "fetch_count":       self._fetch_count,
                "cache_hits":        self._cache_hits,
                "expires_in_secs":   round(remaining),
            }

    def _fetch_from_oauth(self) -> tuple[str, int]:
        body = urlencode({
            "grant_type":    "client_credentials",
            "client_id":     CLIENT_ID,
            "client_secret": CLIENT_SECRET,
        }).encode()
        req = URLRequest(
            f"{OAUTH_SERVER_URL}/token",
            data=body,
            headers={"Content-Type": "application/x-www-form-urlencoded"},
            method="POST",
        )
        with urlopen(req, timeout=5) as resp:
            data = json.loads(resp.read())
        return data["access_token"], int(data.get("expires_in", 3600))


_cache = TokenCache()


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        pass  # suppress per-request noise; we log selectively

    def _send(self, status: int, body: dict, extra_headers: dict | None = None):
        raw = json.dumps(body).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        if extra_headers:
            for k, v in extra_headers.items():
                self.send_header(k, v)
        self.end_headers()
        self.wfile.write(raw)

    def do_GET(self):
        if self.path == "/health":
            self._send(200, {"status": "ok", "service": "token-manager", **_cache.stats()})
            return
        self._authz_check()

    def do_POST(self):   self._authz_check()
    def do_PUT(self):    self._authz_check()
    def do_DELETE(self): self._authz_check()
    def do_PATCH(self):  self._authz_check()

    def _authz_check(self):
        """
        Called by Envoy's ext_authz filter for every upstream request.
        Returns 200 + Authorization header so Envoy injects it into the
        forwarded request.  Returns 403 if the OAuth server is unreachable.
        """
        try:
            token = _cache.get()
            self._send(
                200,
                {"status": "ok"},
                extra_headers={
                    "authorization":          f"Bearer {token}",
                    "x-envoy-injected-token": "true",
                },
            )
        except (URLError, KeyError, Exception) as exc:
            print(f"[token-manager] ERROR: {exc}", flush=True)
            # 403 → Envoy blocks the request (failure_mode_allow: false)
            self._send(403, {"error": "token_unavailable", "detail": str(exc)})


if __name__ == "__main__":
    port = int(os.environ.get("PORT", 8083))
    print(f"[token-manager] Listening on :{port}", flush=True)
    print(f"[token-manager] OAuth server : {OAUTH_SERVER_URL}", flush=True)
    print(f"[token-manager] Refresh buffer: {REFRESH_BUFFER_SECS}s before expiry", flush=True)
    HTTPServer(("0.0.0.0", port), Handler).serve_forever()
