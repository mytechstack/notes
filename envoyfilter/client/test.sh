#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# End-to-end test for the Envoy OAuth token injection demo
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

ENVOY_URL="http://localhost:8080"
OAUTH_URL="http://localhost:8082"
BACKEND_URL="http://localhost:8081"

BOLD="\033[1m"
GREEN="\033[0;32m"
CYAN="\033[0;36m"
YELLOW="\033[0;33m"
RED="\033[0;31m"
RESET="\033[0m"

sep() { echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"; }
header() { sep; echo -e "${BOLD}$1${RESET}"; sep; }

wait_for_service() {
  local url=$1 name=$2
  echo -n "Waiting for $name "
  for i in $(seq 1 20); do
    if curl -sf "$url/health" > /dev/null 2>&1; then
      echo -e " ${GREEN}ready${RESET}"
      return
    fi
    echo -n "."
    sleep 1
  done
  echo -e " ${RED}TIMEOUT${RESET}"
  exit 1
}

header "1. Health checks"
wait_for_service "$OAUTH_URL"   "oauth-server  ($OAUTH_URL)"
wait_for_service "$BACKEND_URL" "backend-service ($BACKEND_URL)"
# Envoy doesn't have /health but admin endpoint works
echo -n "Waiting for envoy        "
for i in $(seq 1 20); do
  if curl -sf "http://localhost:9901/ready" > /dev/null 2>&1; then
    echo -e " ${GREEN}ready${RESET}"; break
  fi
  echo -n "."; sleep 1
done

# ── Test 1: Direct OAuth token fetch ─────────────────────────────────────────
header "2. Direct OAuth token fetch (client_credentials)"
echo -e "${YELLOW}POST $OAUTH_URL/token${RESET}"
TOKEN_RESP=$(curl -sf -X POST "$OAUTH_URL/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials&client_id=envoy-proxy&client_secret=envoy-secret")
echo "$TOKEN_RESP" | python3 -m json.tool
ACCESS_TOKEN=$(echo "$TOKEN_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")
echo -e "\n${GREEN}Got token:${RESET} ${ACCESS_TOKEN:0:60}..."

# ── Test 2: Verify the token directly ────────────────────────────────────────
header "3. Direct token verification on OAuth server"
echo -e "${YELLOW}GET $OAUTH_URL/verify${RESET}"
curl -sf "$OAUTH_URL/verify" \
  -H "Authorization: Bearer $ACCESS_TOKEN" | python3 -m json.tool

# ── Test 3: Request through Envoy (token injected automatically) ──────────────
header "4. Request via Envoy → token injected → backend (no Authorization header sent)"
echo -e "${YELLOW}GET $ENVOY_URL/api/hello  (no Authorization header from client)${RESET}"
RESP=$(curl -sf "$ENVOY_URL/api/hello")
echo "$RESP" | python3 -m json.tool

# Verify injection happened
INJECTED=$(echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['token_validation']['injected_by_envoy'])")
VALID=$(echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['token_validation']['valid'])")
echo ""
if [[ "$VALID" == "True" && "$INJECTED" == "True" ]]; then
  echo -e "${GREEN}PASS${RESET} — Token was injected by Envoy and validated by the backend"
else
  echo -e "${RED}FAIL${RESET} — valid=$VALID  injected=$INJECTED"
  exit 1
fi

# ── Test 4: Client sends its own Authorization header — Envoy REPLACES it ────
header "5. Client sends a FAKE token → Envoy replaces it with a real one"
echo -e "${YELLOW}GET $ENVOY_URL/api/secure  (Authorization: Bearer FAKE-TOKEN)${RESET}"
RESP2=$(curl -sf "$ENVOY_URL/api/secure" \
  -H "Authorization: Bearer FAKE-TOKEN-SHOULD-BE-REPLACED")
echo "$RESP2" | python3 -m json.tool

VALID2=$(echo "$RESP2" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['token_validation']['valid'])")
echo ""
if [[ "$VALID2" == "True" ]]; then
  echo -e "${GREEN}PASS${RESET} — Envoy replaced the fake client token with a valid OAuth token"
else
  echo -e "${RED}FAIL${RESET} — Token was not replaced (valid=$VALID2)"
  exit 1
fi

# ── Test 5: POST request ──────────────────────────────────────────────────────
header "6. POST request through Envoy"
echo -e "${YELLOW}POST $ENVOY_URL/api/data${RESET}"
curl -sf -X POST "$ENVOY_URL/api/data" \
  -H "Content-Type: application/json" \
  -d '{"key": "value"}' | python3 -m json.tool

sep
echo -e "\n${GREEN}${BOLD}All tests passed!${RESET}"
echo -e "Envoy admin dashboard: ${CYAN}http://localhost:9901${RESET}"
echo -e "View cluster stats:    ${CYAN}http://localhost:9901/clusters${RESET}"
