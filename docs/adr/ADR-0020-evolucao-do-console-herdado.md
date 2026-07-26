# ADR-0020 — Evoluir o console herdado (antd) em vez do rewrite Archbase

- **Status:** Aceito (ratificado 2026-07-26)
- **Data:** 2026-07-26
- **Supersede:** ADR-0004 (marcado **Superado** nesta ratificação)
- **Invariantes tocados:** I-7.3, I-7.6 (preservados — ver Decisão)
- **Precedente:** o próprio piloto de produção do ArchGuard (2026-07-25/26)

## Contexto

O ADR-0004 (Aceito em 2026-07-19) decidiu **reescrever o console** em React 19 + Mantine v9 +
Archbase e **remover o console herdado** (CRA + antd). Desde então, três fatos mudaram o cálculo:

1. **Piloto de produção vivo no console herdado.** O ArchGuard foi implantado em
   `app.archguard.com.br` (perfil `production`, conforme, custódia OpenBao) servindo o console
   **herdado**. Há um produto rodando, não um protótipo.
2. **O console herdado foi substancialmente adaptado ao ArchGuard.** Rebrand completo (logos,
   textos "Casdoor"→"ArchGuard", pt-BR), tema verde ArchGuard, ícones migrados para Tabler
   (webfont), remoção do que é morto/fora de escopo (comércio, agentes/MCP), e a marca de
   custódia (INV-7). Ou seja: parte do custo que o ADR-0004 atribuía ao herdado ("modelo mental
   errado", "stack/identidade") já foi mitigada de forma pragmática.
3. **Custo/tempo do greenfield.** O próprio ADR-0004 estima **2–3 meses de squad dedicada** para
   o escopo v1 — investimento difícil de justificar no estágio atual (validação de piloto, time
   enxuto), enquanto há um console utilizável **hoje**.

O **fator de viabilidade** do ADR-0004 continua valendo — o console consome **apenas a API REST
pública**, não há API privada de UI. Mas ele **corta para os dois lados**: se só há API pública,
**evoluir** o console herdado (adicionar as telas de PAM contra o mesmo `/api/v1` do plano de
controle, pacote 011) é um recorte **igualmente limpo** e muito mais barato agora.

## Decisão

**Evoluir o console herdado incrementalmente**, mantendo React (CRA) + antd (com o tema verde,
Tabler e demais adaptações já feitas), e **construir nele as capacidades de PAM que faltam**,
consumindo **exclusivamente** a API pública versionada `/api/v1` (pacote 011). O **rewrite
greenfield em Archbase fica adiado** (pode ser reavaliado pós-piloto, ou quando a convergência
de portfólio com o ArchGate for priorizada).

**Diretrizes preservadas do ADR-0004 (invariantes, independentes de stack):**
- **Nenhum endpoint "só para a UI"** — se o console precisa, é API pública, versionada e
  documentada (I-7.6).
- **Nenhuma regra de autorização no frontend** — esconder botão não é controle de acesso.
- **Sem acesso direto ao banco** pelo console.
- **Contexto de tenant sempre visível** com distinção inequívoca; troca reemite token.
- **Step-up transparente** com retomada da operação.
- **i18n pt-BR primário**, en-US secundário.
- **Acessibilidade por teclado** nos fluxos privilegiados.

O escopo funcional-alvo (v1) é o mesmo do ADR-0004 — organizações/memberships, usuários/grupos,
aplicações e clientes OIDC/SAML, provedores e sincronismos, MFA, papéis/permissões,
**visualizador de trilha com verificação de cadeia**, **break-glass**, **revisão de acesso**,
**saúde dos subsistemas** e **rotação de chaves** — porém **implementado no stack herdado**.

## Alternativas consideradas (agora)

| Opção | Por que não (agora) |
|---|---|
| Manter o rumo greenfield do ADR-0004 | 2–3 meses de squad; descarta trabalho já feito no herdado; piloto ficaria sem evolução de PAM no intervalo |
| Micro-frontend (telas novas em Mantine embutidas no herdado) | Duas stacks convivendo aumenta complexidade sem resolver o custo; adia decisão sem economizar |
| Congelar o console e só usar API | Operador de PAM precisa das telas de break-glass/revisão/auditoria; a API sozinha não é produto |

## Consequências

### Positivas
- Reaproveita o console já rebrandizado/tematizado; **mantém o piloto evoluindo**.
- Telas de PAM entregues **muito mais rápido** e mais barato do que o rewrite.
- Backend continua sendo contrato real (`/api/v1`); nenhuma regressão de invariante.

### Negativas (aceitas conscientemente)
- **Stack fora do padrão da casa** (CRA + antd em vez de Mantine/Archbase) — dívida assumida.
- **Manutenção em duas stacks** se/quando outros produtos exigirem componentes compartilhados.
- Se uma convergência futura de portfólio mandatar Archbase, **este trabalho é descartável** —
  aceito como custo de oportunidade de ter produto agora.
- Acoplamento ao ciclo de frontend do upstream em rebases — mitigado pela disciplina de fork
  (cherry-pick seletivo, `docs/upstream/DIVERGENCE.md`).

## Impacto no corpus de governança
- **ADR-0004** → passa a **Superado por ADR-0020** (na ratificação).
- **RFC-0005** (arquitetura frontend Mantine/Archbase) → **revisado/diferido**: marcar como
  referência da opção greenfield adiada; a arquitetura vigente é a do console herdado evoluído.
- **Pacote 008** (`openspec/changes/008-admin-console`) → **re-escopado** (proposal/design/
  tasks/spec) para evolução incremental do herdado.
- **ADR-0015** (rebranding/redução de escopo) → sem conflito; reforçado pelo trabalho já feito.

## Verificação
Os mesmos gates comportamentais do ADR-0004, agora sobre o console herdado:
- Build/lint falha se um endpoint for criado **só para a UI** (I-7.6) — o console usa `/api/v1`.
- Nenhuma regra de autorização no frontend (revisão de PR).
- Testes de fumaça dos fluxos privilegiados (break-glass, revisão de acesso, verificação de
  trilha) verdes.
