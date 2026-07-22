-- 0020: selos assinados da trilha (pacote 003, T-011). A cada intervalo/volume o
-- head da cadeia de uma organização é assinado (Ed25519 via cofre, T-010); o
-- selo torna a adulteração RETROATIVA detectável mesmo por quem tem acesso ao
-- banco (RFC-0003 §4).
--
-- O selo é APPEND-ONLY como os eventos (decisão do arquiteto): um selo mutável
-- ou apagável deixaria um atacante remover o selo para esconder a violação. As
-- barreiras são as mesmas de audit_event — privilégio (roles.sql, T-006) e
-- trigger (abaixo).
--
-- `seq_start`/`seq_end` delimitam o intervalo coberto; a continuidade entre
-- selos (sem lacuna: seq_start = seq_end do selo anterior + 1) é verificada pelo
-- verificador (T-013). UNIQUE(organization_id, seq_end) impede dois selos com o
-- mesmo fim; `head_hash` é o hash do último evento do intervalo; `key_id`
-- identifica a chave para verificação após rotação.
CREATE TABLE IF NOT EXISTS audit_seal (
    id              uuid        PRIMARY KEY,
    organization_id uuid        NOT NULL,
    seq_start       bigint      NOT NULL CHECK (seq_start >= 1),
    seq_end         bigint      NOT NULL CHECK (seq_end >= seq_start),
    head_hash       bytea       NOT NULL,
    sealed_at       timestamptz NOT NULL,
    key_id          text        NOT NULL,
    signature       bytea       NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, seq_end)
);

CREATE INDEX IF NOT EXISTS audit_seal_org_range_idx ON audit_seal (organization_id, seq_start);

-- Barreira física INV-2: bloqueio de mutação (reutiliza a função da 0018).
DROP TRIGGER IF EXISTS audit_seal_no_update ON audit_seal;
CREATE TRIGGER audit_seal_no_update
    BEFORE UPDATE ON audit_seal
    FOR EACH ROW EXECUTE FUNCTION audit_event_block_mutation();

DROP TRIGGER IF EXISTS audit_seal_no_delete ON audit_seal;
CREATE TRIGGER audit_seal_no_delete
    BEFORE DELETE ON audit_seal
    FOR EACH ROW EXECUTE FUNCTION audit_event_block_mutation();

DROP TRIGGER IF EXISTS audit_seal_no_truncate ON audit_seal;
CREATE TRIGGER audit_seal_no_truncate
    BEFORE TRUNCATE ON audit_seal
    FOR EACH STATEMENT EXECUTE FUNCTION audit_event_block_mutation();
