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
- [x] **T-006** Configurar papel de banco sem `UPDATE`/`DELETE` na auditoria. *(Estende
      `deploy/postgres/roles.sql`: o bloco INV-2 passa a revogar `UPDATE, DELETE, TRUNCATE` de
      `archguard_app` em `audit_event` (append-only) — no PAI e em TODAS as partições (o
      privilégio de DML é checado na partição concreta; partições herdam o GRANT e precisam do
      REVOKE explícito, varridas via pg_inherits). `audit_chain_head` **fica de fora** de
      propósito: é ponteiro mutável que a escrita avança sob trava (não guarda evento). Como no
      pacote 001, a aplicação real do roles.sql é validada no smoke test de deploy (T-022 do 001,
      diferido por falta de Docker); a barreira executável e testável agora é a de triggers
      (T-007).)*
- [x] **T-007** Criar triggers de bloqueio de mutação (defesa em profundidade). *(Migration
      0018: função `audit_event_block_mutation` + triggers `BEFORE UPDATE/DELETE` (FOR EACH ROW,
      cascateiam para as partições no PG13+) e `BEFORE TRUNCATE` (FOR EACH STATEMENT) em
      `audit_event`, que abortam com `insufficient_privilege`. Complementa o privilégio (T-006):
      aborta a mutação NO BANCO independentemente do papel — só `session_replication_role =
      replica` (superusuário, ato deliberado) contorna. Verificado em PG15 real (cenários
      "Tentativa de UPDATE" e "Tentativa de DELETE" + TRUNCATE): as três mutações são rejeitadas
      mesmo como superusuário e o evento permanece íntegro. Cleanup dos testes de auditoria
      passou a usar o bypass documentado. Gate verde.)*
- [x] **T-008** Implementar `AuditSink` síncrono durável (modo fail-closed). *(Decisão do
      arquiteto: composição ATÔMICA na mesma transação. Porto `internal/domain/audit_sink.go`:
      `AuditSink.Record` (o trilho real que substitui os selos provisórios `AccessAuditor`/
      `SessionAuditor` do 002 — rewiring dos chamadores é T-017); `ErrAuditUnavailable` +
      `RecordOrDeny` escrevem a regra I-5.4 uma vez (falha ao persistir ⇒ negação, INV-6);
      `Action.RequirePrivileged` = L3 (privilegiado exige o síncrono; não-L3 pode ir para a fila
      da T-009). `AuditWriter` ganhou `AppendTx(ctx, tx, input)` (grava NA transação do chamador,
      `Append`=WithTx(AppendTx)) e satisfaz `AuditSink` via `Record`. Verificado: unit de
      RecordOrDeny (fail-closed) e RequirePrivileged; integração em PG15 — AppendTx atômico com a
      tx do chamador (rollback descarta evento E a criação lazy do cabeçalho; commit torna
      durável). O dreno do session_event_outbox (002) fica para a T-009 (o outbox é a fila
      durável de eventos não privilegiados). Gate verde.)*
- [x] **T-009** Implementar modo assíncrono com fila durável para eventos não privilegiados.
      *(Decisão do arquiteto: fila genérica + outbox como 1º produtor. Migration 0019:
      `audit_event_queue` (id UUIDv7, organization_id, payload jsonb, enqueued_at) — INSERT
      rápido no caminho da requisição, sem trava de cadeia. `AuditQueue` (`internal/adapters/
      postgres/audit_queue.go`): `Enqueue` valida via `NewAuditEvent`, RECUSA ação privilegiada
      (L3 vai pelo AuditSink síncrono), captura event_id+occurred_at no enfileiramento e grava o
      evento como payload; `Drain` sela cada item na cadeia via `sealEventInTx` e apaga a linha da
      fila na MESMA transação (at-least-once, nunca perdido nem duplo-encadeado), ordem por
      (org, id). Writer refatorado: `sealEventInTx(tx, event)` (núcleo compartilhado que usa o
      occurred_at já carimbado — sync usa o relógio, drain preserva o tempo do enfileiramento).
      **Alça do 002 fechada**: `SwitchOutboxDrainer` drena o `session_event_outbox` (troca de
      tenant = L2) para a cadeia — mapeia a linha para um evento `tenant.switch`, resolve o
      subject opaco do ator pelo identity_id e marca `published_at` na mesma transação.
      Verificado em PG15: enfileira sem tocar a cadeia → drena em ordem, verificável, fila
      esvazia; privilegiado recusado; troca de tenant real → evento tenant.switch selado com o
      subject certo, outbox publicado, re-drain no-op. Gate verde.)*
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
