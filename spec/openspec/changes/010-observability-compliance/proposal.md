# Proposal — 010 · Observabilidade, custódia de chaves e conformidade LGPD

## Por quê

O ArchGuard é dependência crítica de disponibilidade do ArchGate: sua degradação impede novos
acessos privilegiados. Precisa de observabilidade de primeira classe (ADR-0013). Ao mesmo
tempo, três pendências estruturais precisam ser fechadas antes do GA: custódia real de chaves
no cofre (ADR-0012), crypto-shredding para atender à eliminação LGPD sem quebrar a cadeia de
auditoria (ADR-0014) e a separação normativa entre telemetria e auditoria.

## O que muda

- Instrumentação OpenTelemetry (métricas, traces, logs) com exportação OTLP e **sem telemetria
  externa**.
- Regras de higiene: nenhum segredo, token ou dado pessoal em claro em qualquer sinal.
- SLIs, dashboards e alertas versionados como entregável do produto.
- Substituição da custódia provisória pela integração real com o cofre, incluindo rotação.
- Classificação LGPD obrigatória de campos pessoais em migrations (falha de CI se ausente).
- Crypto-shredding por chave de titular; retenção por arquivamento de partição selada.
- Exportação estruturada para atender direitos do titular, com escopo por tenant.

## Impacto

- **Depende de:** 001, 002, 003, 012 (interfaces introduzidas em 002/003).
- **Bloqueia:** GA.
- **Risco:** perda de chave de titular é eliminação irreversível; perda de chave de selagem
  destrói a verificabilidade histórica.
