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

package boot

import (
	"context"

	"github.com/casdoor/casdoor/internal/adapters/globalaccess"
	"github.com/casdoor/casdoor/internal/adapters/postgres"
	"github.com/casdoor/casdoor/internal/domain"
	apihttp "github.com/casdoor/casdoor/internal/http"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// tenantSwitch compõe a troca de tenant da sessão (pacote 008, T-004): resolve o
// membership ATIVO do chamador no tenant de destino e delega ao
// postgres.TenantSwitcher, que em UMA transação identity-scoped aplica a política
// do destino (step-up denial quando mais restritiva), move o contexto, incrementa a
// TokenGeneration (invalida o token anterior) e enfileira o evento de auditoria da
// troca (atômico — I-5.4). O repositório é identity-scoped, construído por request
// com a identidade da sessão (RLS por identidade).
type tenantSwitch struct {
	pool   *pgxpool.Pool
	policy domain.TenantAuthPolicy
	reader *postgres.MembershipReader
}

// newTenantSwitch compõe o switch sobre o pool de runtime, a política real de MFA
// por org (OrgPolicyAuthority, pacote 005) e o leitor de memberships do chamador
// (leitura cross-tenant autorizada/auditada — adapters provisórios de dev aqui, os
// duráveis vêm com o global-access do devops).
func newTenantSwitch(f *Factory) *tenantSwitch {
	global := postgres.NewGlobalRepository(f.Pool(), globalaccess.NewProfileAuthorizer(), globalaccess.NewMemoryAuditor())
	return &tenantSwitch{
		pool:   f.Pool(),
		policy: postgres.NewOrgPolicyAuthority(f.Pool()),
		reader: postgres.NewMembershipReader(global),
	}
}

// Switch implementa apihttp.TenantSwitcher. Resolve o membership de destino entre os
// do próprio chamador (nunca aceita um membership do request — I-4.1/INV-1) e delega
// ao switcher identity-scoped. Um destino sem membership ATIVO do chamador é negado
// (apihttp.ErrDestNotMember → 403).
func (t *tenantSwitch) Switch(ctx context.Context, session *domain.AuthSession, targetOrg uuid.UUID) (*domain.AuthSession, error) {
	memberships, err := t.reader.ListByIdentity(ctx, session.IdentityID)
	if err != nil {
		return nil, err
	}
	var dest domain.Membership
	found := false
	for _, m := range memberships {
		if m.OrganizationID == targetOrg && m.Status == domain.MembershipActive {
			dest = m
			found = true
			break
		}
	}
	if !found {
		return nil, apihttp.ErrDestNotMember
	}

	scope, err := domain.NewIdentityScope(session.IdentityID)
	if err != nil {
		return nil, err
	}
	switcher := postgres.NewTenantSwitcher(postgres.NewIdentityRepository(t.pool, scope), t.policy)
	next, err := switcher.Switch(ctx, session.ID, dest)
	if err != nil {
		return nil, err
	}
	return &next, nil
}
