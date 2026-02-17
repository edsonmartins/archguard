#!/bin/bash
# =============================================================================
# ArchGuard - Kanidm Setup Script (Idempotent)
#
# Initializes Kanidm with everything the Console needs:
#   - Account policy (allows password-only auth for dev)
#   - Groups (archguard_users, super_admins, tenant_admins, service_desk, viewers)
#   - Test persons with passwords
#   - OAuth2 public client (PKCE)
#   - Service account with API token
#   - .env file with generated token
#
# Prerequisites:
#   - Docker running with kanidmd container
#   - expect (brew install expect on macOS)
#
# Usage:
#   ./scripts/setup-kanidm.sh
#   ADMIN_PASSWORD=xxx IDM_ADMIN_PASSWORD=yyy ./scripts/setup-kanidm.sh
#
# The script is idempotent — safe to run multiple times.
# =============================================================================

set -euo pipefail

KANIDM_URL="${KANIDM_URL:-https://localhost:8443}"
KANIDM_CONTAINER="${KANIDM_CONTAINER:-kanidmd}"
CONSOLE_ORIGIN="${CONSOLE_ORIGIN:-http://localhost:3000}"
VAULT_URL="${VAULT_URL:-http://localhost:8080}"
CURL="curl -sk"
TOKENS_FILE="/tmp/kanidm_tokens_setup_$$"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ENV_FILE="$SCRIPT_DIR/../archguard-console/.env"

# Test user passwords (Kanidm requires strong passwords: 3+ words)
TESTADMIN_PASSWORD="ArchGuard2026TestAdmin"
TESTUSER_PASSWORD="ArchGuard2026TestUser"

# Colors
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
log()  { echo -e "${GREEN}  [OK]${NC} $1"; }
warn() { echo -e "${YELLOW}  [!!]${NC} $1"; }
err()  { echo -e "${RED}  [ERR]${NC} $1"; exit 1; }
step() { echo -e "\n${CYAN}--- $1 ---${NC}"; }

cleanup() { rm -f "$TOKENS_FILE" /tmp/kanidm_*.exp /tmp/kanidm_init_headers.txt; }
trap cleanup EXIT

# =============================================================================
# Helpers
# =============================================================================

# REST API 3-step auth → bearer token
kanidm_rest_auth() {
  local username="$1" password="$2"

  local init=$($CURL -X POST "$KANIDM_URL/v1/auth" \
    -H "Content-Type: application/json" \
    -d "{\"step\":{\"init\":\"$username\"}}" \
    -D /tmp/kanidm_init_headers.txt 2>/dev/null)

  local cookie=$(grep -i 'set-cookie' /tmp/kanidm_init_headers.txt 2>/dev/null | sed 's/.*auth-session-id=\([^;]*\).*/\1/')
  [ -z "$cookie" ] && err "Auth init failed for $username"

  $CURL -X POST "$KANIDM_URL/v1/auth" \
    -H "Content-Type: application/json" \
    -H "Cookie: auth-session-id=$cookie" \
    -d '{"step":{"begin":"password"}}' >/dev/null 2>&1

  local response=$($CURL -X POST "$KANIDM_URL/v1/auth" \
    -H "Content-Type: application/json" \
    -H "Cookie: auth-session-id=$cookie" \
    -d "{\"step\":{\"cred\":{\"password\":\"$password\"}}}" 2>/dev/null)

  local token=$(echo "$response" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('state',{}).get('success',''))" 2>/dev/null)
  [ -z "$token" ] && err "Auth failed for $username"
  echo "$token"
}

# REST API helper
api() {
  local token="$1" method="$2" path="$3" body="${4:-}"
  local args=(-X "$method" "$KANIDM_URL$path" -H "Authorization: Bearer $token" -H "Content-Type: application/json")
  [ -n "$body" ] && args+=(-d "$body")
  $CURL "${args[@]}" 2>/dev/null
}

# CLI login via expect (kanidm CLI requires TTY for password)
kanidm_cli_login() {
  local username="$1" password="$2"
  cat > /tmp/kanidm_login_$$.exp << EOF
#!/usr/bin/expect -f
set timeout 30
spawn docker run --rm -it --network host -v $TOKENS_FILE:/root/.cache/kanidm_tokens -e KANIDM_URL=$KANIDM_URL kanidm/tools:latest /sbin/kanidm login --accept-invalid-certs --name $username
expect "Enter password:"
send "$password\r"
expect eof
EOF
  expect /tmp/kanidm_login_$$.exp >/dev/null 2>&1
}

