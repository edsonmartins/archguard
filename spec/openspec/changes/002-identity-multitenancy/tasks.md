# Tasks — 002 · Identidade global e multi-tenancy B2B

- [ ] **T-001** Modelar e migrar `identity` (UUIDv7, `sub` opaco, status, campos cifrados).
- [ ] **T-002** Modelar e migrar `membership` com unicidade `(identity_id, organization_id)`.
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
