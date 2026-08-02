# ArchGuard — Referência de Integração e API

> Público: quem integra com o ArchGuard — aplicações que federam login (OIDC) e ferramentas que
> operam o plano de controle (`/api/v1`). Fonte da verdade dos contratos é o código + os documentos
> referenciados. Status: rascunho vivo (2026-08-02).

O ArchGuard expõe **duas superfícies** de integração:

| Superfície | Para quê | Onde |
|---|---|---|
| **Federação OIDC** | Apps que fazem **login** de usuários via ArchGuard | `docs/oidc/` (ver §1) |
| **Control Plane API `/api/v1`** | Ferramentas/console/automação que **operam** identidade e acesso | Este documento, §2 |

---

## 1. Federação OIDC (login de aplicações)

A integração OIDC — configurar a app-cliente, o fluxo Authorization Code + PKCE (web e mobile),
gerar o PKCE, validar o token — **já está documentada** em detalhe:

- **`docs/oidc/APP-INTEGRATION.md`** — o guia passo a passo (recomendações, endpoints de discovery,
  config no console, fluxo web e Flutter, validação do token).
- **`docs/oidc/CLAIMS-v1.md`** — o **contrato normativo** de claims (o alvo).
- **`docs/oidc/GUACAMOLE-EDGE.md`** — adaptação de borda para o Guacamole (via oauth2-proxy).

### 1.1 O que funciona hoje (leia com atenção)

**Hoje o OAuth é servido pela superfície herdada** (endpoints `/.well-known/openid-configuration`,
`/.well-known/jwks`, `/api/login/oauth/access_token|refresh_token|introspect`). Com `tokenFormat: JWT`,
o token traz **`name`** (o username) e **`groups`** (qualificados como `<org>/<grupo>`), além de `iss`
e `aud` (o `clientId` da app — **uma audience por componente**, o que impede um token de servir em
outro; ADR-0011).

**O contrato v1 rico** (`org`, `mid`, `acr`, `amr`, `sid`, `pcid`; PKCE obrigatório; rotação de
refresh com detecção de reuso; JWKS com `kid`) está **implementado e testado, mas NÃO montado** — ver
**ADR-0023** (montagem adiada, com o design pronto). **Não integre contra os claims v1 como se
estivessem no ar.** Consuma `name`/`groups` hoje; a migração para o contrato v1 será opt-in, cliente a
cliente, quando os endpoints forem montados.

### 1.2 Validação do token (obrigatória)

Todo consumidor deve validar assinatura (via JWKS), `iss`, **`aud` = o próprio `clientId`** e
expiração. Aceitar um token cuja `aud` não é a sua **quebra** o isolamento por componente. Detalhe em
`docs/oidc/APP-INTEGRATION.md` §"Validar o token".

---

## 2. Control Plane API (`/api/v1`)

A API que o console usa e que automações/ferramentas podem usar para **operar** o ArchGuard. Toda
regra de negócio nova vive aqui (não há endpoint "só para a tela" — se o console faz, é API pública
versionada).

### 2.1 Modelo de autenticação e autorização

- **Sessão.** A chamada é autenticada pela **sessão** estabelecida no login (o pipeline resolve a
  sessão do plano de controle). Sem sessão → **401**.
- **Escopo de tenant.** A organização é **sempre a do tenant ativo da sessão** — nunca vem do request
  (INV-1/INV-5). Trocar de tenant é uma operação própria (`POST /session/tenant`).
- **Gate de administração.** Operações de administração exigem `RequireAdmin`.
- **Nível de garantia (INV-8, ADR-0010).** Toda operação declara **L1/L2/L3**. L2/L3 exigem **step-up**
  reforçado (WebAuthn) — uma operação sem a garantia suficiente responde com um **desafio de step-up**
  (401, no espírito do RFC 9470), não executa.