# Set person password via expect (interactive credential update)
kanidm_set_password() {
  local person="$1" password="$2"
  cat > /tmp/kanidm_setpw_$$.exp << EOF
#!/usr/bin/expect -f
set timeout 30
spawn docker run --rm -it --network host -v $TOKENS_FILE:/root/.cache/kanidm_tokens -e KANIDM_URL=$KANIDM_URL kanidm/tools:latest /sbin/kanidm person credential update --accept-invalid-certs --name idm_admin $person
expect "cred update"
send "password\r"
expect "New password:"
send "$password\r"
expect "Confirm password:"
send "$password\r"
expect "cred update"
send "commit\r"
expect "y/n"
send "y\r"
expect eof
EOF
  local output=$(expect /tmp/kanidm_setpw_$$.exp 2>&1)
  if echo "$output" | grep -q "Success"; then
    log "$person password set"
  else
    warn "$person password may not have been set (check manually)"
  fi
}

# kanidm CLI wrapper (non-interactive commands)
kcli() {
  docker run --rm --network host \
    -v "$TOKENS_FILE:/root/.cache/kanidm_tokens:rw" \
    -e "KANIDM_URL=$KANIDM_URL" \
    kanidm/tools:latest \
    /sbin/kanidm "$@" --accept-invalid-certs --name idm_admin 2>&1 | grep -v "WARN" || true
}

# =============================================================================
# Main
# =============================================================================
echo ""
echo "  =================================================="
echo "   ArchGuard - Kanidm Setup"
echo "   Kanidm:  $KANIDM_URL"
echo "   Console: $CONSOLE_ORIGIN"
echo "  =================================================="

# --- Pre-flight checks ---
step "Pre-flight Checks"

command -v expect >/dev/null 2>&1 || err "expect not found. Install: brew install expect"
command -v docker >/dev/null 2>&1 || err "docker not found"
docker ps --format '{{.Names}}' | grep -q "^${KANIDM_CONTAINER}$" || err "Container '$KANIDM_CONTAINER' not running. Run: docker compose up -d"

STATUS=$($CURL "$KANIDM_URL/status" 2>/dev/null || echo "false")
[ "$STATUS" != "true" ] && err "Kanidm not responding at $KANIDM_URL"
log "Kanidm is healthy"

# --- Step 1: Admin passwords ---
step "Step 1: Admin Password Recovery"

if [ -z "${ADMIN_PASSWORD:-}" ]; then
  ADMIN_PASSWORD=$(docker exec "$KANIDM_CONTAINER" /sbin/kanidmd recover-account admin -c /data/server.toml 2>&1 | tail -1 | awk '{print $NF}')
  log "admin password recovered"
fi
if [ -z "${IDM_ADMIN_PASSWORD:-}" ]; then
  IDM_ADMIN_PASSWORD=$(docker exec "$KANIDM_CONTAINER" /sbin/kanidmd recover-account idm_admin -c /data/server.toml 2>&1 | tail -1 | awk '{print $NF}')
  log "idm_admin password recovered"
fi

# --- Step 2: Authenticate ---
step "Step 2: Authentication"

touch "$TOKENS_FILE"
kanidm_cli_login "idm_admin" "$IDM_ADMIN_PASSWORD"
log "CLI session: idm_admin"

IDM_TOKEN=$(kanidm_rest_auth "idm_admin" "$IDM_ADMIN_PASSWORD")
log "REST token: idm_admin"

# --- Step 3: Account Policy (allow password-only for dev) ---
step "Step 3: Account Policy"

kcli group account-policy credential-type-minimum idm_all_persons any | grep -v "^$"
log "credential-type-minimum = any (password-only allowed)"

# --- Step 4: Groups ---
step "Step 4: Groups"

for group in archguard_users archguard_super_admins archguard_tenant_admins archguard_service_desk archguard_viewers; do
  RESULT=$(kcli group create "$group" idm_admins)
  if echo "$RESULT" | grep -qi "exists\|duplicate"; then
    warn "$group already exists"
  else
    log "Created $group"
  fi
done

# Enable account policy on archguard_users too
kcli group account-policy enable archguard_users >/dev/null 2>&1
kcli group account-policy credential-type-minimum archguard_users any >/dev/null 2>&1

# --- Step 5: Test Persons ---
step "Step 5: Test Persons"

for person in testadmin testuser; do
  RESULT=$(kcli person create "$person" "Test $(echo $person | sed 's/test//' | sed 's/^./\U&/')")
  if echo "$RESULT" | grep -qi "exists\|duplicate"; then
    warn "$person already exists"
  else
    log "Created $person"
  fi
done

# Add to groups
kcli group add-members archguard_users testadmin testuser >/dev/null 2>&1
kcli group add-members archguard_super_admins testadmin >/dev/null 2>&1
kcli group add-members archguard_viewers testuser >/dev/null 2>&1
log "testadmin -> archguard_users, archguard_super_admins"
log "testuser  -> archguard_users, archguard_viewers"

# Set passwords (requires expect for interactive credential update)
kanidm_set_password "testadmin" "$TESTADMIN_PASSWORD"
kanidm_set_password "testuser" "$TESTUSER_PASSWORD"

# --- Step 6: OAuth2 Client ---
step "Step 6: OAuth2 Client (archguard-console)"

# Delete if exists (to ensure clean state)
kcli system oauth2 delete archguard-console >/dev/null 2>&1

