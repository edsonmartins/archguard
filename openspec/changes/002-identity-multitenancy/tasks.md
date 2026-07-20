# Tasks — 002 · Identidade global e multi-tenancy B2B

- [x] **T-001** Modelar e migrar `identity` (UUIDv7, `sub` opaco, status, campos cifrados).
      *(Domínio `internal/domain/identity.go`: `Identity` com `IdentityType` (human|service) e
      `IdentityStatus` (active|suspended|deprovisioned); `NewIdentity` gera UUIDv7 (google/uuid) +
      subject opaco de 128 bits (crypto/rand, base64url, distinto do id — não vaza tempo);
      transições Suspend/Reactivate/Deprovision honram o terminal R5. Campos pessoais como
      ciphertext `[]byte` — domínio não vê PII. Migration `0002_create_identity.sql`: tabela
      CROSS-TENANT (R1, sem organization_id), `id uuid` sem default (UUIDv7 da app), CHECKs de
      type/status espelhando o domínio, `subject` UNIQUE, `email_hash` como coluna (índice único +
      login por hash ficam no T-003). Verificado em PostgreSQL 15 real: estrutura, idempotência,
      CHECKs e UNIQUE rejeitam inválidos, defaults. Teste de integração do migrator gated por
      `ARCHGUARD_TEST_DSN` (pula sem PG); domínio 100% unitário. Gate local verde.)*
- [x] **T-002** Modelar e migrar `membership` com unicidade `(identity_id, organization_id)`.
      *(Decisão do arquiteto sobre `organization_id`: `organization` ganha `id uuid` estável
      (migration `0003_organization_stable_id.sql`, DEFAULT gen_random_uuid + índice único;
      backfill por linha validado; struct XORM intocado — id lido via pgx, RFC §5). Domínio
      `internal/domain/membership.go`: `Membership` com `MembershipStatus`
      (invited|active|suspended|revoked) e máquina de estados invited→active→suspended⇄active,
      qualquer→revoked (terminal, R4); `NewMembership` (direto=active), `NewInvitedMembership`
      (invited + invitedBy); atributos de tenant como ciphertext `[]byte`. Migration
      `0004_create_membership.sql`: FKs para identity(id) e organization(id), status NOT NULL SEM
      default (força invited/active explícito), UNIQUE (identity_id, organization_id) = R3,
      índices por identity e por organization. Verificado em PG15 real: FKs, CHECK, NULL-sem-
      default e UNIQUE rejeitam inválidos; backfill de orgs pré-existentes com ids distintos.
      Gate local verde.)*
- [ ] **T-003** Introduzir `email_hash` (HMAC) com índice único e caminho de login por hash.
- [ ] **T-004** Introduzir interface `KeyCustodian` (implementação provisória marcada como
      não suportada em produção).
- [ ] **T-005** Migrar credenciais e fatores MFA para a identidade.
- [ ] **T-006** Repontar papéis e permissões para `membership_id`.
- [ ] **T-007** Backfill de `organization_id` nas tabelas de domínio.
- [ ] **T-008** Implementar repositório com contexto de tenant obrigatório (barreira 1).
- [ ] **T-009** Implementar `GlobalRepository` explícito, autorizado e auditado.
- [ ] **T-010** Habilitar RLS por tabela e configurar papel da aplicação sem `BYPASSRLS`.
- [ ] **T-011** Implementar contexto de tenant ativo na sessão.
- [ ] **T-012** Implementar troca de tenant com reemissão de token e auditoria.
- [ ] **T-013** Implementar fluxo de convite e vinculação de identidade existente.
- [ ] **T-014** Implementar revogação em cascata (identidade → memberships → sessões).
- [ ] **T-015** Ferramenta de inventário e deduplicação com relatório de conflito.
- [ ] **T-016** Rotina de fusão assistida (com aprovação humana obrigatória).
- [ ] **T-017** Testes de travessia entre tenants (barreira 1 e barreira 2 isoladamente).
- [ ] **T-018** Teste automatizado que rejeita query sem predicado de tenant.
- [ ] **T-019** Ensaio de migração em cópia de produção e relatório de resultado.
- [ ] **T-020** Atualizar `DIVERGENCE.md` com o escopo da divergência de modelo de dados.

## Gate de verificação
Testes de travessia verdes com RLS ligada **e** desligada (provando as duas barreiras);
ensaio de migração sem perda de fator MFA; nenhuma identidade humana duplicada no relatório
final.
