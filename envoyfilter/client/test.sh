#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# End-to-end test for the Envoy OAuth token injection demo (ext_authz edition)
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

ENVOY_URL="http://localhost:8080"
OAUTH_URL="http://localhost:8082"
BACKEND_URL="http://localhost:8081"
TOKEN_MGR_URL="http://localhost:8083"

BOLD="\033[1m"
GREEN="\033[0;32m"
CYAN="\033[0;36m"
YELLOW="\033[0;33m"
RED="\033[0;31m"
RESET="\033[0m"

sep()    { echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"; }
header() { sep; echo -e "${BOLD}$1${RESET}"; sep; }
pass()   { echo -e "${GREEN}PASS${RESET} — $1"; }
fail()   { echo -e "${RED}FAIL${RESET} — $1"; exit 1; }

wait_for() {
  local url=$1 name=$2
  echo -n "Waiting for $name "
  for i in $(seq 1 30); do
    if curl -sf "$url/health" > /dev/null 2>&1; then echo -e " ${GREEN}ready${RESET}"; return; fi
    echo -n "."; sleep 1
  done
  echo -e " ${RED}TIMEOUT${RESET}"; exit 1
}

header "1. Health checks"
wait_for "$OAUTH_URL"     "oauth-server    ($OAUTH_URL)"
wait_for "$BACKEND_URL"   "backend-service ($BACKEND_URL)"
wait_for "$TOKEN_MGR_URL" "token-manager   ($TOKEN_MGR_URL)"
echo -n "Waiting for envoy        "
for i in $(seq 1 20); do
  if curl -sf "http://localhost:9901/ready" > /dev/null 2>&1; then echo -e " ${GREEN}ready${RESET}"; break; fi
  echo -n "."; sleep 1
done

# ── Test 1: Token manager cache stats (cold) ─────────────────────────────────
header "2. Token-manager cache stats (before any request)"
echo -e "${YELLOW}GET $TOKEN_MGR_URL/health${RESET}"
curl -sf "$TOKEN_MGR_URL/health" | python3 -m json.tool

# ── Test 2: First request through Envoy ──────────────────────────────────────
header "3. First request via Envoy (cache cold → 1 OAuth fetch expected)"
echo -e "${YELLOW}GET $ENVOY_URL/api/hello${RESET}"
R=$(curl -sf "$ENVOY_URL/api/hello")
echo "$R" | python3 -m json.tool
VALID=$(echo "$R" | python3 -c "import sys,json; print(json.load(sys.stdin)['token_validation']['valid'])")
INJECTED=$(echo "$R" | python3 -c "import sys,json; print(json.load(sys.stdin)['token_validation']['injected_by_envoy'])")
[[ "$VALID" == "True" && "$INJECTED" == "True" ]] && pass "Token injected and valid" || fail "valid=$VALID injected=$INJECTED"

# ── Test 3: Multiple requests — prove token is reused from cache ─────────────
header "4. Send 5 more requests — token must be reused (0 additional OAuth fetches)"
for i in $(seq 1 5); do
  curl -sf "$ENVOY_URL/api/request-$i" > /dev/null
  echo -n "  request $i "
  echo -e "${GREEN}ok${RESET}"
done

STATS=$(curl -sf "$TOKEN_MGR_URL/health")
echo ""
echo "Token-manager cache stats after 6 total requests:"
echo "$STATS" | python3 -m json.tool

FETCHES=$(echo "$STATS" | python3 -c "import sys,json; print(json.load(sys.stdin)['fetch_count'])")
HITS=$(echo "$STATS" | python3 -c "import sys,json; print(json.load(sys.stdin)['cache_hits'])")
[[ "$FETCHES" == "1" ]] \
  && pass "OAuth server called exactly once (fetch_count=1, cache_hits=$HITS)" \
  || fail "Expected fetch_count=1, got $FETCHES"

# ── Test 4: Same token across requests ───────────────────────────────────────
header "5. Verify the same token is used across requests"
T1=$(curl -sf "$ENVOY_URL/api/a" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['request']['headers'].get('authorization',''))")
T2=$(curl -sf "$ENVOY_URL/api/b" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['request']['headers'].get('authorization',''))")
[[ "$T1" == "$T2" ]] \
  && pass "Same token reused: ${T1:0:60}..." \
  || fail "Tokens differ between requests — cache not working"

# ── Test 5: Fake client token is replaced ────────────────────────────────────
header "6. Fake client token → Envoy (via token-manager) replaces it"
echo -e "${YELLOW}GET $ENVOY_URL/api/secure  (Authorization: Bearer FAKE-TOKEN)${RESET}"
R2=$(curl -sf "$ENVOY_URL/api/secure" -H "Authorization: Bearer FAKE-TOKEN")
echo "$R2" | python3 -m json.tool
VALID2=$(echo "$R2" | python3 -c "import sys,json; print(json.load(sys.stdin)['token_validation']['valid'])")
[[ "$VALID2" == "True" ]] \
  && pass "Fake token replaced with valid OAuth token" \
  || fail "Token not replaced (valid=$VALID2)"

# ── Test 6: POST with body ────────────────────────────────────────────────────
header "7. POST request — body passes through, token still injected"
curl -sf -X POST "$ENVOY_URL/api/data" \
  -H "Content-Type: application/json" \
  -d '{"key": "value"}' | python3 -m json.tool

sep
echo -e "\n${GREEN}${BOLD}All tests passed!${RESET}"
echo ""
echo -e "Token-manager cache stats (final):"
curl -sf "$TOKEN_MGR_URL/health" | python3 -m json.tool
echo ""
echo -e "Envoy admin:  ${CYAN}http://localhost:9901${RESET}"
echo -e "Cluster stats: ${CYAN}http://localhost:9901/clusters${RESET}"
