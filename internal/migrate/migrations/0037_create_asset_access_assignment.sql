-- 0037: cria `asset_access_assignment` — a atribuição GRANULAR de acesso a um ativo
-- (pacote 007 M4, T-029). Expressa, como fonte da verdade, que um SUJEITO (hoje um
-- membership) tem uma RELAÇÃO (operator/auditor) sobre um OBJETO (asset ou asset_group).
-- É a autorização granular ReBAC (RFC-0004 §3), distinta do papel administrativo do
-- Casbin herdado (ADR-0005). A projeção para authz_tuple é derivada desta (via
-- ProjectRoleAssignment), de onde o PDP deriva can_open_session (operator OU owner) e,
-- por herança do `parent`, o acesso HERDADO de um asset_group para seus ativos.
--
-- Tabela de DOMÍNIO: organization_id NOT NULL + RLS por tenant (INV-5), padrão de
-- membership/role_assignment (0011). O sujeito e o objeto são do MESMO tenant — barrado
-- pela RLS e pelo serviço.
CREATE TABLE IF NOT EXISTS asset_access_assignment (
    id              uuid        PRIMARY KEY,
    organization_id uuid        NOT NULL REFERENCES organization (id),
    -- sujeito da atribuição: hoje sempre um membership (group vem numa sub-fase).
    subject_type    text        NOT NULL CHECK (subject_type IN ('membership')),
    subject_id      uuid        NOT NULL,
    -- relação concedida: apenas as atribuíveis (operator/auditor); can_open_* são
    -- DERIVADAS pelo PDP, nunca atribuídas.
    relation        text        NOT NULL CHECK (relation IN ('operator', 'auditor')),
    -- objeto: um ativo (acesso direto) ou um grupo de ativos (acesso herdado pelos filhos).
    object_type     text        NOT NULL CHECK (object_type IN ('asset', 'asset_group')),
    object_id       uuid        NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    -- uma atribuição é única por (sujeito, relação, objeto).
    UNIQUE (subject_type, subject_id, relation, object_type, object_id)
);
CREATE INDEX IF NOT EXISTS asset_access_assignment_organization_idx ON asset_access_assignment (organization_id);
CREATE INDEX IF NOT EXISTS asset_access_assignment_subject_idx ON asset_access_assignment (subject_id);
CREATE INDEX IF NOT EXISTS asset_access_assignment_object_idx ON asset_access_assignment (object_id);

ALTER TABLE asset_access_assignment ENABLE ROW LEVEL SECURITY;
ALTER TABLE asset_access_assignment FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS asset_access_assignment_tenant_isolation ON asset_access_assignment;
CREATE POLICY asset_access_assignment_tenant_isolation ON asset_access_assignment
    FOR ALL
    USING (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
        OR NULLIF(current_setting('app.global_read', true), '') = 'on'
    )
    WITH CHECK (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
    );
