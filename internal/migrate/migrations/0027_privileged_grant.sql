-- 0027: concessões privilegiadas e cascata de sessões derivadas (pacote 004).
--
-- `privileged_grant` é a concessão time-limitada (T-001): sujeito = membership
-- (R2), alvo (ativo/escopo), janela, origem (normal|breakglass), status da
-- máquina de estados (T-007), justificativa/incidente no break-glass (T-008).
-- `grant_approval` guarda as aprovações de pares (PK composta = aprovadores
-- distintos no banco, T-010). `auth_session.privileged_grant_id` liga uma sessão
-- DERIVADA a sua concessão, para a revogação em cascata na expiração/revogação
-- (T-012).
--
-- LGPD: `justification` é conteúdo do tenant (pode conter dado contextual — não
-- indexado por pessoa); demais campos são referências pseudonimizadas (uuid) e
-- rótulos. Sem segredo (INV-7). RLS (Barreira 2) por `app.current_organization`.

CREATE TABLE IF NOT EXISTS privileged_grant (
    id                    uuid        PRIMARY KEY,
    organization_id       uuid        NOT NULL REFERENCES organization (id),
    subject_membership_id uuid        NOT NULL,
    target_type           text        NOT NULL,
    target_id             text        NOT NULL,
    target_scope          text        NOT NULL,
    origin                text        NOT NULL CHECK (origin IN ('normal', 'breakglass')),
    status                text        NOT NULL
                                      CHECK (status IN ('requested', 'awaiting_approval', 'active',
                                                        'expired', 'revoked', 'denied', 'rejected')),
    required_approvals    int         NOT NULL CHECK (required_approvals >= 0),
    not_before            timestamptz NOT NULL,
    expires_at            timestamptz NOT NULL,
    justification         text        NOT NULL DEFAULT '',
    incident_ref          text        NOT NULL DEFAULT '',
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    CHECK (expires_at > not_before)
);

CREATE INDEX IF NOT EXISTS privileged_grant_org_idx ON privileged_grant (organization_id);
-- Consulta quente do job de expiração: concessões ativas cuja janela venceu.
CREATE INDEX IF NOT EXISTS privileged_grant_active_expiry_idx
    ON privileged_grant (organization_id, expires_at) WHERE status = 'active';

CREATE TABLE IF NOT EXISTS grant_approval (
    grant_id              uuid        NOT NULL REFERENCES privileged_grant (id),
    approver_membership_id uuid       NOT NULL,
    organization_id       uuid        NOT NULL REFERENCES organization (id),
    created_at            timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (grant_id, approver_membership_id)
);

-- Ligação sessão↔concessão: uma sessão derivada de um break-glass referencia o
-- grant, para a cascata de revogação.
ALTER TABLE auth_session
    ADD COLUMN IF NOT EXISTS privileged_grant_id uuid REFERENCES privileged_grant (id);
CREATE INDEX IF NOT EXISTS auth_session_grant_idx
    ON auth_session (privileged_grant_id) WHERE privileged_grant_id IS NOT NULL;

-- RLS (Barreira 2) por `app.current_organization` nas duas tabelas novas.
ALTER TABLE privileged_grant ENABLE ROW LEVEL SECURITY;
ALTER TABLE privileged_grant FORCE ROW LEVEL SECURITY;
CREATE POLICY privileged_grant_read ON privileged_grant
    FOR SELECT
    USING (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
        OR NULLIF(current_setting('app.global_read', true), '') = 'on'
    );
CREATE POLICY privileged_grant_write ON privileged_grant
    FOR ALL
    USING (organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid)
    WITH CHECK (organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid);

ALTER TABLE grant_approval ENABLE ROW LEVEL SECURITY;
ALTER TABLE grant_approval FORCE ROW LEVEL SECURITY;
CREATE POLICY grant_approval_read ON grant_approval
    FOR SELECT
    USING (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
        OR NULLIF(current_setting('app.global_read', true), '') = 'on'
    );
CREATE POLICY grant_approval_write ON grant_approval
    FOR ALL
    USING (organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid)
    WITH CHECK (organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid);