- **Fail-closed.** Falha de PDP/cofre/auditoria ⇒ **negação** (INV-6). Distinção `denied` (403/negado)
  vs. `error` (5xx) é preservada.

### 2.2 Catálogo de endpoints

Todos sob o prefixo `/api/v1`. "Admin" = requer `RequireAdmin`.

**Sessão e tenant**
| Método | Caminho | Nível | O quê |
|---|---|---|---|
| GET | `/session` | L1 | Contexto da própria sessão (identity, org, membership, AAL, amr) |
| GET | `/tenants` | L1 | Tenants do próprio chamador (base do seletor de tenant) |
| POST | `/session/tenant` | L1 | Troca o tenant ativo da sessão (reemite token, audita) |
| GET | `/health` | L1 | Saúde dos subsistemas (database/custody/deployment) |

**Identidade (admin)**
| GET | `/memberships` | L1+admin | Roster do tenant ativo |
| POST | `/memberships/revoke` | L2+admin | Revoga um membership (encerra sessões, limpa o grafo) |

**Ativos e acesso granular (admin)**
| GET·POST | `/assets` | L1+admin | Catálogo de ativos (registrar/listar) |
| GET·POST | `/access-groups` | L1+admin | Catálogo de grupos de acesso (nome↔id) |
| GET·POST | `/group-memberships` | L1+admin | Vincula membership↔grupo |
| GET·POST | `/access-assignments` | L1+admin | Concede operator/auditor (subject membership ou grupo) sobre ativo |
| GET | `/access/effective` | L1+admin | Decisão do PDP para um par (membership, ativo) |
| GET | `/access/review` | L1+admin | Revisão reversa: quem alcança um ativo e a **origem** (direto/herdado/concessão) |

**Acesso privilegiado e break-glass**
| GET | `/grants` | L1+admin | Concessões privilegiadas vigentes |
| POST | `/grants/revoke` | L2+admin | Revoga uma concessão (encerra sessões derivadas) |
| POST | `/breakglass/request` | **L3** | Solicita acesso emergencial (justificativa + incidente) |
| GET | `/breakglass/pending` | L1+admin | Fila de aprovação |
| POST | `/breakglass/approve` | L2+admin | Aprova uma solicitação (por pares) |

**MFA / step-up**
| POST | `/stepup/totp` · `/stepup/webauthn/begin` · `/stepup/webauthn/finish` | — | Cerimônias de step-up |
| POST | `/factors/totp/begin` · `/factors/totp/verify` | — | Cadastro de fator TOTP |

**Auditoria**
| GET | `/audit/timeline` | L1+admin | Linha do tempo da trilha do tenant (append-only) |
| POST | `/audit/verify` | **L3** | Verifica a integridade da cadeia de hash |

### 2.3 Convenções

- **Erros** retornam `{"error": "<mensagem>"}` com o status HTTP apropriado; `denied` (decisão) é
  distinto de `error` (falha).
- **Escrita audita.** Operações que mudam estado registram evento imutável (INV-2); uma operação cuja
  auditoria falha **não acontece**.
- **Idempotência/ordem** seguem o contrato de cada operação; consulte a spec OpenSpec do pacote
  correspondente (`openspec/changes/**`) para os cenários WHEN/THEN normativos.

### 2.4 Conta de serviço (automação)

Para automação sem usuário interativo, o modelo é uma **app confidencial com `client_credentials`**
(ex.: `archgate-console-sa`): o cliente troca `clientId`+`clientSecret` por um token e o usa como
Bearer contra a API de gestão. É rotacionável e sem segredo de vida longa (preferível a chaves
estáticas). O segredo vive na custódia do consumidor, nunca em query string.

---

*Ver também: `docs/produto/01-visao-e-modelo-de-seguranca.md` (garantias), `docs/produto/02-guia-do-operador.md`
(operação), e as specs OpenSpec (`openspec/changes/**`) para o contrato normativo de cada capacidade.*
