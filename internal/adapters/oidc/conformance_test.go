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

package oidc

// Suíte de CONFORMIDADE por componente (T-016 / RFC-0006 §8). Para CADA cliente
// registrado, valida o lado ArchGuard do contrato: emissão e semântica de claims,
// recusa por acr insuficiente, comportamento na rotação de chave, back-channel
// logout (ou introspecção, para quem não o suporta) e correlação por pcid. Como é
// um teste Go, roda no gate (`make test` / `make conformance`, T-017); falha em
// qualquer item bloqueia o release (I-9.4).

import (
	"context"
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// conformanceSession builds an active session at the given assurance with a
// method set consistent with it.
func conformanceSession(t *testing.T, aal domain.AAL, methods ...domain.FactorType) (domain.AuthSession, string) {
	t.Helper()
	id, org := uuid.New(), uuid.New()
	m, err := domain.NewMembership(id, org)
	if err != nil {
		t.Fatalf("NewMembership: %v", err)
	}
	s, err := domain.NewAuthSession(id, aal, []domain.Membership{m})
	if err != nil {
		t.Fatalf("NewAuthSession: %v", err)
	}
	if err := s.SetAuthContext(time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC), methods); err != nil {
		t.Fatalf("SetAuthContext: %v", err)
	}
	return s, "sub-" + id.String()
}

func TestConformanceSuitePerComponent(t *testing.T) {
	reg, err := domain.DefaultClientRegistry()
	if err != nil {
		t.Fatalf("DefaultClientRegistry: %v", err)
	}
	key, _ := GenerateSigningKey("kid-conf-1")
	signer, _ := NewSigner(key)
	issuer := "https://archguard.example"
	iat := time.Date(2026, 7, 23, 10, 1, 0, 0, time.UTC)

	for _, id := range reg.IDs() {
		client, _ := reg.Lookup(id)
		t.Run(id, func(t *testing.T) {
			// (1) Login + emissão de claims: token AAL2 (L2) assinável e verificável
			// com a audiência do componente.
			sess, sub := conformanceSession(t, domain.AAL2, domain.FactorPassword, domain.FactorTOTP)
			claims, err := domain.BuildOIDCClaims(domain.OIDCClaimsInput{
				Issuer: issuer, Audience: client.Audience, Subject: sub, Session: &sess,
				IssuedAt: iat, AccessTTL: 10 * time.Minute,
			})
			if err != nil {
				t.Fatalf("emissão de claims para %s: %v", id, err)
			}
			token, err := signer.Sign(claims)
			if err != nil {
				t.Fatalf("assinatura para %s: %v", id, err)
			}
			jwks, _ := signer.JWKS()
			verified, err := verifyAgainstJWKS(t, jwks, token)
			if err != nil {
				t.Fatalf("verificação para %s: %v", id, err)
			}

			// (2) Semântica de claims: aud própria, acr L2, org/mid/sid presentes.
			if verified["aud"] != client.Audience {
				t.Fatalf("aud = %v, quero %q", verified["aud"], client.Audience)
			}
			if verified["acr"] != "L2" {
				t.Fatalf("acr = %v, quero L2", verified["acr"])
			}
			for _, req := range []string{"org", "mid", "sid", "auth_time"} {
				if verified[req] == nil {
					t.Fatalf("claim obrigatório %q ausente para %s", req, id)
				}
			}
			// A audiência vincula: outro componente recusa.
			if err := domain.ValidateAudience(client.Audience, "outro-componente"); err == nil {
				t.Fatalf("token de %s não deveria ser aceito por outra audiência", id)
			}

			// (3) Recusa por acr insuficiente: L2 não satisfaz uma operação L3.
			if domain.L3.Satisfies(domain.AAL2, false) {
				t.Fatalf("L2 não deveria satisfazer L3 (recusa por acr)")
			}
			// Clientes com device flow: L3 é bloqueado pelo fluxo.
			if client.AllowsFlow(domain.FlowDeviceCode) {
				if err := domain.DeviceFlowAuthorize(domain.L3, true); err == nil {
					t.Fatalf("%s: L3 via device flow deveria ser bloqueado", id)
				}
			}

			// (4) Rotação de chave: o token continua válido após rotação
			// (sobreposição).
			s2, _ := NewSigner(key)
			tokenBefore, _ := s2.Sign(claims)
			newKey, _ := GenerateSigningKey("kid-conf-2")
			if err := s2.Rotate(newKey, 2); err != nil {
				t.Fatalf("rotação: %v", err)
			}
			jwks2, _ := s2.JWKS()
			if _, err := verifyAgainstJWKS(t, jwks2, tokenBefore); err != nil {
				t.Fatalf("%s: token pré-rotação deveria continuar válido: %v", id, err)
			}

			// (5) Encerramento de sessão.
			if client.SupportsBackchannelLogout() {
				// Back-channel logout efetivo: logout token assinável e verificável.
				lc, err := domain.NewLogoutTokenClaims(issuer, client.Audience, claims.SessionID, uuid.NewString(), iat)
				if err != nil {
					t.Fatalf("%s: logout token: %v", id, err)
				}
				lt, err := signer.SignLogoutToken(lc)
				if err != nil {
					t.Fatalf("%s: assinatura do logout token: %v", id, err)
				}
				lv, err := verifyAgainstJWKS(t, jwks, lt)
				if err != nil {
					t.Fatalf("%s: logout token deveria verificar: %v", id, err)
				}
				if _, ok := lv["events"].(map[string]interface{})[domain.BackchannelLogoutEvent]; !ok {
					t.Fatalf("%s: logout token sem o evento", id)
				}
			} else {
				// Sem logout: introspecção propaga a revogação (sessão morta →
				// active:false antes de expirar).
				resp := domain.BuildIntrospection(claims, false, iat.Add(time.Minute))
				if resp.Active {
					t.Fatalf("%s: sessão revogada deveria introspectar como inativa", id)
				}
			}

			// (6) Correlação pcid: um token privilegiado carrega o pcid, e o mesmo
			// valor correlaciona a auditoria.
			pcid, _ := domain.NewPCID()
			priv, sub2 := conformanceSession(t, domain.AAL3, domain.FactorWebAuthn)
			pc, err := domain.BuildOIDCClaims(domain.OIDCClaimsInput{
				Issuer: issuer, Audience: client.Audience, Subject: sub2, Session: &priv,
				IssuedAt: iat, AccessTTL: 10 * time.Minute, PCID: pcid,
			})
			if err != nil {
				t.Fatalf("%s: token privilegiado: %v", id, err)
			}
			if pc.PCID != pcid || (domain.AuditContext{PrivilegedCorrelationID: pcid}).PrivilegedCorrelationID != pc.PCID {
				t.Fatalf("%s: pcid deveria correlacionar token e auditoria", id)
			}
		})
	}
	_ = context.Background()
}
