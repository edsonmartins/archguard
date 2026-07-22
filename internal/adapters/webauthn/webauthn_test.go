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

package webauthn

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/fxamacker/cbor/v2"
	"github.com/google/uuid"
)

const (
	testRPID   = "archguard.example"
	testOrigin = "https://archguard.example"
)

// virtualAuthenticator is a minimal ES256 (P-256) WebAuthn authenticator for
// tests — it crafts the attestation and assertion responses go-webauthn
// validates, so the full ceremony round-trip runs without a browser or hardware.
type virtualAuthenticator struct {
	key       *ecdsa.PrivateKey
	credID    []byte
	uv        bool // user-verified
	be        bool // backup-eligible (synced passkey)
	signCount uint32
}

func newAuthenticator(t *testing.T, uv, be bool) *virtualAuthenticator {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("gerar chave: %v", err)
	}
	credID := make([]byte, 16)
	if _, err := rand.Read(credID); err != nil {
		t.Fatalf("credID: %v", err)
	}
	return &virtualAuthenticator{key: key, credID: credID, uv: uv, be: be}
}

func (a *virtualAuthenticator) flags(attested bool) byte {
	var f byte = 0x01 // UP (user present)
	if a.uv {
		f |= 0x04 // UV
	}
	if a.be {
		f |= 0x08 // BE (backup eligible)
	}
	if attested {
		f |= 0x40 // AT (attested credential data present)
	}
	return f
}

func pad32(n *big.Int) []byte {
	b := n.Bytes()
	if len(b) >= 32 {
		return b[len(b)-32:]
	}
	out := make([]byte, 32)
	copy(out[32-len(b):], b)
	return out
}

// cosePublicKey builds the COSE_Key (EC2/ES256/P-256) of the authenticator.
func (a *virtualAuthenticator) cosePublicKey(t *testing.T) []byte {
	t.Helper()
	// COSE keys use integer labels: 1=kty, 3=alg, -1=crv, -2=x, -3=y.
	m := map[int]interface{}{
		1:  2,  // kty: EC2
		3:  -7, // alg: ES256
		-1: 1,  // crv: P-256
		-2: pad32(a.key.PublicKey.X),
		-3: pad32(a.key.PublicKey.Y),
	}
	b, err := cbor.Marshal(m)
	if err != nil {
		t.Fatalf("cbor COSE: %v", err)
	}
	return b
}

// authData builds the authenticator data. When attested, it appends the
// attested credential data (aaguid + credId + COSE key).
func (a *virtualAuthenticator) authData(t *testing.T, attested bool) []byte {
	t.Helper()
	rpHash := sha256.Sum256([]byte(testRPID))
	buf := bytes.NewBuffer(nil)
	buf.Write(rpHash[:])
	buf.WriteByte(a.flags(attested))
	sc := make([]byte, 4)
	binary.BigEndian.PutUint32(sc, a.signCount)
	buf.Write(sc)
	if attested {
		buf.Write(make([]byte, 16)) // aaguid (zeros)
		idLen := make([]byte, 2)
		binary.BigEndian.PutUint16(idLen, uint16(len(a.credID)))
		buf.Write(idLen)
		buf.Write(a.credID)
		buf.Write(a.cosePublicKey(t))
	}
	return buf.Bytes()
}

func clientDataJSON(t *testing.T, typ, challenge string) []byte {
	t.Helper()
	cd := map[string]interface{}{"type": typ, "challenge": challenge, "origin": testOrigin}
	b, err := json.Marshal(cd)
	if err != nil {
		t.Fatalf("clientData: %v", err)
	}
	return b
}

