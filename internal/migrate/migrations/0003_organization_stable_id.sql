-- 0003: dá à `organization` herdada (Casdoor) um id UUID estável, decisão do
-- arquiteto (2026-07-20) para o pacote 002. A PK legada é composta e em string
-- (owner, name); `name` é renomeável, logo é fronteira de isolamento inadequada.
-- O `organization_id` do RFC-0002 e a RLS por `organization_id` (Barreira 2)
-- passam a chavear neste UUID, desacoplando o isolamento do nome mutável.
--
-- Ordem de boot (RUNBOOK): as migrations rodam DEPOIS do Sync2 do XORM, então a
-- tabela `organization` já existe aqui — em instalação nova (Sync2 acabou de
-- criá-la) e em instalação existente. `ALTER TABLE IF EXISTS` cobre ambas.
--
-- O DEFAULT é volátil (gen_random_uuid, nativo no PG13+): o PostgreSQL o avalia
-- POR LINHA ao adicionar a coluna, então cada organização existente recebe um id
-- distinto no backfill, e toda inserção futura da app legada (XORM, que não
-- conhece esta coluna) também recebe um id automaticamente. O struct Go NÃO
-- ganha o campo de propósito: a camada de tenancy nova lê/escreve o id via pgx
-- (RFC-0002 §5), mantendo o mundo novo fora do XORM.
ALTER TABLE IF EXISTS organization
    ADD COLUMN IF NOT EXISTS id uuid NOT NULL DEFAULT gen_random_uuid();

-- UNIQUE é pré-requisito para a FK de `membership.organization_id` (0004) e para
-- a chave de RLS. Índice único (não PK: a PK legada composta permanece intacta).
CREATE UNIQUE INDEX IF NOT EXISTS organization_id_key ON organization (id);
