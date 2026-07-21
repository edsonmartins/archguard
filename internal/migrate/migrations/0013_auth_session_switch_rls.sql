-- 0013: troca de tenant (T-012) — geração de token e RLS da `auth_session`.
--
-- 1) `token_generation`: contador de invalidação de token da sessão. Todo token
--    emitido carrega a geração vigente; a TROCA de tenant incrementa o contador,
--    então token emitido antes da troca falha na validação POR CONSTRUÇÃO —
--    "o token anterior não é reaproveitado" (spec "Troca de tenant"). A emissão/
--    validação do JWT em si é o pacote 006; o mecanismo nasce aqui.
--
-- 2) RLS (Barreira 2) na `auth_session` — fecha a pendência registrada na 0012.
--    A tabela tem DOIS eixos legítimos de acesso:
--      * eixo IDENTIDADE (fluxo de login: criar sessão, selecionar tenant,
--        trocar, ler a própria sessão) — inclui linhas pending SEM organização,
--        que o eixo de tenant não alcança. Contexto: `app.current_identity`,
--        fixado por transação (SET LOCAL) pelo IdentityRepository — mesmo
--        contrato de `app.current_organization` (constante travada por teste em
--        domain.RLSIdentitySettingName).
--      * eixo TENANT (gestão da organização e cascata de revogação por
--        membership, T-014) — `app.current_organization`, como nas demais.
--    LÊ: identidade própria OU organização corrente OU leitura global (T-009).
--    ESCREVE (WITH CHECK): identidade própria OU organização corrente — sem
--    escrita via leitura global, como na 0011.
--
-- NULLIF(..., '') protege parâmetro ausente (mesma mecânica da 0011); FORCE
-- aplica a política até ao dono da tabela; o papel de runtime não tem BYPASSRLS
-- (deploy/postgres/roles.sql).

ALTER TABLE auth_session
    ADD COLUMN IF NOT EXISTS token_generation int NOT NULL DEFAULT 1
        CHECK (token_generation >= 1);

ALTER TABLE auth_session ENABLE ROW LEVEL SECURITY;
ALTER TABLE auth_session FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS auth_session_isolation ON auth_session;
CREATE POLICY auth_session_isolation ON auth_session
    FOR ALL
    USING (
        identity_id = NULLIF(current_setting('app.current_identity', true), '')::uuid
        OR organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
        OR NULLIF(current_setting('app.global_read', true), '') = 'on'
    )
    WITH CHECK (
        identity_id = NULLIF(current_setting('app.current_identity', true), '')::uuid
        OR organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
    );
