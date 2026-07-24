# Tasks — 010 · Observabilidade e conformidade

- [ ] **T-001** Instrumentar métricas OpenTelemetry nos caminhos críticos.
- [ ] **T-002** Instrumentar tracing distribuído com propagação de contexto.
- [ ] **T-003** Padronizar logs estruturados com `trace_id`.
- [x] **T-004** Implementar filtro de redação (segredo, token, PII) em todos os sinais.
- [x] **T-005** Teste que falha se dado sensível aparecer em telemetria.
- [x] **T-006** Definir e implementar os SLIs do RFC-0001.
- [ ] **T-007** Versionar dashboards Grafana como artefato do produto.
- [ ] **T-008** Implementar os alertas obrigatórios do design.
- [ ] **T-009** Exportação de cópia da auditoria para Loki (marcada como cópia).
- [ ] **T-010** Integrar `KeyCustodian` real com o cofre (JWKS).
- [ ] **T-011** Integrar assinatura de selagem via transit engine.
- [ ] **T-012** Migrar segredos de client OAuth e credenciais de conector para o cofre.
- [ ] **T-013** Implementar rotação de JWKS com sobreposição.
- [ ] **T-014** Implementar rotação de chave de selagem com validade por `key_id`.
- [ ] **T-015** Implementar cache de assinatura e modo degradado com fail-closed em L3.
- [ ] **T-016** Health check que sinaliza custódia local como instalação não conforme.
- [x] **T-017** Implementar classificação LGPD obrigatória em migrations + gate de CI.
- [x] **T-018** Implementar chaves por titular e cifragem de campos pessoais.
- [x] **T-019** Implementar crypto-shredding com confirmação L3 e auditoria.
- [ ] **T-020** Implementar arquivamento de partição por retenção e restauração auditada.
- [x] **T-021** Implementar exportação estruturada para direitos do titular (escopo por tenant).
- [ ] **T-022** Runbook: DR do cofre, rotação, incidente e notificação (ANPD/titulares).
- [ ] **T-023** Teste: eliminação de titular mantém a cadeia de auditoria verificável.
- [ ] **T-024** Teste: exportação de titular não vaza dados de outra organização.
- [ ] **T-025** Documentação de apoio ao RIPD do cliente (papéis controlador/operador).

## Gate de verificação
Nenhum dado sensível em telemetria (teste automatizado); verificação da cadeia continua verde
após eliminação de titular; rotação de chaves sem indisponibilidade; runbooks ensaiados.
