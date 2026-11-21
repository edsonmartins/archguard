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
	"fmt"
	"strings"
)

// forbiddenTokenFields lists the User fields that must never become a claim, in any token
// format. An access token is readable by anyone who holds it (browser, proxy, log, every
// relying party), so a credential or MFA secret in it defeats the factor it protects (INV-7),
// and personal data not needed by relying parties violates data minimisation (I-3.2).
//
// The upstream legacy "JWT" format serialised the whole user, including these fields; the
// "JWT-Custom" format copies any field named in tokenFields/tokenAttributes by reflection.
// Keys are normalised by normaliseTokenFieldName.
var forbiddenTokenFields = map[string]struct{}{
	// credentials
	"password":             {},
	"passwordsalt":         {},
	"accesstoken":          {},
	"originaltoken":        {},
	"originalrefreshtoken": {},
	"invitationcode":       {},
	"managedaccounts":      {},
	// MFA secrets
	"totpsecret":          {},
	"recoverycodes":       {},
	"mfaaccounts":         {},
	"webauthncredentials": {},
	// personal / security data not needed by relying parties
	"idcard":              {},
	"createdip":           {},
	"lastsigninip":        {},
	"lastsigninwrongtime": {},
	"signinwrongtimes":    {},
}

func normaliseTokenFieldName(name string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), "_", ""))
}

// isOAuthTokenProperty matches the per-provider OAuth tokens kept in User.Properties, with the
// same rule GetMaskedUser uses to hide them from the API.
func isOAuthTokenProperty(key string) bool {
	return strings.HasPrefix(key, "oauth_") &&
		(strings.HasSuffix(key, "_accessToken") || strings.HasSuffix(key, "_refreshToken"))
}

// tokenSafeProperties returns a copy of the user properties without the per-provider OAuth
// tokens. The legacy "JWT" format emits the whole map; the caller's map is left untouched.
func tokenSafeProperties(properties map[string]string) map[string]string {
	if properties == nil {
		return nil
	}
	res := make(map[string]string, len(properties))
	for key, value := range properties {
		if !isOAuthTokenProperty(key) {
			res[key] = value
		}
	}
	return res
}

// IsForbiddenTokenField reports whether a tokenFields entry or an "Existing Field" token
// attribute names a value that must never be emitted as a claim.
func IsForbiddenTokenField(field string) bool {
	if strings.HasPrefix(field, "Properties.") {
		return isOAuthTokenProperty(strings.TrimPrefix(field, "Properties."))
	}
	_, forbidden := forbiddenTokenFields[normaliseTokenFieldName(field)]
	return forbidden
}

// ForbiddenTokenFieldsOf returns the forbidden fields an application configuration asks to put
// in its tokens, in configuration order.
func ForbiddenTokenFieldsOf(application *Application) []string {
	if application == nil {
		return nil
	}
	var found []string
	for _, field := range application.TokenFields {
		if IsForbiddenTokenField(field) {
			found = append(found, field)
		}
	}
	for _, item := range application.TokenAttributes {
		if item != nil && item.Category == "Existing Field" && IsForbiddenTokenField(item.Value) {
			found = append(found, item.Value)
		}
	}
	return found
}

// validateApplicationTokenFields refuses to store a configuration that would put a forbidden
// field in tokens. Issuance skips such fields anyway; refusing here tells the administrator the
// configuration does not take effect instead of silently ignoring it.
func validateApplicationTokenFields(application *Application) error {
	if found := ForbiddenTokenFieldsOf(application); len(found) > 0 {
		return fmt.Errorf("campos não permitidos no token: %s — credenciais, segredos de MFA e dados pessoais "+
			"não podem ser emitidos como claim", strings.Join(found, ", "))
	}
	return nil
}
