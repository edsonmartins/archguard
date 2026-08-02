-- 0039: cria `access_group` — o CATÁLOGO de grupos de acesso do tenant (pacote 007 M4,
-- D1 catálogo). Dá nome a um grupo de acesso cujo id (uuid) antes era opaco: o
-- `group_membership.group_id` e o userset `group:<id>#member` das atribuições passam a
-- ter um nome legível na UI. É só metadado — um grupo NÃO gera tupla por si; as tuplas
-- vêm de group_membership (`member`) e das atribuições (`operator`/`auditor`).
--
-- Nome distinto de `group` (grupos herdados do Casdoor) e de `asset_group`. Tabela de
-- DOMÍNIO: organization_id NOT NULL + RLS por tenant (INV-5), padrão de 0037/0038. Sem FK
-- de group_membership.group_id → access_group: vínculos legados com id opaco (sem entrada
-- no catálogo) continuam válidos; o catálogo é aditivo.
CREATE TABLE IF NOT EXISTS access_group (
    id              uuid        PRIMARY KEY,
    organization_id uuid        NOT NULL REFERENCES organization (id),
    name            text        NOT NULL CHECK (name <> ''),
    display_name    text        NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, name)
);
CREATE INDEX IF NOT EXISTS access_group_organization_idx ON access_group (organization_id);

ALTER TABLE access_group ENABLE ROW LEVEL SECURITY;
ALTER TABLE access_group FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS access_group_tenant_isolation ON access_group;
CREATE POLICY access_group_tenant_isolation ON access_group
    FOR ALL
    USING (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
        OR NULLIF(current_setting('app.global_read', true), '') = 'on'
    )
    WITH CHECK (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
    );
