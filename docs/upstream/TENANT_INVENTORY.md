# Inventário de tenancy — tabelas de domínio

Este é o **inventário** referenciado pela regra R1 do RFC-0002: toda tabela de domínio possui
`organization_id` **NOT NULL** — exceto as explicitamente listadas aqui como *cross-tenant*.
Ele classifica cada tabela herdada do Casdoor e guia o backfill de `organization_id` (T-007) e a
ativação de RLS por tabela (T-010).

Legenda:
- **tenant-scoped** — recebe `organization_id uuid` (FK `organization.id`), populado a partir do
  `owner`. RLS por `organization_id`.
- **cross-tenant** — isenta de `organization_id` por construção (identidade global, credencial,
  configuração global, ou a própria fronteira de tenant). R1 exceção.
- **fora de escopo PAM** — não recebe coluna: billing, UI-only ou funcionalidade removida/irrelevante.

## Linhas globais dentro de tabelas tenant-scoped

Decisão do arquiteto (2026-07-20): linhas com `owner = 'admin'`, a org raiz `built-in`, e
aplicações `IsShared` são **configuração global compartilhada** — permanecem com
`organization_id = NULL` (cross-tenant), registradas aqui. Por isso a coluna só vira **NOT NULL**
(no T-010) nas tabelas cujas linhas **100%** mapeiam a uma organização; onde há linha global, a
coluna fica **nullable** e a RLS trata `NULL` como visível globalmente (leitura controlada).

## Tenant-scoped — backfill no T-007 (núcleo PAM)

| Tabela | owner→org | Linhas globais? | NOT NULL no T-010 |
|---|---|---|---|
| `user` | sim | não | sim |
| `group` | sim | não | sim |
| `role` | sim (já com `id` uuid, T-006) | não | sim |
| `permission` | sim | não | sim |
| `token` | sim | não | sim |
| `session` | sim | não | sim |
| `invitation` | sim | não | sim |
| `resource` | sim | não | sim |
| `syncer` | sim | não | sim |
| `webhook` | sim | não | sim |
| `application` | sim | **sim** (`IsShared`, `owner=admin`) | não (nullable) |
| `provider` | sim | **sim** (`owner=admin` compartilhado) | não (nullable) |
| `cert` | sim | **sim** (pode ser `admin`) | não (nullable) |
| `adapter` | sim | não | sim¹ |
| `enforcer` | sim | não | sim¹ |
| `model` | sim | não | sim¹ |

¹ `adapter`/`enforcer`/`model` são configuração **Casbin**, superada pelo OpenFGA (pacote 007,
I-7.4). Recebem `organization_id` para não deixar buraco de RLS enquanto existirem, mas estão em
rota de remoção — não construir sobre elas.

## Cross-tenant — isentas de `organization_id` (R1)

| Tabela | Motivo |
|---|---|
| `identity` | Identidade global única do deployment (RFC §2.1); credenciais/fatores pertencem a ela |
| `credential` | Fatores da identidade global (RFC §2.4); cross-tenant como `identity` |
| `organization` | É a própria fronteira de tenant (tem `id` uuid, T-002) |

Além destas, as **linhas globais** (`owner=admin`, `built-in`, `IsShared`) dentro das tabelas
tenant-scoped acima são cross-tenant (org_id NULL), conforme a decisão de linhas globais.

## Tabelas novas do modelo (pgx) — nascidas com `organization_id`

| Tabela | organization_id | RLS |
|---|---|---|
| `membership` | NOT NULL (nasce assim, 0004) | ligada + FORCE (T-010) |
| `role_assignment` | NOT NULL (nasce assim, 0009) | ligada + FORCE (T-010) |
| `auth_session` | **nullable por desenho** (0012): sessão `pending_selection` — identidade autenticada com >1 membership, tenant ainda não selecionado — não tem organização; o CHECK `auth_session_tenant_shape` exige org NOT NULL quando `active` | **ligada + FORCE (0013, T-012)** com o contexto de identidade `app.current_identity` (SET LOCAL pelo `IdentityRepository`): lê identidade própria OU org corrente OU `global_read`; escreve identidade própria OU org corrente. Linhas pendentes (org NULL) só pelo eixo da identidade |

A tabela legada `session` (XORM, ids de sessão Beego) permanece e segue a regra tenant-scoped da
tabela acima. A ponte de revogação entre `auth_session` e ela está **diferida**: não existe ainda
vínculo identity↔user legado no esquema (chega com o ensaio de migração T-019 e o rewiring de
auth dos pacotes 005/006); a cascata do T-014 opera o modelo novo (`membership`/`auth_session`).
A RLS de `membership` foi emendada na 0014 com o eixo de identidade (`app.current_identity`)
para a cascata identidade→memberships→sessões rodar em uma transação.

## Fora de escopo PAM — sem coluna

`coupon`, `transaction`, `ticket`, `form`, `entry`, `radius_accounting`, `site`, `rule`, `key`,
`third_party_link` — billing, UI-only ou funcionalidade fora do escopo de PAM (ver DIVERGENCE.md).

## Deferidas (tenant-scoped, backfill posterior)

`verification_record` (dados de verificação efêmeros) e `webhook_event` (log de eventos de alto
volume) são tenant-scoped mas ficam para um backfill posterior, fora do núcleo do T-007.
