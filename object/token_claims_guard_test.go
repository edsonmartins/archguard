// Copyright 2026 IntegrAllTech Ltda.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package object

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// Sentinel values: if any of them appears in a serialised token, a secret or personal datum leaked.
const (
	sentinelPassword     = "SENTINEL-password-hash"
	sentinelSalt         = "SENTINEL-password-salt"
	sentinelTotp         = "SENTINEL-TOTP-SECRET-JBSWY3DPEHPK3PXP"
	sentinelRecovery     = "SENTINEL-recovery-code-1"
	sentinelManagedPwd   = "SENTINEL-managed-account-password"
	sentinelAccessToken  = "SENTINEL-access-token"
	sentinelOrigToken    = "SENTINEL-original-token"
	sentinelOrigRefresh  = "SENTINEL-original-refresh-token"
	sentinelInvitation   = "SENTINEL-invitation-code"
	sentinelIdCard       = "SENTINEL-123.456.789-00"
	sentinelCreatedIp    = "SENTINEL-203.0.113.7"
	sentinelLastIp       = "SENTINEL-198.51.100.9"
	sentinelWrongTime    = "SENTINEL-2026-11-21T10:00:00Z"
	sentinelOAuthProp    = "SENTINEL-github-oauth-access-token"
	sentinelMfaSecretKey = "SENTINEL-mfa-account-secret-key"
)

var sentinels = []string{
	sentinelPassword, sentinelSalt, sentinelTotp, sentinelRecovery, sentinelManagedPwd,
	sentinelAccessToken, sentinelOrigToken, sentinelOrigRefresh, sentinelInvitation,
	sentinelIdCard, sentinelCreatedIp, sentinelLastIp, sentinelWrongTime, sentinelOAuthProp,
	sentinelMfaSecretKey,
}

func userWithSecrets() *User {
	return &User{
		Owner:                "org-test",
		Name:                 "ana",
		Id:                   "11111111-2222-3333-4444-555555555555",
		DisplayName:          "Ana Souza",
		Email:                "ana@example.com",
		EmailVerified:        true,
		Password:             sentinelPassword,
		PasswordSalt:         sentinelSalt,
		TotpSecret:           sentinelTotp,
		RecoveryCodes:        []string{sentinelRecovery},
		ManagedAccounts:      []ManagedAccount{{Application: "app", Username: "ana", Password: sentinelManagedPwd}},
		MfaAccounts:          []MfaAccount{{AccountName: "ana", Issuer: "x", SecretKey: sentinelMfaSecretKey}},
		AccessToken:          sentinelAccessToken,
		OriginalToken:        sentinelOrigToken,
		OriginalRefreshToken: sentinelOrigRefresh,
		InvitationCode:       sentinelInvitation,
		IdCard:               sentinelIdCard,
		CreatedIp:            sentinelCreatedIp,
		LastSigninIp:         sentinelLastIp,
		LastSigninWrongTime:  sentinelWrongTime,
		SigninWrongTimes:     4,
		Properties: map[string]string{
			"oauth_GitHub_accessToken": sentinelOAuthProp,
			"org_id":                   "org-42",
		},
		Roles:       []*Role{},
		Permissions: []*Permission{},
		Groups:      []string{},
	}
}

func claimsFor(user *User) Claims {
	return Claims{
		User:      user,
		TokenType: "access-token",
		Scope:     "openid profile email",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:   "https://idp.example.com",
			Subject:  user.Id,
			Audience: []string{"client-id"},
		},
	}
}

func marshal(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func assertNoSentinel(t *testing.T, format string, payload string) {
	t.Helper()
	for _, s := range sentinels {
		if strings.Contains(payload, s) {
			t.Errorf("%s: token payload leaks %q", format, s)
		}
	}
}

func assertNoKeys(t *testing.T, format string, payload string, keys ...string) {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("%s: unmarshal: %v", format, err)
	}
	for _, k := range keys {
		if _, ok := m[k]; ok {
			t.Errorf("%s: claim %q must not be present", format, k)
		}
	}
}

var forbiddenClaimKeys = []string{
	"password", "passwordSalt", "totpSecret", "recoveryCodes", "managedAccounts", "mfaAccounts",
	"accessToken", "originalToken", "originalRefreshToken", "invitationCode", "idCard",
	"createdIp", "lastSigninIp", "lastSigninWrongTime", "signinWrongTimes", "webauthnCredentials",
}

// WHEN an application uses the legacy default format "JWT"
// THEN the token carries no credential, MFA secret or personal datum, and still carries the profile.
func TestTokenFormatJWTCarriesNoSecrets(t *testing.T) {
	payload := marshal(t, getClaimsWithoutThirdIdp(claimsFor(userWithSecrets())))

	assertNoSentinel(t, "JWT", payload)
	assertNoKeys(t, "JWT", payload, forbiddenClaimKeys...)
	if !strings.Contains(payload, `"email":"ana@example.com"`) || !strings.Contains(payload, `"owner":"org-test"`) ||
		!strings.Contains(payload, `"org_id":"org-42"`) {
		t.Errorf("JWT: profile claims lost: %s", payload)
	}
}

