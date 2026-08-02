# Roadmap de Endurecimento — de fundação sólida a produto sólido

> Orquestração das frentes que levam o ArchGuard de "fundação sólida" (o núcleo governado, validado
> em produção) a "produto de produção endurecido". **Não** é um pacote OpenSpec novo: parte do
> trabalho já é código pronto (pacote 010) que precisa ser **ativado**, parte é **ação de ops/devops**,
> e parte tem design fechado (ADR-0023). Este documento aponta para cada peça e marca o dono.
>
> Escala: esforço S/M/L · risco de execução · impacto no nível do produto. Status: 2026-08-02.
> Dono: **[repo]** código no repo principal · **[devops]** infra (`archguard-devops`) · **[operador]** ação humana.

## Descoberta que enquadra o plano

O pacote **010 (observabilidade + custódia + LGPD)** já tem em código a **custódia real com o cofre**
(`KeyCustodian` — 010/T-010 `[x]`), a **redação de telemetria** (T-004/005 `[x]`) e os **SLIs**
(T-006 `[x]`). O piloto roda em custódia **dev** por **configuração de deploy** (perfil), não por
ausência de código. Logo: o endurecimento é **ativação + ops + os itens ainda `[ ]` do 010** —
não spec nova.

## Fase 0 — Higiene e postura de segurança (dias)

| Item | Ação | Dono | Esf. | Risco | Impacto | Referência |
|---|---|---|---|---|---|---|
| 0.1 | ✅ **FEITO (2026-08-02)** — senha root da VPS rotacionada. *(Recomendado a seguir: SSH chave-only, `PasswordAuthentication no`.)* | operador | S | baixo | alto | `docs/produto/02` §5 |
| 0.2 | **Ativar o perfil conforme + custódia OpenBao no piloto** (sair do keystore dev) | devops | M | médio (unseal/migração) | alto | 010 (código pronto); `docs/produto/02` §3 |
| 0.3 | ✅ **FEITO (2026-08-02)** — ativos de teste e grupo `DBAs` removidos do piloto (built-in). | operador/repo | S | baixo | baixo | — |

## Fase 1 — Operabilidade (semanas) · **pacote 010** (itens `[ ]`)

| Item | Ação | Dono | Esf. | Risco | Impacto |
|---|---|---|---|---|---|
| 1.1 | **Tracing distribuído + logs com `trace_id`** (010/T-002, T-003) | repo | M | baixo | alto |
| 1.2 | **Dashboards + alertas** versionados (010/T-007, T-008); SLIs já definidos | devops | M | baixo | alto |
| 1.3 | **Backup/PITR do PostgreSQL + drill de restore** (WAL archiving) | devops | M | baixo | alto |
| 1.4 | **LGPD:** classificação obrigatória de campo pessoal + crypto-shredding (010/T-011+) | repo | M | médio | médio-alto |

## Fase 2 — Contrato de identidade (semanas) · **ADR-0023**

| Item | Ação | Dono | Esf. | Risco | Impacto |
|---|---|---|---|---|---|
| 2.1 | **Montar OIDC v1:** estender `KeyCustodian` (Encrypt/Decrypt), keyset cifrado, bridge Application→ClientRegistry, mount `/api/v1/oidc`, gate `make conformance`. Design pronto. | repo | **L** | médio-alto (IdP vivo) | alto |

## Fase 3 — Garantia, escala e superfície legada (semanas)

| Item | Ação | Dono | Esf. | Risco | Impacto |
|---|---|---|---|---|---|
| 3.1 | Fechar **fluxos L3 ponta a ponta em produção** (passkey admin, verificar cadeia, break-glass) | repo/operador | M | baixo | médio-alto |
| 3.2 | **Validação de carga/escala** (custo do reconciler, latência do PDP, projeção sob carga) | repo/devops | M | baixo | médio |
| 3.3 | **Auditoria de segurança da superfície legada** do Casdoor em uso + plano de encolhimento pós-2.1 | repo | L | médio | médio-alto |

## Frente transversal — Documentação de produto

**FEITA (2026-08-02):** `docs/produto/` — Visão+Modelo de Segurança, Guia do Operador, Referência de
Integração/API, Guia do Administrador. Manter atualizada conforme as fases avançam (a honestidade de
maturidade depende de refletir o estado real).

## Fase 4 — Resiliência (contínuo)

HA (core + Postgres + OpenBao), TLS ponta a ponta, DR drills, e trocar o auto-update irrestrito de
`:latest` por **releases controlados** (tag/pin + promoção deliberada). Dono: devops.

## Sequência recomendada

**0 → 1 → 2 → 3**, com **0.1 (senha)** e o restante da Fase 0 começando **já**. Racional: tapar
exposição e ativar custódia real primeiro (barato, alto risco se ignorado); enxergar (observabilidade)
antes de mexer mais fundo; então montar o contrato de identidade **com telemetria no ar** para validar
a migração cliente a cliente; por fim garantir/escalar/auditar o legado. Resiliência acompanha o
crescimento.

## O que NÃO fazer

- Não abrir pacote OpenSpec novo para o que o **010 já especifica** — executar o 010.
- Não montar o OIDC v1 sem o design do **ADR-0023** (keyset cifrado por INV-7; coexistência).
- Não apontar consumidor para o issuer v1 antes do `make conformance` verde e de um fluxo real validado.
