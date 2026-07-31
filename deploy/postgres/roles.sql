-- Papéis de banco segregados do ArchGuard (ADR-0009, RFC-0003).
--
-- Executado por um SUPERUSUÁRIO no provisionamento do PostgreSQL, ANTES de subir
-- a aplicação. A criação de roles é privilégio de cluster e tempo de deploy —
-- não é uma migration da aplicação (a aplicação conecta *como* o role app).
--
-- Três papéis, com privilégio mínimo:
--   archguard_migrate  — aplica migrations e DDL (dono do schema). O migrator
--                        (internal/migrate) conecta com este papel.
--   archguard_app      — conexão de runtime da aplicação. Tem DML nas tabelas de
--                        domínio, mas **SEM UPDATE/DELETE nas tabelas de
--                        auditoria** — é a barreira FÍSICA do INV-2 (append-only
--                        e tamper-evident), complementar à barreira de código
--                        (suíte de invariantes).
--   archguard_readonly — SELECT para relatórios/BI.
--
-- Substitua as senhas por segredos reais no deploy (ou use autenticação por
-- certificado/scram). Ajuste <DB> para o nome do banco (dbName em app.conf).

\set db archguard

-- 1. Papéis (NOLOGIN para grupos; a app usa archguard_app com LOGIN).
DO $$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'archguard_migrate') THEN
    CREATE ROLE archguard_migrate LOGIN PASSWORD 'CHANGE_ME_migrate';
  END IF;
  -- NOBYPASSRLS é EXPLÍCITO: a Row-Level Security (Barreira 2, T-010) só contém o
  -- acesso se o papel de runtime não puder ignorá-la. archguard_app também não é
  -- dono das tabelas (quem as cria é archguard_migrate), então a RLS lhe aplica.
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'archguard_app') THEN
    CREATE ROLE archguard_app LOGIN NOBYPASSRLS PASSWORD 'CHANGE_ME_app';
  END IF;
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'archguard_readonly') THEN
    CREATE ROLE archguard_readonly LOGIN NOBYPASSRLS PASSWORD 'CHANGE_ME_readonly';
  END IF;
END
$$;

-- Garante NOBYPASSRLS mesmo em papéis pré-existentes (idempotente).
ALTER ROLE archguard_app NOBYPASSRLS;
ALTER ROLE archguard_readonly NOBYPASSRLS;

-- 2. Acesso ao banco e ao schema public.
GRANT CONNECT ON DATABASE :db TO archguard_migrate, archguard_app, archguard_readonly;
GRANT USAGE ON SCHEMA public TO archguard_migrate, archguard_app, archguard_readonly;
GRANT CREATE ON SCHEMA public TO archguard_migrate;

-- 3. archguard_migrate é o dono efetivo do schema (aplica DDL).
--    As tabelas criadas por ele recebem os privilégios default abaixo.

-- 4. Privilégios DEFAULT para tabelas FUTURAS criadas por archguard_migrate:
--    - app: DML completo (o passo 6 remove UPDATE/DELETE nas de auditoria)
--    - readonly: apenas SELECT
ALTER DEFAULT PRIVILEGES FOR ROLE archguard_migrate IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO archguard_app;
ALTER DEFAULT PRIVILEGES FOR ROLE archguard_migrate IN SCHEMA public
  GRANT SELECT ON TABLES TO archguard_readonly;
ALTER DEFAULT PRIVILEGES FOR ROLE archguard_migrate IN SCHEMA public
  GRANT USAGE, SELECT ON SEQUENCES TO archguard_app;

-- 5. Privilégios em tabelas JÁ existentes (idempotente).
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO archguard_app;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO archguard_readonly;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO archguard_app;

-- 6. BARREIRA FÍSICA DO INV-2: o role da aplicação NÃO pode alterar nem apagar
--    registros de auditoria. Reafirme este bloco a cada nova tabela de auditoria
--    (o conjunto cresce no pacote 003 — selos etc.).
--
--    `audit_event` (pacote 003, T-005/006) é APPEND-ONLY: a app só insere e lê.
--    NÃO revogar em `audit_chain_head` — o cabeçalho de cadeia é um ponteiro
--    mutável que a escrita avança (UPDATE do last_seq/head_hash) sob trava; ele
--    não guarda evento, guarda o estado da cadeia. `record` é a auditoria legada.
--
--    Como `audit_event` é PARTICIONADA, o REVOKE precisa alcançar o pai E as
--    partições (o privilégio de DML é verificado na partição concreta que
--    recebe a linha). O bloco revoga no pai e em toda partição de `audit_event`.
DO $$
DECLARE
  audit_tables text[] := ARRAY['record', 'audit_event', 'audit_seal', 'global_access_audit'];
  t text;
  part text;
BEGIN
  FOREACH t IN ARRAY audit_tables LOOP
    IF EXISTS (SELECT FROM information_schema.tables
               WHERE table_schema = 'public' AND table_name = t) THEN
      EXECUTE format('REVOKE UPDATE, DELETE, TRUNCATE ON public.%I FROM archguard_app', t);
    END IF;
  END LOOP;
  -- Partições de audit_event (herdam GRANT, precisam do REVOKE explícito).
  FOR part IN
    SELECT c.relname
    FROM pg_inherits i
    JOIN pg_class c ON c.oid = i.inhrelid
    JOIN pg_class p ON p.oid = i.inhparent
    WHERE p.relname = 'audit_event'
  LOOP
    EXECUTE format('REVOKE UPDATE, DELETE, TRUNCATE ON public.%I FROM archguard_app', part);
  END LOOP;
END
$$;

-- Verificação (deve retornar 0 linhas): app com UPDATE/DELETE em tabela de auditoria.
-- SELECT grantee, table_name, privilege_type
--   FROM information_schema.role_table_grants
--  WHERE grantee = 'archguard_app' AND table_name IN ('record')
--    AND privilege_type IN ('UPDATE','DELETE');
