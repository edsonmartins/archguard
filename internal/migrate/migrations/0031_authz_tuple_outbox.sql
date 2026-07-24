-- 0031: outbox transacional de tuplas de autorização (pacote 007, T-005;
-- RFC-0004 §4). O PostgreSQL do ArchGuard é a FONTE DA VERDADE; o PDP (OpenFGA,
-- ou o avaliador em domínio) é PROJEÇÃO DERIVADA.
--
-- Problema: uma mutação de domínio que muda a autorização (papel de operador,
-- concessão, filiação a grupo) precisa refletir no grafo do PDP. Escrever a
-- tupla chamando o PDP DENTRO da transação da mutação é chamada remota em
-- transação de banco — proibida (RFC-0002 §5 / CLAUDE.md §6). Uma segunda
-- transação independente poderia commitar a projeção de uma mutação que deu
-- rollback (grafo concede acesso que o banco não tem).
--
-- Correção: a mutação grava as TUPLAS DERIVADAS nesta tabela na MESMA transação
-- (via a mesma Querier/tx do store de negócio). Atomicidade estrutural: se a
-- mutação dá rollback, as linhas do outbox somem junto; se a inserção no outbox
-- falha, a mutação é negada. O publisher idempotente (T-006) DRENA as linhas não
-- publicadas de forma assíncrona — fora da transação de negócio, onde a escrita
-- remota no PDP é permitida — e marca `published_at`. Reaplicar uma linha é
-- seguro: escrita de tupla é idempotente (spec "Reprocessamento").
--
-- Sem RLS FORCE (como o outbox de sessão 0016): a linha é escrita já dentro da
-- transação tenant-fixada da mutação, e o publisher a drena globalmente como
-- projeção confiável. A tupla NUNCA cruza tenants: o adapter valida cada tupla
-- (domain.ValidateTuple) antes de enfileirar, rejeitando ref não qualificado ou
-- cross-tenant (INV-5); `organization_id` é derivado do objeto qualificado.
--
-- LGPD: sem dado pessoal em claro — user/relation/object são referências
-- pseudonimizadas qualificadas por tenant (uuid), nunca segredo (INV-7).
CREATE TABLE IF NOT EXISTS authz_tuple_outbox (
    id              uuid        PRIMARY KEY,
    organization_id uuid        NOT NULL,
    op              text        NOT NULL CHECK (op IN ('write', 'delete')),
    tuple_user      text        NOT NULL,
    tuple_relation  text        NOT NULL,
    tuple_object    text        NOT NULL,
    occurred_at     timestamptz NOT NULL DEFAULT now(),
    published_at    timestamptz
);

-- O publisher varre as linhas ainda não publicadas em ordem de ocorrência.
CREATE INDEX IF NOT EXISTS authz_tuple_outbox_unpublished_idx
    ON authz_tuple_outbox (occurred_at) WHERE published_at IS NULL;
