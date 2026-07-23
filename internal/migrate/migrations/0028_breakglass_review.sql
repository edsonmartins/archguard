-- 0028: revisão pós-uso obrigatória de break-glass (pacote 004 T-014).
--
-- Após um break-glass ser USADO e encerrar (expirado/revogado), uma revisão é
-- obrigatória (ADR-0008 §3). Enquanto não houver revisão, a concessão fica como
-- PENDÊNCIA visível (consulta de pendências = LEFT JOIN sem linha aqui) e os
-- responsáveis são notificados (spec "Revisão pendente").
--
-- `grant_id` é UNIQUE: uma revisão por concessão. LGPD: `notes` é conteúdo do
-- tenant (parecer do revisor), não indexado por pessoa. RLS por org.

CREATE TABLE IF NOT EXISTS breakglass_review (
    id                    uuid        PRIMARY KEY,
    grant_id              uuid        NOT NULL UNIQUE REFERENCES privileged_grant (id),
    organization_id       uuid        NOT NULL REFERENCES organization (id),
    reviewer_membership_id uuid       NOT NULL,
    notes                 text        NOT NULL,
    created_at            timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE breakglass_review ENABLE ROW LEVEL SECURITY;
ALTER TABLE breakglass_review FORCE ROW LEVEL SECURITY;
CREATE POLICY breakglass_review_read ON breakglass_review
    FOR SELECT
    USING (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
        OR NULLIF(current_setting('app.global_read', true), '') = 'on'
    );
CREATE POLICY breakglass_review_write ON breakglass_review
    FOR ALL
    USING (organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid)
    WITH CHECK (organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid);
