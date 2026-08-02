-- 0038: cria `group_membership` (membership↔grupo de acesso) e habilita GRUPO como
-- sujeito de atribuição (pacote 007 M4, T-029 D1). O grupo de acesso é um id OPACO
-- (uuid) — não há entidade `group` própria nesta fatia (catálogo/CRUD é follow-up).
--
-- Cadeia de acesso via grupo: membership --member--> group --operator--> asset. Este
-- arquivo cria as duas peças que faltavam:
--   1) group_membership: a fonte da verdade do vínculo, projetada como tupla `member`
--      (ProjectGroupMembership);
--   2) asset_access_assignment aceita subject_type='group', para o grupo (userset
--      `group:<id>#member`) ser operator/auditor sobre um ativo (ProjectRoleAssignment).
--
-- Tabela de DOMÍNIO: organization_id NOT NULL + RLS por tenant (INV-5), padrão de 0037.
CREATE TABLE IF NOT EXISTS group_membership (
    id              uuid        PRIMARY KEY,
    organization_id uuid        NOT NULL REFERENCES organization (id),
    group_id        uuid        NOT NULL,                       -- grupo de acesso (id opaco)
    membership_id   uuid        NOT NULL REFERENCES membership (id),
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (group_id, membership_id)
);
CREATE INDEX IF NOT EXISTS group_membership_organization_idx ON group_membership (organization_id);
CREATE INDEX IF NOT EXISTS group_membership_group_idx ON group_membership (group_id);
CREATE INDEX IF NOT EXISTS group_membership_membership_idx ON group_membership (membership_id);

ALTER TABLE group_membership ENABLE ROW LEVEL SECURITY;
ALTER TABLE group_membership FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS group_membership_tenant_isolation ON group_membership;
CREATE POLICY group_membership_tenant_isolation ON group_membership
    FOR ALL
    USING (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
        OR NULLIF(current_setting('app.global_read', true), '') = 'on'
    )
    WITH CHECK (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
    );

-- Habilita GRUPO como sujeito de atribuição de acesso (antes só 'membership').
ALTER TABLE asset_access_assignment DROP CONSTRAINT IF EXISTS asset_access_assignment_subject_type_check;
ALTER TABLE asset_access_assignment ADD CONSTRAINT asset_access_assignment_subject_type_check
    CHECK (subject_type IN ('membership', 'group'));
