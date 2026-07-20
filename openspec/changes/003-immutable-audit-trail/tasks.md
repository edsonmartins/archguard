# Tasks — 003 · Trilha de auditoria imutável

- [ ] **T-001** Definir esquema do evento com `schema_version` e catálogo de ações canônicas.
- [ ] **T-002** Implementar canonicalização determinística + testes de vetor fixo.
- [ ] **T-003** Implementar encadeamento por hash com `seq` por organização.
- [ ] **T-004** Implementar serialização de escrita por tenant (sem lacunas, sem corrida).
- [ ] **T-005** Criar tabela particionada por tempo e índices de consulta.
- [ ] **T-006** Configurar papel de banco sem `UPDATE`/`DELETE` na auditoria.
- [ ] **T-007** Criar triggers de bloqueio de mutação (defesa em profundidade).
- [ ] **T-008** Implementar `AuditSink` síncrono durável (modo fail-closed).
- [ ] **T-009** Implementar modo assíncrono com fila durável para eventos não privilegiados.
- [ ] **T-010** Integrar assinatura Ed25519 via cofre para selagem.
- [ ] **T-011** Implementar selagem por intervalo e por volume, com `key_id`.
- [ ] **T-012** Implementar exportação opcional de selos para destino WORM.
- [ ] **T-013** Implementar verificador (recomputação + assinaturas + lacunas).
- [ ] **T-014** Expor verificação como comando CLI e endpoint (operação L3).
- [ ] **T-015** Agendar verificação diária com alerta de severidade máxima.
- [ ] **T-016** Implementar exportação assinada por tenant (NDJSON + selos + chaves).
- [ ] **T-017** Instrumentar eventos de autenticação, autorização e mutação administrativa.
- [ ] **T-018** Implementar arquivamento de partição selada e restauração auditada.
- [ ] **T-019** Teste de adulteração: alterar, remover e reordenar eventos; verificar detecção.
- [ ] **T-020** Teste de fail-closed: indisponibilizar a auditoria e confirmar negação de
      operação privilegiada.
- [ ] **T-021** Teste de carga com medição de impacto na latência de login.

## Gate de verificação
Detecção correta dos três tipos de adulteração; fail-closed comprovado; verificação diária
operando; impacto de latência dentro do orçamento do RFC-0001.
