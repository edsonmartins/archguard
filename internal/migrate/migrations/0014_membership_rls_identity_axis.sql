-- 0014: emenda a policy RLS de `membership` com o EIXO DE IDENTIDADE (T-014).
--
-- Motivo: a revogação em cascata da identidade (RFC-0002 R4 e o cenário
-- "Suspensão da identidade") toca memberships de N organizações e deve rodar em
-- UMA transação (RFC-0002 §5 — cascata parcial é estado inconsistente num
-- produto de segurança). A policy da 0011 só permitia escrita com
-- `organization_id = app.current_organization`; nenhum contexto único de tenant
-- alcança todas as organizações da identidade.
--
-- Decisão do arquiteto (2026-07-21): mesma solução da `auth_session` (0013) —
-- o contexto `app.current_identity` (SET LOCAL pelo IdentityRepository) passa a
-- alcançar as PRÓPRIAS linhas da identidade fixada, para leitura e escrita:
--   * LÊ: org corrente OU leitura global (T-009) OU identidade própria;
--   * ESCREVE (WITH CHECK): org corrente OU identidade própria — segue sem
--     escrita via leitura global.
-- O eixo da identidade não abre travessia entre tenants: uma transação com
-- identidade X fixada só vê/escreve linhas de X — a fronteira cruzada é a da
-- própria pessoa sobre seus vínculos, não a de um tenant sobre outro.
--
-- NULLIF(..., '') protege parâmetro ausente; FORCE segue valendo (0011).

DROP POLICY IF EXISTS membership_tenant_isolation ON membership;
CREATE POLICY membership_tenant_isolation ON membership
    FOR ALL
    USING (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
        OR identity_id = NULLIF(current_setting('app.current_identity', true), '')::uuid
        OR NULLIF(current_setting('app.global_read', true), '') = 'on'
    )
    WITH CHECK (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
        OR identity_id = NULLIF(current_setting('app.current_identity', true), '')::uuid
    );
