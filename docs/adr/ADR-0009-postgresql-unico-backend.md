# ADR-0009 — PostgreSQL 15+ como único backend de persistência

- **Status:** Aceito
- **Data:** 2026-07-19
- **Invariantes tocados:** I-7.1, I-6.3, I-5.2

## Contexto

O upstream suporta múltiplos bancos (MySQL, PostgreSQL, SQLite, SQL Server, entre outros) via
ORM. Essa flexibilidade tem custo direto sobre decisões já tomadas:

- **RLS (Row-Level Security)** é peça central do isolamento de tenant (ADR-0006) e não tem
  equivalente portável.
- **Restrições de escrita da auditoria** (negação de UPDATE/DELETE, triggers de bloqueio)
  dependem de recursos e semântica específicos (ADR-0007).
- Tipos e recursos que queremos usar — `jsonb` com índices GIN, `pgcrypto`, particionamento
  declarativo por tempo para a trilha, `SERIALIZABLE` bem-comportado, extensões — não são
  denominador comum.
- Manter N dialetos multiplica a matriz de testes de um subsistema de segurança.

PostgreSQL 15+ já é o padrão de banco primário da IntegrAllTech.

## Decisão

**Suportar exclusivamente PostgreSQL 15+.** Os demais dialetos são removidos da árvore de
build, da configuração e da documentação.

Consequências técnicas adotadas:
- **Migrations versionadas e ordenadas**, aplicadas de forma controlada e idempotente, com
  travamento para evitar aplicação concorrente em múltiplas réplicas.
- **RLS habilitada** nas tabelas de domínio como segunda barreira de isolamento (a primeira é
  o predicado obrigatório de repositório).
- **Papéis de banco segregados**: papel da aplicação **sem** `UPDATE`/`DELETE` nas tabelas de
  auditoria; papel de migração separado; papel de leitura para relatórios.
- **Particionamento por tempo** na trilha de auditoria, viabilizando arquivamento sem deleção
  lógica.
- **Extensões**: `pgcrypto` (crypto-shredding, ADR-0014); `pg_stat_statements` para
  observabilidade.
- Backup/PITR documentado como requisito de operação, não como opcional.

## Alternativas consideradas

| Opção | Por que não |
|---|---|
| Manter multi-database | Impede RLS e as restrições de auditoria; multiplica matriz de testes de segurança sem demanda real de mercado |
| Adicionar Oracle (base instalada de clientes Consinco) | O ArchGuard **não** roda no banco do cliente; o acesso a Oracle é responsabilidade do proxy JDBC do ArchGate. Persistência de identidade é do produto |
| Abstrair por interface de repositório para trocar depois | Abstração vazada: os recursos que sustentam invariantes de segurança **são** específicos do PostgreSQL |

## Consequências

- Simplificação relevante de código, testes e documentação.
- Perda de mercado teórico de clientes que exigiriam outro banco — **aceita**: instalar um
  PostgreSQL dedicado é requisito trivial em qualquer cliente que compre PAM.
- Cherry-picks do upstream que toquem a camada de persistência exigem adaptação manual.