# Re-login (session may have expired during password setup)
kanidm_cli_login "idm_admin" "$IDM_ADMIN_PASSWORD"

# Create public client with base origin (NOT /callback)
kcli system oauth2 create-public archguard-console "ArchGuard Console" "$CONSOLE_ORIGIN" >/dev/null 2>&1
kcli system oauth2 set-landing-url archguard-console "$CONSOLE_ORIGIN" >/dev/null 2>&1
kcli system oauth2 add-redirect-url archguard-console "${CONSOLE_ORIGIN}/callback" >/dev/null 2>&1
kcli system oauth2 update-scope-map archguard-console archguard_users openid profile email groups >/dev/null 2>&1
log "OAuth2 public client configured (PKCE, scopes: openid profile email groups)"

# --- Step 7: Service Account + API Token ---
step "Step 7: Service Account"

# Re-auth REST (token may have expired)
IDM_TOKEN=$(kanidm_rest_auth "idm_admin" "$IDM_ADMIN_PASSWORD")

# Delete old SA if exists, then create with entry_managed_by
api "$IDM_TOKEN" "DELETE" "/v1/service_account/archguard-sa" >/dev/null 2>&1
api "$IDM_TOKEN" "POST" "/v1/service_account" \
  '{"attrs":{"name":["archguard-sa"],"displayname":["ArchGuard Service Account"],"entry_managed_by":["idm_admins"]}}' >/dev/null 2>&1
log "Created archguard-sa (managed by idm_admins)"

# Grant API access
api "$IDM_TOKEN" "POST" "/v1/group/idm_admins/_attr/member" '["archguard-sa@localhost"]' >/dev/null 2>&1
api "$IDM_TOKEN" "POST" "/v1/group/idm_people_admins/_attr/member" '["archguard-sa@localhost"]' >/dev/null 2>&1
log "Granted idm_admins + idm_people_admins membership"

# Generate API token (expiry: Unix timestamp for 2027-12-31)
SA_TOKEN=$(api "$IDM_TOKEN" "POST" "/v1/service_account/archguard-sa/_api_token" \
  '{"label":"console-token","expiry":1830297600,"read_write":true}' | tr -d '"')

if [ -z "$SA_TOKEN" ] || echo "$SA_TOKEN" | grep -qi "error\|denied\|fail\|deserialize"; then
  err "Failed to generate SA token: $SA_TOKEN"
fi
log "API token generated (expires 2027-12-31)"

# Verify token works
PERSON_COUNT=$(api "$SA_TOKEN" "GET" "/v1/person" | python3 -c "import sys,json; print(len(json.load(sys.stdin)))" 2>/dev/null || echo "0")
[ "$PERSON_COUNT" -gt 0 ] && log "Token verified: $PERSON_COUNT persons visible" || warn "Token verification: 0 persons (check permissions)"

# --- Step 8: Write .env ---
step "Step 8: Generate .env"

cat > "$ENV_FILE" << EOF
# ArchGuard Console - Local Development
# Generated by setup-kanidm.sh on $(date)
# Re-run this script to regenerate: ./scripts/setup-kanidm.sh

ARCHGUARD_ID_URL=$KANIDM_URL
ARCHGUARD_SA_TOKEN=$SA_TOKEN
ARCHGUARD_VAULT_URL=$VAULT_URL
VITE_ARCHGUARD_ID_URL=$KANIDM_URL
SESSION_SECRET=a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2
EOF

log "Written to $ENV_FILE"

# --- Step 9: Verify OIDC ---
step "Step 9: Verification"

OIDC_ISSUER=$($CURL "$KANIDM_URL/oauth2/openid/archguard-console/.well-known/openid-configuration" 2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin).get('issuer',''))" 2>/dev/null)
if [ -n "$OIDC_ISSUER" ]; then
  log "OIDC discovery OK: $OIDC_ISSUER"
else
  warn "OIDC discovery failed — check OAuth2 client config"
fi

# Verify testadmin can authenticate
TEST_AUTH=$(kanidm_rest_auth "testadmin" "$TESTADMIN_PASSWORD" 2>/dev/null || echo "")
if [ -n "$TEST_AUTH" ]; then
  log "testadmin authentication verified"
else
  warn "testadmin authentication failed — check password"
fi

# =============================================================================
echo ""
echo "  =================================================="
echo "   Setup Complete!"
echo "  =================================================="
echo ""
echo "  Services:"
echo "    Kanidm:     $KANIDM_URL"
echo "    AliasVault: $VAULT_URL"
echo "    Console:    $CONSOLE_ORIGIN"
echo ""
echo "  Test Users:"
echo "    testadmin / $TESTADMIN_PASSWORD  (Super Admin)"
echo "    testuser  / $TESTUSER_PASSWORD   (Viewer)"
echo ""
echo "  Next steps:"
echo "    cd archguard-console && npm install && npm run dev"
echo "    Open $CONSOLE_ORIGIN in browser"
echo "    Accept Kanidm TLS cert: $KANIDM_URL"
echo ""
