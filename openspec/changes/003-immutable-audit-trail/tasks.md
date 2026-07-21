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
- [x] **T-002** Implementar canonicalização determinística + testes de vetor fixo. *(Domínio
      `internal/domain/audit_canonical.go`: `Canonical(AuditEvent) []byte` — a entrada exata da
      cadeia de hash (RFC-0003 §3). Decisão (§7, sem dependência nova): JSON canônico PRÓPRIO
      sobre o schema fechado — chaves ordenadas (encoding/json ordena `map[string]any`), HTML
      escaping OFF, strings normalizadas a **NFC** (`golang.org/x/text/unicode/norm`, já na
      árvore, BSD-3), `occurred_at` como microssegundos inteiros UTC (precisão do timestamptz,
      round-trip determinístico da linha), ponteiros nil (membership/session/act) OMITIDOS. Os
      campos de cadeia (seq/prev_hash/hash) são AUSENTES por construção (vivem no SealedEvent) —
      nunca vazam para o próprio hash. `occurred_at` adicionado ao AuditEvent (campo normativo,
      carimbado na escrita, precisa ser hasheado — adulterar tempo tem de ser detectável).
      **Testes de vetor fixo**: bytes canônicos exatos + SHA-256 golden de um evento totalmente
      especificado (drift silencioso quebra o build = invalida verificação histórica);
      determinismo, NFC (é composto == e+acento combinante), sensibilidade (mutar qualquer campo
      muda os bytes), truncamento a microssegundo. Gate verde; go.mod/sum intactos; baseline de
      licença inalterado.)*
- [x] **T-003** Implementar encadeamento por hash com `seq` por organização. *(Domínio
      `internal/domain/audit_chain.go` — a fórmula do RFC-0003 §3, sem material de chave (a
      assinatura Ed25519 é T-010). `GenesisHash(org, nonce)` = `H(org || genesis_nonce)` (nonce
      de 32 bytes por organização — dois tenants não compartilham cadeia); `SealEvent(event,
      prevHash, seq)` = canonicaliza e computa `hash = H(prev_hash || canonical)`, devolvendo o
      `SealedEvent`. Concatenação não-ambígua porque prev_hash é fixo em 32 bytes. O `seq` é
      atribuído pela camada de escrita (serialização por org, T-004); o domínio só exige seq≥1 e
      prev_hash de 32 bytes (ErrInvalidPrevHash/ErrInvalidSeq/ErrInvalidGenesisNonce).
      `VerifyLink` recomputa o elo e detecta adulteração de conteúdo OU de prev_hash (base do
      verificador completo, T-013). Vetores fixos: gênese e primeiro hash da cadeia pinados
      (drift quebra o build); testes de encadeamento de 2 elos, gênese distinta por nonce,
      detecção de adulteração. Gate verde.)*
- [x] **T-004** Implementar serialização de escrita por tenant (sem lacunas, sem corrida).
      *(`internal/adapters/postgres/audit_writer.go`: `AuditWriter.Append` numa transação
      (RFC-0002 §5) — trava a linha de `audit_chain_head` da org (`SELECT ... FOR UPDATE`; cria
      lazy na 1ª escrita da org com `genesis_nonce` aleatório e `head_hash=GenesisHash`,
      race-safe via `INSERT ON CONFLICT DO NOTHING`), lê prev_hash/last_seq, chama
      `domain.SealEvent` (seq=last_seq+1), insere o evento e avança o cabeçalho. `occurred_at`
      carimbado de um `Clock` injetável (determinismo em teste; prod=time.Now). Ator delegado
      como jsonb; amr como text[]. Verificado em PG15 real: append encadeia da gênese e o hash
      RECOMPUTA a partir das colunas (não há blob canônico); cadeias por org independentes
      (seq começa em 1, hashes distintos por nonce); **cenário "Gravações concorrentes": 25
      escritas simultâneas na mesma org → seq 1..25 sem lacuna/duplicata e cadeia verificável**
      (o FOR UPDATE serializa por tenant). Falha na escrita ⇒ nada persistido e cabeçalho
      intacto (base do fail-closed T-008). Gate verde.)*
- [x] **T-005** Criar tabela particionada por tempo e índices de consulta. *(Feita ANTES da
      T-004 por dependência — o writer precisa do esquema. Migration 0017: `audit_event` NOVA
      (pgx, distinta da `record` legada), **particionada por RANGE(occurred_at)** com partição
      `DEFAULT` (partições de intervalo + arquivamento = T-018); colunas do RFC-0003 §2 + seq/
      prev_hash/hash; PK `(organization_id, occurred_at, seq)` (inclui a chave de partição,
      exigência do Postgres); índices por ator, ação, org+seq (scan do verificador), trace_id e
      pcid. **Sem coluna canonical de propósito**: o verificador recomputa o canônico das colunas
      (adulterar coluna consultável quebra a verificação). `audit_chain_head(organization_id PK,
      last_seq, head_hash, genesis_nonce)` — o cabeçalho de cadeia por org que a T-004 trava com
      FOR UPDATE. Classificação LGPD (COMMENT) em actor_subject/context_ip/context_user_agent
      (decisão do arquiteto: IP/UA como coluna classificada). Verificado em PG15 real: parent
      particionado, DEFAULT, índices e chain_head criados.)*
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
