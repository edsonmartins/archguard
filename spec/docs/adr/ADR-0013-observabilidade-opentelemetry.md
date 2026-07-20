# ADR-0013 — Observabilidade: OpenTelemetry, VictoriaMetrics, Loki e Tempo

- **Status:** Aceito
- **Data:** 2026-07-19
- **Invariantes tocados:** I-7.5, I-3.2, I-5.1

## Contexto

O ArchGuard é dependência crítica de disponibilidade do ArchGate (ADR-0011): sua degradação
impede novos acessos privilegiados. Precisa, portanto, de observabilidade de primeira classe,
alinhada ao stack padrão da IntegrAllTech (VictoriaMetrics + Grafana + Loki + Tempo).

Há uma distinção que precisa ser normativa, sob pena de erro grave de arquitetura:
**telemetria não é auditoria**. Telemetria é amostrável, descartável e otimizada para
diagnóstico. Auditoria é durável, íntegra e legalmente relevante (ADR-0007).

## Decisão

**Instrumentar por OpenTelemetry (métricas, traces e logs), exportando via OTLP. Sem
telemetria externa (I-3.2).**

### Separação normativa

| Sinal | Destino | Propriedade |
|---|---|---|
| **Métricas** | VictoriaMetrics → Grafana | Agregado, sem dado pessoal |
| **Traces** | Tempo | Amostrado; sem credencial, sem token, sem PII |
| **Logs operacionais** | Loki | Diagnóstico; **não** é fonte da verdade |
| **Auditoria** | Trilha própria (ADR-0007) | Durável, imutável, verificável — **cópia** pode ir ao Loki para busca, nunca o original |

### Regras de higiene
- **Proibido** em qualquer sinal de telemetria: senha, token, cookie de sessão, segredo de
  client, código de MFA, material de chave, conteúdo de sessão privilegiada.
- Identificadores de usuário em telemetria são **pseudônimos estáveis**, não e-mails.
- `trace_id` correlaciona telemetria e evento de auditoria sem duplicar conteúdo sensível.

### Sinais mínimos (SLI)
Latência e taxa de erro de: autorização OIDC, emissão/renovação de token, validação de MFA,
gravação de auditoria, decisão do PDP (ADR-0005) e chamadas ao cofre (ADR-0012). Métricas
específicas de negócio: solicitações de break-glass, falhas de step-up, sessões privilegiadas
abertas, divergências de reconciliação do PDP e **falhas de verificação de cadeia de
auditoria** (alerta de severidade máxima).

### Alertas obrigatórios
- Falha de gravação de auditoria (implica negação de operação — I-5.4).
- Falha de verificação da cadeia ou de assinatura de selo.
- Indisponibilidade do cofre ou do PDP.
- Pico anômalo de falhas de autenticação por tenant.
- Break-glass solicitado (alerta informativo imediato, sempre).

## Consequências

- Overhead de instrumentação e custo de armazenamento de traces (mitigado por amostragem —
  nunca aplicada à auditoria).
- Coletor OTLP é opcional no deployment mínimo: sua ausência degrada diagnóstico, **jamais**
  auditoria ou autenticação (I-1.3).
- Dashboards e alertas são entregáveis versionados do produto, não configuração manual de
  cliente.
