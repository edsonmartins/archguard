# Proposal — 006 · Federação OIDC com os componentes do ArchGate

## Por quê

Warpgate, Guacamole, NetBird, OpenBao e o proxy Oracle JDBC precisam confiar em uma única
fonte de identidade. Sem contrato explícito, cada integração vira acordo ad-hoc: claims
divergentes, grupos com semântica incompatível e impossibilidade de correlacionar auditoria
entre planos — que é justamente o valor do PAM (ADR-0011).

## O que muda

- Contrato de claims versionado (`org`, `mid`, `acr`, `amr`, `sid`, `act`, `pcid`, `grant_ref`).
- Cliente OIDC dedicado por componente, com audiência própria e escopo mínimo.
- Ciclo de vida de tokens com rotação de refresh e **detecção de reuso**.
- Back-channel logout propagado; revogação efetiva de sessões derivadas.
- Rotação de JWKS com sobreposição e `kid`.
- **Suíte de conformidade por componente como gate de release.**

## O que não muda

Federação de entrada com IdPs corporativos do cliente é escopo do pacote 009.

## Impacto

- **Depende de:** 001, 002, 003, 005.
- **Bloqueia:** virada de componentes (RFC-0007).
- **Risco:** limitações reais das implementações OIDC dos componentes (Guacamole, device flow
  do NetBird).
