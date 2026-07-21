-- 0012: cria `auth_session` — o contexto de tenant ativo da sessão autenticada
-- (design 002 §"Sessão e tenant ativo", T-011). A sessão carrega identity_id +
-- membership_id ATIVO; "exatamente um tenant ativo por sessão" é estrutural:
-- há uma única coluna membership_id.
--
-- O nome é `auth_session` porque `session` já é a tabela LEGADA do Casdoor
-- (XORM, ids de sessão Beego por owner/name/application) — os dois mundos
-- convivem até a ponte de revogação (T-014); divergência registrada no T-020.
--
-- Estados: pending_selection → active → revoked (terminal). Uma identidade com
-- mais de um membership ativo autentica e a sessão nasce `pending_selection`
-- SEM tenant; o token de acesso só pode ser emitido com a sessão `active`
-- (cenário "Múltiplos memberships no login"). O CHECK `auth_session_tenant_shape`
-- faz o banco recusar o atalho: não existe linha active sem membership+org, nem
-- pending com tenant. Na revogação o contexto que a sessão tinha permanece
-- (trilha de auditoria).
--
-- `proven_aal` registra o fator comprovado no login (ADR-0010) — insumo do
-- step-up na troca de tenant (T-012 / pacote 005).
--
-- A FK composta (membership_id, identity_id, organization_id) →
-- membership (id, identity_id, organization_id) amarra a desnormalização: o
-- tenant ativo É um membership DESTA identidade NESTA organização — os campos
-- não podem divergir. (MATCH SIMPLE: com membership/org NULL — pending — a FK
-- não se aplica, que é o desejado.) O índice único de suporte em `membership` é
-- redundante com a PK, existe só para a FK referenciar as três colunas.
--
-- RLS: DIFERIDA para o T-012 (decisão do arquiteto, 2026-07-21). O fluxo de
-- login escreve linhas pending SEM tenant e a política do T-010 não tem escrita
-- global — ligar RLS aqui exigiria um contexto de identidade no banco (ex.:
-- app.current_identity), que será desenhado junto com a troca de tenant.
-- Barreira 1 (predicados identity_id/organization_id nos stores) vale desde já;
-- ativação incremental conforme RFC-0002 §6.
--
-- LGPD: sem campo pessoal novo — apenas referências pseudonimizadas (uuid) e
-- estado de sessão. IP/user-agent NÃO entram aqui; contexto de acesso pertence
-- ao evento de auditoria (RFC-0003).
CREATE UNIQUE INDEX IF NOT EXISTS membership_id_identity_org_key
    ON membership (id, identity_id, organization_id);

CREATE TABLE IF NOT EXISTS auth_session (
    id              uuid        PRIMARY KEY,
    identity_id     uuid        NOT NULL REFERENCES identity (id),
    membership_id   uuid,
    organization_id uuid        REFERENCES organization (id),
    status          text        NOT NULL
                                CHECK (status IN ('pending_selection', 'active', 'revoked')),
    proven_aal      text        NOT NULL
                                CHECK (proven_aal IN ('aal1', 'aal2', 'aal3')),
    revoked_at      timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (membership_id, identity_id, organization_id)
        REFERENCES membership (id, identity_id, organization_id),
    CONSTRAINT auth_session_tenant_shape CHECK (
        (status = 'active'
            AND membership_id IS NOT NULL AND organization_id IS NOT NULL)
        OR (status = 'pending_selection'
            AND membership_id IS NULL AND organization_id IS NULL)
        OR (status = 'revoked')
    )
);

-- Consultas quentes: "as sessões desta identidade" (fluxo de login, cascata da
-- identidade — T-014) e "as sessões ativas desta organização" (gestão do tenant,
-- cascata do membership — T-014).
CREATE INDEX IF NOT EXISTS auth_session_identity_idx ON auth_session (identity_id);
CREATE INDEX IF NOT EXISTS auth_session_organization_idx
    ON auth_session (organization_id) WHERE organization_id IS NOT NULL;
