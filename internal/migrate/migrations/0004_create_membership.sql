-- 0004: cria `membership` — a relação explícita identidade↔organização, núcleo
-- da decisão do ADR-0006 (RFC-0002 §2.3, pacote 002 T-002). Uma identidade
-- global pode pertencer a várias organizações; cada pertencimento é uma linha.
--
-- `id` é UUIDv7 gerado pela aplicação (Go), como em `identity`. As FKs apontam
-- para `identity(id)` (0002) e `organization(id)` (0003, coluna UUID estável) —
-- ambas no mesmo banco, então a FK é válida mesmo a `organization` sendo mundo
-- XORM legado. Isto NÃO é uma FK que atravessa organizações (R6): a `membership`
-- é a própria definição do vínculo com a organização.
--
-- `status` é NOT NULL SEM default: invited vs active depende do fluxo (convite,
-- T-013, nasce `invited`; adição direta/migração nasce `active`). Sem default, a
-- aplicação é obrigada a declarar o estado — nada de pertencimento ativo por
-- omissão. O CHECK espelha MembershipStatus.Valid() do domínio (2ª barreira).
--
-- `attributes_enc` guarda atributos ESPECÍFICOS do tenant (matrícula, centro de
-- custo) cifrados — nunca compartilhados entre tenants (segregação, RFC-0002).
-- `invited_by` referencia a identidade que convidou (trilha de ciclo de vida).
--
-- R3: unicidade do par (identity_id, organization_id) — uma identidade tem no
-- máximo um membership por organização.
CREATE TABLE IF NOT EXISTS membership (
    id              uuid        PRIMARY KEY,
    identity_id     uuid        NOT NULL REFERENCES identity (id),
    organization_id uuid        NOT NULL REFERENCES organization (id),
    status          text        NOT NULL
                                CHECK (status IN ('invited', 'active', 'suspended', 'revoked')),
    attributes_enc  bytea,
    invited_by      uuid        REFERENCES identity (id),
    activated_at    timestamptz,
    revoked_at      timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (identity_id, organization_id)
);

-- Consultas quentes são "memberships desta identidade" (montar o seletor de
-- tenant, cache por identidade) e "memberships desta organização" (RLS, gestão).
CREATE INDEX IF NOT EXISTS membership_identity_idx ON membership (identity_id);
CREATE INDEX IF NOT EXISTS membership_organization_idx ON membership (organization_id);
