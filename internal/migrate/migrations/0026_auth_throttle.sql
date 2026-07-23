-- 0026: limitação de taxa e bloqueio progressivo por identidade (T-014).
--
-- Cada identidade acumula falhas CONSECUTIVAS de autenticação; ao passar o
-- limiar, entra em bloqueio que cresce a cada falha (spec "Tentativas
-- repetidas" / ADR-0010). Um login bem-sucedido zera o estado. O evento de
-- bloqueio é auditado (T-016).
--
-- LGPD: sem campo pessoal — contadores e um instante de bloqueio por uuid de
-- identidade. RLS (Barreira 2) pelo eixo IDENTIDADE (`app.current_identity`,
-- como a auth_session da 0013): o fluxo de login fixa a identidade tentada.

CREATE TABLE IF NOT EXISTS auth_throttle (
    identity_id  uuid        PRIMARY KEY REFERENCES identity (id),
    failures     int         NOT NULL DEFAULT 0 CHECK (failures >= 0),
    locked_until timestamptz,
    updated_at   timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE auth_throttle ENABLE ROW LEVEL SECURITY;
ALTER TABLE auth_throttle FORCE ROW LEVEL SECURITY;

-- LÊ/ESCREVE: só a própria identidade corrente (o fluxo de login), OU leitura
-- global (T-009) apenas para SELECT. NULLIF protege parâmetro ausente.
CREATE POLICY auth_throttle_read ON auth_throttle
    FOR SELECT
    USING (
        identity_id = NULLIF(current_setting('app.current_identity', true), '')::uuid
        OR NULLIF(current_setting('app.global_read', true), '') = 'on'
    );
CREATE POLICY auth_throttle_write ON auth_throttle
    FOR ALL
    USING (identity_id = NULLIF(current_setting('app.current_identity', true), '')::uuid)
    WITH CHECK (identity_id = NULLIF(current_setting('app.current_identity', true), '')::uuid);
