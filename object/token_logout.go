// Copyright 2026 The Casdoor Authors. All Rights Reserved.
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
	"net/http"
	"net/url"
	"time"

	"github.com/beego/beego/v2/core/logs"

	"github.com/casdoor/casdoor/util"
	"github.com/golang-jwt/jwt/v5"
)

// LogoutTokenClaims represents the claims in an OIDC Back-Channel Logout token.
// See https://openid.net/specs/openid-connect-backchannel-1_0.html
type LogoutTokenClaims struct {
	Events map[string]interface{} `json:"events"`
	Sid    string                 `json:"sid,omitempty"`
	jwt.RegisteredClaims
}

// logoutTokenTTL bounds the logout token's validity (spec §2.4 makes exp required):
// it is posted immediately, so a short window only limits replay.
const logoutTokenTTL = 2 * time.Minute

// backchannelHTTPTimeout bounds each delivery; the default client has none, which
// left a goroutine hanging per unresponsive RP.
const backchannelHTTPTimeout = 5 * time.Second

var backchannelClient = &http.Client{Timeout: backchannelHTTPTimeout}

func generateLogoutToken(application *Application, user *User, sessionId string, host string) (string, error) {
	_, originBackend := getOriginFromHost(host)

	cert, err := getCertByApplication(application)
	if err != nil {
		return "", err
	}
	if cert == nil {
		return "", fmt.Errorf("no cert found for application: %s", application.GetId())
	}
	return signLogoutToken(application, cert, user, sessionId, originBackend, time.Now())
}

// signLogoutToken builds and signs an OIDC Back-Channel Logout token
// (https://openid.net/specs/openid-connect-backchannel-1_0.html §2.4) the same way
// access tokens are signed: same algorithm, same kid, and the private key resolved
// through the key custodian (CertPrivateKeyPEM) — never read straight from the cert
// row, which only holds a reference when the key is sealed or vaulted (B5).
func signLogoutToken(application *Application, cert *Cert, user *User, sessionId, issuer string, now time.Time) (string, error) {
	claims := LogoutTokenClaims{
		Events: map[string]interface{}{
			"http://schemas.openid.net/event/backchannel-logout": map[string]interface{}{},
		},
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   user.Id,
			Audience:  []string{application.ClientId},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(logoutTokenTTL)),
			ID:        util.GetId(application.Owner, util.GenerateId()),
		},
	}
	if sessionId != "" {
		claims.Sid = sessionId
	}

	privateKeyPEM, err := CertPrivateKeyPEM(cert)
	if err != nil {
		return "", err
	}
	key, err := parseSigningKey(application.TokenSigningMethod, privateKeyPEM)
	if err != nil {
		return "", err
	}

	token := jwt.NewWithClaims(tokenSigningMethod(application.TokenSigningMethod), claims)
	token.Header["kid"] = cert.Name
	token.Header["typ"] = "logout+jwt" // spec §2.4: explicit typing
	return token.SignedString(key)
}

// SendBackchannelLogout sends OIDC Back-Channel Logout tokens to all registered
// backchannel_logout_uri endpoints for applications that have active tokens for the user.
// See https://openid.net/specs/openid-connect-backchannel-1_0.html
func SendBackchannelLogout(organization, username, sessionId, host string) {
	tokens, err := GetActiveTokensByUser(organization, username)
	if err != nil || len(tokens) == 0 {
		return
	}

	// Deduplicate applications
	seen := map[string]bool{}
	for _, token := range tokens {
		appId := util.GetId(token.Owner, token.Application)
		if seen[appId] {
			continue
		}
		seen[appId] = true

		application, err := GetApplication(appId)
		if err != nil || application == nil || application.BackchannelLogoutUri == "" {
			continue
		}

		user, err := GetUser(util.GetId(organization, username))
		if err != nil || user == nil {
			continue
		}

		logoutToken, err := generateLogoutToken(application, user, sessionId, host)
		if err != nil {
			// Was a silent continue: a sealed key or a bad cert meant no RP was ever told.
			logs.Error("backchannel logout: token for %s not generated: %v", appId, err)
			continue
		}

		go postBackchannelLogout(application.BackchannelLogoutUri, logoutToken)
	}
}

func postBackchannelLogout(logoutUri, logoutToken string) {
	body := url.Values{"logout_token": {logoutToken}}
	resp, err := backchannelClient.PostForm(logoutUri, body)
	if err != nil {
		logs.Warning("backchannel logout: POST %s failed: %v", logoutUri, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		logs.Warning("backchannel logout: POST %s answered %d", logoutUri, resp.StatusCode)
	}
}
