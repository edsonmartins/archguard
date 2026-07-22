-- 0021: registro local de âncora externa de selos (pacote 003, T-012). Quando um
-- selo (0020) é exportado para o destino WORM do cliente (S3 Object Lock ou
-- storage imutável on-prem — RFC-0003 §4), a referência devolvida pelo destino é
-- registrada aqui, para saber quais selos já foram ancorados e não re-exportar.
--
-- ESTA TABELA É APENAS BOOKKEEPING LOCAL. A propriedade anti-adulteração vem do
-- DESTINO WORM (write-once, fora do alcance da instância), não daqui: mesmo que
-- um atacante apague estas linhas, os objetos já gravados no WORM permanecem e a
-- divergência continua detectável. Por isso a tabela não precisa ser append-only.
--
-- UNIQUE(seal_id, destination): um selo é ancorado uma vez por destino
-- (idempotência do exportador).
CREATE TABLE IF NOT EXISTS audit_seal_anchor (
    id          uuid        PRIMARY KEY,
    seal_id     uuid        NOT NULL REFERENCES audit_seal (id),
    destination text        NOT NULL,
    ref         text        NOT NULL,
    anchored_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (seal_id, destination)
);

CREATE INDEX IF NOT EXISTS audit_seal_anchor_seal_idx ON audit_seal_anchor (seal_id);
