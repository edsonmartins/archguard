# Tasks — 011 · Control Plane API

## Fundação do composition root

- [x] **T-001** Pool pgx de runtime no boot (`postgres.NewPool` após `RunMigrations`), com posse
  e `Close` no shutdown.
- [ ] **T-002** Factory de adapters por perfil de implantação: dev = local/provisional; conforme
  exige backend real e **recusa servir** a capacidade se indisponível (INV-6/INV-7).
- [ ] **T-003** Controller-ponte Beego→net/http montando `/api/v1/*` pelo padrão SCIM (auth +
  trim de prefixo + `ServeHTTP`).
- [ ] **T-004** Seam `LegacyBinding` (adapter Beego lê identidade+sessão da sessão do framework)
  + fiação de `SessionResolver`/`OrgResolver` reais e do `AssuranceMiddleware` no pipeline.

## Montar o que já existe

- [ ] **T-005** Montar audit-verify (`AuditVerifier` → handler) sob `/api/v1`, classificado L3.
- [ ] **T-006** Compor as portas OIDC (`AuthCodeIssuer`/`AuthCodeGrant`/`RefreshGrant`/
  `EndSession` dos stores + `oidc.Signer`) e montar `OIDCServer.Handler()`.
- [ ] **T-007** Montar SCIM Users/Groups (`DirectoryProvisioner` → handlers) sob a API versionada.

## API voltada ao console (handlers finos sobre stores existentes)

- [ ] **T-008** API de identidades: usuários, memberships, grupos, contas de serviço (→ 008 T-008).
- [ ] **T-009** API de organização: configuração, domínios, políticas (MFA/break-glass/retenção)
  (→ 008 T-009).
- [ ] **T-010** API de acesso privilegiado: ativos/hierarquia e concessões vigentes (listar +
  revogar) (→ 008 T-012/T-013).
- [ ] **T-011** API de break-glass: solicitação, fila de aprovação, revogação (→ 008 T-014/T-015).
- [ ] **T-012** API de auditoria: timeline com filtros + correlação por `pcid` + verificação de
  integridade + exportação assinada (→ 008 T-016..T-019).
- [ ] **T-013** API de revisão de acesso: acesso efetivo do PDP com origem + decisões em lote
  auditadas (→ 008 T-020).
- [ ] **T-014** API de saúde dos subsistemas (PDP, cofre, auditoria) (→ 008 T-021).
- [ ] **T-015** API de chaves e rotação, classificada L3 (→ 008 T-022).

## Contrato e gate

- [ ] **T-016** Contrato OpenAPI 3 do `/api/v1` (fonte da verdade) + gate de CI: handler montado
  sem entrada no contrato falha o build (base do 008 T-002).
- [ ] **T-017** Suíte de invariantes estendida à camada montada: endpoint sem tenant nega
  (INV-5), fail-closed em dependência crítica (INV-6, incl. teste dedicado que falta), garantia
  por operação (INV-8).
- [ ] **T-018** Atualizar `docs/DEVOPS-HANDOFF.md` e `docs/upstream/DIVERGENCE.md`: composition
  root migrou para o repo principal; devops mantém só infra/endpoint.

## Gate de verificação
`make gate` verde; cada endpoint montado alcançável (não órfão) e presente no OpenAPI; cenários
WHEN/THEN da spec com teste executando de verdade; nenhuma regra de domínio nova introduzida.
