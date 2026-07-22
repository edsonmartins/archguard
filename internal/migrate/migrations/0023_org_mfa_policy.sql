-- 0023: política de MFA por organização (T-010) — o piso de garantia do tenant.
--
-- Cada organização pode declarar o nível MÍNIMO de garantia que uma sessão
-- precisa comprovar para operar nela (ADR-0010, "Política de MFA por
-- organização"). AAL2 = "MFA obrigatório"; AAL3 = "WebAuthn obrigatório". Uma
-- organização SEM linha aqui usa o piso-base da plataforma (AAL1) — o default
-- é DECIDIDO no adapter (ausência de linha ⇒ baseline), não neste esquema.
--
-- Precedência: este piso é o eixo-tenant; a troca de tenant (T-011) e o guard de
-- garantia sempre tomam o MAIS restritivo entre este piso e o nível da operação.
--
-- LGPD: sem campo pessoal — só o id da organização e um rótulo de nível. RLS
-- (Barreira 2) pelo eixo-tenant `app.current_organization`, como nas demais
-- tabelas de domínio; a leitura de política pela troca de tenant/login usa o
-- contexto da organização de destino.

CREATE TABLE IF NOT EXISTS organization_mfa_policy (
    organization_id uuid        PRIMARY KEY REFERENCES organization (id),
    minimum_aal     text        NOT NULL
                                CHECK (minimum_aal IN ('aal1', 'aal2', 'aal3')),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE organization_mfa_policy ENABLE ROW LEVEL SECURITY;
ALTER TABLE organization_mfa_policy FORCE ROW LEVEL SECURITY;

-- LÊ: a própria organização OU leitura global (T-009). ESCREVE (WITH CHECK): só
-- a organização corrente — sem escrita via leitura global (mesma mecânica da
-- 0011/0013). NULLIF protege parâmetro ausente.
CREATE POLICY organization_mfa_policy_read ON organization_mfa_policy
    FOR SELECT
    USING (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
        OR NULLIF(current_setting('app.global_read', true), '') = 'on'
    );

CREATE POLICY organization_mfa_policy_write ON organization_mfa_policy
    FOR ALL
    USING (organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid)
    WITH CHECK (organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid);