// WHEN the formats "JWT-Empty" and "JWT-Standard" are used THEN no secret leaks either.
func TestTokenFormatsShortAndStandardCarryNoSecrets(t *testing.T) {
	assertNoSentinel(t, "JWT-Empty", marshal(t, getShortClaims(claimsFor(userWithSecrets()))))

	standard := marshal(t, getStandardClaims(claimsFor(userWithSecrets())))
	assertNoSentinel(t, "JWT-Standard", standard)
	if !strings.Contains(standard, `"email":"ana@example.com"`) {
		t.Errorf("JWT-Standard: email lost: %s", standard)
	}
}

// WHEN an administrator lists forbidden fields in tokenFields / Existing Field attributes of
// "JWT-Custom" (in any casing) THEN they are not emitted, and the allowed ones still are.
func TestTokenFormatCustomSkipsForbiddenFields(t *testing.T) {
	fields := []string{
		"Password", "PasswordSalt", "TotpSecret", "totpSecret", "totp_secret", "RecoveryCodes",
		"ManagedAccounts", "MfaAccounts", "AccessToken", "OriginalToken", "OriginalRefreshToken",
		"InvitationCode", "IdCard", "CreatedIp", "LastSigninIp", "LastSigninWrongTime",
		"SigninWrongTimes", "WebauthnCredentials", "Properties.oauth_GitHub_accessToken",
		// allowed
		"Name", "Email", "Properties.org_id",
	}
	attributes := []*JwtItem{
		{Name: "segredo", Category: "Existing Field", Value: "TotpSecret"},
		{Name: "codigos", Category: "Existing Field", Value: "RecoveryCodes"},
		{Name: "mail", Category: "Existing Field", Value: "Email"},
	}

	payload := marshal(t, getClaimsCustom(claimsFor(userWithSecrets()), fields, attributes))

	assertNoSentinel(t, "JWT-Custom", payload)
	assertNoKeys(t, "JWT-Custom", payload, append(forbiddenClaimKeys, "segredo", "codigos",
		"oauth_GitHub_accessToken")...)
	for _, want := range []string{`"email":"ana@example.com"`, `"org_id":"org-42"`, `"mail":"ana@example.com"`} {
		if !strings.Contains(payload, want) {
			t.Errorf("JWT-Custom: allowed claim %s lost: %s", want, payload)
		}
	}
}

// Issuance must not mutate the caller's user: the same pointer may be persisted afterwards, and
// clearing it would wipe the user's TOTP and recovery codes in the database.
func TestTokenClaimsDoNotMutateUser(t *testing.T) {
	user := userWithSecrets()
	_ = getClaimsWithoutThirdIdp(claimsFor(user))
	_ = getClaimsCustom(claimsFor(user), []string{"TotpSecret"}, nil)

	if user.TotpSecret != sentinelTotp || len(user.RecoveryCodes) != 1 || user.PasswordSalt != sentinelSalt ||
		user.Properties["oauth_GitHub_accessToken"] != sentinelOAuthProp {
		t.Fatal("claim building mutated the user's secrets")
	}
}

func TestIsForbiddenTokenField(t *testing.T) {
	cases := map[string]bool{
		"TotpSecret": true, "totpSecret": true, "TOTP_SECRET": true, " PasswordSalt ": true,
		"Properties.oauth_Google_refreshToken": true, "Properties.oauth_GitHub_accessToken": true,
		"Name": false, "Email": false, "DisplayName": false, "Roles": false, "permissionNames": false,
		"Properties.org_id": false, "Properties.pessoa_id": false, "Properties.oauth_GitHub_id": false,
	}
	for field, want := range cases {
		if got := IsForbiddenTokenField(field); got != want {
			t.Errorf("IsForbiddenTokenField(%q) = %v, want %v", field, got, want)
		}
	}
}

// WHEN an application configuration asks for a forbidden field THEN saving it is refused;
// a configuration with only allowed fields (e.g. the one in production for alcada) is accepted.
func TestValidateApplicationTokenFields(t *testing.T) {
	refused := &Application{
		TokenFields:     []string{"Email", "TotpSecret"},
		TokenAttributes: []*JwtItem{{Name: "x", Category: "Existing Field", Value: "PasswordSalt"}},
	}
	err := validateApplicationTokenFields(refused)
	if err == nil || !strings.Contains(err.Error(), "TotpSecret") || !strings.Contains(err.Error(), "PasswordSalt") {
		t.Fatalf("expected refusal naming both fields, got %v", err)
	}

	accepted := &Application{
		TokenFields: []string{"Name", "DisplayName", "Email", "Properties.org_id", "Properties.pessoa_id"},
		TokenAttributes: []*JwtItem{
			{Name: "papel", Category: "Static Value", Value: "$user.roles"},
			{Name: "mail", Category: "Existing Field", Value: "Email"},
		},
	}
	if err := validateApplicationTokenFields(accepted); err != nil {
		t.Fatalf("allowed configuration refused: %v", err)
	}
	if err := validateApplicationTokenFields(&Application{}); err != nil {
		t.Fatalf("empty configuration refused: %v", err)
	}
}
