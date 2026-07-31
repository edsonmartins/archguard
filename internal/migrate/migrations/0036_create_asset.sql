-- 0036: cria `asset_group` e `asset` — o catálogo de ATIVOS do tenant (pacote 007
-- M4, T-026). Fonte da verdade dos ativos que a autorização granular protege; a
-- projeção para `authz_tuple` é derivada destes (RFC-0004 §4). A identidade canônica
-- é o ID do ArchGuard mesmo quando o recurso é importado de um broker (Warpgate/
-- NetBird) — `external_ref` guarda o id opaco do broker, NUNCA um segredo (INV-7,
-- RFC-0004 §9). Espelham `domain.Asset` / `domain.AssetGroup`.
--
-- Tabelas de DOMÍNIO: carregam `organization_id` NOT NULL e RLS por tenant (INV-5),
-- no mesmo padrão de membership/role_assignment (0011). FKs same-tenant (parent group,
-- owner membership) — o cruzamento entre tenants é barrado pela RLS e pelo serviço.

CREATE TABLE IF NOT EXISTS asset_group (
    id              uuid        PRIMARY KEY,
    organization_id uuid        NOT NULL REFERENCES organization (id),
    name            text        NOT NULL CHECK (name <> ''),
    -- aninhamento (mesmo tenant); NULL = grupo de topo. Sem ON DELETE CASCADE: a
    -- remoção com filhos é decisão do serviço (validada por ValidateAssetGroupHierarchy).
    parent_group_id uuid        REFERENCES asset_group (id),
    created_at      timestamptz NOT NULL DEFAULT now(),
    CHECK (parent_group_id IS NULL OR parent_group_id <> id) -- não é pai de si mesmo
);
CREATE INDEX IF NOT EXISTS asset_group_organization_idx ON asset_group (organization_id);
CREATE INDEX IF NOT EXISTS asset_group_parent_idx ON asset_group (parent_group_id);

CREATE TABLE IF NOT EXISTS asset (
    id                  uuid        PRIMARY KEY,
    organization_id     uuid        NOT NULL REFERENCES organization (id),
    -- kind é rótulo livre de granularidade ("host"/"service"/"account"…) — não é enum
    -- fechado de propósito (RFC-0004 §9.2 em aberto).
    kind                text        NOT NULL CHECK (kind <> ''),
    name                text        NOT NULL CHECK (name <> ''),
    -- id opaco do recurso no componente que faz o brokering; '' para ativo registrado
    -- diretamente. Nunca segredo (INV-7).
    external_ref        text        NOT NULL DEFAULT '',
    parent_group_id     uuid        REFERENCES asset_group (id),   -- grupo (mesmo tenant)
    owner_membership_id uuid        REFERENCES membership (id),     -- dono (mesmo tenant)
    created_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS asset_organization_idx ON asset (organization_id);
CREATE INDEX IF NOT EXISTS asset_parent_group_idx ON asset (parent_group_id);
CREATE INDEX IF NOT EXISTS asset_owner_idx ON asset (owner_membership_id);

-- RLS por tenant (INV-5), idêntica ao padrão de membership/role_assignment (0011):
-- a linha é visível/gravável no tenant ativo (app.current_organization) ou sob leitura
-- global (app.global_read=on, autorizada/auditada pelo GlobalRepository).
ALTER TABLE asset_group ENABLE ROW LEVEL SECURITY;
ALTER TABLE asset_group FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS asset_group_tenant_isolation ON asset_group;
CREATE POLICY asset_group_tenant_isolation ON asset_group
    FOR ALL
    USING (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
        OR NULLIF(current_setting('app.global_read', true), '') = 'on'
    )
    WITH CHECK (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
    );

ALTER TABLE asset ENABLE ROW LEVEL SECURITY;
ALTER TABLE asset FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS asset_tenant_isolation ON asset;
CREATE POLICY asset_tenant_isolation ON asset
    FOR ALL
    USING (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
        OR NULLIF(current_setting('app.global_read', true), '') = 'on'
    )
    WITH CHECK (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
    );
