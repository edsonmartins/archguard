-- 0001: remove as colunas órfãs deixadas pelas remoções de escopo do pacote 001.
-- Os campos correspondentes já saíram dos structs (XORM Sync2 é aditivo e não
-- dropa); esta é a remoção destrutiva EXPLÍCITA (ADR-0009), nunca por auto-sync.
--   cart            -> removido em T-008 (pagamento/produto)
--   face_ids        -> removido em T-010 (reconhecimento facial)
--   master_password -> removido em T-011 (senha-mestra, INV-1)
-- IF EXISTS torna a migration idempotente e segura em instalação nova (onde as
-- colunas nunca foram criadas).
ALTER TABLE IF EXISTS "user" DROP COLUMN IF EXISTS cart;
ALTER TABLE IF EXISTS "user" DROP COLUMN IF EXISTS face_ids;
ALTER TABLE IF EXISTS organization DROP COLUMN IF EXISTS master_password;
