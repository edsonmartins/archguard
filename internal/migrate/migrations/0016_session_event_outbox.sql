-- 0016: outbox transacional de eventos de sessão (achado de revisão do pacote
-- 002; RFC-0004 §4, CLAUDE.md §6 "Nunca chamada remota dentro de transação de
-- banco. Use outbox transacional").
--
-- Problema: a troca de tenant (T-012) registrava o evento chamando o porto
-- `SessionAuditor.RecordTenantSwitch` DENTRO da transação da troca. O porto não
-- tem como participar da transação pgx (o domínio não importa driver, INV-3), e
-- a implementação durável (trilha do pacote 003) seria ou uma chamada remota
-- dentro da transação (proibida) ou uma segunda transação independente cujo
-- commit poderia sobreviver ao rollback da troca — um registro de auditoria de
-- uma troca que não aconteceu (inverte I-5.4).
--
-- Correção: o evento é ESCRITO NESTA TABELA pela mesma transação da troca (via
-- IdentityTx). A durabilidade e a atomicidade passam a ser estruturais: se a
-- troca dá rollback, a linha do outbox some junto; se a inserção no outbox
-- falha, a troca é negada. A trilha tamper-evident do pacote 003 DRENA o outbox
-- de forma assíncrona (fora da transação da troca — aí a chamada remota é
-- permitida), marcando `published_at`.
--
-- LGPD: sem dado pessoal em claro — apenas referências pseudonimizadas (uuid),
-- nível de garantia e carimbos de tempo (mesma avaliação da auth_session, 0012).
CREATE TABLE IF NOT EXISTS session_event_outbox (
    id                   uuid        PRIMARY KEY,
    event_type           text        NOT NULL CHECK (event_type IN ('tenant_switch')),
    session_id           uuid        NOT NULL,
    identity_id          uuid        NOT NULL,
    from_membership_id   uuid        NOT NULL,
    from_organization_id uuid        NOT NULL,
    to_membership_id     uuid        NOT NULL,
    to_organization_id   uuid        NOT NULL,
    proven_aal           text        NOT NULL CHECK (proven_aal IN ('aal1', 'aal2', 'aal3')),
    token_generation     int         NOT NULL CHECK (token_generation >= 2),
    occurred_at          timestamptz NOT NULL DEFAULT now(),
    published_at         timestamptz
);

-- O dreno do pacote 003 varre as linhas ainda não publicadas em ordem.
CREATE INDEX IF NOT EXISTS session_event_outbox_unpublished_idx
    ON session_event_outbox (occurred_at) WHERE published_at IS NULL;
