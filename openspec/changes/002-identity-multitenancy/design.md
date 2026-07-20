# Design — 002 · Identidade global e multi-tenancy B2B

Base normativa: RFC-0002.

## Esquema

Novas tabelas: `identity`, `membership`. `organization` estendida com políticas.
Tabelas herdadas de usuário são migradas e passam a apontar para `identity`/`membership`.

Campos pessoais (`primary_email_enc`, `display_name_enc`, `attributes_enc`) são cifrados por
chave de titular; `email_hash` (HMAC com chave de deployment) sustenta unicidade e login sem
descriptografar. A gestão de chaves entra plenamente no pacote 010 — aqui a interface
`KeyCustodian` é introduzida com implementação provisória claramente marcada como não
suportada em produção.

## Isolamento

**Barreira 1:** não existe construtor de repositório sem contexto de tenant. Consulta
*cross-tenant* usa tipo distinto (`GlobalRepository`), com autorização própria e auditoria.

**Barreira 2:** RLS por `organization_id` em todas as tabelas de domínio; parâmetro de sessão
definido pela aplicação a cada transação; papel da aplicação sem `BYPASSRLS`.

## Sessão e tenant ativo

A sessão carrega `identity_id` + `membership_id` ativo. Troca de tenant:
1. valida membership ativo;
2. reavalia política de MFA do destino (pode exigir step-up — integração no pacote 005);
3. **emite novo token**;
4. registra evento de auditoria.

Token nunca carrega dados de mais de um tenant simultaneamente.

## Precedência de políticas

Para identidade com múltiplos memberships, a política aplicada é a do **tenant ativo**. Quando
uma decisão for global (ex.: exigência mínima de fator para a identidade), vence a mais
restritiva entre os tenants ativos.

## Migração (RFC-0002, §6)

Ordem: inventário → deduplicação por `email_hash` → relatório de conflito → fusão assistida →
migração de credenciais e fatores → backfill de `organization_id` → ativação de RLS por
tabela → validação.

**Fusão automática silenciosa é proibida.** Conflitos vão para revisão humana.

## Convivência XORM/pgx

Tabelas novas em `pgx`. Operações que tocam ambos os mundos compartilham **uma transação**
(RFC-0002, §5).
