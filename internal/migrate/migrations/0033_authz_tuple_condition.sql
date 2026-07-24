-- 0033: condição temporal nas tuplas de autorização (pacote 007, T-010). Fecha o
-- caminho da tupla CONDICIONADA ponta a ponta: a concessão (has_active_grant) é
-- uma tupla com a condição valid_window e a janela [not_before, expires_at)
-- (RFC-0004 §3). A concessão EXPIRA NO GRAFO — o reader devolve a tupla com sua
-- janela e o resolvedor a nega fora do intervalo — não por um job de limpeza.
--
-- Colunas anuláveis: tuplas estruturais (parent/owner/operator/member) não têm
-- condição (condition_name NULL). O publisher (0032) e o outbox (0031) passam a
-- carregar a condição quando presente.
ALTER TABLE authz_tuple_outbox
    ADD COLUMN IF NOT EXISTS condition_name text,
    ADD COLUMN IF NOT EXISTS not_before     timestamptz,
    ADD COLUMN IF NOT EXISTS expires_at      timestamptz;

ALTER TABLE authz_tuple
    ADD COLUMN IF NOT EXISTS condition_name text,
    ADD COLUMN IF NOT EXISTS not_before     timestamptz,
    ADD COLUMN IF NOT EXISTS expires_at      timestamptz;
