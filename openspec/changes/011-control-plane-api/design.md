# Design — 011 · Control Plane API

Base normativa: RFC-0001 (arquitetura de referência), RFC-0002 (tenant/RLS), ADR-0016 (Beego
mantido), ADR-0017 (perfis e custódia), ADR-0004/RFC-0005 (o console consome esta API).

## Ponto de partida (fatos do código)

- Handlers prontos e testados em `internal/http`: **SCIM** (Users/Groups), **OIDC**
  (discovery/jwks/authorize/token/introspection/logout, agregados por `OIDCServer.Handler()`),
  **audit-verify**, o middleware de garantia `AssuranceMiddleware.Require(op, next)` e a porta
  `SessionResolver` (impl. `BridgingResolver`).
- Stores `postgres` prontos, em três padrões de construção: `Querier` (pool ou tx),
  `Beginner` (pool abre a própria tx), e `*TenantTx` (tenant obrigatório, construído dentro de
  `TenantRepository.WithTenantTx`, que faz `SET LOCAL` do org para o RLS — as duas barreiras).
- **Não existe** pool de runtime, factory de adapters por perfil, nem qualquer montagem no
  Beego. `postgres.NewPool` existe mas nunca é chamado.
- O padrão de ponte Beego→net/http já em uso é o do SCIM legado (`controllers/scim.go`):
  rota wildcard → método de controller que faz auth, apara o prefixo e chama `ServeHTTP`.
- O `swagger.json` é estático, gerado de anotações `@router` de controllers Beego; handlers
  net/http não entram nele.

## Decisões

### D1 — Composition root no repo principal, como camada de infra
Um pacote novo (proposto: `internal/boot`, ou `object/apiwiring.go` se precisar do `conf`
Beego) que **importa framework e pgx** — é a camada autorizada a isso; `internal/domain/**`
permanece puro (INV-3, deps-check segue verde). Ele inicializa **um** `*pgxpool.Pool` de
runtime via `postgres.NewPool(ctx, conf.GetConfigDataSourceName())` **após** `RunMigrations()`,
e é dono do seu ciclo de vida (Close no shutdown).

### D2 — Montagem Beego→net/http pelo padrão SCIM comprovado
Nada de `web.Handler` (não é usado no repo). Um controller-ponte monta cada `http.Handler`
sob `/api/v1/*`: `web.Router("/api/v1/*", &ControlPlaneController{}, "*:Handle")`, que faz o
gate de sessão, apara `/api/v1` e delega ao mux que compõe os handlers. `OIDCServer.Handler()`
já entrega um mux pronto para esse esquema.

### D3 — Tenant e garantia no pipeline montado
Todo endpoint de domínio passa pelo `AssuranceMiddleware.Require(operationID, next)` (INV-8:
garantia insuficiente ⇒ 401/challenge RFC 9470, fail-closed) e resolve tenant fora do store
(`OrgResolver`/`SessionResolver`). Stores tenant-scoped são construídos **dentro** de
`WithTenantTx` (INV-5: predicado de tenant + RLS). O seam **`LegacyBinding`** (adapter Beego que
lê identidade+sessão da sessão do framework Casdoor) é construído aqui — é o que liga o login
legado herdado à sessão nova (`BridgingResolver` + `postgres.SessionBridge`).

### D4 — Seleção de adapter por perfil, centralizada e fail-closed
Hoje a seleção por perfil está dispersa (cada provisional consulta `deploy.Active()`). O
composition root centraliza numa factory: perfil **dev** → keystore local / provisional;
perfil **conforme** → exige o backend real (OpenBao, cujo *endpoint/infra* é config do
`archguard-devops`) e, se ele não estiver disponível, **recusa servir a capacidade** (INV-6,
INV-7 — nunca expõe custódia dev em produção; ADR-0017).

### D5 — Duas naturezas de trabalho, nenhuma regra de domínio nova
1. **Montar o que existe**: SCIM, OIDC (compondo as portas `AuthCodeIssuer`/`AuthCodeGrant`/
   `RefreshGrant`/`EndSession` a partir dos stores + `oidc.Signer`), audit-verify.
2. **Handlers finos novos de leitura/comando para o console** sobre stores já testados:
   identidades/memberships/grupos, organização/políticas, ativos/concessões, break-glass,
   timeline de auditoria + correlação `pcid`, revisão de acesso, saúde, chaves/rotação. São
   **handlers finos** (traduzem HTTP ↔ domínio; regra vive no domínio já pronto), não
   controllers Beego com lógica.

### D6 — Contrato OpenAPI 3 próprio do `/api/v1`, escrito à mão, com gate de CI
O `swagger.json` legado (anotação `@router`) não cobre net/http. O `/api/v1` ganha um contrato
**OpenAPI 3** versionado como fonte da verdade (RFC-0005 "contrato primeiro"). Gate de CI:
endpoint montado sem entrada no contrato ⇒ falha; é dele que o 008 (T-002) gera o cliente.

### D7 — O console NÃO mistura endpoints legados Casdoor
O contrato do console é o `/api/v1` coerente sobre a arquitetura hexagonal (tenant, garantia,
domínio de PAM). Endpoints legados do Casdoor não entram no contrato do console — evita
reintroduzir o modelo mental de IAM genérico que o ADR-0004 rejeita.

## Regras que o wiring preserva
- **Uma transação por operação de negócio** (RFC-0002 §5): stores compartilham o `TenantTx`.
- **Nunca chamada remota dentro de transação** (RFC-0004 §4): envios (logout back-channel,
  projeção) via outbox/best-effort fora da tx.
- **Handlers finos** (CLAUDE.md §6): nenhuma regra nova em controller.
- **`denied` ≠ `error`** na auditoria.

## Fronteira com o archguard-devops
Fica **aqui**: pool de runtime e seu ciclo de vida, factory de seleção por perfil, montagem de
rotas, contrato OpenAPI. Fica **no devops**: endpoint/infra do OpenBao, *sizing* do pool de
produção, exportador OTLP, e os artefatos de deploy (pacote 001/010). Atualizar
`docs/DEVOPS-HANDOFF.md` para refletir que o composition root deixou de ser tarefa do devops.
