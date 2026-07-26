# Proposal — 008 · ArchGuard Console (evolução do console herdado)

> Base normativa: **ADR-0020** (ratificado 2026-07-26, supersede o ADR-0004). Re-escopo do 008:
> evoluir o console herdado em vez do rewrite greenfield (histórico anterior no git).

## Por quê

O ArchGuard já está em **piloto de produção** (`app.archguard.com.br`) servindo o console
**herdado** (CRA + antd), que foi **substancialmente adaptado** ao produto: rebrand completo,
tema verde ArchGuard, ícones Tabler, pt-BR, remoção do que é fora de escopo (comércio, agentes/
MCP) e marca de custódia. O rewrite greenfield do ADR-0004 custa 2–3 meses de squad — não se
justifica no estágio atual, enquanto há um console utilizável hoje. O fator que tornava o rewrite
"limpo" (o console consome **apenas a API pública**) também torna a **evolução** limpa: adicionar
as telas de PAM contra o mesmo `/api/v1` (pacote 011).

## O que muda

- **Evoluir o console herdado** em vez de reescrevê-lo. Mantém React (CRA) + antd já adaptados.
- Construir nele as **telas de PAM** que faltam: break-glass, revisão de acesso, timeline de
  auditoria com verificação de cadeia, concessões vigentes, saúde dos subsistemas, rotação de
  chaves — além de seletor de tenant permanente e step-up transparente.
- Camada de API do console apontando **exclusivamente** para o `/api/v1` público (pacote 011),
  com teste de contrato no CI para detectar *drift*.
- Auditar/adaptar as telas herdadas ao modelo mental de PAM.

## O que não muda

Nenhuma regra de autorização migra para o frontend. Nenhum endpoint "só para a UI" — se o console
precisa, é API pública versionada (I-7.6). Sem acesso direto ao banco. Tenant sempre visível,
step-up transparente, pt-BR primário, acessibilidade por teclado nos fluxos privilegiados.

## O que fica adiado

O rewrite em React 19 + Mantine v9 + Archbase (ADR-0004/RFC-0005) fica **diferido** — reavaliável
pós-piloto ou quando a convergência de portfólio com o ArchGate for priorizada. Este re-escopo
assume conscientemente a dívida de manter o stack fora do padrão da casa (ADR-0020).

## Impacto

- **Depende de:** 002, 003, 004, 005, 007 (capacidades) + 011 (API `/api/v1`) + **ratificação do
  ADR-0020**.
- **Risco:** lacunas do `/api/v1` viram trabalho de backend antes de virar tela; *drift*
  contrato↔console (mitigado por teste de contrato); dívida de stack (aceita no ADR-0020).
