-- 0022: contexto de autenticação da sessão (T-005) — `auth_time` e os métodos.
--
-- A sessão passa a registrar QUANDO a identidade autenticou e QUAIS fatores a
-- comprovaram, os insumos dos claims OIDC `auth_time`, `amr` e `acr` (ADR-0010,
-- preparação do pacote 006).
--
-- 1) `auth_time`: momento da autenticação. Distinto de `created_at` — a sessão
--    sobrevive a reautenticações (step-up, T-008/T-009), que avançam `auth_time`
--    sem recriar a linha. Backfill das linhas existentes com `created_at` (no
--    login, autenticação == criação); depois NOT NULL com DEFAULT now().
--
-- 2) `auth_methods`: os TIPOS de fator provados nesta autenticação, na ordem —
--    a FONTE de verdade. O `amr` (tokens RFC 8176: pwd/otp/hwk + mfa) e o `acr`
--    (o nível `aal*`, já em `proven_aal`) são DERIVADOS disto no domínio
--    (AuthSession.AMR()/ACR()), não persistidos redundantemente. O CHECK garante
--    que todo elemento é um tipo de fator conhecido — nada de "sms" (que não
--    existe como fator; spec "SMS como fator → rejeitado").
--
-- LGPD: sem campo pessoal novo — `auth_time` é timestamp de evento e
-- `auth_methods` são rótulos de tipo de fator (não identificam pessoa nem
-- dispositivo). Nenhum segredo entra aqui (INV-7): são só os TIPOS de fator,
-- nunca material de credencial.

ALTER TABLE auth_session
    ADD COLUMN IF NOT EXISTS auth_time timestamptz;

UPDATE auth_session SET auth_time = created_at WHERE auth_time IS NULL;

ALTER TABLE auth_session
    ALTER COLUMN auth_time SET DEFAULT now(),
    ALTER COLUMN auth_time SET NOT NULL;

ALTER TABLE auth_session
    ADD COLUMN IF NOT EXISTS auth_methods text[] NOT NULL DEFAULT '{}';

-- Todo método registrado deve ser um tipo de fator conhecido (o mesmo conjunto
-- de domain.FactorType.Valid()). Fecha a porta a rótulos arbitrários — em
-- especial "sms", que não é um fator suportado.
ALTER TABLE auth_session
    ADD CONSTRAINT auth_session_auth_methods_known CHECK (
        auth_methods <@ ARRAY['password', 'totp', 'webauthn', 'recovery_code']::text[]
    );
