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

package domain

import (
	"regexp"
	"strings"
)

// Redaction of telemetry signals (pacote 010, T-004 / spec "Higiene de dados
// sensíveis em telemetria" / I-3.2 / INV-7). NO signal — a log line, a metric
// label, a trace attribute — may carry a secret, a token, or plaintext personal
// data. This is the last line before emission: a caller that forgets to
// pseudonymize is still scrubbed. Telemetry references a user by a stable
// pseudonym (identity.Subject), never an e-mail — an e-mail that reaches here is
// redacted.

// Redacted is the placeholder emitted in place of sensitive material.
const Redacted = "[REDACTED]"

var (
	// jwtPattern matches a compact JWS/JWT (three base64url segments).
	jwtPattern = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)
	// bearerPattern matches an Authorization bearer/basic credential.
	bearerPattern = regexp.MustCompile(`(?i)(bearer|basic)\s+[A-Za-z0-9._~+/=-]+`)
	// emailPattern matches an e-mail address (plaintext PII).
	emailPattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
)

// sensitiveKeys are attribute keys whose VALUE is always redacted regardless of
// content, because the key names sensitive data.
var sensitiveKeys = map[string]bool{
	"password": true, "passwd": true, "secret": true, "client_secret": true,
	"token": true, "access_token": true, "refresh_token": true, "id_token": true,
	"authorization": true, "auth": true, "cookie": true, "set-cookie": true,
	"api_key": true, "apikey": true, "private_key": true, "credential": true,
	"bearer": true, "jwt": true, "seed": true, "totp": true, "otp": true,
	"email": true, "e-mail": true, "primary_email": true, "mail": true,
	"credential_ref": false, // a vault REFERENCE is not a secret — allowed
}

// sensitiveKeysStripped is sensitiveKeys keyed by the separator-free lowercase
// form, so "Client-Secret", "clientSecret" and "client_secret" all resolve to the
// same entry.
var sensitiveKeysStripped = func() map[string]bool {
	out := make(map[string]bool, len(sensitiveKeys))
	for k, v := range sensitiveKeys {
		out[stripSeparators(k)] = v
	}
	return out
}()

// SensitiveKey reports whether an attribute key names sensitive data whose value
// must be redacted wholesale. Comparison is case-insensitive and ignores common
// separators, so "Client-Secret", "clientSecret" and "client_secret" all match.
func SensitiveKey(key string) bool {
	if v, ok := sensitiveKeysStripped[stripSeparators(key)]; ok {
		return v
	}
	return false
}

// stripSeparators lowercases and removes '-', '_' and spaces, collapsing
// camelCase and separated forms to one comparable token.
func stripSeparators(key string) string {
	k := strings.ToLower(strings.TrimSpace(key))
	k = strings.ReplaceAll(k, "-", "")
	k = strings.ReplaceAll(k, "_", "")
	k = strings.ReplaceAll(k, " ", "")
	return k
}

// RedactValue scrubs a free-text value of embedded secrets: compact JWTs, bearer
// credentials, and e-mail addresses. Non-sensitive text passes through unchanged.
func RedactValue(s string) string {
	// Bearer/basic first, so "Bearer <jwt>" is redacted as one credential rather
	// than leaving a dangling "Bearer" after the inner JWT is scrubbed.
	s = bearerPattern.ReplaceAllString(s, "[REDACTED-AUTH]")
	s = jwtPattern.ReplaceAllString(s, "[REDACTED-JWT]")
	s = emailPattern.ReplaceAllString(s, "[REDACTED-EMAIL]")
	return s
}

// RedactAttr redacts one telemetry attribute. A sensitive KEY redacts the whole
// value; otherwise the value is scrubbed for embedded secrets (RedactValue). The
// key itself is returned unchanged (keys are not sensitive; values are).
func RedactAttr(key, value string) string {
	if SensitiveKey(key) {
		return Redacted
	}
	return RedactValue(value)
}

// ContainsSensitive reports whether a string still carries a recognizable secret
// or PII after (or without) redaction — the predicate the telemetry conformance
// test (T-005) uses to fail the build on a leak.
func ContainsSensitive(s string) bool {
	return jwtPattern.MatchString(s) || bearerPattern.MatchString(s) || emailPattern.MatchString(s)
}
