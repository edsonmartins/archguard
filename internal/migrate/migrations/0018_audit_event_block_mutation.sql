-- 0018: triggers de bloqueio de mutação na trilha (pacote 003, T-007) — defesa
-- em profundidade do INV-2, complementar ao privilégio de banco (T-006).
--
-- O privilégio (REVOKE UPDATE/DELETE) já impede a app de mutar a trilha; a
-- trigger fecha o caso de quem tem privilégio por outro caminho (dono do schema,
-- operador com acesso ao banco): a mutação é abortada NO BANCO,
-- independentemente do papel. Só `session_replication_role = replica` (exige
-- superusuário e é ato deliberado) contorna — o que fica registrado como
-- procedimento excepcional, não caminho de código.
--
-- `audit_event` é PARTICIONADA: triggers BEFORE ROW no pai particionado (PG13+)
-- cascateiam para as partições. TRUNCATE é bloqueado por trigger de statement
-- (não é pego por trigger de linha).
CREATE OR REPLACE FUNCTION audit_event_block_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_event é append-only (INV-2): % proibido', TG_OP
        USING ERRCODE = 'insufficient_privilege';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS audit_event_no_update ON audit_event;
CREATE TRIGGER audit_event_no_update
    BEFORE UPDATE ON audit_event
    FOR EACH ROW EXECUTE FUNCTION audit_event_block_mutation();

DROP TRIGGER IF EXISTS audit_event_no_delete ON audit_event;
CREATE TRIGGER audit_event_no_delete
    BEFORE DELETE ON audit_event
    FOR EACH ROW EXECUTE FUNCTION audit_event_block_mutation();

DROP TRIGGER IF EXISTS audit_event_no_truncate ON audit_event;
CREATE TRIGGER audit_event_no_truncate
    BEFORE TRUNCATE ON audit_event
    FOR EACH STATEMENT EXECUTE FUNCTION audit_event_block_mutation();
