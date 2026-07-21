-- 0011: ativa Row-Level Security (Barreira 2, RFC-0002 §4, pacote 002 T-010) nas
-- tabelas NOVAS totalmente mediadas pelo TenantRepository: `membership` e
-- `role_assignment`. As tabelas LEGADAS ficam de fora por ora — o código XORM
-- herdado as consulta SEM fixar `app.current_organization`, então ligar RLS nelas
-- devolveria zero linhas e quebraria a aplicação. A ativação é por tabela, em
-- ordem controlada (RFC §6): as legadas entram quando seu acesso migrar para o
-- repositório tenant-scoped.
--
-- Política (FOR ALL):
--   * LEITURA (USING): a linha é visível se pertence ao tenant ativo
--     (app.current_organization) OU o modo de leitura global está ligado
--     (app.global_read = 'on', estabelecido só pelo GlobalRepository após
--     autorização + auditoria).
--   * ESCRITA (WITH CHECK): SEM modo global — toda escrita DEVE ter
--     organization_id = tenant ativo. Não existe escrita cross-tenant, nem via
--     leitura global.
--
-- NULLIF(..., '') protege contra o parâmetro ausente: current_setting(name, true)
-- devolve '' quando não definido; '' vira NULL e a comparação é NULL (não visível,
-- não erro de cast).
--
-- FORCE: a política vale inclusive para o dono da tabela (defesa em profundidade).
-- O papel da aplicação (archguard_app, deploy/postgres/roles.sql) NÃO tem
-- BYPASSRLS — condição para a RLS de fato conter o acesso.

ALTER TABLE membership ENABLE ROW LEVEL SECURITY;
ALTER TABLE membership FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS membership_tenant_isolation ON membership;
CREATE POLICY membership_tenant_isolation ON membership
    FOR ALL
    USING (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
        OR NULLIF(current_setting('app.global_read', true), '') = 'on'
    )
    WITH CHECK (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
    );

ALTER TABLE role_assignment ENABLE ROW LEVEL SECURITY;
ALTER TABLE role_assignment FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS role_assignment_tenant_isolation ON role_assignment;
CREATE POLICY role_assignment_tenant_isolation ON role_assignment
    FOR ALL
    USING (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
        OR NULLIF(current_setting('app.global_read', true), '') = 'on'
    )
    WITH CHECK (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
    );
