#!/usr/bin/env bash
# Build once, promote the same manifest dev → sit → perf → uat → prod, with
# blue-green, canary and instant rollback inside each environment.
#
#   config change   -> WCR builds v(N+1) ONCE (environment-agnostic) -> live in dev
#   promote         -> next environment's candidate (gated: one hop at a time,
#                      only versions that have been stable in the previous env)
#   canary/rollback -> that environment's pointer only
#
# Also: creating a tenant from a template (lands in dev only), and modules
# as shared, versioned building blocks that each tenant pins.
#
# Requires: curl, jq. Run `docker compose up --build` first.
set -euo pipefail

CP=http://localhost:8093         # control plane (config in its own DB)
WCR=http://localhost:8091        # Web Composition Registry
WRS=http://localhost:8080        # Web Runtime Service (reads S3, picks version per user)
OIDC=http://localhost:8081
S3=http://localhost:9000/web-manifests
TENANT=startup-banking
ROLE=finance-user
ENVS=(dev sit perf uat prod)

step() { echo; echo "=== $1 ==="; }
pause() { read -r -p "  (press enter to continue) " _ || true; }
pipeline() {
  local out=""
  for e in "${ENVS[@]}"; do
    out+=$(curl -s "$S3/pointers/$TENANT/$e.json" | jq -r --arg e "$e" \
      '"\($e)=v\(.stable)" + (if .candidate != .stable then "(cand v\(.candidate))" else "" end)
       + (if .canary.percent > 0 then "(canary \(.canary.percent)%)" else "" end) + "  "')
  done
  echo "  $out"
}
promote_to() {   # version from to
  curl -s -X POST "$WCR/manifests/$TENANT/promote" -d "{\"version\":$1,\"from\":\"$2\",\"to\":\"$3\"}" | jq -r '.error // "  ok: v\(.version) is candidate in \(.env)"'
}
make_stable() { curl -s -X POST "$WCR/manifests/$TENANT/$1/promote" >/dev/null; }
USERS=$(seq -f "user-%g" 1 100 | paste -sd, -)
who() {
  curl -s "$WRS/debug/assign?tenant=$TENANT&env=$1&users=$USERS" | jq -r --arg e "$1" '
    .assignments | group_by(.selectedBy)[] | "  \($e): \(length) of 100 users on v\(.[0].version) (\(.[0].selectedBy))"'
}

step "0a. Create a tenant from a template — v1 goes live in dev ONLY"
NEWT="acme-$(date +%s | tail -c 5)"
curl -s -X POST "$CP/api/tenants" -d "{\"tenantId\":\"$NEWT\",\"displayName\":\"Acme\",\"template\":\"banking-standard\"}" | jq -c .
for _ in $(seq 1 20); do curl -s "$S3/pointers/$NEWT/dev.json" | jq -e .stable >/dev/null 2>&1 && break; sleep 0.5; done
for e in "${ENVS[@]}"; do echo -n "  $e=$(curl -s "$S3/pointers/$NEWT/$e.json" | jq -r '"v\(.stable)"' 2>/dev/null || echo 'not deployed') "; done; echo
TOKEN=$(curl -s -D - -o /dev/null "$OIDC/login?tenant=$NEWT&role=$NEWT-user&user=alice&env=sit" | grep -i '^location:' | sed -E 's/.*token=([^&]+)&.*/\1/' | tr -d '\r')
echo "  $NEWT in sit → HTTP $(curl -s -o /dev/null -w '%{http_code}' "$WRS/?token=$TOKEN&env=sit") (not deployed yet; reaches sit only by promotion)"
echo "  pinned module versions: $(curl -s "$CP/api/configs/$NEWT" | jq -c '[.config.pages[] | "\(.module)@\(.moduleVersion)"]')"
pause

step "0b. Modules are shared and versioned — $TENANT pins home@$(curl -s "$CP/api/configs/$TENANT" | jq -r '.config.pages[] | select(.module=="home") | .moduleVersion'); catalog latest is $(curl -s "$CP/api/modules" | jq -r .modules.home.latest)"
curl -s "$CP/api/modules" | jq -c '.modules.home | {latest, versions: (.versions | keys)}'
pause

step "0c. The experience frame is config too: toggle the nav between left and top"
FRAME_REV=$(curl -s "$CP/api/configs/$TENANT" \
  | jq '.config | .frame.nav.settings.position = (if .frame.nav.settings.position == "top" then "left" else "top" end)' \
  | curl -s -X PUT "$CP/api/configs/$TENANT" -H 'Content-Type: application/json' -d @- | jq .revision)
for _ in $(seq 1 30); do
  curl -s "$WCR/manifests/$TENANT/versions" | jq -e --argjson r "$FRAME_REV" '.versions[] | select(.sourceRevision == $r)' >/dev/null && break
  sleep 0.5
done
frame_of() {  # env → nav position + theme the user's page was rendered with
  local token
  token=$(curl -s -D - -o /dev/null "$OIDC/login?tenant=$TENANT&role=$ROLE&user=alice&env=$1" | grep -i '^location:' | sed -E 's/.*token=([^&]+)&.*/\1/' | tr -d '\r')
  curl -s "$WRS/?token=$token&env=$1" | grep -oE '<body[^>]*>'
}
echo "  dev : $(frame_of dev)"
echo "  prod: $(frame_of prod)   (unchanged until the new version is promoted)"
pause

step "1. One pointer per environment"
pipeline
pause

step "2. Change CONFIG once (toggle an 'Insights' page) → WCR builds it once → live in dev only"
REV=$(curl -s "$CP/api/configs/$TENANT" \
  | jq '.config
        | if any(.pages[]; .id == "insights")
          then .pages |= map(select(.id != "insights"))
          else .pages += [{"id": "insights", "label": "Insights", "icon": "chart", "default": false, "module": "transactions"}]
          end' \
  | curl -s -X PUT "$CP/api/configs/$TENANT" -H 'Content-Type: application/json' -d @- | jq .revision)
NEW=""
for _ in $(seq 1 30); do
  NEW=$(curl -s "$WCR/manifests/$TENANT/versions" \
    | jq --argjson r "$REV" '[.versions[] | select(.sourceRevision == $r and (.stableIn | index("dev"))) | .version][0] // empty')
  [ -n "$NEW" ] && break
  sleep 0.5
done
[ -n "$NEW" ] || { echo "WCR did not build revision $REV"; exit 1; }
echo "  config revision $REV → manifests/$TENANT/v$NEW.json"
echo "  modules carry paths, not hosts: $(curl -s "$S3/manifests/$TENANT/v$NEW.json" | jq -c '[.shell.nav[].experience | {path, remote}][0]')"
pipeline
pause

step "3. Gates: skipping an environment is refused"
echo "  dev → uat: $(promote_to "$NEW" dev uat)"
pause

step "4. Walk v$NEW through every environment (candidate → stable in each)"
prev=dev
for e in sit perf uat prod; do
  echo "  $prev → $e:"; promote_to "$NEW" "$prev" "$e"
  if [ "$e" != prod ]; then make_stable "$e"; fi
  prev=$e
done
pipeline
pause

step "5. In prod: canary 10%, then promote"
curl -s -X POST "$WCR/manifests/$TENANT/prod/canary" -d '{"percent":10}' >/dev/null; sleep 1
who prod; who uat
make_stable prod; sleep 1
echo "  promoted:"; who prod
echo "  stable in: $(curl -s "$WCR/manifests/$TENANT/versions" | jq -c --argjson v "$NEW" '.versions[] | select(.version == $v) | .stableIn')"
echo "  one file for all of them — ETag $(curl -sI "$S3/manifests/$TENANT/v$NEW.json" | grep -i '^etag' | tr -d '\r' | cut -d' ' -f2)"
pause

step "6. Instant rollback in prod only"
OLD=$((NEW - 1))
curl -s -X POST "$WCR/manifests/$TENANT/prod/rollback" -d "{\"version\": $OLD, \"slot\": \"stable\"}" >/dev/null; sleep 1
who prod
pipeline
pause

step "7. Access control still enforced: wrong role -> 403"
TOKEN=$(curl -s -D - -o /dev/null "$OIDC/login?tenant=$TENANT&role=wrong-role&user=mallory&env=prod" \
  | grep -i '^location:' | sed -E 's/.*token=([^&]+)&.*/\1/' | tr -d '\r')
curl -s -o /dev/null -w "  http_status=%{http_code}\n" "$WRS/?token=$TOKEN&env=prod"

echo
echo "Done. Control plane UI: $CP   S3 (SeaweedFS): $S3"
