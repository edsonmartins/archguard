-- 0029: famílias de refresh token com rotação e detecção de reuso (pacote 006).
--
-- Cada refresh token pertence a uma FAMÍLIA (cadeia de rotações da sessão). A
-- rotação marca o antigo `rotated` e insere um sucessor `active`. Apresentar um
-- token `rotated`/`revoked` é REUSO ⇒ toda a família é revogada + evento de
-- severidade alta (RFC-0006 §5). Só o HASH do segredo é guardado (INV-7 — o
-- segredo vai ao cliente uma vez e nunca é persistido).
--
-- LGPD: sem campo pessoal — hashes, uuids e status. RLS (Barreira 2) por
-- `app.current_organization`.

CREATE TABLE IF NOT EXISTS refresh_token (
    id              uuid        PRIMARY KEY,
    family_id       uuid        NOT NULL,
    session_id      uuid        NOT NULL REFERENCES auth_session (id),
    organization_id uuid        NOT NULL REFERENCES organization (id),
    token_hash      bytea       NOT NULL,
    status          text        NOT NULL CHECK (status IN ('active', 'rotated', 'revoked')),
    expires_at      timestamptz NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

-- Busca no momento da troca: pelo hash do token apresentado (uso único de fato).
CREATE UNIQUE INDEX IF NOT EXISTS refresh_token_hash_key ON refresh_token (token_hash);
CREATE INDEX IF NOT EXISTS refresh_token_family_idx ON refresh_token (family_id);
CREATE INDEX IF NOT EXISTS refresh_token_session_idx ON refresh_token (session_id);

ALTER TABLE refresh_token ENABLE ROW LEVEL SECURITY;
ALTER TABLE refresh_token FORCE ROW LEVEL SECURITY;
CREATE POLICY refresh_token_read ON refresh_token
    FOR SELECT
    USING (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
        OR NULLIF(current_setting('app.global_read', true), '') = 'on'
    );
CREATE POLICY refresh_token_write ON refresh_token
    FOR ALL
    USING (organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid)
    WITH CHECK (organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid);
