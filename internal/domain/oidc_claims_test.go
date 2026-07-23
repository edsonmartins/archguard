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
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func validClaims() OIDCClaims {
	return OIDCClaims{
		Issuer:        "https://archguard.example",
		Subject:       "sub-opaque",
		Audience:      "warpgate",
		ExpiresAt:     1700000900,
		IssuedAt:      1700000000,
		Organization:  "018f-org",
		MembershipID:  "018f-mid",
		ACR:           "L2",
		AMR:           []string{"pwd", "webauthn"},
		AuthTime:      1700000000,
		SessionID:     "018f-sid",
		ClaimsVersion: OIDCClaimsVersion,
	}
}

func TestOIDCClaimsWellFormed(t *testing.T) {
	if err := validClaims().WellFormed(); err != nil {
		t.Fatalf("claims válidos deveriam passar: %v", err)
	}
}

func TestOIDCClaimsWellFormedRejects(t *testing.T) {
	cases := map[string]func(*OIDCClaims){
		"sem iss":       func(c *OIDCClaims) { c.Issuer = "" },
		"sem sub":       func(c *OIDCClaims) { c.Subject = "" },
		"sem aud":       func(c *OIDCClaims) { c.Audience = "" },
		"sem org":       func(c *OIDCClaims) { c.Organization = "" },
		"sem mid":       func(c *OIDCClaims) { c.MembershipID = "" },
		"sem sid":       func(c *OIDCClaims) { c.SessionID = "" },
		"acr inválido":  func(c *OIDCClaims) { c.ACR = "L9" },
		"amr vazio":     func(c *OIDCClaims) { c.AMR = nil },
		"sem auth_time": func(c *OIDCClaims) { c.AuthTime = 0 },
		"exp <= iat":    func(c *OIDCClaims) { c.ExpiresAt = c.IssuedAt },
		"versão errada": func(c *OIDCClaims) { c.ClaimsVersion = "v0" },
	}
	for name, mangle := range cases {
		t.Run(name, func(t *testing.T) {
			c := validClaims()
			mangle(&c)
			if err := c.WellFormed(); !errors.Is(err, ErrInvalidClaims) {
				t.Fatalf("%s deveria ser inválido, err = %v", name, err)
			}
		})
	}
}

// O JSON serializado usa exatamente os nomes de claim do contrato (RFC-0006 §3);
// campos opcionais ausentes não aparecem; e-mail nunca vaza sem escopo.
func TestOIDCClaimsJSONContract(t *testing.T) {
	b, err := json.Marshal(validClaims())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	for _, required := range []string{"iss", "sub", "aud", "exp", "iat", "org", "mid", "acr", "amr", "auth_time", "sid", "archguard_claims_version"} {
		if _, ok := m[required]; !ok {
			t.Fatalf("claim obrigatório %q ausente do JSON", required)
		}
	}
	// Sem escopo de e-mail: o claim email não aparece (I-3.2).
	if _, ok := m["email"]; ok {
		t.Fatalf("email não deveria aparecer sem escopo")
	}
	// Opcionais ausentes não poluem o token.
	for _, optional := range []string{"act", "pcid", "grant_ref", "groups", "roles"} {
		if _, ok := m[optional]; ok {
			t.Fatalf("claim opcional %q não deveria aparecer quando vazio", optional)
		}
	}
}

// Emissão padrão (cenário): o token montado a partir da sessão contém iss, sub,
// org, mid, acr, amr, auth_time e sid — do TENANT ATIVO.
func TestBuildOIDCClaimsFromSession(t *testing.T) {
	id, org := uuid.New(), uuid.New()
	m, err := NewMembership(id, org)
	if err != nil {
		t.Fatalf("NewMembership: %v", err)
	}
	s, err := NewAuthSession(id, AAL2, []Membership{m}) // 1 membership -> ativa
	if err != nil {
		t.Fatalf("NewAuthSession: %v", err)
	}
	at := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	if err := s.SetAuthContext(at, []FactorType{FactorPassword, FactorTOTP}); err != nil {
		t.Fatalf("SetAuthContext: %v", err)
	}

	claims, err := BuildOIDCClaims(OIDCClaimsInput{
		Issuer:    "https://archguard.example",
		Audience:  "warpgate",
		Subject:   "sub-opaque",
		Session:   &s,
		IssuedAt:  at.Add(time.Minute),
		AccessTTL: 10 * time.Minute,
		Roles:     []string{"operator"},
	})
	if err != nil {
		t.Fatalf("BuildOIDCClaims: %v", err)
	}
	if claims.Organization != org.String() || claims.MembershipID != m.ID.String() {
		t.Fatalf("org/mid deveriam ser do tenant ativo: %+v", claims)
	}
	if claims.ACR != "L2" {
		t.Fatalf("acr = %q, quero L2", claims.ACR)
	}
	if len(claims.AMR) != 3 || claims.AMR[0] != "pwd" {
		t.Fatalf("amr = %v, quero [pwd otp mfa]", claims.AMR)
	}
	if claims.AuthTime != at.Unix() || claims.SessionID != s.ID.String() {
		t.Fatalf("auth_time/sid inesperados: %+v", claims)
	}
	if claims.ExpiresAt != claims.IssuedAt+600 {
		t.Fatalf("exp deveria ser iat + TTL")
	}
	if err := claims.WellFormed(); err != nil {
		t.Fatalf("claims montados deveriam ser WellFormed: %v", err)
	}
}

