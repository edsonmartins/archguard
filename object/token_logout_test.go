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
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/adapters/keystore"
	"github.com/golang-jwt/jwt/v5"
)

const backchannelEvent = "http://schemas.openid.net/event/backchannel-logout"

func privatePEM(t *testing.T, key crypto.Signer) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func parseLogoutToken(t *testing.T, signed string, pub interface{}) (*jwt.Token, *LogoutTokenClaims) {
	t.Helper()
	claims := &LogoutTokenClaims{}
	tok, err := jwt.ParseWithClaims(signed, claims, func(*jwt.Token) (interface{}, error) { return pub, nil })
	if err != nil {
		t.Fatalf("logout token does not verify with the cert's public key: %v", err)
	}
	return tok, claims
}

// B5: with the key sealed in the keystore, the cert row only holds a reference.
// The logout token used to parse that reference as PEM, fail, and silently skip
// every RP. It must resolve the key through the custodian, like access tokens.
func TestSignLogoutTokenResolvesSealedKey(t *testing.T) {
	ks, err := keystore.Open(filepath.Join(t.TempDir(), "ks"), []byte("material"))
	if err != nil {
		t.Fatal(err)
	}
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	if err := ks.Put("admin/cert-built-in", privatePEM(t, rsaKey)); err != nil {
		t.Fatal(err)
	}
	prevKS, prevVault := devKeystore, vaultSecretStore
	t.Cleanup(func() { devKeystore, vaultSecretStore = prevKS, prevVault })
	devKeystore, vaultSecretStore = ks, nil

	app := &Application{Owner: "admin", Name: "app-archbase", ClientId: "client-archbase", TokenSigningMethod: "RS256"}
	cert := &Cert{Owner: "admin", Name: "cert-built-in", PrivateKey: keystoreRefPrefix + "admin/cert-built-in"}
	user := &User{Owner: "acme", Name: "ana", Id: "user-id-ana"}
	now := time.Now().Truncate(time.Second)

	signed, err := signLogoutToken(app, cert, user, "sess-1", "https://idp.example.com", now)
	if err != nil {
		t.Fatalf("signLogoutToken with sealed key: %v", err)
	}
	tok, claims := parseLogoutToken(t, signed, &rsaKey.PublicKey)

	if tok.Method.Alg() != "RS256" || tok.Header["kid"] != "cert-built-in" || tok.Header["typ"] != "logout+jwt" {
		t.Fatalf("header = %v, want alg RS256, kid cert-built-in, typ logout+jwt", tok.Header)
	}
	if claims.Issuer != "https://idp.example.com" || claims.Subject != "user-id-ana" || claims.Sid != "sess-1" {
		t.Fatalf("iss/sub/sid = %q/%q/%q", claims.Issuer, claims.Subject, claims.Sid)
	}
	if len(claims.Audience) != 1 || claims.Audience[0] != "client-archbase" {
		t.Fatalf("aud = %v, want the client_id", claims.Audience)
	}
	if claims.IssuedAt == nil || claims.ExpiresAt == nil || claims.ExpiresAt.Sub(claims.IssuedAt.Time) != logoutTokenTTL {
		t.Fatalf("iat/exp = %v/%v, want exp = iat + %v (spec §2.4: exp required)", claims.IssuedAt, claims.ExpiresAt, logoutTokenTTL)
	}
	if claims.ID == "" {
		t.Fatal("jti required")
	}
	if _, ok := claims.Events[backchannelEvent].(map[string]interface{}); !ok {
		t.Fatalf("events = %v, want the back-channel logout member as an object", claims.Events)
	}

	// Without sid, the claim is omitted (sub alone identifies the user).
	signed, err = signLogoutToken(app, cert, user, "", "https://idp.example.com", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, claims = parseLogoutToken(t, signed, &rsaKey.PublicKey); claims.Sid != "" {
		t.Fatalf("sid = %q, want omitted", claims.Sid)
	}
}

// The logout token used ES256 for every ES* application: with a P-384 key
// (TokenSigningMethod ES384) signing failed and no RP was notified. It must use
// the same algorithm as the application's access tokens.
func TestSignLogoutTokenUsesTheApplicationAlgorithm(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	app := &Application{Owner: "admin", Name: "app", ClientId: "c", TokenSigningMethod: "ES384"}
	cert := &Cert{Owner: "admin", Name: "cert-ec", PrivateKey: privatePEM(t, ecKey)}

	signed, err := signLogoutToken(app, cert, &User{Id: "u"}, "", "https://idp", time.Now())
	if err != nil {
		t.Fatalf("signLogoutToken ES384: %v", err)
	}
	if tok, _ := parseLogoutToken(t, signed, &ecKey.PublicKey); tok.Method.Alg() != "ES384" {
		t.Fatalf("alg = %s, want ES384 (same as access tokens)", tok.Method.Alg())
	}
	if got := tokenSigningMethod("ES384").Alg(); got != "ES384" {
		t.Fatalf("tokenSigningMethod(ES384) = %s", got)
	}
}

// A key reference with no custody open is an error, never a silently unsigned or
// skipped token: the caller now logs it.
func TestSignLogoutTokenFailsClosedWithoutCustody(t *testing.T) {
	prevKS, prevVault := devKeystore, vaultSecretStore
	t.Cleanup(func() { devKeystore, vaultSecretStore = prevKS, prevVault })
	devKeystore, vaultSecretStore = nil, nil

	cert := &Cert{Owner: "admin", Name: "x", PrivateKey: keystoreRefPrefix + "admin/x"}
	if _, err := signLogoutToken(&Application{TokenSigningMethod: "RS256"}, cert, &User{Id: "u"}, "", "https://idp", time.Now()); err == nil {
		t.Fatal("sealed key with no custody must error")
	}
}

// Delivery posts logout_token as a form field and is bounded by a timeout: the
// default client had none, leaving a goroutine per unresponsive RP.
func TestPostBackchannelLogoutDeliversAndTimesOut(t *testing.T) {
	received := make(chan string, 1)
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r.PostFormValue("logout_token")
	}))
	defer ok.Close()

	postBackchannelLogout(ok.URL, "the-logout-token")
	select {
	case got := <-received:
		if got != "the-logout-token" {
			t.Fatalf("logout_token = %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RP never received the logout token")
	}

	release := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))
	defer slow.Close()
	defer close(release)

	prev := backchannelClient
	backchannelClient = &http.Client{Timeout: 100 * time.Millisecond}
	t.Cleanup(func() { backchannelClient = prev })

	start := time.Now()
	postBackchannelLogout(slow.URL, "t")
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("delivery took %v, want bounded by the client timeout", elapsed)
	}
	if backchannelHTTPTimeout <= 0 || prev.Timeout != backchannelHTTPTimeout {
		t.Fatalf("production client timeout = %v, want %v", prev.Timeout, backchannelHTTPTimeout)
	}
}
