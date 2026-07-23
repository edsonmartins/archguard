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
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// T-019 — "logout no ArchGuard encerra sessões nos componentes": o logout revoga
// localmente e envia back-channel logout a cada componente REGISTRADO que tem
// logout (cenário "Logout no ArchGuard").
func TestAcceptanceLogoutEndsComponentSessions(t *testing.T) {
	reg, err := DefaultClientRegistry()
	if err != nil {
		t.Fatalf("DefaultClientRegistry: %v", err)
	}
	// Os componentes com back-channel logout.
	var clients []LogoutClient
	for _, id := range reg.IDs() {
		c, _ := reg.Lookup(id)
		if c.SupportsBackchannelLogout() {
			clients = append(clients, LogoutClient{Audience: c.Audience, Endpoint: c.BackchannelLogoutURI})
		}
	}
	if len(clients) == 0 {
		t.Fatalf("deveria haver componentes com back-channel logout (Warpgate, NetBird)")
	}

	rev := &fakeRevoker{}
	notif := &fakeLogoutNotifier{}
	p := NewLogoutPropagator("https://archguard.example", fakeLogoutSigner{}, notif, rev)

	if err := p.Logout(context.Background(), uuid.New(), "sid-1", clients, time.Now()); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if !rev.called {
		t.Fatalf("as sessões derivadas deveriam ter sido revogadas localmente")
	}
	if len(notif.sent) != len(clients) {
		t.Fatalf("o back-channel logout deveria ter ido a todos os componentes com suporte: %d/%d", len(notif.sent), len(clients))
	}
}

// T-020 — "correlação pcid reconstrói a linha do tempo ponta a ponta": o mesmo
// pcid está no token do componente E nos eventos de auditoria do ArchGuard,
// permitindo unir as duas trilhas (cenário "Linha do tempo unificada").
func TestAcceptancePCIDCorrelation(t *testing.T) {
	pcid, err := NewPCID()
	if err != nil {
		t.Fatalf("NewPCID: %v", err)
	}

	// Sessão privilegiada -> token carrega o pcid.
	id, org := uuid.New(), uuid.New()
	m, _ := NewMembership(id, org)
	s, _ := NewAuthSession(id, AAL3, []Membership{m})
	at := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	_ = s.SetAuthContext(at, []FactorType{FactorWebAuthn})
	token, err := BuildOIDCClaims(OIDCClaimsInput{
		Issuer: "iss", Audience: "warpgate", Subject: "sub", Session: &s,
		IssuedAt: at, AccessTTL: 10 * time.Minute, PCID: pcid,
	})
	if err != nil {
		t.Fatalf("BuildOIDCClaims: %v", err)
	}

	// Os eventos de auditoria do ArchGuard carregam o mesmo pcid.
	archguardEvent := AuditContext{PrivilegedCorrelationID: pcid}
	// O componente correlaciona os SEUS eventos pelo mesmo valor recebido no token.
	componentEventPCID := token.PCID

	if token.PCID != pcid {
		t.Fatalf("o token deveria carregar o pcid")
	}
	if archguardEvent.PrivilegedCorrelationID != componentEventPCID {
		t.Fatalf("o pcid do ArchGuard e o do componente deveriam ser o MESMO — não haveria como reconstruir a linha do tempo")
	}
}
