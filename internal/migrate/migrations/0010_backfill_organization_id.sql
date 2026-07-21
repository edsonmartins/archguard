-- 0010: backfill de `organization_id` nas tabelas de domínio tenant-scoped do
-- núcleo PAM (RFC-0002 R1/§6, pacote 002 T-007). Prepara o terreno para a RLS
-- (T-010), que chaveia em `organization_id`.
--
-- Para cada tabela do inventário (docs/upstream/TENANT_INVENTORY.md):
--   1. adiciona `organization_id uuid` (NULLABLE aqui);
--   2. popula a partir do `owner` legado: organization_id = organization.id onde
--      organization.name = owner;
--   3. FK para organization(id) e índice (a RLS filtra por esta coluna).
--
-- LINHAS GLOBAIS (owner='admin', org raiz 'built-in', apps IsShared) NÃO casam
-- com nenhuma organização pelo nome, então permanecem com organization_id NULL —
-- são configuração global compartilhada (cross-tenant, R1), por decisão do
-- arquiteto. Por isso o NOT NULL NÃO é imposto aqui: fica para o T-010, por
-- tabela, só onde 100% das linhas mapearem a uma org (as demais seguem nullable).
--
-- Idempotente e tolerante a tabela ausente (feature desabilitada / instalação
-- enxuta): o bloco checa a existência de cada tabela antes de alterá-la. As
-- migrations rodam após o Sync2, então as tabelas legadas já existem.
DO $$
DECLARE
    t text;
    -- Núcleo PAM tenant-scoped (TENANT_INVENTORY.md). `role` já ganhou `id` no
    -- 0008; aqui ganha `organization_id` como as demais.
    tenant_tables text[] := ARRAY[
        'user', 'group', 'role', 'permission', 'token', 'session',
        'invitation', 'resource', 'syncer', 'webhook',
        'application', 'provider', 'cert',
        'adapter', 'enforcer', 'model'
    ];
BEGIN
    FOREACH t IN ARRAY tenant_tables LOOP
        IF to_regclass(format('%I', t)) IS NULL THEN
            CONTINUE; -- tabela não existe nesta instalação; nada a fazer
        END IF;

        EXECUTE format('ALTER TABLE %I ADD COLUMN IF NOT EXISTS organization_id uuid', t);

        EXECUTE format(
            'UPDATE %I s SET organization_id = o.id FROM organization o '
            'WHERE s.owner = o.name AND s.organization_id IS NULL', t);

        EXECUTE format(
            'CREATE INDEX IF NOT EXISTS %I ON %I (organization_id)',
            t || '_organization_id_idx', t);

        IF NOT EXISTS (
            SELECT 1 FROM pg_constraint WHERE conname = t || '_organization_id_fkey'
        ) THEN
            EXECUTE format(
                'ALTER TABLE %I ADD CONSTRAINT %I '
                'FOREIGN KEY (organization_id) REFERENCES organization (id)',
                t, t || '_organization_id_fkey');
        END IF;
    END LOOP;
END $$;
