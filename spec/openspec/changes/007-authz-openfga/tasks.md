# Tasks — 007 · Autorização granular

- [ ] **T-001** Definir a interface `PolicyDecisionPoint` no domínio (sem tipos de SDK).
- [ ] **T-002** Escrever o modelo de autorização (tipos, relações, herança, condições).
- [ ] **T-003** Implementar qualificação de objetos por tenant no identificador.
- [ ] **T-004** Modelar `asset` e `asset_group` com hierarquia no ArchGuard.
- [ ] **T-005** Implementar outbox transacional para mutações relevantes.
- [ ] **T-006** Implementar publisher idempotente de tuplas.
- [ ] **T-007** Implementar projeção de memberships, grupos e concessões em tuplas.
- [ ] **T-008** Implementar reconciliação periódica com política assimétrica (restringe:
      automático; amplia: revisão humana).
- [ ] **T-009** Implementar bootstrap/replay completo do store a partir do banco.
- [ ] **T-010** Integrar decisão de abertura de sessão privilegiada (sem cache).
- [ ] **T-011** Implementar cache curto apenas para listagens.
- [ ] **T-012** Anexar justificativa da decisão ao evento de auditoria.
- [ ] **T-013** Implementar fail-closed com distinção entre `denied` e `error`.
- [ ] **T-014** Implementar consulta reversa para revisão de acesso (`listObjects`).
- [ ] **T-015** Escrever testes declarativos do modelo (permitido/negado, herança, expiração).
- [ ] **T-016** Teste de travessia: nenhuma relação concede acesso a objeto de outro tenant.
- [ ] **T-017** Teste de reconciliação com divergência injetada.
- [ ] **T-018** Teste: PDP indisponível ⇒ AuthN funciona, decisões privilegiadas negadas.
- [ ] **T-019** Métricas de latência de decisão e de divergência de reconciliação.
- [ ] **T-020** Documentar a fronteira Casbin × OpenFGA e o checklist de PR.

## Gate de verificação
Testes declarativos verdes; nenhuma decisão duplicada entre os dois planos; fail-closed
comprovado; replay reconstrói o store de forma idêntica.
