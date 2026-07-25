-- 0008: dá à `role` herdada (Casdoor) um id UUID estável, para que o vínculo de
-- papel possa referenciá-la por chave estável (RFC-0002 §2.4, T-006). Mesma
-- decisão e mecânica de `organization` (0003): a PK legada composta em string
-- (owner, name) permanece; a coluna nova é o alvo da FK de `role_assignment`.
--
-- A definição do papel segue no mundo XORM legado; só o VÍNCULO (quem tem o
-- papel) migra para `role_assignment` por membership (R2). O struct Go de `role`
-- não ganha o campo — o id é lido via pgx.
--
-- Ordem de boot: migrations rodam após o Sync2, então `role` já existe aqui.
ALTER TABLE IF EXISTS role
    ADD COLUMN IF NOT EXISTS id uuid NOT NULL DEFAULT gen_random_uuid();

-- Sufixo `_pkey` DE PROPÓSITO: `role` é tabela gerenciada pelo XORM (Sync2 a cada
-- boot), que dropa índices do DB ausentes no struct e só PULA os terminados em
-- `_pkey`. Ver a nota detalhada em 0003_organization_stable_id.sql.
CREATE UNIQUE INDEX IF NOT EXISTS role_id_pkey ON role (id);
