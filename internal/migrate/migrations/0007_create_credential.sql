-- 0007: cria `credential` — os fatores de autenticação da identidade global
-- (RFC-0002 §2.4, pacote 002 T-005). Como `identity`, é CROSS-TENANT (R1): sem
-- organization_id; credenciais pertencem à identidade, nunca ao membership.
--
-- INV-7 ESTRUTURAL: não existe coluna que guarde segredo reversível em claro.
--   * verifier         — hash UNIDIRECIONAL (senha, recovery code). Não é segredo.
--   * secret_ref       — REFERÊNCIA a segredo reversível no cofre (seed TOTP via
--                        SecretStore/OpenBao). O seed jamais entra no banco.
--   * public_material  — chave PÚBLICA (WebAuthn). Seguro no banco.
-- O CHECK `credential_shape` amarra cada tipo ao seu material exato: um TOTP só
-- pode carregar secret_ref (nunca verifier/material), então nenhum caminho de
-- código consegue persistir um seed em claro.
--
-- `aal` é o nível de garantia do fator (ADR-0010): alimenta a política de step-up
-- L1/L2/L3. O CHECK espelha AAL.Valid() do domínio.
CREATE TABLE IF NOT EXISTS credential (
    id              uuid        PRIMARY KEY,
    identity_id     uuid        NOT NULL REFERENCES identity (id),
    type            text        NOT NULL
                                CHECK (type IN ('password', 'totp', 'webauthn', 'recovery_code')),
    aal             text        NOT NULL CHECK (aal IN ('aal1', 'aal2', 'aal3')),
    verifier        bytea,
    secret_ref      text,
    public_material bytea,
    params          jsonb       NOT NULL DEFAULT '{}',
    label           text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    last_used_at    timestamptz,
    CONSTRAINT credential_shape CHECK (
        (type IN ('password', 'recovery_code')
            AND verifier IS NOT NULL AND secret_ref IS NULL AND public_material IS NULL)
        OR (type = 'totp'
            AND secret_ref IS NOT NULL AND verifier IS NULL AND public_material IS NULL)
        OR (type = 'webauthn'
            AND public_material IS NOT NULL AND secret_ref IS NULL AND verifier IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS credential_identity_idx ON credential (identity_id);

-- Uma senha e um autenticador TOTP por identidade (fatores únicos); WebAuthn e
-- recovery codes são múltiplos por natureza.
CREATE UNIQUE INDEX IF NOT EXISTS credential_one_password_idx
    ON credential (identity_id) WHERE type = 'password';
CREATE UNIQUE INDEX IF NOT EXISTS credential_one_totp_idx
    ON credential (identity_id) WHERE type = 'totp';

-- Classificação LGPD (I-3.3 / ADR-0014): material de autenticação relaciona-se ao
-- titular, logo é dado pessoal.
COMMENT ON COLUMN credential.verifier IS
	'LGPD | categoria=verificador de autenticação (hash de senha/recovery code), pessoal | finalidade=autenticação do titular | base_legal=execução de contrato / legítimo interesse em segurança (controlador decide) | retencao=enquanto o fator existir; unidirecional, não reversível';

COMMENT ON COLUMN credential.secret_ref IS
	'LGPD | categoria=referência a segredo de autenticação no cofre (seed TOTP), pessoal | finalidade=autenticação do titular (MFA) | base_legal=execução de contrato / legítimo interesse em segurança (controlador decide) | retencao=enquanto o fator existir; o segredo vive no cofre (INV-7), não no banco';

COMMENT ON COLUMN credential.public_material IS
	'LGPD | categoria=material público de autenticação (chave pública WebAuthn), pessoal | finalidade=autenticação do titular (passkey) | base_legal=execução de contrato / legítimo interesse em segurança (controlador decide) | retencao=enquanto o fator existir; material público';
