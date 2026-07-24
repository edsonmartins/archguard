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

## Gate de verificação
Testes declarativos verdes; nenhuma decisão duplicada entre os dois planos; fail-closed
comprovado; replay reconstrói o store de forma idêntica.
