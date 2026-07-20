# Design — 010 · Observabilidade e conformidade

Base normativa: ADR-0012, ADR-0013, ADR-0014, RFC-0003.

## Separação normativa

| Sinal | Destino | Propriedade |
|---|---|---|
| Métricas | VictoriaMetrics → Grafana | Agregado, sem dado pessoal |
| Traces | Tempo | Amostrado, sem segredo |
| Logs operacionais | Loki | Diagnóstico, **não** é fonte da verdade |
| Auditoria | Trilha própria | Durável, imutável, verificável |

Cópia da auditoria pode ir ao Loki para busca; o original nunca sai da trilha. `trace_id`
correlaciona sem duplicar conteúdo sensível. Identificadores de usuário em telemetria são
pseudônimos estáveis, nunca e-mails.

## SLIs e alertas

SLIs: latência e erro de autorização OIDC, emissão/renovação de token, validação de MFA,
gravação de auditoria, decisão do PDP e chamadas ao cofre.

Alertas obrigatórios: falha de gravação de auditoria; falha de verificação de cadeia ou selo
(**severidade máxima**); indisponibilidade de cofre ou PDP; pico anômalo de falhas de
autenticação por tenant; break-glass solicitado (informativo imediato, sempre).

## Custódia de chaves

Substituir a implementação provisória de `KeyCustodian` pela integração com o cofre:
- JWKS gerado e assinado no cofre (chave privada não deixa o cofre);
- chave de selagem Ed25519 no *transit engine*;
- segredos de client OAuth e credenciais de conector no cofre, com referência no banco;
- chaves por titular para crypto-shredding.

Rotação: JWKS com sobreposição maior que o TTL máximo de token; selagem com registro de
intervalo de validade por `key_id`. Toda rotação é operação L3 e evento de auditoria.

Modo degradado: cache curto de capacidade de assinatura; expirado, emissão degrada e L3 falha
fechado. O perfil de custódia local é marcado como **não suportado em produção** e reportado
por health check como instalação não conforme.

## LGPD

Classificação obrigatória de campos pessoais em metadados de migration (categoria, finalidade,
base legal, retenção). **Migration sem classificação é rejeitada no CI.**

Eliminação = destruição da chave do titular. Eventos permanecem com pseudônimo; a cadeia
permanece verificável. O ato de eliminação é, ele próprio, evento de auditoria e operação L3
com confirmação que descreve a irreversibilidade.

Retenção: expiração leva a arquivamento de partição selada; nunca deleção seletiva.
