-- 0034: cria `directory_connector` — a configuração de sincronismo com diretório
-- corporativo (LDAP/AD) por organização (pacote 009, T-001; RFC-0007 §5.1).
--
-- Tabela de DOMÍNIO, logo tenant-scoped: organization_id NOT NULL e RLS FORCE
-- (Barreira 2), como membership/role_assignment (0011). O acesso de runtime passa
-- pelo repositório tenant-scoped (Barreira 1).
--
-- Segurança embutida no esquema:
--   * scope_filter NOT NULL + CHECK <> '': sincronizar "toda a árvore" é proibido
--     (RFC-0007 §5.1). Um conector sem escopo não pode existir.
--   * credential_ref guarda a REFERÊNCIA da credencial no cofre (OpenBao), NUNCA
--     a credencial. Segredo jamais no banco (INV-7 / ADR-0012).
--   * mapping (jsonb) é o mapeamento diretório→ArchGuard VERSIONADO; mapping_version
--     é a versão corrente (>= 1). Cada revisão avança a versão (auditável).
--
-- Sem dado pessoal em claro: nome do conector, filtro (metadados de diretório) e
-- mapeamento de atributos/grupos — não são dados de pessoa (LGPD: metadados de
-- integração). enabled nasce false (default seguro): sincronismo é ligado após
-- revisão do mapeamento.
CREATE TABLE IF NOT EXISTS directory_connector (
    id              uuid        PRIMARY KEY,
    organization_id uuid        NOT NULL REFERENCES organization (id),
    kind            text        NOT NULL CHECK (kind IN ('ldap', 'ad')),
    name            text        NOT NULL,
    scope_filter    text        NOT NULL CHECK (scope_filter <> ''),
    credential_ref  text        NOT NULL CHECK (credential_ref <> ''),
    enabled         boolean     NOT NULL DEFAULT false,
    mapping_version int         NOT NULL DEFAULT 1 CHECK (mapping_version >= 1),
    mapping         jsonb       NOT NULL DEFAULT '{"Version":1,"Attributes":[],"Groups":[]}'::jsonb,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, name)
);

CREATE INDEX IF NOT EXISTS directory_connector_organization_idx
    ON directory_connector (organization_id);

-- RLS FORCE tenant-isolation (mesmo padrão de membership/role_assignment, 0011):
-- linha visível só no tenant ativo (ou leitura global autorizada); escrita só no
-- tenant ativo, sem modo global. FORCE vale inclusive para o dono da tabela.
ALTER TABLE directory_connector ENABLE ROW LEVEL SECURITY;
ALTER TABLE directory_connector FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS directory_connector_tenant_isolation ON directory_connector;
CREATE POLICY directory_connector_tenant_isolation ON directory_connector
    FOR ALL
    USING (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
        OR NULLIF(current_setting('app.global_read', true), '') = 'on'
    )
    WITH CHECK (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
    );
