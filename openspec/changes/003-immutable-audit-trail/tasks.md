# Tasks — 003 · Trilha de auditoria imutável

- [x] **T-001** Definir esquema do evento com `schema_version` e catálogo de ações canônicas.
      *(Domínio puro `internal/domain/audit_event.go` (INV-3). Decisões do arquiteto: catálogo
      FECHADO e separação conteúdo/cadeia. `AuditSchemaVersion=1` (parte do conteúdo canônico).
      `Action` = verbo canônico com catálogo fechado (`actionCatalog`): `NewAuditEvent` recusa
      ação não registrada (`ErrUnknownAction`) — auditar verbo desconhecido é pior que recusar.
      Cada ação carrega seu nível `AssuranceLevel` (L1/L2/L3, ADR-0010/INV-8): privileged/
      breakglass/key.rotate/audit.* = L3. `AuditEvent` = SÓ o conteúdo canonicalizável (campos
      normativos do RFC-0003 §2: schema_version, event_id UUIDv7, organization_id, action, actor,
      target, outcome, reason, context) — SEM seq/prev_hash/hash. `SealedEvent{Event, Seq,
      PrevHash, Hash}` carrega a cadeia, atribuída na escrita (T-003/T-004): impossível, por
      construção, incluir o hash no próprio hash. `Outcome` reúsa o primitivo Allowed/Denied/
      Failed; `SerializedOutcome` mapeia para success|denied|error (RFC §2), preservando a
      distinção INV-6. `AuditActor.IdentitySubject` = pseudônimo opaco (nunca e-mail/nome);
      LGPD dos campos de origem (IP/UA) fica no COMMENT da tabela (T-005). Sem relógio no domínio
      (occurred_at carimbado na escrita). Testes cobrem catálogo fechado+níveis, sucesso/negado/
      erro, rejeições do construtor, composição do SealedEvent. Gate verde.)*
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
