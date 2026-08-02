# Documentação de Produto — ArchGuard

Documentação voltada a **avaliadores, integradores, operadores e administradores** — distinta da
governança de desenvolvimento (ADR/RFC/OpenSpec em `docs/adr`, `docs/rfc`, `openspec/`).

Princípio: **honestidade de maturidade.** Onde uma garantia é projetada mas não está no ar, o
documento diz isso. A fonte da verdade do comportamento é o código; onde divergir, o código vence e o
documento deve ser corrigido.

| # | Documento | Para quem |
|---|---|---|
| 01 | [Visão e Modelo de Segurança](01-visao-e-modelo-de-seguranca.md) | Avaliadores, decisores — o que é, as garantias (invariantes), o estado de maturidade |
| 02 | [Guia do Operador (Runbooks)](02-guia-do-operador.md) | Operadores — deploy, unseal, migrations, rotação, backup/DR, incidentes |
| 03 | [Referência de Integração e API](03-referencia-de-integracao.md) | Integradores — federação OIDC (`docs/oidc/`) e Control Plane API `/api/v1` |
| 04 | [Guia do Administrador (Console)](04-guia-do-administrador.md) | Admins — tenants, usuários, ativos, acesso, break-glass, auditoria |

**Relacionados:** `docs/oidc/` (integração OIDC detalhada), `docs/RUNBOOK.md` (operação mínima),
`docs/upstream/DIVERGENCE.md` (o que divergiu do Casdoor), `CONSTITUTION.md` (os invariantes).
