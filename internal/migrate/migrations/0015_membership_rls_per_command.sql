-- 0015: fecha o alargamento da superfície de ESCRITA que a 0014 abriu na
-- `membership` (achado de revisão do pacote 002).
--
-- Problema: a 0014 fez a policy `FOR ALL` com o eixo de identidade também no
-- WITH CHECK. Isso permitia que qualquer transação com `app.current_identity`
-- fixado (o fluxo de login) INSERISSE uma `membership` `active` para a própria
-- identidade em QUALQUER organização — um self-grant cross-tenant que a Barreira
-- 2 (RLS) deveria impedir mesmo com a Barreira 1 contornada. O eixo de
-- identidade só era necessário para o UPDATE da cascata (identidade→memberships,
-- T-014), nunca para INSERT.
--
-- Correção — policies POR COMANDO (a RLS aplica a que casa o comando):
--   * SELECT: org corrente OU identidade própria OU leitura global (T-009).
--   * INSERT (WITH CHECK): SÓ o eixo de tenant — uma `membership` nasce pelo
--     tenant que a possui (convite via TenantMembershipStore, contexto de org).
--     SEM eixo de identidade: acaba o self-grant.
--   * UPDATE (USING + WITH CHECK): org corrente OU identidade própria — a
--     cascata da identidade precisa alterar status das PRÓPRIAS linhas sob
--     `app.current_identity` (sem contexto de org). A imutabilidade de
--     organization_id/identity_id (trigger abaixo) impede que esse eixo seja
--     usado para "mover" a membership para outro tenant (self-move).
--   * DELETE: SEM policy ⇒ negado por padrão. `membership` não é apagada
--     fisicamente — o ciclo é revogação (R4), preservada na trilha.
--
-- Runtime: o papel da aplicação nunca INSERE membership sob contexto de
-- identidade (convites usam contexto de tenant); a cascata só faz UPDATE de
-- status. O ensaio de migração (migrehearsal), que INSERE memberships de uma
-- identidade fundida sob contexto de identidade em N orgs numa transação, é uma
-- operação de MIGRAÇÃO e roda como papel privilegiado (BYPASSRLS), não como o
-- papel de runtime — como todo o restante da migração.

DROP POLICY IF EXISTS membership_tenant_isolation ON membership;

CREATE POLICY membership_select ON membership
    FOR SELECT
    USING (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
        OR identity_id = NULLIF(current_setting('app.current_identity', true), '')::uuid
        OR NULLIF(current_setting('app.global_read', true), '') = 'on'
    );

CREATE POLICY membership_insert ON membership
    FOR INSERT
    WITH CHECK (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
    );

CREATE POLICY membership_update ON membership
    FOR UPDATE
    USING (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
        OR identity_id = NULLIF(current_setting('app.current_identity', true), '')::uuid
    )
    WITH CHECK (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
        OR identity_id = NULLIF(current_setting('app.current_identity', true), '')::uuid
    );

-- Imutabilidade das chaves de tenant: nem a cascata nem ninguém pode re-tenantar
-- uma membership (o UPDATE do eixo de identidade só deve mexer em status/tempos).
-- Fecha o self-move que o UPDATE sob identidade poderia tentar.
CREATE OR REPLACE FUNCTION membership_keys_immutable() RETURNS trigger AS $$
BEGIN
    IF NEW.organization_id <> OLD.organization_id OR NEW.identity_id <> OLD.identity_id THEN
        RAISE EXCEPTION 'membership: organization_id e identity_id são imutáveis';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS membership_keys_immutable_trg ON membership;
CREATE TRIGGER membership_keys_immutable_trg
    BEFORE UPDATE ON membership
    FOR EACH ROW EXECUTE FUNCTION membership_keys_immutable();
