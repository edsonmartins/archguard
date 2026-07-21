-- 0009: cria `role_assignment` — o vínculo explícito papel↔membership (RFC-0002
-- §2.4, R2, T-006). Substitui a lista denormalizada `Role.Users[]` do legado.
--
-- R2: o vínculo referencia `membership_id`, JAMAIS `identity_id` — não existe
-- papel de tenant atribuído diretamente à identidade global. A pessoa autentica
-- uma vez (identity) e, no contexto de um tenant (membership), carrega papéis.
--
-- R1: `role_assignment` é tabela de DOMÍNIO, logo carrega `organization_id`
-- NOT NULL (a RLS do T-010 chaveia nele). O organization_id é o do próprio
-- membership; a disciplina de atribuição (papel e membership no MESMO tenant) é
-- do mecanismo de migração e do serviço — o cruzamento entre tenants é barrado
-- por teste de travessia (T-017/T-018) e pela RLS.
--
-- FKs para organization(id) (0003), role(id) (0008) e membership(id) (0004).
CREATE TABLE IF NOT EXISTS role_assignment (
    id              uuid        PRIMARY KEY,
    organization_id uuid        NOT NULL REFERENCES organization (id),
    role_id         uuid        NOT NULL REFERENCES role (id),
    membership_id   uuid        NOT NULL REFERENCES membership (id),
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (role_id, membership_id)
);

CREATE INDEX IF NOT EXISTS role_assignment_membership_idx ON role_assignment (membership_id);
CREATE INDEX IF NOT EXISTS role_assignment_organization_idx ON role_assignment (organization_id);
