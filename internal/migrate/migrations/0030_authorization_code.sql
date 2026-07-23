-- 0030: códigos de autorização (Authorization Code + PKCE, pacote 006).
--
-- Código de USO ÚNICO e curta duração (≤60s) que liga uma sessão autenticada a
-- um cliente, seu desafio PKCE (S256), o redirect_uri e os escopos concedidos
-- (RFC-0006 §5). Só o HASH do segredo é guardado (INV-7 — o código vai ao cliente
-- pelo redirect, uma vez).
--
-- LGPD: sem campo pessoal — hashes, uuids, escopos e um instante. RLS por
-- `app.current_organization`.

CREATE TABLE IF NOT EXISTS authorization_code (
    id              uuid        PRIMARY KEY,
    code_hash       bytea       NOT NULL UNIQUE,
    client_id       text        NOT NULL,
    redirect_uri    text        NOT NULL,
    code_challenge  text        NOT NULL,
    session_id      uuid        NOT NULL REFERENCES auth_session (id),
    organization_id uuid        NOT NULL REFERENCES organization (id),
    scopes          text[]      NOT NULL DEFAULT '{}',
    expires_at      timestamptz NOT NULL,
    used            boolean     NOT NULL DEFAULT false,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS authorization_code_expiry_idx ON authorization_code (expires_at);

ALTER TABLE authorization_code ENABLE ROW LEVEL SECURITY;
ALTER TABLE authorization_code FORCE ROW LEVEL SECURITY;
CREATE POLICY authorization_code_read ON authorization_code
    FOR SELECT
    USING (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
        OR NULLIF(current_setting('app.global_read', true), '') = 'on'
    );
CREATE POLICY authorization_code_write ON authorization_code
    FOR ALL
    USING (organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid)
    WITH CHECK (organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid);
