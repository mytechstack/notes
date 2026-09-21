#!/usr/bin/env bash
# Walks through WRCS's config-management capabilities (RFC capabilities
# #2-4, #9-10: immutable versioning, blue-green slots, rollback, diff) and
# shows the Web Runtime Service picking up changes without a restart.
#
# Requires: curl, jq. Run `docker compose up --build` first.
set -euo pipefail

WRCS=http://localhost:8091
WRS=http://localhost:8081   # oidc-shell, used to mint a token
TENANT=retail-checkout
ENV=production

step() { echo; echo "=== $1 ==="; }
pause() { read -r -p "  (press enter to continue) " _ || true; }

step "1. Current stable manifest (v1, seeded on startup)"
curl -s "$WRCS/manifests/$TENANT/$ENV/stable" | jq '{version: .runtime.version, nav: [.shell.nav[].label]}'
pause

step "2. Publish v2 — add a 'Rewards' nav item (goes to the *candidate* slot)"
curl -s "$WRCS/manifests/$TENANT/$ENV/stable" \
  | jq '.shell.nav += [{
      "id": "rewards", "label": "Rewards", "icon": "gift", "path": "/rewards",
      "default": false, "permission": null,
      "experience": {
        "type": "module-federation",
        "remote": "http://localhost:8092/overview/App.js",
        "scope": "rewards", "module": "./App",
        "moduleVersion": "1.0.0", "deployedAt": "2026-09-20T02:00:00Z"
      }
    }]' \
  | curl -s -X POST "$WRCS/manifests/$TENANT/$ENV/publish" -d @- | jq .
pause

step "3. Candidate now has 3 nav items; stable is still v1 with 1"
echo "candidate:"; curl -s "$WRCS/manifests/$TENANT/$ENV/candidate" | jq '[.shell.nav[].label]'
echo "stable:   "; curl -s "$WRCS/manifests/$TENANT/$ENV/stable" | jq '[.shell.nav[].label]'
pause

step "4. Promote candidate -> stable (blue-green swap, one API call)"
curl -s -X POST "$WRCS/manifests/$TENANT/$ENV/promote" | jq .
echo "stable now:"; curl -s "$WRCS/manifests/$TENANT/$ENV/stable" | jq '[.shell.nav[].label]'
pause

step "5. Version history"
curl -s "$WRCS/manifests/$TENANT/$ENV/versions" | jq .
pause

step "6. Diff v1 -> v2"
curl -s "$WRCS/manifests/$TENANT/$ENV/diff?from=1&to=2" | jq .
pause

step "7. Rollback stable to v1 (no redeploy, instant)"
curl -s -X POST "$WRCS/manifests/$TENANT/$ENV/rollback" -d '{"version": 1}' | jq .
echo "stable now:"; curl -s "$WRCS/manifests/$TENANT/$ENV/stable" | jq '[.shell.nav[].label]'
pause

step "8. Re-promote v2 so the browser demo has the full nav"
curl -s -X POST "$WRCS/manifests/$TENANT/$ENV/rollback" -d '{"version": 2, "slot": "stable"}' | jq .

step "9. Access control: log in with the wrong role and hit the Web Runtime Service"
TOKEN=$(curl -s -D - -o /dev/null "$WRS/login?tenant=$TENANT&role=wrong-role" \
  | grep -i '^location:' | sed -E 's/.*token=([^&]+)&.*/\1/' | tr -d '\r')
echo "Expect HTTP 403:"
curl -s -o /dev/null -w "  http_status=%{http_code}\n" "http://localhost:8080/?token=$TOKEN&slot=stable"

echo
echo "Done. Open http://localhost:8081 in a browser to log in and see the rendered runtime."
