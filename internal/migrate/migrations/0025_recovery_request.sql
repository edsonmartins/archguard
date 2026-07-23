-- 0025: recuperação de fator com aprovação de pares (T-013).
--
-- Uma identidade que perde o autenticador e não tem código de recuperação abre
-- uma solicitação COM JUSTIFICATIVA; a recuperação só prossegue após um LIMIAR
-- de aprovações de PARES distintos (spec "Perda de dispositivo" / "Recuperação
-- sem credencial administrativa universal": nenhum ator único reseta um fator).
-- Todo o processo é auditado (T-016) e o alvo notificado.
--
-- LGPD: `justification` é texto livre fornecido pelo solicitante — pode conter
-- dado pessoal contextual; tratado como campo de conteúdo do tenant (não indexado
-- por pessoa, sem hash). Referências de identidade são uuid pseudonimizado. Sem
-- segredo aqui (INV-7): a recuperação AUTORIZA o reset de fator, o material do
-- novo fator segue as regras de credential/cofre.

CREATE TABLE IF NOT EXISTS recovery_request (
    id                 uuid        PRIMARY KEY,
    target_identity_id uuid        NOT NULL REFERENCES identity (id),
    organization_id    uuid        NOT NULL REFERENCES organization (id),
    requested_by       uuid        NOT NULL REFERENCES identity (id),
    justification      text        NOT NULL,
    status             text        NOT NULL
                                   CHECK (status IN ('pending', 'approved', 'rejected', 'consumed')),
    required_approvals int         NOT NULL CHECK (required_approvals >= 1),
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS recovery_request_org_idx ON recovery_request (organization_id);
CREATE INDEX IF NOT EXISTS recovery_request_target_idx ON recovery_request (target_identity_id);

-- Aprovações: a PK composta garante APROVADORES DISTINTOS no próprio banco
-- (barreira física do limiar por pares, além da checagem de domínio).
CREATE TABLE IF NOT EXISTS recovery_approval (
    recovery_request_id  uuid        NOT NULL REFERENCES recovery_request (id),
    approver_identity_id uuid        NOT NULL REFERENCES identity (id),
    organization_id      uuid        NOT NULL REFERENCES organization (id),
    created_at           timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (recovery_request_id, approver_identity_id)
);

-- RLS (Barreira 2) por `app.current_organization` nas duas tabelas — mesma
-- mecânica das demais tabelas de domínio (0011/0013/0023). LÊ: org corrente OU
-- leitura global; ESCREVE: só org corrente.
ALTER TABLE recovery_request ENABLE ROW LEVEL SECURITY;
ALTER TABLE recovery_request FORCE ROW LEVEL SECURITY;
CREATE POLICY recovery_request_read ON recovery_request
    FOR SELECT
    USING (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
        OR NULLIF(current_setting('app.global_read', true), '') = 'on'
    );
CREATE POLICY recovery_request_write ON recovery_request
    FOR ALL
    USING (organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid)
    WITH CHECK (organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid);

ALTER TABLE recovery_approval ENABLE ROW LEVEL SECURITY;
ALTER TABLE recovery_approval FORCE ROW LEVEL SECURITY;
CREATE POLICY recovery_approval_read ON recovery_approval
    FOR SELECT
    USING (
        organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid
        OR NULLIF(current_setting('app.global_read', true), '') = 'on'
    );
CREATE POLICY recovery_approval_write ON recovery_approval
    FOR ALL
    USING (organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid)
    WITH CHECK (organization_id = NULLIF(current_setting('app.current_organization', true), '')::uuid);
