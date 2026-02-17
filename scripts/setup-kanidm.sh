#!/bin/bash
# =============================================================================
# ArchGuard - Kanidm Setup Script
# Initializes Kanidm with groups, users, OAuth2 client, and service account
#
# Uses: kanidm CLI via docker (kanidm/tools:latest) + REST API for auth
#
# Prerequisites:
#   - Docker running with kanidmd container
#   - expect installed (brew install expect)
#
# Usage:
#   ADMIN_PASSWORD=xxx IDM_ADMIN_PASSWORD=yyy ./scripts/setup-kanidm.sh
#   (passwords will be recovered automatically if not provided)
# =============================================================================

set -euo pipefail

KANIDM_URL="https://localhost:8443"
CURL="curl -sk"
TOKENS_FILE="/tmp/kanidm_tokens_setup"
KANIDM_CLI="docker run --rm --network host -v $TOKENS_FILE:/root/.cache/kanidm_tokens:rw -e KANIDM_URL=$KANIDM_URL kanidm/tools:latest /sbin/kanidm"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo -e "${GREEN}[OK]${NC} $1"; }
warn() { echo -e "${YELLOW}[!!]${NC} $1"; }
err() { echo -e "${RED}[ERR]${NC} $1"; exit 1; }

# =============================================================================
# Auth Helpers
# =============================================================================

# REST API auth: 3-step flow → returns bearer token
kanidm_rest_auth() {
  local username="$1"
  local password="$2"

  local init_response
  init_response=$($CURL -X POST "$KANIDM_URL/v1/auth" \
    -H "Content-Type: application/json" \
    -d "{\"step\":{\"init\":\"$username\"}}" \
    -D /tmp/kanidm_init_headers.txt 2>/dev/null)

  local cookie
  cookie=$(grep -i 'set-cookie' /tmp/kanidm_init_headers.txt | sed 's/.*auth-session-id=\([^;]*\).*/\1/')
  [ -z "$cookie" ] && err "Failed to get auth session cookie for $username"

  $CURL -X POST "$KANIDM_URL/v1/auth" \
    -H "Content-Type: application/json" \
    -H "Cookie: auth-session-id=$cookie" \
    -d '{"step":{"begin":"password"}}' >/dev/null 2>&1

  local auth_response
  auth_response=$($CURL -X POST "$KANIDM_URL/v1/auth" \
    -H "Content-Type: application/json" \
    -H "Cookie: auth-session-id=$cookie" \
    -d "{\"step\":{\"cred\":{\"password\":\"$password\"}}}" 2>/dev/null)

  local token
  token=$(echo "$auth_response" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('state',{}).get('success',''))" 2>/dev/null)
  [ -z "$token" ] && err "Auth failed for $username: $auth_response"
  echo "$token"
}

# CLI login via expect (needed for commands requiring CLI session)
kanidm_cli_login() {
  local username="$1"
  local password="$2"

  cat > /tmp/kanidm_login.exp << EXPECT_SCRIPT
#!/usr/bin/expect -f
set timeout 30
spawn docker run --rm -it --network host \
  -v $TOKENS_FILE:/root/.cache/kanidm_tokens \
  -e KANIDM_URL=$KANIDM_URL \
  kanidm/tools:latest \
  /sbin/kanidm login --accept-invalid-certs --name $username
expect "Enter password:"
send "$password\r"
expect eof
EXPECT_SCRIPT

  expect /tmp/kanidm_login.exp >/dev/null 2>&1
}

# REST API helper
api() {
  local token="$1" method="$2" path="$3" body="${4:-}"
  local args=(-X "$method" "$KANIDM_URL$path"
    -H "Authorization: Bearer $token"
    -H "Content-Type: application/json")
  [ -n "$body" ] && args+=(-d "$body")
  $CURL "${args[@]}" 2>/dev/null
}

# =============================================================================
# Main
# =============================================================================
echo "=================================================="
echo "  ArchGuard - Kanidm Initialization"
echo "=================================================="
echo ""

STATUS=$($CURL "$KANIDM_URL/status" 2>/dev/null || echo "false")
[ "$STATUS" != "true" ] && err "Kanidm is not running at $KANIDM_URL"
log "Kanidm is running"

# --- Step 1: Recover admin passwords ---
echo ""
echo "--- Step 1: Admin Password Recovery ---"

if [ -z "${ADMIN_PASSWORD:-}" ]; then
  ADMIN_PASSWORD=$(docker exec kanidmd /sbin/kanidmd recover-account admin -c /data/server.toml 2>&1 | tail -1 | awk '{print $NF}')
  log "admin password recovered"
fi

if [ -z "${IDM_ADMIN_PASSWORD:-}" ]; then
  IDM_ADMIN_PASSWORD=$(docker exec kanidmd /sbin/kanidmd recover-account idm_admin -c /data/server.toml 2>&1 | tail -1 | awk '{print $NF}')
  log "idm_admin password recovered"
fi

echo "  admin:     $ADMIN_PASSWORD"
echo "  idm_admin: $IDM_ADMIN_PASSWORD"

# --- Step 2: Authenticate (CLI + REST) ---
echo ""
echo "--- Step 2: Authentication ---"

touch "$TOKENS_FILE"
kanidm_cli_login "idm_admin" "$IDM_ADMIN_PASSWORD"
log "CLI login: idm_admin"

kanidm_cli_login "admin" "$ADMIN_PASSWORD"
log "CLI login: admin"