// Recusa: sessão pendente (sem tenant ativo) não emite claims; TTL fora da faixa
// é recusado.
func TestBuildOIDCClaimsRejects(t *testing.T) {
	id, org := uuid.New(), uuid.New()
	m1, _ := NewMembership(id, org)
	m2, _ := NewMembership(id, uuid.New())
	pending, err := NewAuthSession(id, AAL2, []Membership{m1, m2}) // 2 -> pending_selection
	if err != nil {
		t.Fatalf("NewAuthSession: %v", err)
	}
	at := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	base := OIDCClaimsInput{Issuer: "iss", Audience: "aud", Subject: "sub", Session: &pending, IssuedAt: at, AccessTTL: 10 * time.Minute}
	if _, err := BuildOIDCClaims(base); !errors.Is(err, ErrInvalidClaims) {
		t.Fatalf("sessão pendente não deveria emitir claims: %v", err)
	}

	// TTL fora da faixa (RFC-0006 §5).
	active, _ := NewAuthSession(id, AAL2, []Membership{m1})
	_ = active.SetAuthContext(at, []FactorType{FactorPassword})
	tooLong := OIDCClaimsInput{Issuer: "iss", Audience: "aud", Subject: "sub", Session: &active, IssuedAt: at, AccessTTL: time.Hour}
	if _, err := BuildOIDCClaims(tooLong); !errors.Is(err, ErrInvalidClaims) {
		t.Fatalf("TTL longo demais deveria ser recusado: %v", err)
	}
}

// pcid: opaco, único, e propagado ao token; o MESMO valor vai ao contexto de
// auditoria (cenário "Linha do tempo unificada").
func TestPCIDGenerationAndPropagation(t *testing.T) {
	a, err := NewPCID()
	if err != nil {
		t.Fatalf("NewPCID: %v", err)
	}
	b, _ := NewPCID()
	if a == b {
		t.Fatalf("pcids deveriam ser únicos")
	}
	if len(a) < 10 || a[:5] != "pcid_" {
		t.Fatalf("pcid deveria ser opaco com prefixo pcid_: %q", a)
	}

	// Propagação: o builder carrega o pcid no token; o mesmo valor iria ao
	// AuditContext.PrivilegedCorrelationID (mesma string, unindo as trilhas).
	id, org := uuid.New(), uuid.New()
	m, _ := NewMembership(id, org)
	s, _ := NewAuthSession(id, AAL3, []Membership{m})
	at := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	_ = s.SetAuthContext(at, []FactorType{FactorWebAuthn})

	claims, err := BuildOIDCClaims(OIDCClaimsInput{
		Issuer: "iss", Audience: "warpgate", Subject: "sub", Session: &s,
		IssuedAt: at, AccessTTL: 10 * time.Minute, PCID: a,
	})
	if err != nil {
		t.Fatalf("BuildOIDCClaims: %v", err)
	}
	if claims.PCID != a {
		t.Fatalf("o token deveria carregar o pcid: %q != %q", claims.PCID, a)
	}
	ac := AuditContext{PrivilegedCorrelationID: a}
	if ac.PrivilegedCorrelationID != claims.PCID {
		t.Fatalf("o pcid do token e o da auditoria deveriam ser o MESMO valor")
	}
}

// Delegação: o token carrega act (ator real, do pacote 004) e grant_ref quando
// emitido sob concessão (T-004).
func TestOIDCClaimsActAndGrantRef(t *testing.T) {
	id, org := uuid.New(), uuid.New()
	m, _ := NewMembership(id, org)
	s, _ := NewAuthSession(id, AAL3, []Membership{m})
	at := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	_ = s.SetAuthContext(at, []FactorType{FactorWebAuthn})

	// act vem de uma delegação (pacote 004).
	nb := at
	d, err := NewDelegation(org, uuid.New(), "sub-admin", uuid.New(), "sub-target", IdentityHuman, nb, nb.Add(time.Hour))
	if err != nil {
		t.Fatalf("NewDelegation: %v", err)
	}
	_ = d.Consent()
	dclaims, err := d.TokenClaims(nb.Add(time.Minute))
	if err != nil {
		t.Fatalf("Delegation.TokenClaims: %v", err)
	}

	grantID := uuid.New()
	claims, err := BuildOIDCClaims(OIDCClaimsInput{
		Issuer: "iss", Audience: "warpgate", Subject: "sub-target", Session: &s,
		IssuedAt: at, AccessTTL: 10 * time.Minute,
		Act: &dclaims.Act, GrantRef: grantID.String(),
	})
	if err != nil {
		t.Fatalf("BuildOIDCClaims: %v", err)
	}
	if claims.Act == nil || claims.Act.Sub != "sub-admin" {
		t.Fatalf("act deveria nomear o ator real: %+v", claims.Act)
	}
	if claims.GrantRef != grantID.String() {
		t.Fatalf("grant_ref = %q, quero %q", claims.GrantRef, grantID.String())
	}
}

// Um act sem sub (delegação quebrada) não é montado.
func TestOIDCClaimsRejectsActWithoutSub(t *testing.T) {
	id, org := uuid.New(), uuid.New()
	m, _ := NewMembership(id, org)
	s, _ := NewAuthSession(id, AAL2, []Membership{m})
	at := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	_ = s.SetAuthContext(at, []FactorType{FactorPassword})

	if _, err := BuildOIDCClaims(OIDCClaimsInput{
		Issuer: "iss", Audience: "aud", Subject: "sub", Session: &s,
		IssuedAt: at, AccessTTL: 10 * time.Minute, Act: &ActClaim{},
	}); !errors.Is(err, ErrInvalidClaims) {
		t.Fatalf("act sem sub deveria ser recusado: %v", err)
	}
}