// registrationResponse crafts the attestation ("none") response body.
func (a *virtualAuthenticator) registrationResponse(t *testing.T, challenge string) []byte {
	t.Helper()
	authData := a.authData(t, true)
	att := map[string]interface{}{"fmt": "none", "attStmt": map[string]interface{}{}, "authData": authData}
	attObj, err := cbor.Marshal(att)
	if err != nil {
		t.Fatalf("cbor attestation: %v", err)
	}
	cd := clientDataJSON(t, "webauthn.create", challenge)
	resp := map[string]interface{}{
		"id":    base64.RawURLEncoding.EncodeToString(a.credID),
		"rawId": base64.RawURLEncoding.EncodeToString(a.credID),
		"type":  "public-key",
		"response": map[string]interface{}{
			"clientDataJSON":    base64.RawURLEncoding.EncodeToString(cd),
			"attestationObject": base64.RawURLEncoding.EncodeToString(attObj),
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

// assertionResponse crafts the assertion (login) response body, signing over
// authData || sha256(clientDataJSON).
func (a *virtualAuthenticator) assertionResponse(t *testing.T, challenge string) []byte {
	t.Helper()
	a.signCount++
	authData := a.authData(t, false)
	cd := clientDataJSON(t, "webauthn.get", challenge)
	cdHash := sha256.Sum256(cd)
	signed := append(append([]byte{}, authData...), cdHash[:]...)
	digest := sha256.Sum256(signed)
	sig, err := ecdsa.SignASN1(rand.Reader, a.key, digest[:])
	if err != nil {
		t.Fatalf("assinar assertion: %v", err)
	}
	resp := map[string]interface{}{
		"id":    base64.RawURLEncoding.EncodeToString(a.credID),
		"rawId": base64.RawURLEncoding.EncodeToString(a.credID),
		"type":  "public-key",
		"response": map[string]interface{}{
			"clientDataJSON":    base64.RawURLEncoding.EncodeToString(cd),
			"authenticatorData": base64.RawURLEncoding.EncodeToString(authData),
			"signature":         base64.RawURLEncoding.EncodeToString(sig),
			"userHandle":        base64.RawURLEncoding.EncodeToString([]byte("sub-webauthn")),
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	s, err := NewService("ArchGuard", testRPID, []string{testOrigin})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return s
}

// Ciclo completo: registro de um autenticador de hardware (UV, não-backup) →
// credencial de domínio AAL3 (forma INV-7); depois login verificado.
func TestRegisterAndLoginHardwareKeyIsAAL3(t *testing.T) {
	svc := newTestService(t)
	idID := uuid.New()
	auth := newAuthenticator(t, true /*uv*/, false /*backup-eligible*/)

	// --- registro ---
	user, _ := UserFromIdentity("sub-webauthn", "Operador", nil)
	_, session, err := svc.BeginRegistration(user)
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}
	regResp := auth.registrationResponse(t, session.Challenge)
	cred, err := svc.FinishRegistration(idID, user, *session, bytes.NewReader(regResp))
	if err != nil {
		t.Fatalf("FinishRegistration: %v", err)
	}
	if cred.Type != domain.FactorWebAuthn || !cred.WellFormed() {
		t.Fatalf("credencial WebAuthn malformada: %+v", cred)
	}
	if cred.AAL != domain.AAL3 {
		t.Fatalf("hardware user-verified deveria ser AAL3, veio %s", cred.AAL)
	}
	if !cred.PhishingResistant() {
		t.Fatalf("WebAuthn deveria ser phishing-resistant")
	}
	// INV-7: nada de segredo — só material público.
	if len(cred.Verifier) != 0 || cred.SecretRef != "" || len(cred.PublicMaterial) == 0 {
		t.Fatalf("credencial WebAuthn não deveria ter verifier/secret_ref")
	}

	// --- login com a credencial registrada ---
	user2, err := UserFromIdentity("sub-webauthn", "Operador", []domain.Credential{cred})
	if err != nil {
		t.Fatalf("UserFromIdentity: %v", err)
	}
	_, loginSession, err := svc.BeginLogin(user2)
	if err != nil {
		t.Fatalf("BeginLogin: %v", err)
	}
	loginResp := auth.assertionResponse(t, loginSession.Challenge)
	res, err := svc.FinishLogin(user2, *loginSession, bytes.NewReader(loginResp))
	if err != nil {
		t.Fatalf("FinishLogin: %v", err)
	}
	if !bytes.Equal(res.CredentialID, auth.credID) {
		t.Fatalf("login usou credencial errada")
	}
	if res.CloneWarning {
		t.Fatalf("login legítimo não deveria acusar clone")
	}
}

// Passkey sincronizada (backup-eligible) é AAL2, não AAL3 — a distinção
// hardware-vs-passkey do ADR-0010.
func TestSyncedPasskeyIsAAL2(t *testing.T) {
	svc := newTestService(t)
	auth := newAuthenticator(t, true /*uv*/, true /*backup-eligible*/)
	user, _ := UserFromIdentity("sub-passkey", "User", nil)
	_, session, err := svc.BeginRegistration(user)
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}
	cred, err := svc.FinishRegistration(uuid.New(), user, *session, bytes.NewReader(auth.registrationResponse(t, session.Challenge)))
	if err != nil {
		t.Fatalf("FinishRegistration: %v", err)
	}
	if cred.AAL != domain.AAL2 {
		t.Fatalf("passkey sincronizada deveria ser AAL2, veio %s", cred.AAL)
	}
}

// Uma resposta de registro adulterada (challenge errado) é recusada.
func TestRegistrationRejectsWrongChallenge(t *testing.T) {
	svc := newTestService(t)
	auth := newAuthenticator(t, true, false)
	user, _ := UserFromIdentity("sub-x", "X", nil)
	_, session, err := svc.BeginRegistration(user)
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}
	wrong := base64.RawURLEncoding.EncodeToString([]byte("desafio-errado-de-32-bytes-aqui!"))
	if _, err := svc.FinishRegistration(uuid.New(), user, *session, bytes.NewReader(auth.registrationResponse(t, wrong))); err == nil {
		t.Fatalf("registro com challenge errado deveria ser recusado")
	}
}
