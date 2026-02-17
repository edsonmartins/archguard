# ArchGuard Console — ADRs & RFCs

### Architecture Decision Records & Request for Comments

---

**Versão:** 1.0  
**Data:** Fevereiro 2026  
**Autor:** IntegrAllTech  
**Projeto:** ArchGuard Console  
**Status:** Draft — Awaiting Review  

---

## Índice

### Architecture Decision Records (ADRs)

| # | Título | Status |
|---|---|---|
| [ADR-001](#adr-001) | Framework — TanStack Start | Aceito |
| [ADR-002](#adr-002) | UI Component Library — Shadcn/ui + Tailwind | Aceito |
| [ADR-003](#adr-003) | State Management — TanStack Query + Store | Aceito |
| [ADR-004](#adr-004) | Autenticação — OIDC PKCE via Kanidm | Aceito |
| [ADR-005](#adr-005) | API Client — Gerado via OpenAPI Generator | Aceito |
| [ADR-006](#adr-006) | Autorização — RBAC por Grupos OIDC | Aceito |
| [ADR-007](#adr-007) | Multi-Tenancy — Grupo hierárquico com filtro no Console | Aceito |
| [ADR-008](#adr-008) | Internacionalização — PT-BR first, i18n ready | Aceito |
| [ADR-009](#adr-009) | Observabilidade — Structured Logging + OpenTelemetry | Aceito |
| [ADR-010](#adr-010) | Deploy — Container Docker multi-stage | Aceito |

### Request for Comments (RFCs)

| # | Título | Status |
|---|---|---|
| [RFC-001](#rfc-001) | Especificação de Funcionalidades e Fluxos de UX | Draft |
| [RFC-002](#rfc-002) | Modelo de Dados e Contratos de API | Draft |
| [RFC-003](#rfc-003) | Estrutura de Rotas, Componentes e State Management | Draft |
| [RFC-004](#rfc-004) | Regras de Negócio e Permissões por Role/Grupo | Draft |

---

# ARCHITECTURE DECISION RECORDS

---

<a id="adr-001"></a>
## ADR-001: Framework — TanStack Start

**Status:** Aceito  
**Data:** 2026-02-16  
**Decisores:** Edson (CTO), Equipe IntegrAllTech  

### Contexto

O ArchGuard Console é uma SPA administrativa que consome APIs REST do Kanidm e AliasVault. Precisa de: file-based routing, data loading eficiente, autenticação via OIDC, SSR opcional para SEO de docs públicas, e runtime leve para rodar em container Docker mínimo.

### Opções Avaliadas

| Critério | TanStack Start | Next.js (App Router) | Remix | Vite + React Router |
|---|---|---|---|---|
| Bundle size | ~45KB | ~85KB+ | ~60KB | ~30KB |
| SSR | Opcional (opt-in) | Obrigatório (RSC) | Obrigatório | Manual |
| File-based routing | Sim (type-safe) | Sim | Sim | Não |
| Data loading | TanStack Query nativo | Server Actions/fetch | Loaders | Manual |
| Type safety | End-to-end (rotas → params → data) | Parcial | Parcial | Manual |
| Server functions | Sim (RPC-like) | Server Actions | Actions | Não |
| Vinculou opinião | Flexível | Forte (Vercel-centric) | Flexível | Mínima |
| Complexidade | Média | Alta (RSC mental model) | Média | Baixa |
| Ecossistema TanStack | Nativo (Query, Router, Table, Form) | Plugin | Plugin | Plugin |

### Decisão

**TanStack Start** — Full-stack framework baseado em TanStack Router + Vinxi.

### Justificativas

1. **Type-safe routing end-to-end** — Rotas, parâmetros, search params e loaders tipados em tempo de compilação. Elimina classe inteira de bugs em app com muitas rotas (Console tem 40+ rotas).

2. **TanStack Query integrado nativamente** — O Console é fundamentalmente uma UI sobre APIs REST. TanStack Query é o melhor client para isso (cache, mutations, optimistic updates, polling). No TanStack Start, os loaders das rotas usam Query nativamente.

3. **SSR opcional, não obrigatório** — Console admin é SPA; SSR é overhead desnecessário para 95% das páginas. Podemos habilitar SSR apenas para landing/docs públicas. Next.js força RSC como default.

4. **Sem vendor lock-in** — Next.js otimiza para deploy na Vercel. ArchGuard é self-hosted. TanStack Start roda em qualquer runtime Node.js ou container.

5. **Bundle leve** — Container Docker menor, startup rápido, menos recursos. Importa para deploy self-hosted onde o cliente pode ter infra limitada.

6. **Ecossistema coeso** — TanStack Table (tabelas de usuários/grupos), TanStack Form (formulários complexos de CRUD), TanStack Query (data fetching) — tudo integrado sem wrappers.

### Consequências

- Ecossistema mais novo que Next.js — menos tutoriais e exemplos comunitários
- Equipe precisa aprender TanStack Router (diferente de React Router)
- Deploy: Node.js server ou static export, não há hosting otimizado como Vercel
- Documentação do TanStack Start ainda em evolução (beta → stable em 2025)

### Riscos e Mitigações

| Risco | Mitigação |
|---|---|
| TanStack Start ainda jovem | Core (Router, Query) é battle-tested com milhões de downloads. Start é a cola. |
| Menos exemplos comunitários | Documentação oficial é excelente; equipe tem senioridade para navegar |
| Breaking changes | Lock de versão no package.json; testes e2e cobrem rotas críticas |

---

<a id="adr-002"></a>
## ADR-002: UI Component Library — Shadcn/ui + Tailwind CSS

**Status:** Aceito  
**Data:** 2026-02-16  

### Contexto

Console administrativo precisa de componentes ricos (DataTable, Dialog, Command Palette, Tabs, Forms) com aparência profissional, customizável e acessível.

### Opções Avaliadas

| Critério | Shadcn/ui | Ant Design | MUI (Material) | Mantine |
|---|---|---|---|---|
| Ownership do código | Copia para o repo | Dependência | Dependência | Dependência |
| Customização | Total (é seu código) | Temas limitados | Theme override | Boa |
| Bundle impact | Só o que usar | Pesado (~1MB) | Pesado | Médio |
| Acessibilidade | Radix primitives (ARIA) | Boa | Boa | Boa |
| Design system | Tailwind-native | Próprio | Material Design | Próprio |
| Componentes admin | Excelente (Table, Form, Sheet) | Excelente | Bom | Bom |

### Decisão

**Shadcn/ui** com Tailwind CSS v4.

### Justificativas

1. **Ownership total** — Componentes copiados para o repo, não dependência. Podemos modificar livremente sem conflitos de versão.
2. **Radix UI por baixo** — Primitivos acessíveis, composáveis, headless. ARIA compliance out of the box.
3. **Tailwind-native** — Sem CSS-in-JS, sem runtime overhead. Classes utilitárias são tree-shaked.
4. **Componentes admin-ready** — DataTable (baseado em TanStack Table), Command (command palette), Sheet (sidepanels), Dialog, Tabs, Form (react-hook-form integrado) — tudo que um Console precisa.
5. **Dark mode trivial** — CSS variables + `dark:` classes.

### Componentes Shadcn que serão utilizados

| Componente | Uso no Console |
|---|---|
| `DataTable` | Listagem de persons, groups, OAuth2 clients |
| `Dialog` | Confirmações, modais de criação |
| `Sheet` | Painéis laterais de detalhes |
| `Command` | Command palette (⌘K) para navegação rápida |
| `Form` | Todos os formulários de CRUD |
| `Tabs` | Navegação dentro de detalhes (atributos, grupos, credenciais) |
| `Card` | Dashboard widgets |
| `Badge` | Status indicators (ativo, bloqueado, pendente) |
| `Toast` | Notificações de ações |
| `Breadcrumb` | Navegação hierárquica |
| `Sidebar` | Navegação principal do Console |
| `Alert` | Avisos de segurança, erros |
| `Skeleton` | Loading states |
| `DropdownMenu` | Ações contextuais |
| `Avatar` | Foto/iniciais do usuário |

---

<a id="adr-003"></a>
## ADR-003: State Management — TanStack Query + TanStack Store

**Status:** Aceito  
**Data:** 2026-02-16  

### Contexto

Console consome duas APIs (Kanidm REST + AliasVault REST) e precisa de: cache inteligente, invalidação após mutations, polling para dados live, estado global para auth/preferences.

### Decisão

| Tipo de Estado | Solução | Justificativa |
|---|---|---|
| Server state (API data) | **TanStack Query v5** | Cache, background refresh, mutations, optimistic updates |
| Client state (UI) | **TanStack Store** | Leve, reativo, integrado ao ecossistema |
| Auth state | **oidc-client-ts** + TanStack Store | Token, user info, refresh |
| Form state | **TanStack Form** | Validação, type-safe, integrado com Query mutations |
| URL state | **TanStack Router** search params | Filtros, paginação, tabs ativas persistem na URL |

### Query Key Strategy

```
Convenção: [domínio, recurso, ...identificadores]

["id", "persons"]                    → lista de persons
["id", "persons", "abc123"]          → person específica
["id", "persons", "abc123", "creds"] → credenciais de uma person
["id", "groups"]                     → lista de grupos
["id", "groups", "gid", "members"]   → membros de um grupo
["id", "oauth2"]                     → lista de OAuth2 clients
["id", "oauth2", "vendax"]           → client específico
["id", "system", "status"]           → health check Kanidm
["vault", "status"]                  → health check AliasVault
["vault", "stats"]                   → estatísticas do vault
```

### Invalidação após Mutations

```
Mutation: criar person    → invalidar ["id", "persons"]
Mutation: editar person   → invalidar ["id", "persons", id]
Mutation: add to group    → invalidar ["id", "groups", gid, "members"]
                            + invalidar ["id", "persons", personId]
Mutation: criar OAuth2    → invalidar ["id", "oauth2"]
Mutation: reset creds     → invalidar ["id", "persons", id, "creds"]
```

### Stale/Cache Times

| Dado | Stale Time | Cache Time | Justificativa |
|---|---|---|---|
| Person list | 30s | 5min | Muda com frequência moderada |
| Person detail | 60s | 10min | Menos volátil |
| Group list | 60s | 10min | Muda raramente |
| OAuth2 clients | 5min | 30min | Quase nunca muda |
| System status | 10s | 30s | Precisa ser fresh |
| Vault stats | 30s | 2min | Informativo |

---

<a id="adr-004"></a>
## ADR-004: Autenticação — OIDC PKCE via Kanidm

**Status:** Aceito  
**Data:** 2026-02-16  

### Contexto

Console precisa autenticar administradores contra o ArchGuard ID (Kanidm) usando OIDC. Kanidm exige PKCE S256 para todos os OAuth2 clients.

### Decisão

**oidc-client-ts** com Authorization Code + PKCE S256.

### Fluxo de Autenticação

```
┌──────────────┐     ┌────────────────────┐     ┌──────────────┐
│   Console    │     │  ArchGuard Gateway  │     │ ArchGuard ID │
│   (Browser)  │     │     (nginx)         │     │  (Kanidm)    │
└──────┬───────┘     └─────────┬──────────┘     └──────┬───────┘
       │                        │                       │
       │  1. Acessa /dashboard  │                       │
       │───────────────────────>│                       │
       │                        │                       │
       │  2. Sem token → redirect to /authorize         │
       │<─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─│                       │
       │                        │                       │
       │  3. GET /authorize (code_challenge + state)    │
       │───────────────────────────────────────────────>│
       │                        │                       │
       │  4. Login page (username + password/passkey)   │
       │<───────────────────────────────────────────────│
       │                        │                       │
       │  5. User autentica     │                       │
       │───────────────────────────────────────────────>│
       │                        │                       │
       │  6. Redirect /callback?code=xxx&state=yyy      │
       │<───────────────────────────────────────────────│
       │                        │                       │
       │  7. POST /token (code + code_verifier)         │
       │───────────────────────────────────────────────>│
       │                        │                       │
       │  8. { access_token, id_token, refresh_token }  │
       │<───────────────────────────────────────────────│
       │                        │                       │
       │  9. API calls com Bearer token                 │
       │───────────────────────>│  proxy /api/id/ →     │
       │                        │───────────────────────>│
       │                        │                       │
```

### Configuração OIDC

```typescript
// auth/oidc-config.ts
import { UserManager, WebStorageStateStore } from 'oidc-client-ts';

export const OIDC_CONFIG = {
  authority: `${import.meta.env.VITE_ARCHGUARD_ID_URL}/oauth2/openid/archguard-console`,
  client_id: 'archguard-console',
  redirect_uri: `${window.location.origin}/auth/callback`,
  post_logout_redirect_uri: window.location.origin,
  response_type: 'code',
  scope: 'openid profile email groups',
  automaticSilentRenew: true,
  userStore: new WebStorageStateStore({ store: sessionStorage }),
  // Kanidm PKCE S256 é automático no oidc-client-ts
};

export const userManager = new UserManager(OIDC_CONFIG);
```

### Token Claims (Kanidm OIDC)

```typescript
interface ArchGuardIdToken {
  sub: string;           // UUID do usuário
  name: string;          // Display name
  email: string;         // Email
  email_verified: boolean;
  groups: string[];      // ["archguard_admins", "rio_quality_admins", ...]
  iss: string;           // Issuer URL
  aud: string;           // Client ID
  exp: number;           // Expiration
  iat: number;           // Issued at
  nonce: string;         // PKCE nonce
  s_claims: {            // Scoped claims
    [clientId: string]: {
      groups: string[];
    };
  };
}
```

### Session Management

| Evento | Ação |
|---|---|
| Token próximo de expirar | Silent renew automático via iframe |
| Silent renew falha | Redirect para login |
| Usuário fecha/abre aba | Restaura sessão do sessionStorage |
| Logout | Limpa tokens + redirect para Kanidm /logout |
| 401 em API call | Tenta refresh; se falhar, redirect para login |

---

<a id="adr-005"></a>
## ADR-005: API Client — Gerado via OpenAPI Generator

**Status:** Aceito  
**Data:** 2026-02-16  

### Contexto

Kanidm expõe OpenAPI schema em `/docs/v1/openapi.json`. Manter tipos e chamadas manualmente é propenso a erros e não escala com a evolução da API.

### Decisão

Gerar **TypeScript client** automaticamente usando `@openapitools/openapi-generator-cli` com target `typescript-fetch`.

### Pipeline de Geração

```bash
# scripts/generate-api-client.sh

#!/bin/bash
set -e

KANIDM_URL="${KANIDM_URL:-https://auth.localhost:8443}"
OUTPUT_DIR="console/src/api/generated"

echo "→ Baixando OpenAPI schema do Kanidm..."
curl -sk "$KANIDM_URL/docs/v1/openapi.json" -o /tmp/kanidm-openapi.json

echo "→ Gerando TypeScript client..."
npx @openapitools/openapi-generator-cli generate \
  -i /tmp/kanidm-openapi.json \
  -g typescript-fetch \
  -o "$OUTPUT_DIR" \
  --additional-properties=supportsES6=true,typescriptThreePlus=true,enumPropertyNaming=UPPERCASE

echo "→ Client gerado em $OUTPUT_DIR"
```

### Uso no código

```typescript
// api/kanidm-client.ts
import { Configuration, PersonApi, GroupApi, Oauth2Api, SystemApi } from './generated';
import { userManager } from '../auth/oidc-config';

async function getConfig(): Promise<Configuration> {
  const user = await userManager.getUser();
  return new Configuration({
    basePath: '/api/id/v1',
    headers: {
      Authorization: `Bearer ${user?.access_token}`,
    },
  });
}

export async function getPersonApi() {
  return new PersonApi(await getConfig());
}

export async function getGroupApi() {
  return new GroupApi(await getConfig());
}

export async function getOAuth2Api() {
  return new Oauth2Api(await getConfig());
}

export async function getSystemApi() {
  return new SystemApi(await getConfig());
}
```

### Quando regenerar

| Trigger | Ação |
|---|---|
| Atualização do Kanidm | Regenerar client, rodar diff, testar |
| CI pipeline | Validar que schema não quebrou tipos existentes |
| Nova feature no Console que usa endpoint novo | Regenerar + verificar cobertura |

---

<a id="adr-006"></a>
## ADR-006: Autorização — RBAC por Grupos OIDC

**Status:** Aceito  
**Data:** 2026-02-16  

### Contexto

O Console precisa controlar visibilidade e ações baseado no papel do usuário logado. Kanidm já provê grupos no token OIDC. Não queremos duplicar lógica de autorização.

### Decisão

**Role-Based Access Control (RBAC)** derivado dos grupos presentes no claim `groups` do ID Token do Kanidm.

### Mapeamento de Roles

```typescript
// auth/roles.ts

export enum ConsoleRole {
  SUPER_ADMIN = 'super_admin',     // IntegrAllTech: tudo
  TENANT_ADMIN = 'tenant_admin',   // Admin do cliente: seu tenant
  SERVICE_DESK = 'service_desk',   // Reset de senhas, suporte
  VIEWER = 'viewer',               // Somente leitura
}

// Mapeamento: grupos Kanidm → role no Console
const GROUP_ROLE_MAP: Record<string, ConsoleRole> = {
  'archguard_admins': ConsoleRole.SUPER_ADMIN,
  'idm_admins': ConsoleRole.SUPER_ADMIN,
  // Padrão: *_admins de qualquer tenant → TENANT_ADMIN
  // Padrão: idm_service_desk → SERVICE_DESK
  // Default: VIEWER
};

export function deriveRole(groups: string[]): ConsoleRole {
  // Prioridade: maior privilégio ganha
  if (groups.some(g => g === 'archguard_admins' || g === 'idm_admins')) {
    return ConsoleRole.SUPER_ADMIN;
  }
  if (groups.some(g => g.endsWith('_admins'))) {
    return ConsoleRole.TENANT_ADMIN;
  }
  if (groups.includes('idm_service_desk')) {
    return ConsoleRole.SERVICE_DESK;
  }
  return ConsoleRole.VIEWER;
}

export function deriveTenants(groups: string[]): string[] {
  // Extrai tenants dos grupos: "rio_quality_admins" → "rio_quality"
  return groups
    .filter(g => g.endsWith('_admins') && g !== 'archguard_admins' && g !== 'idm_admins')
    .map(g => g.replace('_admins', ''));
}
```

### Matriz de Permissões

Detalhada na [RFC-004](#rfc-004).

---

<a id="adr-007"></a>
## ADR-007: Multi-Tenancy — Grupo hierárquico com filtro no Console

**Status:** Aceito  
**Data:** 2026-02-16  

### Contexto

ArchGuard atende múltiplos clientes (Rio Quality, outros distribuidores). Cada cliente precisa ver apenas seus dados. O Kanidm não tem conceito nativo de tenant — usa grupos flat.

### Decisão

**Convenção de nomenclatura de grupos + filtro no Console.**

### Convenção

```
{tenant}                    → grupo raiz do tenant
{tenant}_admins             → administradores do tenant
{tenant}_users              → usuários comuns
{tenant}_{role}             → roles customizados
```

### Lógica de Filtro no Console

```typescript
// hooks/useTenantFilter.ts

export function useTenantFilter() {
  const { user } = useAuth();
  const role = deriveRole(user.groups);
  const tenants = deriveTenants(user.groups);

  // Super Admin: vê tudo, pode trocar contexto de tenant
  // Tenant Admin: vê apenas seu(s) tenant(s)
  // Service Desk: vê apenas membros dos tenants atribuídos
  // Viewer: vê apenas suas próprias informações

  return {
    role,
    tenants,
    isSuperAdmin: role === ConsoleRole.SUPER_ADMIN,
    filterGroups: (groups: Group[]) => {
      if (role === ConsoleRole.SUPER_ADMIN) return groups;
      return groups.filter(g =>
        tenants.some(t => g.name.startsWith(t))
      );
    },
    filterPersons: (persons: Person[]) => {
      if (role === ConsoleRole.SUPER_ADMIN) return persons;
      // Filtra persons que pertencem a grupos do tenant
      return persons.filter(p =>
        p.memberof?.some(g =>
          tenants.some(t => g.startsWith(t))
        )
      );
    },
  };
}
```

### Segurança

O filtro no Console é **UX only** — não é barreira de segurança. A segurança real está no Kanidm: o token do TENANT_ADMIN não tem permissão para ler/modificar recursos de outros tenants via API. O Console apenas esconde o que o admin não precisa ver.

---

<a id="adr-008"></a>
## ADR-008: Internacionalização — PT-BR first, i18n ready

**Status:** Aceito  
**Data:** 2026-02-16  

### Decisão

- **Idioma padrão:** Português Brasileiro
- **Framework i18n:** `i18next` + `react-i18next`
- **Namespace por módulo:** `identity.json`, `groups.json`, `oauth2.json`, `vault.json`, `common.json`
- **Idiomas planejados:** pt-BR (v1), en-US (v1.1), es (v2)
- **Detecção:** `navigator.language` → fallback para pt-BR

---

<a id="adr-009"></a>
## ADR-009: Observabilidade — Structured Logging + OpenTelemetry

**Status:** Aceito  
**Data:** 2026-02-16  

### Decisão

| Camada | Ferramenta | Destino |
|---|---|---|
| Client errors | `window.onerror` + custom reporter | Console stdout → Loki |
| API request tracing | TanStack Query `onError` global | Structured JSON logs |
| Audit trail | Middleware no Gateway + Kanidm logs nativos | Loki / Grafana |
| Métricas | OpenTelemetry (futuro) | Prometheus / Grafana |

### Log Format

```json
{
  "timestamp": "2026-02-16T14:30:00Z",
  "level": "warn",
  "source": "console",
  "action": "person.create",
  "actor": "edson@integralltech.com.br",
  "actor_groups": ["archguard_admins"],
  "target": "joao.silva",
  "result": "success",
  "duration_ms": 230
}
```

---

<a id="adr-010"></a>
## ADR-010: Deploy — Container Docker multi-stage

**Status:** Aceito  
**Data:** 2026-02-16  

### Decisão

```dockerfile
# console/Dockerfile
FROM node:22-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:alpine AS runtime
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
```

### Resultado

| Métrica | Valor |
|---|---|
| Imagem final | ~25MB (nginx:alpine + static files) |
| Build time | ~30s |
| Startup | <1s |
| RAM | ~10MB |

---

# REQUEST FOR COMMENTS

---

<a id="rfc-001"></a>
## RFC-001: Especificação de Funcionalidades e Fluxos de UX

**Status:** Draft  
**Autor:** IntegrAllTech  
**Data:** 2026-02-16  

### 1. Visão Geral

O ArchGuard Console é a interface administrativa unificada. Todos os fluxos assumem que o usuário já está autenticado via OIDC (ADR-004). O Console detecta o role do usuário (ADR-006) e adapta a UI.

### 2. Layout Principal

```
┌───────────────────────────────────────────────────────────────────┐
│  ArchGuard Console                          🔍 ⌘K    👤 Edson ▾  │
├──────────────┬────────────────────────────────────────────────────┤
│              │                                                    │
│  🛡️ ArchGuard│  [Breadcrumb: Dashboard > ...]                    │
│              │                                                    │
│  📊 Dashboard│  ┌──────────────────────────────────────────────┐  │
│              │  │                                              │  │
│  👤 Identida.│  │              CONTENT AREA                    │  │
│    Persons   │  │                                              │  │
│    Service   │  │  (muda conforme a rota selecionada)          │  │
│    Accounts  │  │                                              │  │
│              │  │                                              │  │
│  👥 Grupos   │  │                                              │  │
│              │  │                                              │  │
│  🔐 OAuth2   │  │                                              │  │
│    Clients   │  │                                              │  │
│    Scopes    │  │                                              │  │
│              │  │                                              │  │
│  🗝️ Vault    │  │                                              │  │
│    Status    │  │                                              │  │
│    Config    │  │                                              │  │
│              │  └──────────────────────────────────────────────┘  │
│  📋 Audit    │                                                    │
│              │                                                    │
│  ⚙️ Settings │                                                    │
│              │                                                    │
├──────────────┴────────────────────────────────────────────────────┤
│  ArchGuard v1.0 • Kanidm OK • Vault OK        IntegrAllTech 2026│
└───────────────────────────────────────────────────────────────────┘
```

### 3. Funcionalidades Detalhadas

---

#### 3.1 Dashboard

**Objetivo:** Visão geral do estado da plataforma em um olhar.

**Wireframe:**

```
┌────────────────────────────────────────────────────────────────┐
│  Dashboard                                                      │
│                                                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐       │
│  │    47     │  │    12    │  │     8    │  │    ✅    │       │
│  │  Persons  │  │  Groups  │  │  OAuth2  │  │  Vault   │       │
│  │  +3 esta  │  │          │  │  Clients │  │  Online  │       │
│  │  semana   │  │          │  │          │  │          │       │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘       │
│                                                                  │
│  Atividade Recente                        Quick Actions          │
│  ┌──────────────────────────────┐  ┌───────────────────────┐    │
│  │ 14:23  João fez login       │  │  [+ Nova Pessoa]      │    │
│  │ 14:10  Maria reset senha    │  │  [+ Novo Grupo]       │    │
│  │ 13:45  Admin criou grupo    │  │  [+ Novo OAuth2]      │    │
│  │ 13:30  API token gerado     │  │  [🔄 Reset Credencial]│    │
│  │ 12:15  OAuth2 client criado │  │                       │    │
│  │                              │  │                       │    │
│  │  [Ver todos os logs →]       │  │                       │    │
│  └──────────────────────────────┘  └───────────────────────┘    │
│                                                                  │
│  Saúde dos Serviços                                             │
│  ┌──────────────────────────────────────────────────────┐       │
│  │  ArchGuard ID (Kanidm)    ● Online   Uptime: 99.9%  │       │
│  │  ArchGuard Vault          ● Online   Uptime: 99.8%  │       │
│  │  Gateway (nginx)          ● Online                   │       │
│  └──────────────────────────────────────────────────────┘       │
└────────────────────────────────────────────────────────────────┘
```

**Dados:**
- Cards: queries `GET /v1/person` (count), `GET /v1/group` (count), `GET /v1/oauth2` (count), vault status
- Atividade: `GET /v1/system/_audit` (se disponível) ou logs do Gateway
- Saúde: `GET /status` do Kanidm + health check do Vault

**Visibilidade por Role:**
- SUPER_ADMIN: Todos os cards, todos os tenants, todas as quick actions
- TENANT_ADMIN: Cards filtrados para seu tenant, quick actions limitadas
- SERVICE_DESK: Cards de persons do seu escopo, only reset credential
- VIEWER: Apenas seus dados pessoais, sem quick actions

---

#### 3.2 Persons Management

**Objetivo:** CRUD completo de identidades (persons) no Kanidm.

##### 3.2.1 Lista de Persons

```
┌────────────────────────────────────────────────────────────────┐
│  Identidades > Persons                        [+ Nova Pessoa]   │
│                                                                  │
│  🔍 Buscar por nome, email ou username...    Filtros ▾          │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ ☐  Nome              Email                 Grupos  Status│   │
│  │─────────────────────────────────────────────────────────│   │
│  │ ☐  João Silva        joao@rioquality...    2       🟢   │   │
│  │ ☐  Maria Santos      maria@rioquality...   3       🟢   │   │
│  │ ☐  Carlos Oliveira   carlos@integrall...   1       🟡   │   │
│  │ ☐  Ana Costa         ana@rioquality...     2       🔴   │   │
│  │─────────────────────────────────────────────────────────│   │
│  │                                     Página 1 de 3  < >  │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  Com selecionados: [Adicionar a grupo] [Gerar convite] [Remover]│
└────────────────────────────────────────────────────────────────┘
```

**Filtros disponíveis:**
- Status: Ativo, Pendente (sem credenciais), Bloqueado
- Grupo: Dropdown multi-select com grupos visíveis para o role
- Tenant: Apenas para SUPER_ADMIN — selector de tenant
- MFA: Com MFA, Sem MFA
- Último login: Hoje, 7 dias, 30 dias, Nunca

**Status indicators:**
- 🟢 Ativo — credenciais válidas, login recente
- 🟡 Pendente — conta criada mas sem credenciais configuradas
- 🔴 Bloqueado — conta locked (muitas tentativas, ou manual)

**Bulk actions (com checkbox):**
- Adicionar a grupo (dialog com selector de grupo)
- Gerar convite em massa (emails com link de setup)
- Remover (com confirmação, não reversível)
- Exportar CSV

**API Endpoints:**
- `GET /v1/person` — lista
- `GET /v1/person?filter=...` — busca (Kanidm suporta filtros LDAP-like)

##### 3.2.2 Criar Pessoa — Wizard

```
┌────────────────────────────────────────────────────────────────┐
│  Nova Pessoa                                            Passo 1│
│                                                        de 3    │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                                                          │  │
│  │  Nome Completo *          [________________________]     │  │
│  │                                                          │  │
│  │  Nome de Usuário *        [________________________]     │  │
│  │  (será o login)           Auto: joao.silva               │  │
│  │                                                          │  │
│  │  Email *                  [________________________]     │  │
│  │                                                          │  │
│  │  Título / Cargo           [________________________]     │  │
│  │                                                          │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                  │
│                                  [Cancelar]  [Próximo: Grupos →]│
└────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────┐
│  Nova Pessoa                                            Passo 2│
│                                                        de 3    │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  Selecione os grupos:                                    │  │
│  │                                                          │  │
│  │  ☑ rio_quality              (tenant)                     │  │
│  │  ☑ rio_quality_vendedores   (role)                       │  │
│  │  ☐ rio_quality_gestores     (role)                       │  │
│  │  ☐ rio_quality_admins       (admin)                      │  │
│  │                                                          │  │
│  │  [🔍 Buscar grupo...]                                    │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                  │
│                        [← Voltar]  [Próximo: Credenciais →]     │
└────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────┐
│  Nova Pessoa                                            Passo 3│
│                                                        de 3    │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  Como essa pessoa vai configurar as credenciais?         │  │
│  │                                                          │  │
│  │  ◉ Enviar convite por email                              │  │
│  │    Link temporário para setup de senha + passkey         │  │
│  │    (expira em 48h)                                       │  │
│  │                                                          │  │
│  │  ○ Gerar link de convite (copiar)                        │  │
│  │    Para envio manual via WhatsApp, Slack, etc.           │  │
│  │                                                          │  │
│  │  ○ Definir senha temporária agora                        │  │
│  │    (forçar troca no primeiro login)                       │  │
│  │                                                          │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                  │
│  Resumo: João Silva (joao.silva) → rio_quality_vendedores       │
│                                                                  │
│                        [← Voltar]  [✓ Criar Pessoa]             │
└────────────────────────────────────────────────────────────────┘
```

**API Flow:**
1. `POST /v1/person` — cria a pessoa com `name`, `displayname`, `mail`
2. `POST /v1/group/{gid}/_attr/member` — adiciona aos grupos selecionados (para cada grupo)
3. `POST /v1/person/{id}/_credential/_update_intent/{ttl}` — gera token de reset (link de convite)

**Validações:**
- Nome de usuário: único, lowercase, alfanumérico + ponto/underscore
- Email: formato válido, único no sistema
- Pelo menos 1 grupo deve ser selecionado
- TENANT_ADMIN só vê e seleciona grupos do seu tenant

##### 3.2.3 Detalhe da Pessoa

```
┌────────────────────────────────────────────────────────────────┐
│  ← Persons    João Silva                     🟢 Ativo          │
│                                                                  │
│  ┌────────────────────────────────────────────────────┐         │
│  │  👤  João Silva                                     │         │
│  │      joao.silva                                     │         │
│  │      joao@rioquality.com.br                        │         │
│  │      Vendedor Externo                               │         │
│  │      Último login: 16/02/2026 14:23                │         │
│  │                                                     │         │
│  │      [Editar] [Reset Credenciais] [Bloquear] [...]  │         │
│  └────────────────────────────────────────────────────┘         │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ [Atributos]  [Grupos]  [Credenciais]  [Sessões]  [Log]  │   │
│  ├──────────────────────────────────────────────────────────┤   │
│  │                                                          │   │
│  │  TAB: Atributos                                          │   │
│  │  ─────────────────                                       │   │
│  │  UUID:          3f7a...b2c1                              │   │
│  │  Nome:          João Silva                               │   │
│  │  Username:      joao.silva                               │   │
│  │  Email:         joao@rioquality.com.br                   │   │
│  │  Título:        Vendedor Externo                         │   │
│  │  Criado em:     2026-01-15T10:00:00Z                     │   │
│  │  Modificado em: 2026-02-10T14:30:00Z                     │   │
│  │                                                          │   │
│  │  TAB: Grupos                                             │   │
│  │  ─────────────                                           │   │
│  │  • rio_quality                [Remover]                  │   │
│  │  • rio_quality_vendedores     [Remover]                  │   │
│  │                                                          │   │
│  │  [+ Adicionar a grupo]                                   │   │
│  │                                                          │   │
│  │  TAB: Credenciais                                        │   │
│  │  ──────────────────                                       │   │
│  │  Senha:    ● Configurada     [Forçar Reset]              │   │
│  │  Passkey:  ● 2 registradas   [Ver detalhes]              │   │
│  │  TOTP:     ○ Não configurado                             │   │
│  │  MFA:      ● Ativo (passkey)                             │   │
│  │                                                          │   │
│  │  [Gerar Link de Reset Completo]                          │   │
│  │                                                          │   │
│  │  TAB: Sessões                                            │   │
│  │  ────────────────                                         │   │
│  │  • Chrome/Windows — 2026-02-16 14:23   [Encerrar]        │   │
│  │  • VendaX Mobile — 2026-02-16 09:00    [Encerrar]        │   │
│  │                                                          │   │
│  │  [Encerrar todas as sessões]                             │   │
│  │                                                          │   │
│  └──────────────────────────────────────────────────────────┘   │
└────────────────────────────────────────────────────────────────┘
```

**API Endpoints por Tab:**
- Atributos: `GET /v1/person/{id}` → todos os atributos
- Grupos: `GET /v1/person/{id}/_attr/memberof`
- Credenciais: `GET /v1/person/{id}/_credential/_status`
- Sessões: `GET /v1/person/{id}/_auth_session` (se disponível)
- Adicionar grupo: `POST /v1/group/{gid}/_attr/member` com `[person_id]`
- Remover de grupo: `DELETE /v1/group/{gid}/_attr/member` com `[person_id]`
- Reset: `POST /v1/person/{id}/_credential/_update_intent/{ttl}`
- Bloquear: `POST /v1/person/{id}/_attr/account_expire` ou `account_valid_from`

##### 3.2.4 Credential Reset Flow

```
Admin clica "Reset Credenciais" em uma Person
         │
         ▼
┌──────────────────────────────────┐
│  Reset de Credenciais            │
│                                  │
│  Pessoa: João Silva              │
│                                  │
│  Validade do link:               │
│  ○ 1 hora                       │
│  ◉ 24 horas                     │
│  ○ 48 horas                     │
│  ○ 7 dias                       │
│                                  │
│  [Cancelar]  [Gerar Link]       │
└──────────────────────────────────┘
         │
         ▼  POST /v1/person/{id}/_credential/_update_intent/{ttl}
         │  Response: { token: "abc123..." }
         ▼
┌──────────────────────────────────┐
│  ✅ Link Gerado                  │
│                                  │
│  https://auth.domain.com/       │
│  ui/reset?token=abc123...       │
│                                  │
│  [📋 Copiar Link]               │
│  [📧 Enviar por Email]          │
│                                  │
│  ⚠️ Este link expira em 24h     │
│  e pode ser usado apenas 1 vez. │
│                                  │
│  [Fechar]                        │
└──────────────────────────────────┘
```

---

#### 3.3 Service Accounts

**Objetivo:** Gerenciar contas de serviço para automação e integração máquina-a-máquina.

##### 3.3.1 Lista

Mesma estrutura da lista de Persons, mas com colunas específicas:

| Coluna | Descrição |
|---|---|
| Nome | Display name do service account |
| SPN | Service Principal Name |
| API Tokens | Quantidade de tokens ativos |
| Criado em | Data de criação |
| Último uso | Último uso de token (se rastreável) |

##### 3.3.2 Criar Service Account

```
┌────────────────────────────────────────────────────────────────┐
│  Novo Service Account                                           │
│                                                                  │
│  Nome *                  [________________________]             │
│  (identificador único)   Ex: vendax-api, mentors-sync          │
│                                                                  │
│  Display Name *          [________________________]             │
│                                                                  │
│  Descrição               [________________________]             │
│                                                                  │
│  Grupos                                                         │
│  ☐ rio_quality_users                                            │
│  ☐ vendax_service                                               │
│  [🔍 Buscar grupo...]                                           │
│                                                                  │
│  [Cancelar]  [✓ Criar Service Account]                          │
└────────────────────────────────────────────────────────────────┘
         │
         ▼  Após criação, gera primeiro API Token:
┌────────────────────────────────────────────────────────────────┐
│  ✅ Service Account Criado                                      │
│                                                                  │
│  API Token (exibido apenas UMA VEZ):                            │
│  ┌──────────────────────────────────────────────┐               │
│  │  eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9...    │  [📋 Copiar]  │
│  └──────────────────────────────────────────────┘               │
│                                                                  │
│  ⚠️ Salve este token agora. Ele não será exibido novamente.     │
│     Recomendação: armazene no ArchGuard Vault.                  │
│                                                                  │
│  [Ir para detalhes]  [Fechar]                                   │
└────────────────────────────────────────────────────────────────┘
```

**API Flow:**
1. `POST /v1/service_account` — cria com `name`, `displayname`
2. `POST /v1/group/{gid}/_attr/member` — adiciona aos grupos
3. `POST /v1/service_account/{id}/_api_token` — gera token

##### 3.3.3 Detalhe do Service Account

Tabs: Atributos | Grupos | API Tokens

**Tab API Tokens:**

```
┌──────────────────────────────────────────────────────────────┐
│  API Tokens                                  [+ Novo Token]  │
│                                                              │
│  Label          Criado em         Expira em       Ação       │
│  ──────────────────────────────────────────────────────      │
│  deploy-ci      2026-01-15        2026-07-15      [Revogar]  │
│  local-dev      2026-02-01        Nunca           [Revogar]  │
│                                                              │
│  ⚠️ Tokens revogados param de funcionar imediatamente.       │
└──────────────────────────────────────────────────────────────┘
```

---

#### 3.4 Groups Management

##### 3.4.1 Lista de Grupos

```
┌────────────────────────────────────────────────────────────────┐
│  Grupos                                       [+ Novo Grupo]   │
│                                                                  │
│  🔍 Buscar por nome...                      Filtros ▾           │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ Nome                     Tipo       Membros   Criado     │   │
│  │──────────────────────────────────────────────────────────│   │
│  │ 📁 archguard_admins     sistema     3         builtin   │   │
│  │ 📁 rio_quality           tenant     15        2026-01   │   │
│  │   📁 rio_quality_admins  role        2         2026-01   │   │
│  │   📁 rio_quality_vende.  role       10         2026-01   │   │
│  │   📁 rio_quality_gestor  role        3         2026-01   │   │
│  │ 📁 outro_cliente         tenant      8         2026-02   │   │
│  │   📁 outro_cliente_adm.  role        1         2026-02   │   │
│  │   📁 outro_cliente_user  role        7         2026-02   │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  Visualização: [Lista] [Hierarquia]                             │
└────────────────────────────────────────────────────────────────┘
```

**Tipo (derivado da convenção de nomenclatura):**
- `sistema` — grupos nativos do Kanidm (idm_admins, etc.)
- `tenant` — grupo raiz de um cliente (rio_quality)
- `role` — grupo funcional dentro de um tenant (rio_quality_vendedores)
- `app` — grupo de acesso a uma aplicação (vendax_users)

##### 3.4.2 Criar Grupo

```
┌────────────────────────────────────────────────────────────────┐
│  Novo Grupo                                                     │
│                                                                  │
│  Tenant (prefixo) *     [rio_quality ▾]                         │
│                         (SUPER_ADMIN: todos | TENANT: seus)     │
│                                                                  │
│  Nome do Grupo *        [{tenant}_________________]             │
│  (auto-prefixado)       Resultado: rio_quality_supervisores     │
│                                                                  │
│  Descrição              [________________________]              │
│                                                                  │
│  Membros iniciais       [🔍 Buscar persons...]                  │
│                         ┌─────────────────────────┐             │
│                         │ ☑ João Silva            │             │
│                         │ ☑ Maria Santos          │             │
│                         │ ☐ Carlos Oliveira       │             │
│                         └─────────────────────────┘             │
│                                                                  │
│  [Cancelar]  [✓ Criar Grupo]                                    │
└────────────────────────────────────────────────────────────────┘
```

**API Flow:**
1. `POST /v1/group` — cria grupo com `name`
2. `POST /v1/group/{gid}/_attr/member` — adiciona membros selecionados

##### 3.4.3 Detalhe do Grupo

```
┌────────────────────────────────────────────────────────────────┐
│  ← Grupos    rio_quality_vendedores                             │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ [Membros (10)]  [OAuth2 Scopes]  [Configurações]        │   │
│  ├──────────────────────────────────────────────────────────┤   │
│  │                                                          │   │
│  │  Membros                          [+ Adicionar Membro]   │   │
│  │                                                          │   │
│  │  👤 João Silva        joao.silva        [Remover]        │   │
│  │  👤 Maria Santos      maria.santos      [Remover]        │   │
│  │  👤 Pedro Lima        pedro.lima         [Remover]        │   │
│  │  👤 Ana Costa         ana.costa          [Remover]        │   │
│  │  ... +6 mais                                             │   │
│  │                                                          │   │
│  │  OAuth2 Scopes (apps que confiam neste grupo)            │   │
│  │  ─────────────────────────────────────                   │   │
│  │  vendax-rioquality → openid, email, profile, groups      │   │
│  │  mentors-rioquality → openid, email                      │   │
│  │                                                          │   │
│  └──────────────────────────────────────────────────────────┘   │
└────────────────────────────────────────────────────────────────┘
```

**Adicionar Membro — Dialog:**

```
┌────────────────────────────────────────┐
│  Adicionar Membro                      │
│                                        │
│  🔍 [Buscar por nome ou email...]     │
│                                        │
│  Resultados:                           │
│  ☐ Carlos Oliveira (carlos@int...)     │
│  ☐ Fernanda Silva (fernanda@rio...)    │
│  ☐ Roberto Santos (roberto@rio...)     │
│                                        │
│  [Cancelar]  [Adicionar Selecionados]  │
└────────────────────────────────────────┘
```

---

#### 3.5 OAuth2 / SSO Management

##### 3.5.1 Lista de Clients

```
┌────────────────────────────────────────────────────────────────┐
│  OAuth2 > Clients                           [+ Novo Client]    │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ Nome                Tipo     Origin              Scopes  │   │
│  │──────────────────────────────────────────────────────────│   │
│  │ archguard-console   public   console.arch...     4       │   │
│  │ vendax-rioquality   basic    vendax.integr...    4       │   │
│  │ vendax-mobile       public   com.integrall...    4       │   │
│  │ mentors-ipaaas      basic    mentors.integr...   3       │   │
│  │ powerview           basic    power.integr...     3       │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  Tipo: Basic = com client_secret | Public = PKCE only (SPAs)   │
└────────────────────────────────────────────────────────────────┘
```

##### 3.5.2 Criar OAuth2 Client — Wizard

```
┌────────────────────────────────────────────────────────────────┐
│  Novo OAuth2 Client                                    Passo 1 │
│                                                        de 4    │
│                                                                  │
│  Tipo de aplicação:                                             │
│                                                                  │
│  ┌─────────────────────────────┐  ┌────────────────────────┐   │
│  │ 🖥️  Web Application         │  │ 📱 SPA / Mobile        │   │
│  │                             │  │                        │   │
│  │ Backend server-side         │  │ Frontend only          │   │
│  │ (Spring Boot, Node, etc.)   │  │ (React, Flutter, etc.) │   │
│  │                             │  │                        │   │
│  │ → Basic client (com secret) │  │ → Public client (PKCE) │   │
│  └─────────────────────────────┘  └────────────────────────┘   │
└────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────┐
│  Novo OAuth2 Client                                    Passo 2 │
│                                                                  │
│  Nome do Client *        [________________________]             │
│  (slug, lowercase)       Ex: vendax-rioquality                  │
│                                                                  │
│  Display Name *          [________________________]             │
│                          Ex: VendaX - Rio Quality               │
│                                                                  │
│  Origin URL *            [________________________]             │
│                          Ex: https://vendax.integralltech.com.br│
│                                                                  │
│  Redirect URIs                                                  │
│  [https://vendax.integralltech.com.br/callback    ] [×]         │
│  [+ Adicionar URI]                                              │
│                                                                  │
│  ☐ Permitir localhost redirects (apenas para dev)               │
│                                                                  │
│                        [← Voltar]  [Próximo: Scopes →]          │
└────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────┐
│  Novo OAuth2 Client                                    Passo 3 │
│                                                                  │
│  Scope Mapping                                                  │
│  ──────────────                                                 │
│  Selecione quais grupos podem acessar este client e             │
│  quais scopes recebem:                                          │
│                                                                  │
│  Grupo                        Scopes                            │
│  ┌───────────────────────────────────────────────────────┐      │
│  │ rio_quality                ☑ openid ☑ email           │      │
│  │                            ☑ profile ☑ groups         │      │
│  │────────────────────────────────────────────────────── │      │
│  │ rio_quality_admins         ☑ openid ☑ email           │      │
│  │                            ☑ profile ☑ groups         │      │
│  │                            ☑ admin                    │      │
│  └───────────────────────────────────────────────────────┘      │
│                                                                  │
│  [+ Adicionar grupo ao scope map]                               │
│                                                                  │
│                        [← Voltar]  [Próximo: Resumo →]          │
└────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────┐
│  Novo OAuth2 Client                                    Passo 4 │
│                                                                  │
│  Resumo                                                         │
│  ──────                                                         │
│  Nome:           vendax-rioquality                              │
│  Display:        VendaX - Rio Quality                           │
│  Tipo:           Basic (com secret)                             │
│  Origin:         https://vendax.integralltech.com.br            │
│  Redirects:      /callback                                      │
│  PKCE:           S256 (obrigatório)                             │
│  Token signing:  ES256                                          │
│  Scope map:      rio_quality → openid,email,profile,groups      │
│                  rio_quality_admins → +admin                    │
│                                                                  │
│                        [← Voltar]  [✓ Criar Client]             │
└────────────────────────────────────────────────────────────────┘
         │
         ▼  Se tipo Basic:
┌────────────────────────────────────────────────────────────────┐
│  ✅ OAuth2 Client Criado                                        │
│                                                                  │
│  Client ID:      vendax-rioquality                              │
│  Client Secret:  xxxxxxxxxxxxxxxxxxxxx              [📋 Copiar]  │
│                                                                  │
│  Discovery URL:                                                 │
│  https://auth.domain.com/oauth2/openid/vendax-rioquality/      │
│  .well-known/openid-configuration                               │
│                                                                  │
│  ⚠️ O secret será exibido apenas esta vez.                      │
│     Armazene no ArchGuard Vault.                                │
│                                                                  │
│  [📋 Copiar como application.yml]   (gera snippet Spring Boot)  │
│  [📋 Copiar como .env]              (gera KEY=VALUE)            │
│                                                                  │
│  [Ir para detalhes]  [Fechar]                                   │
└────────────────────────────────────────────────────────────────┘
```

**API Flow:**
1. Tipo basic: `POST /v1/oauth2/_basic` com `{ name, displayname, origin }`
   Tipo public: `POST /v1/oauth2/_public` com `{ name, displayname, origin }`
2. `POST /v1/oauth2/{name}/_scopemap/{group}` — para cada grupo/scope pair
3. Se basic: `GET /v1/oauth2/{name}/_basic_secret` — retorna o secret
4. PKCE é habilitado por default no Kanidm

##### 3.5.3 Detalhe do OAuth2 Client

```
Tabs: [Configuração] [Scope Maps] [Redirect URIs] [Snippets] [Danger Zone]

Tab Snippets — gerador de código de integração:
┌──────────────────────────────────────────────────────────────┐
│  Snippets de Integração                                      │
│                                                              │
│  Framework: [Spring Boot ▾]                                  │
│                                                              │
│  ```yaml                                                     │
│  spring:                                                     │
│    security:                                                 │
│      oauth2:                                                 │
│        client:                                               │
│          registration:                                       │
│            archguard:                                        │
│              client-id: vendax-rioquality                    │
│              client-secret: ${ARCHGUARD_SECRET}              │
│  ...                                                         │
│  ```                                                         │
│                                                              │
│  [📋 Copiar]                                                 │
│                                                              │
│  Frameworks disponíveis:                                     │
│  Spring Boot | React | Flutter | Node.js | Python            │
└──────────────────────────────────────────────────────────────┘

Tab Danger Zone:
┌──────────────────────────────────────────────────────────────┐
│  ⚠️ Zona de Perigo                                           │
│                                                              │
│  Rotacionar Secret                                           │
│  Gera um novo secret e invalida o anterior.                  │
│  Todas as aplicações usando o secret antigo pararão.         │
│  [🔄 Rotacionar Secret]                                      │
│                                                              │
│  Excluir Client                                              │
│  Remove permanentemente este OAuth2 client.                  │
│  Todos os usuários perderão acesso à aplicação.              │
│  [🗑️ Excluir Client]  (pede confirmação digitando o nome)    │
└──────────────────────────────────────────────────────────────┘
```

---

#### 3.6 Vault Status

**Objetivo:** Monitoramento e acesso rápido ao ArchGuard Vault. A gestão detalhada do vault ocorre no próprio AliasVault Web UI.

```
┌────────────────────────────────────────────────────────────────┐
│  Vault                                                          │
│                                                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐       │
│  │    ✅    │  │    23    │  │    15    │  │    45    │       │
│  │  Online  │  │  Vaults  │  │  Aliases │  │  Emails  │       │
│  │          │  │  ativos  │  │  ativos  │  │ recebidos│       │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘       │
│                                                                  │
│  Servidor SMTP                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  Status:    ● Online                                     │   │
│  │  Domínio:   vault.integralltech.com.br                   │   │
│  │  MX Record: ✅ Configurado                               │   │
│  │  SPF:       ✅ Válido                                    │   │
│  │  DKIM:      ✅ Válido                                    │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  Ações                                                          │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  [🔗 Abrir Vault Web UI]     (abre AliasVault em nova aba) │   │
│  │  [📊 Ver métricas detalhadas]                             │   │
│  │  [💾 Backup do Vault]                                     │   │
│  │  [⚙️  Configurações do Vault]                              │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  Integração com ArchGuard ID                                    │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  Status: ⚠️ Login separado (SSO Bridge não ativado)      │   │
│  │                                                          │   │
│  │  Quando ativado, os usuários poderão fazer login no      │   │
│  │  Vault usando suas credenciais do ArchGuard ID.          │   │
│  │                                                          │   │
│  │  [📖 Ver documentação]  [⚙️ Configurar SSO Bridge]       │   │
│  └──────────────────────────────────────────────────────────┘   │
└────────────────────────────────────────────────────────────────┘
```

---

#### 3.7 Auditoria

```
┌────────────────────────────────────────────────────────────────┐
│  Auditoria                                    [📥 Exportar]    │
│                                                                  │
│  Filtros:                                                       │
│  Período: [Hoje ▾]  Ator: [Todos ▾]  Ação: [Todas ▾]          │
│  Recurso: [Todos ▾]  Resultado: [Todos ▾]                      │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ Horário   Ator              Ação           Alvo    Res.  │   │
│  │──────────────────────────────────────────────────────────│   │
│  │ 14:23     joao.silva        login.success  -       ✅    │   │
│  │ 14:10     admin             cred.reset     maria   ✅    │   │
│  │ 13:45     admin             group.create   rq_sup  ✅    │   │
│  │ 13:30     admin             token.create   vendax  ✅    │   │
│  │ 12:15     admin             oauth2.create  mentors ✅    │   │
│  │ 11:00     desconhecido      login.fail     -       ❌    │   │
│  │ 10:45     ana.costa         login.fail     -       ❌    │   │
│  │ 10:44     ana.costa         login.fail     -       ❌    │   │
│  │ 10:43     ana.costa         login.fail     -       ⛔    │   │
│  │           → Conta bloqueada após 3 tentativas            │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  Alertas Ativos                                                 │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  ⚠️ ana.costa bloqueada por tentativas excessivas (10:43)│   │
│  │  ⚠️ 3 logins falhos de IP desconhecido (11:00)          │   │
│  │                                                          │   │
│  │  [Desbloquear ana.costa]  [Ver detalhes do IP]           │   │
│  └──────────────────────────────────────────────────────────┘   │
└────────────────────────────────────────────────────────────────┘
```

---

#### 3.8 Settings

```
┌────────────────────────────────────────────────────────────────┐
│  Configurações                                                  │
│                                                                  │
│  [Geral] [Políticas] [Backup] [Replicação] [Sobre]             │
│                                                                  │
│  TAB: Geral                                                     │
│  ──────────                                                     │
│  Domínio:         auth.integralltech.com.br                     │
│  Origin:          https://auth.integralltech.com.br             │
│  TLS:             Let's Encrypt (válido até 2026-05-01)         │
│  Versão Kanidm:   1.5.0                                        │
│  Versão AliasVault: 0.26.0                                     │
│  Versão Console:  1.0.0                                        │
│                                                                  │
│  TAB: Políticas                                                 │
│  ──────────────                                                 │
│  MFA obrigatório para admins:     [✅ Ativo]                    │
│  MFA obrigatório para todos:      [☐ Inativo]                  │
│  Sessão máxima:                   [8 horas ▾]                   │
│  Tentativas antes do bloqueio:    [5 ▾]                         │
│  Tempo de bloqueio:               [15 min ▾]                   │
│  Complexidade de senha:           [Média ▾]                     │
│                                                                  │
│  TAB: Backup                                                    │
│  ────────────                                                   │
│  Último backup:    2026-02-16 03:00 ✅                          │
│  Frequência:       [Diário ▾]  Horário: [03:00]                │
│  Retenção:         [30 dias ▾]                                  │
│  Destino:          /backups/archguard/                           │
│  [🔄 Backup agora]  [📥 Download último backup]                 │
│                                                                  │
│  TAB: Sobre                                                     │
│  ──────────                                                     │
│  ArchGuard v1.0.0 — Identity & Secrets Platform                │
│  Powered by Kanidm and AliasVault                              │
│  Open source by IntegrAllTech                                   │
│  github.com/integralltech/archguard                             │
└────────────────────────────────────────────────────────────────┘
```

---

#### 3.9 Command Palette (⌘K)

```
┌────────────────────────────────────────────────────────────────┐
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  🔍 Buscar persons, grupos, clients, ações...           │   │
│  ├──────────────────────────────────────────────────────────┤   │
│  │                                                          │   │
│  │  Navegação                                               │   │
│  │  📊  Dashboard                                           │   │
│  │  👤  Persons                                             │   │
│  │  👥  Grupos                                              │   │
│  │  🔐  OAuth2 Clients                                     │   │
│  │                                                          │   │
│  │  Ações Rápidas                                           │   │
│  │  ➕  Criar nova pessoa                                   │   │
│  │  ➕  Criar novo grupo                                    │   │
│  │  ➕  Criar OAuth2 client                                 │   │
│  │  🔄  Reset credenciais                                   │   │
│  │                                                          │   │
│  │  Recentes                                                │   │
│  │  👤  João Silva                                          │   │
│  │  👥  rio_quality_vendedores                              │   │
│  │  🔐  vendax-rioquality                                   │   │
│  │                                                          │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
└────────────────────────────────────────────────────────────────┘
```

Implementado com Shadcn `<Command>` component. Busca local nos dados em cache do TanStack Query.

---

<a id="rfc-002"></a>
## RFC-002: Modelo de Dados e Contratos de API

**Status:** Draft  
**Autor:** IntegrAllTech  
**Data:** 2026-02-16  

### 1. Tipos Core (TypeScript)

```typescript
// types/kanidm.ts — Tipos alinhados ao schema Kanidm

// ═══════════════════════════════════════════
// PERSON
// ═══════════════════════════════════════════

export interface Person {
  attrs: PersonAttrs;
}

export interface PersonAttrs {
  uuid: string[];
  name: string[];
  displayname: string[];
  mail?: string[];
  title?: string[];
  memberof?: string[];           // UUIDs dos grupos
  class: string[];               // object classes
  gidnumber?: string[];
  loginshell?: string[];
  ssh_publickey?: string[];
  account_expire?: string[];     // ISO datetime
  account_valid_from?: string[]; // ISO datetime
}

// Tipo simplificado para uso nos componentes
export interface PersonSummary {
  uuid: string;
  name: string;
  displayName: string;
  email: string | null;
  title: string | null;
  groups: string[];
  status: PersonStatus;
  lastLogin: string | null;
}

export enum PersonStatus {
  ACTIVE = 'active',
  PENDING = 'pending',      // sem credenciais
  LOCKED = 'locked',        // bloqueado
  EXPIRED = 'expired',      // expirado
}

export interface CreatePersonRequest {
  attrs: {
    name: string[];
    displayname: string[];
    mail: string[];
    title?: string[];
  };
}

export interface PersonCredentialStatus {
  creds: CredentialDetail[];
}

export interface CredentialDetail {
  type_: 'Password' | 'GeneratedPassword' | 'Passkey' | 'AttestedPasskey';
  uuid: string;
  label?: string;
}

export interface CredentialResetIntent {
  token: string;
  expiry: string; // ISO datetime
}

// ═══════════════════════════════════════════
// SERVICE ACCOUNT
// ═══════════════════════════════════════════

export interface ServiceAccount {
  attrs: ServiceAccountAttrs;
}

export interface ServiceAccountAttrs {
  uuid: string[];
  name: string[];
  displayname: string[];
  spn: string[];               // service principal name
  memberof?: string[];
  class: string[];
  api_token_session?: string[];
}

export interface CreateServiceAccountRequest {
  attrs: {
    name: string[];
    displayname: string[];
  };
}

export interface ApiToken {
  token_id: string;
  label: string;
  expiry: string | null;
  issued_at: string;
}

export interface GeneratedApiToken {
  token: string; // JWT — exibido apenas na criação
}

// ═══════════════════════════════════════════
// GROUP
// ═══════════════════════════════════════════

export interface Group {
  attrs: GroupAttrs;
}

export interface GroupAttrs {
  uuid: string[];
  name: string[];
  description?: string[];
  member?: string[];          // UUIDs dos membros
  class: string[];
  gidnumber?: string[];
}

// Tipo enriquecido para o Console
export interface GroupSummary {
  uuid: string;
  name: string;
  description: string | null;
  memberCount: number;
  type: GroupType;
  tenant: string | null;
}

export enum GroupType {
  SYSTEM = 'system',       // nativos Kanidm
  TENANT = 'tenant',       // grupo raiz de cliente
  ROLE = 'role',           // role dentro de tenant
  APP = 'app',             // acesso a uma aplicação
  CUSTOM = 'custom',       // outros
}

export interface CreateGroupRequest {
  attrs: {
    name: string[];
    description?: string[];
  };
}

export interface ModifyGroupMembersRequest {
  attrs: string[]; // UUIDs dos membros a adicionar/remover
}

// ═══════════════════════════════════════════
// OAUTH2 CLIENT
// ═══════════════════════════════════════════

export interface OAuth2Client {
  attrs: OAuth2ClientAttrs;
}

export interface OAuth2ClientAttrs {
  uuid: string[];
  name: string[];                        // client_id
  displayname: string[];
  oauth2_rs_origin: string[];            // origin URL
  oauth2_rs_origin_landing: string[];    // landing page
  oauth2_rs_scope_map?: string[];        // scope mappings
  oauth2_rs_sup_scope_map?: string[];    // supplementary scope maps
  oauth2_rs_token_key: string[];         // ES256
  class: string[];
  oauth2_allow_localhost_redirect?: string[];
}

export interface CreateOAuth2BasicRequest {
  attrs: {
    name: string[];
    displayname: string[];
    oauth2_rs_origin: string[];
  };
}

export interface CreateOAuth2PublicRequest {
  attrs: {
    name: string[];
    displayname: string[];
    oauth2_rs_origin: string[];
  };
}

export interface OAuth2ScopeMapRequest {
  attrs: string[]; // scopes: ["openid", "email", "profile", "groups"]
}

export interface OAuth2Secret {
  secret: string;
}

// OAuth2 enriched para o Console
export interface OAuth2ClientSummary {
  uuid: string;
  name: string;             // client_id
  displayName: string;
  origin: string;
  type: 'basic' | 'public';
  scopeMaps: OAuth2ScopeMap[];
  pkceEnabled: boolean;
}

export interface OAuth2ScopeMap {
  group: string;       // group name ou UUID
  scopes: string[];    // ["openid", "email", "profile", "groups"]
}

// ═══════════════════════════════════════════
// SYSTEM
// ═══════════════════════════════════════════

export interface SystemStatus {
  running: boolean;
  version: string;
}

// ═══════════════════════════════════════════
// VAULT (AliasVault)
// ═══════════════════════════════════════════

export interface VaultStatus {
  online: boolean;
  version: string;
  totalVaults: number;
  activeAliases: number;
  smtpStatus: SmtpStatus;
}

export interface SmtpStatus {
  online: boolean;
  domain: string;
  mxConfigured: boolean;
  spfValid: boolean;
  dkimValid: boolean;
}

// ═══════════════════════════════════════════
// AUDIT
// ═══════════════════════════════════════════

export interface AuditEntry {
  timestamp: string;
  actor: string;
  action: AuditAction;
  target: string | null;
  result: 'success' | 'failure' | 'blocked';
  details: Record<string, unknown>;
  ip: string;
  userAgent: string;
}

export type AuditAction =
  | 'login.success'
  | 'login.failure'
  | 'login.blocked'
  | 'logout'
  | 'person.create'
  | 'person.update'
  | 'person.delete'
  | 'person.lock'
  | 'person.unlock'
  | 'credential.reset'
  | 'credential.update'
  | 'group.create'
  | 'group.update'
  | 'group.delete'
  | 'group.member.add'
  | 'group.member.remove'
  | 'oauth2.create'
  | 'oauth2.update'
  | 'oauth2.delete'
  | 'oauth2.secret.rotate'
  | 'token.create'
  | 'token.revoke'
  | 'system.backup'
  | 'system.config.update';

// ═══════════════════════════════════════════
// AUTH CONTEXT
// ═══════════════════════════════════════════

export interface AuthUser {
  sub: string;
  name: string;
  email: string;
  groups: string[];
  role: ConsoleRole;
  tenants: string[];
  accessToken: string;
  idToken: string;
  expiresAt: number;
}
```

### 2. Contratos de API — Kanidm Endpoints

```typescript
// api/contracts.ts — Mapeamento de todos os endpoints utilizados

export const API_CONTRACTS = {

  // ── AUTENTICAÇÃO ──────────────────────────────────
  auth: {
    login: {
      method: 'POST',
      path: '/v1/auth',
      body: { step: { init: 'username' } },  // multi-step
      response: 'AuthState',
    },
    validate: {
      method: 'GET',
      path: '/v1/auth_valid',
      response: '200 OK | 401',
    },
    logout: {
      method: 'GET',
      path: '/v1/logout',
      response: '200 OK',
    },
  },

  // ── PERSONS ───────────────────────────────────────
  persons: {
    list: {
      method: 'GET',
      path: '/v1/person',
      response: 'Person[]',
      queryKeys: ['id', 'persons'],
    },
    get: {
      method: 'GET',
      path: '/v1/person/:id',
      response: 'Person',
      queryKeys: ['id', 'persons', ':id'],
    },
    create: {
      method: 'POST',
      path: '/v1/person',
      body: 'CreatePersonRequest',
      response: '200 OK',
      invalidates: [['id', 'persons']],
    },
    update: {
      method: 'PATCH',
      path: '/v1/person/:id',
      body: 'Partial<PersonAttrs>',
      response: '200 OK',
      invalidates: [['id', 'persons'], ['id', 'persons', ':id']],
    },
    delete: {
      method: 'DELETE',
      path: '/v1/person/:id',
      response: '200 OK',
      invalidates: [['id', 'persons']],
    },
    getAttr: {
      method: 'GET',
      path: '/v1/person/:id/_attr/:attr',
      response: 'string[]',
    },
    putAttr: {
      method: 'PUT',
      path: '/v1/person/:id/_attr/:attr',
      body: 'string[]',
      response: '200 OK',
    },
    credentialStatus: {
      method: 'GET',
      path: '/v1/person/:id/_credential/_status',
      response: 'PersonCredentialStatus',
      queryKeys: ['id', 'persons', ':id', 'creds'],
    },
    credentialResetIntent: {
      method: 'POST',
      path: '/v1/person/:id/_credential/_update_intent/:ttl',
      response: 'CredentialResetIntent',
      note: 'ttl in seconds. Ex: 86400 = 24h',
    },
  },

  // ── SERVICE ACCOUNTS ──────────────────────────────
  serviceAccounts: {
    list: {
      method: 'GET',
      path: '/v1/service_account',
      response: 'ServiceAccount[]',
      queryKeys: ['id', 'service_accounts'],
    },
    get: {
      method: 'GET',
      path: '/v1/service_account/:id',
      response: 'ServiceAccount',
    },
    create: {
      method: 'POST',
      path: '/v1/service_account',
      body: 'CreateServiceAccountRequest',
      response: '200 OK',
      invalidates: [['id', 'service_accounts']],
    },
    delete: {
      method: 'DELETE',
      path: '/v1/service_account/:id',
      response: '200 OK',
      invalidates: [['id', 'service_accounts']],
    },
    createApiToken: {
      method: 'POST',
      path: '/v1/service_account/:id/_api_token',
      body: '{ label: string, expiry?: string }',
      response: 'GeneratedApiToken',
    },
    listApiTokens: {
      method: 'GET',
      path: '/v1/service_account/:id/_api_token',
      response: 'ApiToken[]',
    },
    revokeApiToken: {
      method: 'DELETE',
      path: '/v1/service_account/:id/_api_token/:token_id',
      response: '200 OK',
    },
  },

  // ── GROUPS ────────────────────────────────────────
  groups: {
    list: {
      method: 'GET',
      path: '/v1/group',
      response: 'Group[]',
      queryKeys: ['id', 'groups'],
    },
    get: {
      method: 'GET',
      path: '/v1/group/:id',
      response: 'Group',
      queryKeys: ['id', 'groups', ':id'],
    },
    create: {
      method: 'POST',
      path: '/v1/group',
      body: 'CreateGroupRequest',
      response: '200 OK',
      invalidates: [['id', 'groups']],
    },
    delete: {
      method: 'DELETE',
      path: '/v1/group/:id',
      response: '200 OK',
      invalidates: [['id', 'groups']],
    },
    getMembers: {
      method: 'GET',
      path: '/v1/group/:id/_attr/member',
      response: 'string[]',  // UUIDs
      queryKeys: ['id', 'groups', ':id', 'members'],
    },
    addMembers: {
      method: 'POST',
      path: '/v1/group/:id/_attr/member',
      body: 'string[]',      // UUIDs to add
      response: '200 OK',
      invalidates: [
        ['id', 'groups', ':id', 'members'],
        ['id', 'persons'],  // memberof mudou
      ],
    },
    removeMembers: {
      method: 'DELETE',
      path: '/v1/group/:id/_attr/member',
      body: 'string[]',
      response: '200 OK',
      invalidates: [
        ['id', 'groups', ':id', 'members'],
        ['id', 'persons'],
      ],
    },
  },

  // ── OAUTH2 ───────────────────────────────────────
  oauth2: {
    list: {
      method: 'GET',
      path: '/v1/oauth2',
      response: 'OAuth2Client[]',
      queryKeys: ['id', 'oauth2'],
    },
    get: {
      method: 'GET',
      path: '/v1/oauth2/:id',
      response: 'OAuth2Client',
      queryKeys: ['id', 'oauth2', ':id'],
    },
    createBasic: {
      method: 'POST',
      path: '/v1/oauth2/_basic',
      body: 'CreateOAuth2BasicRequest',
      response: '200 OK',
      invalidates: [['id', 'oauth2']],
    },
    createPublic: {
      method: 'POST',
      path: '/v1/oauth2/_public',
      body: 'CreateOAuth2PublicRequest',
      response: '200 OK',
      invalidates: [['id', 'oauth2']],
    },
    delete: {
      method: 'DELETE',
      path: '/v1/oauth2/:id',
      response: '200 OK',
      invalidates: [['id', 'oauth2']],
    },
    getBasicSecret: {
      method: 'GET',
      path: '/v1/oauth2/:id/_basic_secret',
      response: 'string',
    },
    updateScopeMap: {
      method: 'POST',
      path: '/v1/oauth2/:id/_scopemap/:group',
      body: 'string[]',        // scopes
      response: '200 OK',
      invalidates: [['id', 'oauth2', ':id']],
    },
    deleteScopeMap: {
      method: 'DELETE',
      path: '/v1/oauth2/:id/_scopemap/:group',
      response: '200 OK',
      invalidates: [['id', 'oauth2', ':id']],
    },
    updateSupScopeMap: {
      method: 'POST',
      path: '/v1/oauth2/:id/_sup_scopemap/:group',
      body: 'string[]',
      response: '200 OK',
    },
    enablePkce: {
      method: 'GET',
      path: '/v1/oauth2/:id/_pkce_enable',
      response: '200 OK',
      note: 'Kanidm habilita PKCE por default',
    },
    enableLocalhostRedirects: {
      method: 'GET',
      path: '/v1/oauth2/:id/_localhost_redirect_enable',
      response: '200 OK',
    },
    patchAttrs: {
      method: 'PATCH',
      path: '/v1/oauth2/:id',
      body: 'Partial<OAuth2ClientAttrs>',
      response: '200 OK',
    },
  },

  // ── SYSTEM ────────────────────────────────────────
  system: {
    status: {
      method: 'GET',
      path: '/status',
      response: 'SystemStatus',
      queryKeys: ['id', 'system', 'status'],
      note: 'Não requer autenticação',
    },
    schema: {
      method: 'GET',
      path: '/v1/schema',
      response: 'Schema',
    },
    openapi: {
      method: 'GET',
      path: '/docs/v1/openapi.json',
      response: 'OpenAPI spec',
    },
  },
} as const;
```

### 3. TanStack Query Hooks

```typescript
// api/hooks/usePersons.ts — Exemplo completo de hooks para Persons

import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query';
import { getPersonApi } from '../kanidm-client';
import type { CreatePersonRequest, PersonSummary } from '../../types/kanidm';

// ── Query Options (reutilizáveis em loaders e componentes) ──

export const personsQueryOptions = () =>
  queryOptions({
    queryKey: ['id', 'persons'],
    queryFn: async () => {
      const api = await getPersonApi();
      const persons = await api.listPersons();
      return persons.map(toPersonSummary);
    },
    staleTime: 30_000,     // 30s
    gcTime: 5 * 60_000,    // 5min
  });

export const personQueryOptions = (id: string) =>
  queryOptions({
    queryKey: ['id', 'persons', id],
    queryFn: async () => {
      const api = await getPersonApi();
      return api.getPerson({ id });
    },
    staleTime: 60_000,
  });

export const personCredsQueryOptions = (id: string) =>
  queryOptions({
    queryKey: ['id', 'persons', id, 'creds'],
    queryFn: async () => {
      const api = await getPersonApi();
      return api.getPersonCredentialStatus({ id });
    },
    staleTime: 60_000,
  });

// ── Mutations ──

export function useCreatePerson() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (data: CreatePersonRequest) => {
      const api = await getPersonApi();
      return api.createPerson({ body: data });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['id', 'persons'] });
    },
  });
}

export function useDeletePerson() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      const api = await getPersonApi();
      return api.deletePerson({ id });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['id', 'persons'] });
    },
  });
}

export function useAddPersonToGroup() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ groupId, personId }: { groupId: string; personId: string }) => {
      const api = await getGroupApi();
      return api.addGroupMembers({ id: groupId, body: [personId] });
    },
    onSuccess: (_, { groupId, personId }) => {
      queryClient.invalidateQueries({ queryKey: ['id', 'groups', groupId, 'members'] });
      queryClient.invalidateQueries({ queryKey: ['id', 'persons', personId] });
    },
  });
}

export function useResetCredentials() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ personId, ttlSeconds }: { personId: string; ttlSeconds: number }) => {
      const api = await getPersonApi();
      return api.createCredentialResetIntent({ id: personId, ttl: ttlSeconds });
    },
    // Não invalida — apenas gera token
  });
}

// ── Helpers ──

function toPersonSummary(person: Person): PersonSummary {
  const attrs = person.attrs;
  return {
    uuid: attrs.uuid[0],
    name: attrs.name[0],
    displayName: attrs.displayname[0],
    email: attrs.mail?.[0] ?? null,
    title: attrs.title?.[0] ?? null,
    groups: attrs.memberof ?? [],
    status: derivePersonStatus(attrs),
    lastLogin: null, // TODO: fonte de dados para isso
  };
}

function derivePersonStatus(attrs: PersonAttrs): PersonStatus {
  if (attrs.account_expire) {
    const expiry = new Date(attrs.account_expire[0]);
    if (expiry < new Date()) return PersonStatus.EXPIRED;
  }
  // Sem credenciais → pendente (requer checagem de credential_status)
  // Padrão: ativo
  return PersonStatus.ACTIVE;
}
```

---

<a id="rfc-003"></a>
## RFC-003: Estrutura de Rotas, Componentes e State Management

**Status:** Draft  
**Autor:** IntegrAllTech  
**Data:** 2026-02-16  

### 1. Árvore de Rotas (TanStack Router — File-Based)

```
console/src/routes/
│
├── __root.tsx                          # Root layout (sidebar, topbar, auth guard)
├── index.tsx                           # Redirect → /dashboard
│
├── auth/
│   ├── callback.tsx                    # OIDC callback handler
│   └── logout.tsx                      # Logout + cleanup
│
├── _authenticated/                     # Layout com auth guard
│   ├── dashboard/
│   │   └── index.tsx                   # Dashboard principal
│   │
│   ├── identity/
│   │   ├── persons/
│   │   │   ├── index.tsx              # Lista de persons
│   │   │   ├── new.tsx                # Wizard criar person
│   │   │   └── $personId/
│   │   │       ├── index.tsx          # Detalhe da person (tabs)
│   │   │       └── edit.tsx           # Editar person
│   │   │
│   │   └── service-accounts/
│   │       ├── index.tsx              # Lista de service accounts
│   │       ├── new.tsx                # Criar service account
│   │       └── $accountId/
│   │           └── index.tsx          # Detalhe (tabs)
│   │
│   ├── groups/
│   │   ├── index.tsx                  # Lista de grupos
│   │   ├── new.tsx                    # Criar grupo
│   │   └── $groupId/
│   │       └── index.tsx              # Detalhe do grupo (members, scopes)
│   │
│   ├── oauth2/
│   │   ├── index.tsx                  # Lista de OAuth2 clients
│   │   ├── new.tsx                    # Wizard criar client
│   │   └── $clientId/
│   │       └── index.tsx              # Detalhe do client (tabs)
│   │
│   ├── vault/
│   │   └── index.tsx                  # Vault status + links
│   │
│   ├── audit/
│   │   └── index.tsx                  # Log de auditoria
│   │
│   └── settings/
│       └── index.tsx                  # Configurações (tabs)
│
└── _public/                            # Layout sem auth
    ├── login.tsx                       # Landing → redirect to OIDC
    └── setup.tsx                       # Bootstrap wizard (primeiro acesso)
```

### 2. Route Definitions com Loaders

```typescript
// routes/_authenticated/identity/persons/index.tsx

import { createFileRoute } from '@tanstack/react-router';
import { personsQueryOptions } from '@/api/hooks/usePersons';
import { groupsQueryOptions } from '@/api/hooks/useGroups';
import { PersonsListPage } from '@/components/identity/PersonsListPage';

// Search params tipados — filtros persistem na URL
interface PersonsSearch {
  status?: PersonStatus;
  group?: string;
  mfa?: 'with' | 'without';
  q?: string;
  page?: number;
  pageSize?: number;
}

export const Route = createFileRoute('/_authenticated/identity/persons/')({
  // Valida e tipa os search params
  validateSearch: (search: Record<string, unknown>): PersonsSearch => ({
    status: search.status as PersonStatus | undefined,
    group: search.group as string | undefined,
    mfa: search.mfa as 'with' | 'without' | undefined,
    q: search.q as string | undefined,
    page: Number(search.page) || 1,
    pageSize: Number(search.pageSize) || 20,
  }),

  // Loader: pre-fetch dados antes de renderizar
  loader: async ({ context: { queryClient } }) => {
    // Prefetch em paralelo
    await Promise.all([
      queryClient.ensureQueryData(personsQueryOptions()),
      queryClient.ensureQueryData(groupsQueryOptions()),
    ]);
  },

  // Componente
  component: PersonsListPage,

  // Head (para SSR se habilitado)
  head: () => ({
    meta: [{ title: 'Persons | ArchGuard Console' }],
  }),
});
```

```typescript
// routes/_authenticated/identity/persons/$personId/index.tsx

import { createFileRoute } from '@tanstack/react-router';
import { personQueryOptions, personCredsQueryOptions } from '@/api/hooks/usePersons';
import { PersonDetailPage } from '@/components/identity/PersonDetailPage';

interface PersonDetailSearch {
  tab?: 'attributes' | 'groups' | 'credentials' | 'sessions' | 'log';
}

export const Route = createFileRoute('/_authenticated/identity/persons/$personId/')({
  validateSearch: (search): PersonDetailSearch => ({
    tab: (search.tab as PersonDetailSearch['tab']) || 'attributes',
  }),

  loader: async ({ context: { queryClient }, params: { personId } }) => {
    await Promise.all([
      queryClient.ensureQueryData(personQueryOptions(personId)),
      queryClient.ensureQueryData(personCredsQueryOptions(personId)),
    ]);
  },

  component: PersonDetailPage,
});
```

### 3. Root Layout

```typescript
// routes/__root.tsx

import { createRootRouteWithContext, Outlet } from '@tanstack/react-router';
import type { QueryClient } from '@tanstack/react-query';
import type { AuthUser } from '@/types/kanidm';

interface RouterContext {
  queryClient: QueryClient;
  auth: AuthUser | null;
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: RootLayout,
  beforeLoad: async ({ context }) => {
    // Auth check happens here — redirect to login if needed
    if (!context.auth) {
      throw redirect({ to: '/login' });
    }
  },
});

function RootLayout() {
  return (
    <div className="flex h-screen">
      <AppSidebar />
      <div className="flex-1 flex flex-col overflow-hidden">
        <TopBar />
        <main className="flex-1 overflow-auto p-6">
          <Outlet />
        </main>
        <StatusBar />
      </div>
      <CommandPalette />
    </div>
  );
}
```

### 4. Authenticated Layout Guard

```typescript
// routes/_authenticated.tsx

import { createFileRoute, redirect, Outlet } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated')({
  beforeLoad: async ({ context }) => {
    if (!context.auth) {
      throw redirect({
        to: '/login',
        search: { returnTo: window.location.pathname },
      });
    }
  },
  component: () => <Outlet />,
});
```

### 5. Árvore de Componentes

```
console/src/components/
│
├── ui/                                    # Shadcn/ui (copiados)
│   ├── button.tsx
│   ├── card.tsx
│   ├── command.tsx
│   ├── data-table.tsx
│   ├── dialog.tsx
│   ├── dropdown-menu.tsx
│   ├── form.tsx
│   ├── input.tsx
│   ├── select.tsx
│   ├── sheet.tsx
│   ├── sidebar.tsx
│   ├── skeleton.tsx
│   ├── tabs.tsx
│   ├── toast.tsx
│   └── ...
│
├── layout/                                # Estrutura geral
│   ├── AppSidebar.tsx                     # Menu lateral
│   ├── TopBar.tsx                         # Barra superior (search, user menu)
│   ├── StatusBar.tsx                      # Barra inferior (status dos serviços)
│   ├── CommandPalette.tsx                 # ⌘K
│   ├── Breadcrumbs.tsx
│   └── PageHeader.tsx                     # Título + ações da página
│
├── auth/                                  # Autenticação
│   ├── AuthProvider.tsx                   # Context de auth
│   ├── OidcCallback.tsx                   # Handler do callback
│   ├── LoginRedirect.tsx                  # Redirect para OIDC
│   └── UserMenu.tsx                       # Menu do usuário logado
│
├── dashboard/                             # Dashboard
│   ├── DashboardPage.tsx
│   ├── StatsCards.tsx
│   ├── RecentActivity.tsx
│   ├── QuickActions.tsx
│   └── ServiceHealth.tsx
│
├── identity/                              # Persons + Service Accounts
│   ├── PersonsListPage.tsx
│   ├── PersonDetailPage.tsx
│   ├── CreatePersonWizard.tsx
│   ├── EditPersonForm.tsx
│   ├── PersonAttributesTab.tsx
│   ├── PersonGroupsTab.tsx
│   ├── PersonCredentialsTab.tsx
│   ├── PersonSessionsTab.tsx
│   ├── CredentialResetDialog.tsx
│   ├── InviteLinkDialog.tsx
│   ├── BulkOperationsDialog.tsx
│   ├── ServiceAccountsListPage.tsx
│   ├── ServiceAccountDetailPage.tsx
│   ├── CreateServiceAccountForm.tsx
│   ├── ApiTokenCreateDialog.tsx
│   └── ApiTokenList.tsx
│
├── groups/                                # Grupos
│   ├── GroupsListPage.tsx
│   ├── GroupDetailPage.tsx
│   ├── CreateGroupForm.tsx
│   ├── GroupMembersTab.tsx
│   ├── GroupScopesTab.tsx
│   ├── AddMemberDialog.tsx
│   └── GroupHierarchyView.tsx
│
├── oauth2/                                # OAuth2 Clients
│   ├── OAuth2ListPage.tsx
│   ├── OAuth2DetailPage.tsx
│   ├── CreateOAuth2Wizard.tsx
│   ├── OAuth2ConfigTab.tsx
│   ├── OAuth2ScopeMapsTab.tsx
│   ├── OAuth2SnippetsTab.tsx
│   ├── OAuth2DangerZoneTab.tsx
│   ├── ScopeMapEditor.tsx
│   ├── SecretRotateDialog.tsx
│   └── IntegrationSnippets.tsx
│
├── vault/                                 # Vault Status
│   ├── VaultStatusPage.tsx
│   ├── VaultStatsCards.tsx
│   ├── SmtpStatusCard.tsx
│   └── SsoBridgeStatusCard.tsx
│
├── audit/                                 # Auditoria
│   ├── AuditPage.tsx
│   ├── AuditTable.tsx
│   ├── AuditFilters.tsx
│   └── AuditAlerts.tsx
│
├── settings/                              # Configurações
│   ├── SettingsPage.tsx
│   ├── GeneralTab.tsx
│   ├── PoliciesTab.tsx
│   ├── BackupTab.tsx
│   ├── ReplicationTab.tsx
│   └── AboutTab.tsx
│
└── shared/                                # Componentes compartilhados
    ├── PersonAvatar.tsx                   # Avatar com iniciais/foto
    ├── GroupBadge.tsx                      # Badge colorido por tipo de grupo
    ├── StatusIndicator.tsx                # 🟢🟡🔴 
    ├── CopyButton.tsx                     # Copiar para clipboard
    ├── ConfirmDialog.tsx                  # Dialog de confirmação genérico
    ├── EmptyState.tsx                     # Estado vazio com call to action
    ├── ErrorBoundary.tsx                  # Tratamento de erros
    ├── LoadingState.tsx                   # Skeletons
    ├── SearchInput.tsx                    # Input de busca com debounce
    ├── TenantSelector.tsx                 # Selector de tenant (SUPER_ADMIN)
    └── PermissionGate.tsx                 # Renderiza children apenas se role permite
```

### 6. State Management Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    STATE ARCHITECTURE                             │
│                                                                   │
│  ┌───────────────────────────────────────────────────────────┐   │
│  │                    URL STATE                               │   │
│  │              (TanStack Router)                             │   │
│  │                                                           │   │
│  │  /identity/persons?status=active&group=rio&page=2         │   │
│  │  /identity/persons/abc123?tab=credentials                 │   │
│  │  /oauth2/new?step=2                                       │   │
│  │                                                           │   │
│  │  → Filtros, paginação, tabs, wizard steps                 │   │
│  │  → Compartilhável via URL, sobrevive F5                   │   │
│  └───────────────────────────────────────────────────────────┘   │
│                         ▲                                         │
│                         │ useSearch() / useNavigate()             │
│                         ▼                                         │
│  ┌───────────────────────────────────────────────────────────┐   │
│  │                  SERVER STATE                              │   │
│  │             (TanStack Query v5)                            │   │
│  │                                                           │   │
│  │  Cache:     Persons[], Groups[], OAuth2Clients[]          │   │
│  │  Mutations: createPerson, addMember, resetCredential      │   │
│  │  Polling:   systemStatus (10s), vaultStatus (30s)         │   │
│  │  Prefetch:  Route loaders ensureQueryData()               │   │
│  │                                                           │   │
│  │  → Source of truth para dados do Kanidm + AliasVault      │   │
│  └───────────────────────────────────────────────────────────┘   │
│                         ▲                                         │
│                         │ useQuery() / useMutation()             │
│                         ▼                                         │
│  ┌───────────────────────────────────────────────────────────┐   │
│  │                  CLIENT STATE                              │   │
│  │             (TanStack Store)                               │   │
│  │                                                           │   │
│  │  authStore:                                               │   │
│  │    user: AuthUser | null                                  │   │
│  │    isAuthenticated: boolean                               │   │
│  │    role: ConsoleRole                                      │   │
│  │    tenants: string[]                                      │   │
│  │                                                           │   │
│  │  uiStore:                                                 │   │
│  │    sidebarCollapsed: boolean                              │   │
│  │    commandPaletteOpen: boolean                            │   │
│  │    theme: 'light' | 'dark' | 'system'                    │   │
│  │    locale: 'pt-BR' | 'en-US'                             │   │
│  │    selectedTenant: string | null (SUPER_ADMIN only)       │   │
│  │                                                           │   │
│  │  → Estado da sessão do browser, preferências efêmeras     │   │
│  └───────────────────────────────────────────────────────────┘   │
│                         ▲                                         │
│                         │ useStore()                              │
│                         ▼                                         │
│  ┌───────────────────────────────────────────────────────────┐   │
│  │                  FORM STATE                                │   │
│  │             (TanStack Form)                                │   │
│  │                                                           │   │
│  │  CreatePersonForm → validates → mutação via Query         │   │
│  │  CreateOAuth2Wizard → step state + validação por step     │   │
│  │  EditPersonForm → dirty tracking, optimistic update       │   │
│  │                                                           │   │
│  │  → Estado temporário de formulários em preenchimento      │   │
│  └───────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### 7. TanStack Store Definitions

```typescript
// stores/auth-store.ts

import { Store } from '@tanstack/store';
import type { AuthUser, ConsoleRole } from '@/types/kanidm';
import { deriveRole, deriveTenants } from '@/auth/roles';

interface AuthState {
  user: AuthUser | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  role: ConsoleRole;
  tenants: string[];
}

export const authStore = new Store<AuthState>({
  user: null,
  isAuthenticated: false,
  isLoading: true,
  role: ConsoleRole.VIEWER,
  tenants: [],
});

export function setAuthUser(user: AuthUser) {
  authStore.setState((prev) => ({
    ...prev,
    user,
    isAuthenticated: true,
    isLoading: false,
    role: deriveRole(user.groups),
    tenants: deriveTenants(user.groups),
  }));
}

export function clearAuth() {
  authStore.setState((prev) => ({
    ...prev,
    user: null,
    isAuthenticated: false,
    isLoading: false,
    role: ConsoleRole.VIEWER,
    tenants: [],
  }));
}

// stores/ui-store.ts

import { Store } from '@tanstack/store';

interface UiState {
  sidebarCollapsed: boolean;
  commandPaletteOpen: boolean;
  theme: 'light' | 'dark' | 'system';
  locale: 'pt-BR' | 'en-US';
  selectedTenant: string | null;
}

export const uiStore = new Store<UiState>({
  sidebarCollapsed: false,
  commandPaletteOpen: false,
  theme: 'system',
  locale: 'pt-BR',
  selectedTenant: null,
});

export function toggleSidebar() {
  uiStore.setState((prev) => ({
    ...prev,
    sidebarCollapsed: !prev.sidebarCollapsed,
  }));
}

export function toggleCommandPalette() {
  uiStore.setState((prev) => ({
    ...prev,
    commandPaletteOpen: !prev.commandPaletteOpen,
  }));
}

export function selectTenant(tenant: string | null) {
  uiStore.setState((prev) => ({
    ...prev,
    selectedTenant: tenant,
  }));
}
```

---

<a id="rfc-004"></a>
## RFC-004: Regras de Negócio e Permissões por Role/Grupo

**Status:** Draft  
**Autor:** IntegrAllTech  
**Data:** 2026-02-16  

### 1. Roles do Console

```
SUPER_ADMIN    → IntegrAllTech: controle total da plataforma
                 Grupos: archguard_admins, idm_admins

TENANT_ADMIN   → Admin do cliente: gerencia seu tenant
                 Grupos: {tenant}_admins

SERVICE_DESK   → Suporte: reset de credenciais
                 Grupos: idm_service_desk

VIEWER         → Somente leitura
                 Grupos: qualquer outro grupo
```

### 2. Matriz de Permissões Completa

#### 2.1 Dashboard

| Funcionalidade | SUPER_ADMIN | TENANT_ADMIN | SERVICE_DESK | VIEWER |
|---|---|---|---|---|
| Ver stats globais | ✅ | ❌ | ❌ | ❌ |
| Ver stats do tenant | ✅ | ✅ (seu) | ❌ | ❌ |
| Ver stats pessoais | ✅ | ✅ | ✅ | ✅ |
| Quick action: criar person | ✅ | ✅ (no tenant) | ❌ | ❌ |
| Quick action: criar grupo | ✅ | ✅ (no tenant) | ❌ | ❌ |
| Quick action: criar OAuth2 | ✅ | ❌ | ❌ | ❌ |
| Quick action: reset creds | ✅ | ✅ (no tenant) | ✅ (escopo) | ❌ |
| Ver saúde dos serviços | ✅ | ❌ | ❌ | ❌ |
| Ver atividade recente | ✅ | ✅ (filtrada) | ✅ (limitada) | ❌ |
| Selector de tenant | ✅ | ❌ (fixo) | ❌ | ❌ |

#### 2.2 Persons

| Funcionalidade | SUPER_ADMIN | TENANT_ADMIN | SERVICE_DESK | VIEWER |
|---|---|---|---|---|
| Listar todas as persons | ✅ | ❌ | ❌ | ❌ |
| Listar persons do tenant | ✅ | ✅ | ✅ | ❌ |
| Ver detalhes de person | ✅ | ✅ (tenant) | ✅ (tenant) | ❌ (só self) |
| Criar person | ✅ | ✅ (no tenant) | ❌ | ❌ |
| Editar person (atributos) | ✅ | ✅ (tenant) | ❌ | ❌ |
| Deletar person | ✅ | ❌ | ❌ | ❌ |
| Bloquear/desbloquear | ✅ | ✅ (tenant) | ❌ | ❌ |
| Reset credentials | ✅ | ✅ (tenant) | ✅ (tenant) | ❌ |
| Ver credential status | ✅ | ✅ (tenant) | ✅ (tenant) | ✅ (self) |
| Adicionar a grupo | ✅ | ✅ (tenant groups) | ❌ | ❌ |
| Remover de grupo | ✅ | ✅ (tenant groups) | ❌ | ❌ |
| Bulk import CSV | ✅ | ✅ (no tenant) | ❌ | ❌ |
| Bulk export CSV | ✅ | ✅ (tenant) | ❌ | ❌ |
| Ver sessões | ✅ | ✅ (tenant) | ❌ | ✅ (self) |
| Encerrar sessão de outro | ✅ | ✅ (tenant) | ❌ | ❌ |

#### 2.3 Service Accounts

| Funcionalidade | SUPER_ADMIN | TENANT_ADMIN | SERVICE_DESK | VIEWER |
|---|---|---|---|---|
| Listar service accounts | ✅ | ❌ | ❌ | ❌ |
| Criar service account | ✅ | ❌ | ❌ | ❌ |
| Deletar service account | ✅ | ❌ | ❌ | ❌ |
| Gerar API token | ✅ | ❌ | ❌ | ❌ |
| Revogar API token | ✅ | ❌ | ❌ | ❌ |
| Ver tokens | ✅ | ❌ | ❌ | ❌ |

#### 2.4 Groups

| Funcionalidade | SUPER_ADMIN | TENANT_ADMIN | SERVICE_DESK | VIEWER |
|---|---|---|---|---|
| Listar todos os grupos | ✅ | ❌ | ❌ | ❌ |
| Listar grupos do tenant | ✅ | ✅ | ❌ | ❌ |
| Ver detalhes do grupo | ✅ | ✅ (tenant) | ❌ | ❌ |
| Criar grupo | ✅ | ✅ (prefixado) | ❌ | ❌ |
| Deletar grupo | ✅ | ❌ | ❌ | ❌ |
| Adicionar membro | ✅ | ✅ (tenant) | ❌ | ❌ |
| Remover membro | ✅ | ✅ (tenant) | ❌ | ❌ |
| Ver scope maps | ✅ | ✅ (tenant) | ❌ | ❌ |
| Editar scope maps | ✅ | ❌ | ❌ | ❌ |
| Visualização hierárquica | ✅ | ✅ (tenant) | ❌ | ❌ |

#### 2.5 OAuth2 Clients

| Funcionalidade | SUPER_ADMIN | TENANT_ADMIN | SERVICE_DESK | VIEWER |
|---|---|---|---|---|
| Listar clients | ✅ | ❌ | ❌ | ❌ |
| Ver detalhes | ✅ | ❌ | ❌ | ❌ |
| Criar client | ✅ | ❌ | ❌ | ❌ |
| Deletar client | ✅ | ❌ | ❌ | ❌ |
| Rotacionar secret | ✅ | ❌ | ❌ | ❌ |
| Editar scope maps | ✅ | ❌ | ❌ | ❌ |
| Editar redirect URIs | ✅ | ❌ | ❌ | ❌ |
| Gerar snippets | ✅ | ❌ | ❌ | ❌ |

#### 2.6 Vault

| Funcionalidade | SUPER_ADMIN | TENANT_ADMIN | SERVICE_DESK | VIEWER |
|---|---|---|---|---|
| Ver status | ✅ | ❌ | ❌ | ❌ |
| Ver métricas | ✅ | ❌ | ❌ | ❌ |
| Abrir Vault UI | ✅ | ✅ | ✅ | ✅ |
| Configurar SSO Bridge | ✅ | ❌ | ❌ | ❌ |
| Backup | ✅ | ❌ | ❌ | ❌ |

#### 2.7 Auditoria

| Funcionalidade | SUPER_ADMIN | TENANT_ADMIN | SERVICE_DESK | VIEWER |
|---|---|---|---|---|
| Ver todos os logs | ✅ | ❌ | ❌ | ❌ |
| Ver logs do tenant | ✅ | ✅ | ❌ | ❌ |
| Ver seus próprios logs | ✅ | ✅ | ✅ | ✅ |
| Exportar logs | ✅ | ✅ (tenant) | ❌ | ❌ |
| Configurar alertas | ✅ | ❌ | ❌ | ❌ |

#### 2.8 Settings

| Funcionalidade | SUPER_ADMIN | TENANT_ADMIN | SERVICE_DESK | VIEWER |
|---|---|---|---|---|
| Ver configurações gerais | ✅ | ❌ | ❌ | ❌ |
| Editar políticas | ✅ | ❌ | ❌ | ❌ |
| Gerenciar backups | ✅ | ❌ | ❌ | ❌ |
| Ver replicação | ✅ | ❌ | ❌ | ❌ |
| Ver "Sobre" | ✅ | ✅ | ✅ | ✅ |

### 3. Implementação da Autorização

#### 3.1 PermissionGate Component

```typescript
// components/shared/PermissionGate.tsx

import { useAuth } from '@/auth/AuthProvider';
import { ConsoleRole } from '@/auth/roles';

interface PermissionGateProps {
  /** Roles que têm acesso */
  allow: ConsoleRole[];

  /** Se true, verifica se o recurso pertence ao tenant do admin */
  tenantScoped?: boolean;

  /** Tenant do recurso sendo acessado */
  resourceTenant?: string;

  /** Componente a renderizar se sem permissão (default: null) */
  fallback?: React.ReactNode;

  children: React.ReactNode;
}

export function PermissionGate({
  allow,
  tenantScoped = false,
  resourceTenant,
  fallback = null,
  children,
}: PermissionGateProps) {
  const { role, tenants, isSuperAdmin } = useAuth();

  // Super Admin sempre tem acesso
  if (isSuperAdmin) return <>{children}</>;

  // Verifica role
  if (!allow.includes(role)) return <>{fallback}</>;

  // Se tenant-scoped, verifica se o admin tem acesso ao tenant
  if (tenantScoped && resourceTenant) {
    if (!tenants.includes(resourceTenant)) return <>{fallback}</>;
  }

  return <>{children}</>;
}
```

**Uso:**

```tsx
// Na lista de persons
<PermissionGate allow={[ConsoleRole.SUPER_ADMIN, ConsoleRole.TENANT_ADMIN]}>
  <Button onClick={() => navigate({ to: '/identity/persons/new' })}>
    + Nova Pessoa
  </Button>
</PermissionGate>

// No detalhe de uma person
<PermissionGate
  allow={[ConsoleRole.SUPER_ADMIN, ConsoleRole.TENANT_ADMIN, ConsoleRole.SERVICE_DESK]}
  tenantScoped
  resourceTenant={person.tenant}
>
  <Button onClick={handleResetCredentials}>
    Reset Credenciais
  </Button>
</PermissionGate>

// Seção inteira visível apenas para SUPER_ADMIN
<PermissionGate allow={[ConsoleRole.SUPER_ADMIN]}>
  <OAuthManagementSection />
</PermissionGate>
```

#### 3.2 Route-Level Permission

```typescript
// routes/_authenticated/oauth2/index.tsx

export const Route = createFileRoute('/_authenticated/oauth2/')({
  beforeLoad: ({ context }) => {
    const role = deriveRole(context.auth.groups);
    if (role !== ConsoleRole.SUPER_ADMIN) {
      throw redirect({ to: '/dashboard' });
    }
  },
  component: OAuth2ListPage,
});
```

#### 3.3 usePermission Hook

```typescript
// hooks/usePermission.ts

import { useAuth } from '@/auth/AuthProvider';
import { ConsoleRole } from '@/auth/roles';

type Permission =
  | 'person.create'
  | 'person.delete'
  | 'person.edit'
  | 'person.view'
  | 'person.reset_creds'
  | 'person.lock'
  | 'person.bulk'
  | 'group.create'
  | 'group.delete'
  | 'group.edit_members'
  | 'group.view'
  | 'oauth2.manage'
  | 'service_account.manage'
  | 'vault.manage'
  | 'audit.view_all'
  | 'audit.view_tenant'
  | 'audit.export'
  | 'settings.manage'
  | 'tenant.select';

const PERMISSION_MAP: Record<Permission, ConsoleRole[]> = {
  'person.create':       [ConsoleRole.SUPER_ADMIN, ConsoleRole.TENANT_ADMIN],
  'person.delete':       [ConsoleRole.SUPER_ADMIN],
  'person.edit':         [ConsoleRole.SUPER_ADMIN, ConsoleRole.TENANT_ADMIN],
  'person.view':         [ConsoleRole.SUPER_ADMIN, ConsoleRole.TENANT_ADMIN, ConsoleRole.SERVICE_DESK],
  'person.reset_creds':  [ConsoleRole.SUPER_ADMIN, ConsoleRole.TENANT_ADMIN, ConsoleRole.SERVICE_DESK],
  'person.lock':         [ConsoleRole.SUPER_ADMIN, ConsoleRole.TENANT_ADMIN],
  'person.bulk':         [ConsoleRole.SUPER_ADMIN, ConsoleRole.TENANT_ADMIN],
  'group.create':        [ConsoleRole.SUPER_ADMIN, ConsoleRole.TENANT_ADMIN],
  'group.delete':        [ConsoleRole.SUPER_ADMIN],
  'group.edit_members':  [ConsoleRole.SUPER_ADMIN, ConsoleRole.TENANT_ADMIN],
  'group.view':          [ConsoleRole.SUPER_ADMIN, ConsoleRole.TENANT_ADMIN],
  'oauth2.manage':       [ConsoleRole.SUPER_ADMIN],
  'service_account.manage': [ConsoleRole.SUPER_ADMIN],
  'vault.manage':        [ConsoleRole.SUPER_ADMIN],
  'audit.view_all':      [ConsoleRole.SUPER_ADMIN],
  'audit.view_tenant':   [ConsoleRole.SUPER_ADMIN, ConsoleRole.TENANT_ADMIN],
  'audit.export':        [ConsoleRole.SUPER_ADMIN, ConsoleRole.TENANT_ADMIN],
  'settings.manage':     [ConsoleRole.SUPER_ADMIN],
  'tenant.select':       [ConsoleRole.SUPER_ADMIN],
};

export function usePermission() {
  const { role, tenants } = useAuth();

  return {
    can: (permission: Permission) => {
      return PERMISSION_MAP[permission]?.includes(role) ?? false;
    },

    canForTenant: (permission: Permission, resourceTenant: string) => {
      if (role === ConsoleRole.SUPER_ADMIN) return true;
      if (!PERMISSION_MAP[permission]?.includes(role)) return false;
      return tenants.includes(resourceTenant);
    },
  };
}
```

**Uso:**

```tsx
function PersonActions({ person }: { person: PersonSummary }) {
  const { can, canForTenant } = usePermission();
  const tenant = extractTenant(person.groups);

  return (
    <DropdownMenu>
      {canForTenant('person.edit', tenant) && (
        <DropdownMenuItem onClick={handleEdit}>Editar</DropdownMenuItem>
      )}
      {canForTenant('person.reset_creds', tenant) && (
        <DropdownMenuItem onClick={handleReset}>Reset Credenciais</DropdownMenuItem>
      )}
      {canForTenant('person.lock', tenant) && (
        <DropdownMenuItem onClick={handleLock}>Bloquear</DropdownMenuItem>
      )}
      {can('person.delete') && (
        <DropdownMenuItem variant="destructive" onClick={handleDelete}>
          Excluir
        </DropdownMenuItem>
      )}
    </DropdownMenu>
  );
}
```

### 4. Regras de Negócio por Módulo

#### 4.1 Persons

| Regra | Descrição |
|---|---|
| **P-001** | Username deve ser único, lowercase, `[a-z0-9._]`, 3-64 chars |
| **P-002** | Email deve ser único no sistema |
| **P-003** | TENANT_ADMIN só pode criar persons em grupos do seu tenant |
| **P-004** | TENANT_ADMIN deve auto-adicionar nova person ao grupo raiz do tenant |
| **P-005** | Deletar person requer confirmação digitando o username |
| **P-006** | Bloquear define `account_expire` no passado |
| **P-007** | Desbloquear remove `account_expire` ou define no futuro |
| **P-008** | Reset credential gera token com TTL; token é single-use |
| **P-009** | Link de convite = `{origin}/ui/reset?token={token}` |
| **P-010** | Bulk import aceita CSV com colunas: name, displayname, mail, groups |
| **P-011** | Ao criar via wizard, Step 1 (dados) → Step 2 (grupos) → Step 3 (credenciais) |

#### 4.2 Service Accounts

| Regra | Descrição |
|---|---|
| **SA-001** | Apenas SUPER_ADMIN gerencia service accounts |
| **SA-002** | API Token exibido apenas na criação (não armazenado no Console) |
| **SA-003** | Token revogado é invalidado imediatamente (sem grace period) |
| **SA-004** | Label do token é obrigatório e descritivo (ex: "deploy-ci") |
| **SA-005** | Expiry é opcional; sem expiry = token permanente (desaconselhado) |

#### 4.3 Groups

| Regra | Descrição |
|---|---|
| **G-001** | Nome de grupo é auto-prefixado com o tenant selecionado |
| **G-002** | TENANT_ADMIN não pode criar grupos sem prefixo do tenant |
| **G-003** | Grupos de sistema (idm_*) são readonly no Console |
| **G-004** | Deletar grupo que é scope map de um OAuth2 client requer confirmação extra |
| **G-005** | GroupType é derivado da convenção de nome (ver ADR-007) |
| **G-006** | Visualização hierárquica agrupa por tenant |

#### 4.4 OAuth2 Clients

| Regra | Descrição |
|---|---|
| **O-001** | Apenas SUPER_ADMIN gerencia OAuth2 clients |
| **O-002** | Client name (= client_id) deve ser slug válido, único |
| **O-003** | Origin URL deve ser HTTPS (exceto localhost para dev) |
| **O-004** | PKCE S256 é sempre habilitado (Kanidm requirement) |
| **O-005** | Basic secret é exibido apenas na criação |
| **O-006** | Rotacionar secret invalida o anterior imediatamente |
| **O-007** | Deletar client requer confirmação digitando o client name |
| **O-008** | Snippets são gerados client-side com base nos dados do client |
| **O-009** | Wizard: Step 1 (tipo) → Step 2 (dados) → Step 3 (scope maps) → Step 4 (resumo) |

#### 4.5 Vault

| Regra | Descrição |
|---|---|
| **V-001** | Status do vault é polling a cada 30s |
| **V-002** | "Abrir Vault UI" abre nova aba para o AliasVault |
| **V-003** | SSO Bridge configuration é uma feature flag |
| **V-004** | Backup do vault requer SUPER_ADMIN |

#### 4.6 Auditoria

| Regra | Descrição |
|---|---|
| **A-001** | Logs de auditoria são somente leitura (append-only) |
| **A-002** | TENANT_ADMIN vê apenas logs de ações em recursos do seu tenant |
| **A-003** | Exportação gera CSV ou JSON |
| **A-004** | Logs retêm 90 dias por padrão |
| **A-005** | Alertas: 3+ login failures → flag na lista, com ação de unlock |

#### 4.7 Sidebar Navigation

| Item | SUPER_ADMIN | TENANT_ADMIN | SERVICE_DESK | VIEWER |
|---|---|---|---|---|
| Dashboard | ✅ | ✅ | ✅ | ✅ |
| Identidades > Persons | ✅ | ✅ | ✅ | ❌ |
| Identidades > Service Accounts | ✅ | ❌ | ❌ | ❌ |
| Grupos | ✅ | ✅ | ❌ | ❌ |
| OAuth2 | ✅ | ❌ | ❌ | ❌ |
| Vault | ✅ | Link only | Link only | Link only |
| Auditoria | ✅ | ✅ | ❌ | ❌ |
| Settings | ✅ | ❌ | ❌ | ❌ |

---

## Apêndice A: Dependências do Projeto

```json
{
  "dependencies": {
    "@tanstack/react-query": "^5.x",
    "@tanstack/react-router": "^1.x",
    "@tanstack/react-store": "^0.x",
    "@tanstack/react-table": "^8.x",
    "@tanstack/react-form": "^0.x",
    "@tanstack/start": "^1.x",
    "oidc-client-ts": "^3.x",
    "i18next": "^23.x",
    "react-i18next": "^14.x",
    "tailwindcss": "^4.x",
    "class-variance-authority": "^0.x",
    "clsx": "^2.x",
    "lucide-react": "^0.x",
    "date-fns": "^3.x",
    "zod": "^3.x"
  },
  "devDependencies": {
    "@openapitools/openapi-generator-cli": "^2.x",
    "typescript": "^5.x",
    "vitest": "^2.x",
    "@testing-library/react": "^16.x",
    "playwright": "^1.x"
  }
}
```

## Apêndice B: Convenções de Código

| Aspecto | Convenção |
|---|---|
| File naming | kebab-case: `person-detail-page.tsx` |
| Component naming | PascalCase: `PersonDetailPage` |
| Hook naming | camelCase com `use` prefix: `usePersons` |
| Store naming | camelCase com `Store` suffix: `authStore` |
| Query keys | Array de strings: `['id', 'persons', id]` |
| Route paths | kebab-case: `/identity/persons/$personId` |
| i18n keys | dot-notation: `identity.persons.create.title` |
| API types | PascalCase: `CreatePersonRequest` |
| CSS | Tailwind utilities only (no custom CSS) |
| Exports | Named exports (no default except route components) |

---

*ArchGuard Console — ADRs & RFCs v1.0*  
*IntegrAllTech — Fevereiro 2026*
