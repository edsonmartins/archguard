-- 0017: trilha de auditoria imutável (pacote 003, T-005). Tabela de eventos
-- append-only particionada por tempo + cabeçalho de cadeia por organização.
--
-- É uma tabela NOVA (pgx), distinta da `record` legada do Casdoor (que o
-- roles.sql já protege). A imutabilidade é imposta em três camadas: privilégio
-- de banco (só INSERT/SELECT para a app, T-006), triggers de bloqueio (T-007) e
-- o encadeamento por hash + selagem assinada (T-002/003/010). Esta migration
-- cria o armazenamento; as barreiras vêm nas tarefas seguintes.
--
-- CADEIA POR ORGANIZAÇÃO (RFC-0003 §3): cada org tem uma linha em
-- `audit_chain_head` com o último seq, o hash do head e o genesis_nonce. A
-- escrita (T-004) trava essa linha (SELECT ... FOR UPDATE), lê prev_hash/seq,
-- sela o evento e avança o head — serialização por tenant sem lacuna e sem
-- corrida em prev_hash.
CREATE TABLE IF NOT EXISTS audit_chain_head (
    organization_id uuid        PRIMARY KEY,
    last_seq        bigint      NOT NULL DEFAULT 0 CHECK (last_seq >= 0),
    head_hash       bytea       NOT NULL,          -- 32 bytes: genesis, depois o hash do último evento
    genesis_nonce   bytea       NOT NULL,          -- 32 bytes por org (cadeias de tenants não colidem)
    updated_at      timestamptz NOT NULL DEFAULT now()
);

-- Tabela de eventos, PARTICIONADA POR TEMPO (RFC-0003 §5.4: arquivamento move
-- partições seladas, nunca deleta evento individual). A PK inclui a chave de
-- partição (occurred_at), exigência do Postgres; a unicidade real de
-- (organization_id, seq) é garantida pela serialização do cabeçalho (T-004).
--
-- Sem coluna `canonical` armazenada de propósito: o verificador (T-013)
-- RECOMPUTA o canônico a partir DESTAS colunas e compara com `hash` — assim
-- adulterar qualquer coluna consultável quebra a verificação (se guardássemos o
-- blob canônico, uma coluna poderia mentir sem quebrar a cadeia).
CREATE TABLE IF NOT EXISTS audit_event (
    organization_id      uuid        NOT NULL,
    seq                  bigint      NOT NULL,
    occurred_at          timestamptz NOT NULL,
    event_id             uuid        NOT NULL,
    schema_version       int         NOT NULL,
    action               text        NOT NULL,
    outcome              text        NOT NULL CHECK (outcome IN ('success', 'denied', 'error')),
    -- Ator (RFC-0003 §2): subject é pseudônimo opaco; delegação como jsonb.
    actor_subject        text        NOT NULL,
    actor_membership_id  uuid,
    actor_session_id     uuid,
    actor_act            jsonb,
    -- Alvo.
    target_type          text        NOT NULL DEFAULT '',
    target_id            text        NOT NULL DEFAULT '',
    target_label         text        NOT NULL DEFAULT '',
    reason               text        NOT NULL DEFAULT '',
    -- Contexto de origem/correlação.
    context_ip           text        NOT NULL DEFAULT '',
    context_user_agent   text        NOT NULL DEFAULT '',
    context_acr          text        NOT NULL DEFAULT '',
    context_amr          text[]      NOT NULL DEFAULT '{}',
    context_trace_id     text        NOT NULL DEFAULT '',
    context_pcid         text        NOT NULL DEFAULT '',
    -- Encadeamento (T-003).
    prev_hash            bytea       NOT NULL,
    hash                 bytea       NOT NULL,
    PRIMARY KEY (organization_id, occurred_at, seq)
) PARTITION BY RANGE (occurred_at);

-- Partição DEFAULT: garante que todo insert cai em algum lugar. As partições de
-- intervalo de tempo e o arquivamento por período são o T-018; por ora a DEFAULT
-- captura tudo.
CREATE TABLE IF NOT EXISTS audit_event_default PARTITION OF audit_event DEFAULT;

-- Índices de consulta (RFC-0003 §5, design §"Restrições de banco"): por ator,
-- por período/verificação em ordem de seq, e por correlação.
CREATE INDEX IF NOT EXISTS audit_event_org_seq_idx ON audit_event (organization_id, seq);
CREATE INDEX IF NOT EXISTS audit_event_actor_idx ON audit_event (organization_id, actor_subject);
CREATE INDEX IF NOT EXISTS audit_event_action_idx ON audit_event (organization_id, action);
CREATE INDEX IF NOT EXISTS audit_event_trace_idx ON audit_event (context_trace_id) WHERE context_trace_id <> '';
CREATE INDEX IF NOT EXISTS audit_event_pcid_idx ON audit_event (context_pcid) WHERE context_pcid <> '';

-- Classificação LGPD (I-3.3 / ADR-0014) dos campos com dado pessoal. A
-- eliminação do titular é por crypto-shredding do subject (pacote 010); os
-- eventos permanecem e a cadeia continua verificável (I-3.4).
COMMENT ON COLUMN audit_event.actor_subject IS
    'LGPD: categoria=identificador pseudonimizado; finalidade=trilha de auditoria/segurança (PAM); base_legal=legitimo_interesse+obrigacao_legal; retencao=por politica do tenant (padrao 5 anos, RFC-0003 §8); eliminacao=crypto-shredding da chave do titular (ADR-0014)';
COMMENT ON COLUMN audit_event.context_ip IS
    'LGPD: categoria=dado pessoal (endereco IP de origem); finalidade=deteccao de abuso/rastreabilidade de acesso; base_legal=legitimo_interesse; retencao=por politica do tenant; eliminacao=arquivamento da particao selada (RFC-0003 §8)';
COMMENT ON COLUMN audit_event.context_user_agent IS
    'LGPD: categoria=dado pessoal (agente de usuario/dispositivo); finalidade=deteccao de abuso/rastreabilidade; base_legal=legitimo_interesse; retencao=por politica do tenant; eliminacao=arquivamento da particao selada';
