-- 0032: store de projeção de tuplas de autorização (pacote 007, T-006). É a
-- PROJEÇÃO DERIVADA que o PDP em domínio lê para decidir (RFC-0004 §4): o
-- publisher idempotente drena o authz_tuple_outbox (0031) para cá.
--
-- Guarda apenas tuplas ESTRUTURAIS e estáveis (parent, owner, operator/auditor
-- por membership/role, member de grupo). Concessões (has_active_grant) NÃO são
-- materializadas aqui: são voláteis (expiram a todo momento) e são lidas AO VIVO
-- da tabela privileged_grant no instante da decisão (T-010, "sem cache para
-- abertura de sessão privilegiada", RFC-0004 §5) — a concessão expira no grafo
-- porque o reader só devolve a que está vigente e o resolvedor confere a janela.
--
-- Aplicação idempotente (spec "Reprocessamento"): write é INSERT ... ON CONFLICT
-- DO NOTHING; delete é DELETE do casamento exato. Reaplicar uma linha do outbox
-- leva ao MESMO estado final.
--
-- Sem RLS: o publisher escreve globalmente (projeção confiável, como os drains de
-- outbox). O reader do PDP SEMPRE consulta com predicado explícito de
-- organization_id (Barreira 1) e a decisão é guardada por domain.GuardSameTenant,
-- de modo que nenhuma relação atravessa tenants (INV-5). Refs são pseudônimos
-- qualificados por tenant, sem dado pessoal/segredo (INV-7).
CREATE TABLE IF NOT EXISTS authz_tuple (
    organization_id uuid NOT NULL,
    tuple_user      text NOT NULL,
    tuple_relation  text NOT NULL,
    tuple_object    text NOT NULL,
    PRIMARY KEY (tuple_user, tuple_relation, tuple_object)
);

-- Consulta direta do resolvedor: sujeitos de (object, relation).
CREATE INDEX IF NOT EXISTS authz_tuple_object_idx
    ON authz_tuple (tuple_object, tuple_relation);

-- Consulta reversa (revisão de acesso, T-014): objetos de (user, relation) — já
-- coberta pelo prefixo (tuple_user, tuple_relation) da PK.

-- Escopo por tenant no bootstrap/reconciliação.
CREATE INDEX IF NOT EXISTS authz_tuple_org_idx ON authz_tuple (organization_id);
