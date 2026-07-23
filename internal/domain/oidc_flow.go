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
	"errors"
	"fmt"
)

// OAuthFlow is an authorization flow the ArchGuard AS supports (RFC-0006 §2 /
// design 006). Only these two exist — Authorization Code (with mandatory PKCE)
// and Device Authorization Grant (for browserless clients). Implicit and Resource
// Owner Password Credentials (ROPC) are NOT flows here: they are refused, not
// disabled by config (there is no supported value that names them).
type OAuthFlow string

const (
	FlowAuthorizationCode OAuthFlow = "authorization_code"
	FlowDeviceCode        OAuthFlow = "device_code"
)

// Errors of flow validation.
var (
	// ErrUnsupportedFlow is returned for implicit / ROPC or any flow that is not
	// Authorization Code or Device Code (spec "Fluxo obsoleto").
	ErrUnsupportedFlow = errors.New("oidc: fluxo de autorização não suportado")
	// ErrPKCERequired is returned when an Authorization Code request lacks a valid
	// PKCE challenge (spec "PKCE ausente").
	ErrPKCERequired = errors.New("oidc: PKCE (S256) é obrigatório no Authorization Code")
)

// pkceMethodS256 is the ONLY accepted PKCE method — "plain" is refused, so a
// downgraded challenge cannot weaken the exchange.
const pkceMethodS256 = "S256"

// ValidateResponseType refuses the implicit flow: the only accepted OAuth
// response_type is "code" (RFC-0006 §2 — implicit is not supported). A
// response_type containing "token" (implicit / hybrid returning a token from the
// authorization endpoint) is refused.
func ValidateResponseType(responseType string) error {
	if responseType != "code" {
		return fmt.Errorf("%w: response_type %q (apenas 'code')", ErrUnsupportedFlow, responseType)
	}
	return nil
}

// ValidateGrantType refuses ROPC and any grant that is not one of the supported
// token-endpoint grants: authorization_code, refresh_token, urn:ietf:params:oauth:grant-type:device_code.
// The password grant (ROPC) is refused outright.
func ValidateGrantType(grantType string) error {
	switch grantType {
	case "authorization_code", "refresh_token", "urn:ietf:params:oauth:grant-type:device_code":
		return nil
	case "password":
		return fmt.Errorf("%w: ROPC (grant_type=password)", ErrUnsupportedFlow)
	default:
		return fmt.Errorf("%w: grant_type %q", ErrUnsupportedFlow, grantType)
	}
}

// ValidatePKCE requires a non-empty code challenge and the S256 method — PKCE is
// mandatory on every interactive Authorization Code request (RFC-0006 §2 / §5).
// "plain" is refused (only S256), and an absent challenge is refused (spec "PKCE
// ausente").
func ValidatePKCE(codeChallenge, method string) error {
	if codeChallenge == "" {
		return fmt.Errorf("%w: code_challenge ausente", ErrPKCERequired)
	}
	if method != pkceMethodS256 {
		return fmt.Errorf("%w: método %q (apenas S256)", ErrPKCERequired, method)
	}
	return nil
}

// ValidateAuthorizationCodeRequest is the combined gate for an Authorization Code
// authorization request: the response_type must be "code" (no implicit) AND a
// valid S256 PKCE challenge must be present. It is fail-closed — any deviation
// refuses the request before an authorization code is ever issued.
func ValidateAuthorizationCodeRequest(responseType, codeChallenge, codeChallengeMethod string) error {
	if err := ValidateResponseType(responseType); err != nil {
		return err
	}
	return ValidatePKCE(codeChallenge, codeChallengeMethod)
}
