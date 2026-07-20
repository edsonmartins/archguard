# Mapa de remoção de escopo (T-007)

> Relatório do T-007 (pacote 001). Mapeia os módulos fora de escopo (ADR-0015 §2, ADR-0019),
> seus arquivos, consumidores internos e **riscos de acoplamento não óbvio** (o modo de falha
> que o ADR-0015 §"Negativas" adverte). Alimenta T-008…T-014 e o `DIVERGENCE.md`. Remoções em
> commits pequenos e independentes, build+test a cada uma, contagem INV-4 antes/depois.

## Contagem INV-4 de partida (2026-07-20)

17 achados: **7 MPL-2.0** (proibidas no regime vigente) + **~9 indeterminadas** (fail-closed) +
0 GPL (goldap já removido em T-010a). Alvo: cada remoção abaixo reduz esta contagem de forma
verificável.

## Alvos por tarefa

### T-008 — Pagamento / produto / assinatura
- **Arquivos:** `object/{order,order_pay,payment,plan,pricing,product,subscription}.go`;
  `controllers/{payment,plan,pricing,product,subscription}.go`; `pp/` inteiro (adyen, airwallex,
  alipay, balance, dummy, fastspring, gc, lemonsqueezy, paddle, paypal, stripe, wechat…).
- **Reduz INV-4:** `github.com/hashicorp/go-cleanhttp` (MPL, via PaddleHQ) e possivelmente
  outras deps de gateway de pagamento.
- **⚠️ Acoplamento:** `object/user.go` referencia saldo/pagamento do usuário. Remover exige
  editar `user.go` (campo `Balance`, métodos de pagamento) sem quebrar o núcleo de identidade.
  **Risco alto** — é o acoplamento não óbvio típico. Tratar user.go como cirurgia mínima.

### T-009 — Agentes de IA / MCP
- **Arquivos:** `object/{agent,mcp_server}.go`; `controllers/mcp_server.go`;
  `routers/mcp_util.go`; `mcpself/`; refs em `controllers/server*.go`, `routers/{base,router,
  authz_filter,auto_signin_filter}.go`.
- **Reduz INV-4:** a verificar (deps específicas de MCP).
- **⚠️ Acoplamento:** MCP está entrelaçado nos routers e no filtro de authz — remoção exige
  cuidado para não quebrar o pipeline de rotas. **Risco médio-alto.**

### T-010 — Provedores fora do catálogo curado (ADR-0015 §3)
- **IdP (`idp/`, 33 arquivos):** manter Entra/AD, Google, Okta, SAML/OIDC genéricos, GitHub,
  GitLab. Remover os não-curados (baidu, bilibili, dingtalk, douyin, kwai, lark, qq, telegram,
  wechat*, weibo, wecom*, infoflow*, metamask, web3onboard, twitter, facebook, linkedin…).
- **Notificação:** `notification/matrix.go` → **reduz INV-4** (`maunium.net/go/mautrix`,
  `go.mau.fi/util` — MPL). Revisar demais provedores de notificação não curados.
- **faceId / idv (Alibaba):** `faceId/`, `idv/` → **reduz INV-4** (6 `alibabacloud-go/*`
  indeterminados). **⚠️ Acoplamento:** `object/user.go` e `object/provider.go` referenciam
  faceId/idv. Reconhecimento facial e verificação de identidade não são PAM — remover, com
  edição mínima em user.go/provider.go. **Risco médio.**
- **captcha:** avaliar aliyun/geetest/hcaptcha não curados; manter default/recaptcha/turnstile.
- **freetype:** `go mod why` indica que o módulo principal **não precisa** de freetype —
  provável entrada órfã de `go.sum`, candidata a sair com `go mod tidy` após as remoções.
  **Confirmar aqui**; se sumir, a eleição pendente em `LICENSE_ELECTIONS.md` é retirada, não
  registrada. `mrjones/oauth` (OAuth1, indeterminado): verificar consumidor e eleger/remover.

### T-011 — Senha-mestra (INV-1) — CRÍTICA
- **Arquivos:** `object/{check,organization,application_util}.go` (as 3 entradas do
  `known_violations.txt`), + coluna `master_password` no esquema da organização (migration).
- **Subtarefa obrigatória:** deletar `test/invariants/known_violations.txt` **no mesmo commit**;
  a suíte passa a falhar se ele reaparecer (trava (c)).
- **⚠️** Detector INV-1 ativo desde T-018 — a remoção nasce verificada por teste.

### T-012 — Dialetos de banco não-PostgreSQL (ADR-0009)
- **Arquivos:** `object/ormer.go`, `object/syncer_database.go`, `sync*/` (drivers).
- **Reduz INV-4:** `github.com/go-sql-driver/mysql` (MPL) e `modernc.org/*` (SQLite,
  indeterminados) — os maiores blocos de achados indeterminados.
- **⚠️ Acoplamento:** XORM configura o dialeto no `ormer`; remover MySQL/SQLite/SQLServer sem
  quebrar a inicialização PostgreSQL. **Risco médio.**

### T-013 / T-014 — Migrations versionadas + papéis de banco segregados
- Sem impacto em INV-4; infra de persistência (ADR-0009). Ver design.md.

## MPL sobreviventes (pendência ADR-0019, não removíveis por escopo)
- `github.com/hashicorp/golang-lru` (via **Beego** — fica, ADR-0016);
- `github.com/hashicorp/go-uuid` (via **gokrb5**, Kerberos/LDAP cliente — fica);
- `layeh.com/radius` (**servidor RADIUS** — fica, ADR-0019).

Estas **não** saem no Bloco 3. Permanecem vermelhas até a ratificação do ADR-0019 flipar
`mplLinkedAllowed`. É o resíduo esperado do INV-4 após o Bloco 3.
