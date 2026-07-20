-- 0002: cria a tabela `identity` — a identidade global única do deployment
-- (RFC-0002 §2.1, pacote 002 T-001).
--
-- `identity` é CROSS-TENANT por construção: a R1 do RFC-0002 lista `identity`
-- entre as exceções que NÃO carregam `organization_id`. O pertencimento a
-- organizações é a tabela `membership` (T-002); credenciais e fatores MFA
-- pertencem à identidade, não ao membership.
--
-- `id` é UUIDv7 gerado pela APLICAÇÃO (Go: google/uuid NewV7) — ordenável no
-- tempo, bom para localidade de índice. Não há default no banco: PostgreSQL 15
-- não gera UUIDv7 nativamente e a geração é responsabilidade do domínio.
--
-- Campos pessoais são cifrados (…_enc bytea, chave por titular, ADR-0014); o
-- banco nunca vê PII em claro. `email_hash` (HMAC com chave de deployment)
-- sustenta busca e login sem descriptografar — a COLUNA nasce aqui; o índice
-- ÚNICO e o caminho de login por hash entram no T-003.
--
-- Os CHECKs de `type` e `status` espelham os IdentityType.Valid() /
-- IdentityStatus.Valid() do domínio: a barreira física do banco não confia na
-- aplicação. `deprovisioned` é terminal (R5): sem deleção física, o ciclo é
-- crypto-shredding (ADR-0014).
CREATE TABLE IF NOT EXISTS identity (
    id                uuid        PRIMARY KEY,
    subject           text        NOT NULL UNIQUE,
    type              text        NOT NULL CHECK (type IN ('human', 'service')),
    status            text        NOT NULL DEFAULT 'active'
                                  CHECK (status IN ('active', 'suspended', 'deprovisioned')),
    primary_email_enc bytea,
    email_hash        bytea,
    display_name_enc  bytea,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);
