# Proposal — 002 · Identidade global e multi-tenancy B2B

## Por quê

O modelo herdado amarra o usuário a exatamente uma organização. Isso quebra o caso de uso
central do ArchGate: um operador que atende múltiplos tenants precisaria de múltiplas contas —
multiplicando credenciais e destruindo a rastreabilidade que é a razão de existir de um PAM
(ADR-0006).

## O que muda

- Promoção da relação usuário↔organização a entidade explícita (`membership`).
- Identidade global única com `sub` opaco e estável; credenciais e fatores MFA passam a
  pertencer à identidade, não ao membership.
- Papéis e permissões passam a referenciar `membership_id`.
- Contexto de tenant ativo por sessão, com troca explícita e auditada.
- Isolamento em duas barreiras: predicado obrigatório de repositório + RLS no PostgreSQL.
- Migração com deduplicação e fusão assistida de identidades.

## O que não muda

Protocolos de autenticação, fluxos OIDC e console (pacotes 005, 006 e 008).

## Impacto

- **Depende de:** 001.
- **Bloqueia:** 003 (cadeia de auditoria por tenant), 007 (objetos qualificados por tenant),
  008 (seletor de tenant).
- **Risco:** maior divergência estrutural do upstream; migração de dados irreversível na
  prática após a fusão.