IDM_TOKEN=$(kanidm_rest_auth "idm_admin" "$IDM_ADMIN_PASSWORD")
log "REST auth: idm_admin"

FLAGS="--accept-invalid-certs --name idm_admin"

# --- Step 3: Create Groups ---
echo ""
echo "--- Step 3: Creating Groups ---"

for group in archguard_users archguard_super_admins archguard_tenant_admins archguard_service_desk archguard_viewers; do
  $KANIDM_CLI group create $FLAGS "$group" idm_admins 2>&1 | grep -v "WARN" | grep -v "^$" || true
done

# --- Step 4: Create Test Persons ---
echo ""
echo "--- Step 4: Creating Test Persons ---"

$KANIDM_CLI person create $FLAGS testadmin "Test Admin" 2>&1 | grep -v "WARN" || true
$KANIDM_CLI person create $FLAGS testuser "Test User" 2>&1 | grep -v "WARN" || true

# Set passwords via REST API
api "$IDM_TOKEN" "PUT" "/v1/person/testadmin/_credential/primary/set_password" '{"password":"TestAdmin123!"}' >/dev/null 2>&1
log "Set password for testadmin: TestAdmin123!"

api "$IDM_TOKEN" "PUT" "/v1/person/testuser/_credential/primary/set_password" '{"password":"TestUser123!"}' >/dev/null 2>&1
log "Set password for testuser: TestUser123!"

# --- Step 5: Add persons to groups ---
echo ""
echo "--- Step 5: Adding persons to groups ---"

$KANIDM_CLI group add-members $FLAGS archguard_users testadmin testuser 2>&1 | grep -v "WARN" || true
$KANIDM_CLI group add-members $FLAGS archguard_super_admins testadmin 2>&1 | grep -v "WARN" || true
$KANIDM_CLI group add-members $FLAGS archguard_viewers testuser 2>&1 | grep -v "WARN" || true
log "Users added to groups"

# --- Step 6: Create OAuth2 Public Client ---
echo ""
echo "--- Step 6: Creating OAuth2 Client ---"

$KANIDM_CLI system oauth2 create-public $FLAGS archguard-console "ArchGuard Console" "http://localhost:3000" 2>&1 | grep -v "WARN" || true
$KANIDM_CLI system oauth2 set-landing-url $FLAGS archguard-console "http://localhost:3000/callback" 2>&1 | grep -v "WARN" || true
$KANIDM_CLI system oauth2 add-redirect-url $FLAGS archguard-console "http://localhost:3000/callback" 2>&1 | grep -v "WARN" || true
$KANIDM_CLI system oauth2 update-scope-map $FLAGS archguard-console archguard_users openid profile email 2>&1 | grep -v "WARN" || true
log "OAuth2 client configured"

# --- Step 7: Create Service Account + API Token ---
echo ""
echo "--- Step 7: Creating Service Account ---"

# SA must have entry_managed_by=idm_admins for token generation
api "$IDM_TOKEN" "POST" "/v1/service_account" \
  '{"attrs":{"name":["archguard-sa"],"displayname":["ArchGuard Service Account"],"entry_managed_by":["idm_admins"]}}' >/dev/null 2>&1
log "Created service account: archguard-sa"

# Add to idm_admins for API read/write access
api "$IDM_TOKEN" "POST" "/v1/group/idm_admins/_attr/member" '["archguard-sa@localhost"]' >/dev/null 2>&1
api "$IDM_TOKEN" "POST" "/v1/group/idm_people_admins/_attr/member" '["archguard-sa@localhost"]' >/dev/null 2>&1
log "Added archguard-sa to idm groups"

# Generate API token (expiry as Unix timestamp: 2027-01-01)
SA_TOKEN=$(api "$IDM_TOKEN" "POST" "/v1/service_account/archguard-sa/_api_token" \
  '{"label":"console-token","expiry":1798761600,"read_write":true}' | tr -d '"')

if [ -z "$SA_TOKEN" ] || echo "$SA_TOKEN" | grep -q "error\|denied\|fail"; then
  err "Failed to generate SA token: $SA_TOKEN"
fi
log "API token generated"

# --- Step 8: Update .env ---
echo ""
echo "--- Step 8: Updating .env ---"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ENV_FILE="$SCRIPT_DIR/../archguard-console/.env"

cat > "$ENV_FILE" << EOF
# ArchGuard Console - Local Development
# Generated by setup-kanidm.sh on $(date)

ARCHGUARD_ID_URL=https://localhost:8443
ARCHGUARD_SA_TOKEN=$SA_TOKEN
ARCHGUARD_VAULT_URL=http://localhost:8080
VITE_ARCHGUARD_ID_URL=https://localhost:8443
SESSION_SECRET=a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2
EOF

log "Updated $ENV_FILE"

# =============================================================================
echo ""
echo "=================================================="
echo "  Setup Complete!"
echo "=================================================="
echo ""
echo "  Kanidm URL:     $KANIDM_URL"
echo "  OAuth2 Client:  archguard-console (public, PKCE)"
echo "  Redirect URL:   http://localhost:3000/callback"
echo ""
echo "  Test Users:"
echo "    testadmin / TestAdmin123!  (Super Admin)"
echo "    testuser  / TestUser123!   (Viewer)"
echo ""
echo "  SA Token: ${SA_TOKEN:0:40}..."
echo ""
echo "  Next: cd archguard-console && npm run dev"
echo ""
