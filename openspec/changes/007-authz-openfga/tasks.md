# Tasks — 007 · Autorização granular

- [x] **T-001** Definir a interface `PolicyDecisionPoint` no domínio (sem tipos de SDK).
- [x] **T-002** Escrever o modelo de autorização (tipos, relações, herança, condições).
- [x] **T-003** Implementar qualificação de objetos por tenant no identificador.
- [x] **T-004** Modelar `asset` e `asset_group` com hierarquia no ArchGuard.
      (Domínio puro; persistência/importação diferidas ao M4 — questões abertas RFC-0004 §9.)
- [x] **T-005** Implementar outbox transacional para mutações relevantes.
- [x] **T-006** Implementar publisher idempotente de tuplas.
- [x] **T-007** Implementar projeção de memberships, grupos e concessões em tuplas.
- [x] **T-008** Implementar reconciliação periódica com política assimétrica (restringe:
      automático; amplia: revisão humana).
- [x] **T-009** Implementar bootstrap/replay completo do store a partir do banco.
- [x] **T-010** Integrar decisão de abertura de sessão privilegiada (sem cache).
- [x] **T-011** Implementar cache curto apenas para listagens.
- [x] **T-012** Anexar justificativa da decisão ao evento de auditoria.
- [x] **T-013** Implementar fail-closed com distinção entre `denied` e `error`.
- [x] **T-014** Implementar consulta reversa para revisão de acesso (`listObjects`).
- [x] **T-015** Escrever testes declarativos do modelo (permitido/negado, herança, expiração).
- [x] **T-016** Teste de travessia: nenhuma relação concede acesso a objeto de outro tenant.
- [x] **T-017** Teste de reconciliação com divergência injetada.
- [x] **T-018** Teste: PDP indisponível ⇒ AuthN funciona, decisões privilegiadas negadas.
- [x] **T-019** Métricas de latência de decisão e de divergência de reconciliação.
- [x] **T-020** Documentar a fronteira Casbin × OpenFGA e o checklist de PR.

## GlobalAuthorizer real cross-tenant (ADR-0022) — o gate de login em perfil conforme
Contexto: o `PolicyDecisionPoint` granular (recursos) está completo (`PostgresPDP`). O que faltava
é o **`GlobalAuthorizer`** cross-tenant real — porta DISTINTA que o login usa para ler os próprios
memberships. Em perfil conforme só havia o `ProfileAuthorizer` provisional (nega fora de dev),
travando a sessão do `/api/v1` no piloto (viola I-1.3). NÃO exige OpenFGA (I-1.3 proíbe dep externa
no login) — é avaliador Go. Diagnóstico completo e desenho em ADR-0022.
- [x] **T-021** Fase 1 — domínio: `GlobalAccessScope` (`self`/`cross-tenant`, default restrito) no
      `domain.GlobalAccess`; os 2 call-sites reais de `WithGlobalTx` (login, seletor de tenant)
      declaram `ScopeSelf`; testes de domínio. Commit `c4931d6c`. ADR-0022 aceito.
- [ ] **T-022** Fase 2 — auditor durável de acesso GLOBAL. **Decisão de design pendente:** a trilha
      003 é POR-TENANT (`AuditEventInput` exige `OrganizationID`); um acesso global/self não tem
      tenant. Definir o destino: trilha append-only GLOBAL dedicada (nova migração + adapter
      `AccessAuditor`) vs. redefinição de semântica. Hoje só há o `MemoryAuditor` (dev). Substituir
      o `MemoryAuditor` pelo durável em perfil conforme.
- [ ] **T-023** Fase 2 — `GlobalAuthorizer` real (avaliador Go): `self` permitido em qualquer
      perfil; `cross-tenant` fail-closed em conforme (INV-6) até política real; `dev` permite ambos.
      Testes (self permitido, cross negado em conforme, fail-closed).
- [ ] **T-024** Fase 3 — boot liga o real + auditor durável por perfil nos 4 sites de
      `NewGlobalRepository` (bridge/pipeline/mounts/tenant_switch), centralizado numa factory.
      Provisional+memory permanece só em dev/teste. Deploy no piloto → login estabelece a sessão
      do `/api/v1`; validar (`/api/v1/session` 200 após login).
- [ ] **T-025** Fase 4 — invariante: perfil conforme nunca fica fail-open no acesso global; `self`
      permitido; `cross-tenant` amplo negado-e-auditado.

## Gate de verificação
Testes declarativos verdes; nenhuma decisão duplicada entre os dois planos; fail-closed
comprovado; replay reconstrói o store de forma idêntica.
