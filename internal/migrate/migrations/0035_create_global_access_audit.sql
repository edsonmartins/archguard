-- 0035: trilha append-only de ACESSO CROSS-TENANT (pacote 007, ADR-0022).
--
-- Toda leitura que atravessa tenants (o login resolvendo os próprios memberships,
-- um relatório global) passa pelo GlobalAuthorizer e DEVE ser registrada de forma
-- durável antes de acontecer (RFC-0002 §4, I-5.4). A trilha imutável do pacote 003
-- (`audit_event`) é encadeada POR TENANT — exige organization_id —, mas um acesso
-- global não pertence a um tenant. Este é o destino próprio desses acessos: um log
-- append-only simples (sem cadeia hash por-tenant), com principal, motivo e escopo.
--
-- APPEND-ONLY (espírito do INV-2), em duas camadas como a trilha 003: privilégio de
-- banco (roles.sql revoga UPDATE/DELETE/TRUNCATE para archguard_app) e triggers de
-- bloqueio abaixo (defesa em profundidade, contra quem tenha privilégio por outro
-- caminho). Não é a trilha selada/assinada — é o registro de acesso global.
CREATE TABLE IF NOT EXISTS global_access_audit (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    occurred_at  timestamptz NOT NULL DEFAULT now(),
    -- Principal (quem faz o acesso) e reason (por quê) são obrigatórios — um acesso
    -- cross-tenant sem principal ou motivo é recusado antes de chegar aqui (domínio).
    principal    text        NOT NULL CHECK (principal <> ''),
    reason       text        NOT NULL CHECK (reason <> ''),
    -- Escopo declarado do acesso (ADR-0022): 'self' (confinado à própria identidade)
    -- ou 'cross_tenant' (leitura ampla). Auditar o escopo torna a intenção rastreável.
    scope        text        NOT NULL CHECK (scope IN ('self', 'cross_tenant'))
);

CREATE INDEX IF NOT EXISTS global_access_audit_occurred_at_idx
    ON global_access_audit (occurred_at);

-- Triggers de bloqueio de mutação (espelha 0018): a linha registrada não muda nem
-- é apagada. Abortado NO BANCO, independentemente do papel.
CREATE OR REPLACE FUNCTION global_access_audit_block_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'global_access_audit é append-only (INV-2): % proibido', TG_OP
        USING ERRCODE = 'insufficient_privilege';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS global_access_audit_no_update ON global_access_audit;
CREATE TRIGGER global_access_audit_no_update
    BEFORE UPDATE ON global_access_audit
    FOR EACH ROW EXECUTE FUNCTION global_access_audit_block_mutation();

DROP TRIGGER IF EXISTS global_access_audit_no_delete ON global_access_audit;
CREATE TRIGGER global_access_audit_no_delete
    BEFORE DELETE ON global_access_audit
    FOR EACH ROW EXECUTE FUNCTION global_access_audit_block_mutation();
