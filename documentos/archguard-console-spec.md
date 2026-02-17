# ArchGuard Console — Especificação Funcional e Técnica

### TanStack Start + React + Shadcn/ui

**Versão:** 1.0  
**Data:** Fevereiro 2026  
**Stack:** TanStack Start · TanStack Router · TanStack Query · Shadcn/ui · oidc-client-ts  
**Engines:** Kanidm (Identity) · AliasVault (Vault)

---

## Índice

1. [Stack e Decisões de Arquitetura](#1-stack-e-decisões-de-arquitetura)
2. [Estrutura de Rotas e Projeto](#2-estrutura-de-rotas-e-projeto)
3. [Autenticação e Autorização](#3-autenticação-e-autorização)
4. [Modelo de Dados e Contratos de API](#4-modelo-de-dados-e-contratos-de-api)
5. [Módulo: Dashboard](#5-módulo-dashboard)
6. [Módulo: Identidades (Persons)](#6-módulo-identidades-persons)
7. [Módulo: Service Accounts](#7-módulo-service-accounts)
8. [Módulo: Grupos](#8-módulo-grupos)
9. [Módulo: OAuth2 / SSO](#9-módulo-oauth2--sso)
10. [Módulo: Vault](#10-módulo-vault)
11. [Módulo: Auditoria](#11-módulo-auditoria)
12. [Módulo: Configurações](#12-módulo-configurações)
13. [Regras de Negócio e Permissões](#13-regras-de-negócio-e-permissões)
14. [State Management](#14-state-management)
15. [Componentes Compartilhados](#15-componentes-compartilhados)
16. [Tratamento de Erros](#16-tratamento-de-erros)
17. [Internacionalização](#17-internacionalização)

---

## 1. Stack e Decisões de Arquitetura

### 1.1 Por que TanStack Start

| Critério | TanStack Start | Next.js |
|---|---|---|
| **Footprint** | Leve, Vite-based, sem runtime complexo | Pesado, runtime Next.js, Vercel-optimized |
| **Routing** | TanStack Router — type-safe nativo, search params first-class | App Router — RSC complexo, convenções rígidas |
| **Data fetching** | TanStack Query integrado — cache, mutations, invalidation | Server Actions + cache instável, fetch deduplication |
| **SSR** | Opcional — SPA puro ou SSR conforme necessidade | SSR/RSC por padrão, opt-out complexo |
| **Auth** | beforeLoad + context — controle total, sem magic | Middleware + next-auth — opinionated |
| **Docker** | Build simples, output estático ou Node server | Requer Node runtime, standalone mode |
| **Fit para admin console** | Ideal — SPA com API calls, type-safety end-to-end | Overkill — SEO irrelevante, RSC desnecessário |

**Decisão:** TanStack Start como full-stack framework, operando primariamente como SPA com server functions para operações sensíveis (gerenciamento de sessão OIDC).

### 1.2 Stack Completa

| Camada | Tecnologia | Papel |
|---|---|---|
| Framework | TanStack Start | Full-stack, file-based routing, SSR opcional |
| Router | TanStack Router | Type-safe routing, beforeLoad guards, search params |
| Data | TanStack Query v5 | Cache, mutations, optimistic updates, invalidation |
| UI | Shadcn/ui | Primitivos acessíveis, composáveis, customizáveis |
| Styling | Tailwind CSS v4 | Utility-first, design tokens |
| Forms | TanStack Form | Type-safe forms, validação integrada |
| Tables | TanStack Table v8 | Sorting, filtering, pagination server-side |
| Auth | oidc-client-ts | OIDC/OAuth2, PKCE S256, token lifecycle |
| API Client | openapi-fetch | Type-safe HTTP client gerado do OpenAPI schema |
| Validação | Zod v3 | Schema validation, inferência de tipos |
| Ícones | Lucide React | Consistente com Shadcn |
| i18n | i18next | PT-BR / EN |
| Testes | Vitest + Testing Library | Unit + integration |
| Build | Vite v6 | HMR, tree-shaking, bundling |

### 1.3 Arquitetura de Comunicação

```
┌─────────────────────────────────────────────┐
│           ArchGuard Console                  │
│          (TanStack Start)                    │
│                                              │
│  ┌────────────┐  ┌────────────────────────┐ │
│  │ Server Fns │  │    Client Components   │ │
│  │ (session,  │  │  (React + TanStack Q.) │ │
│  │  OIDC cb)  │  │                        │ │
│  └─────┬──────┘  └───────────┬────────────┘ │
│        │                     │               │
└────────┼─────────────────────┼───────────────┘
         │                     │
    ┌────┴─────┐         ┌────┴─────┐
    │ Kanidm   │         │ Kanidm   │
    │ Auth API │         │ Admin API│
    │ (OIDC)   │         │ (REST)   │
    └──────────┘         └──────────┘
         │
    ┌────┴─────┐
    │AliasVault│
    │  API     │
    └──────────┘
```

**Dois modos de comunicação com Kanidm:**

1. **OIDC Flow (autenticação):** Console ↔ Kanidm via OAuth2/OIDC standard. O Console é um OAuth2 public client registrado no Kanidm. Usa PKCE S256.

2. **Admin API (gestão):** Console faz chamadas REST à API `/v1/` do Kanidm. O admin logado autentica via Kanidm API flow (POST `/v1/auth`) com o bearer token JWT resultante. Tokens OAuth2/OIDC do Kanidm NÃO concedem acesso à API administrativa — são fluxos separados.

**Implicação de design:** O Console mantém duas sessões:
- Sessão OIDC: identifica o usuário, fornece claims (groups, email, name)
- Sessão Kanidm API: bearer token para chamadas administrativas

---

## 2. Estrutura de Rotas e Projeto

### 2.1 Árvore de Rotas (file-based)

```
src/routes/
├── __root.tsx                          # Root layout: <html>, <body>, providers
├── index.tsx                           # "/" → redirect /dashboard ou /login
│
├── login.tsx                           # "/login"
├── callback.tsx                        # "/callback" — OIDC callback
├── logout.tsx                          # "/logout" — clear sessions
│
├── _authed.tsx                         # Layout guard — beforeLoad verifica sessão
├── _authed/
│   ├── dashboard.tsx                   # "/dashboard"
│   │
│   ├── identities.tsx                  # "/identities" — layout com Outlet
│   ├── identities/
│   │   ├── index.tsx                   # listagem
│   │   ├── create.tsx                  # wizard criação
│   │   ├── $personId.tsx               # layout detalhe
│   │   ├── $personId/
│   │   │   ├── index.tsx              # tab overview
│   │   │   ├── groups.tsx             # tab grupos
│   │   │   ├── credentials.tsx        # tab credenciais
│   │   │   ├── sessions.tsx           # tab sessões ativas
│   │   │   └── audit.tsx              # tab histórico
│   │   └── import.tsx                  # bulk CSV import
│   │
│   ├── service-accounts.tsx            # layout
│   ├── service-accounts/
│   │   ├── index.tsx
│   │   ├── create.tsx
│   │   └── $accountId.tsx
│   │
│   ├── groups.tsx                      # layout
│   ├── groups/
│   │   ├── index.tsx
│   │   ├── create.tsx
│   │   └── $groupId.tsx
│   │
│   ├── oauth2.tsx                      # layout
│   ├── oauth2/
│   │   ├── index.tsx
│   │   ├── create.tsx
│   │   └── $clientId.tsx
│   │
│   ├── vault.tsx                       # status do AliasVault
│   ├── audit.tsx                       # logs centralizados
│   │
│   └── settings.tsx                    # layout
│       ├── settings/
│       │   ├── index.tsx              # geral
│       │   ├── security.tsx           # políticas
│       │   ├── backup.tsx             # backup & restore
│       │   └── system.tsx             # info do sistema
```

### 2.2 Estrutura de Pastas Completa

```
archguard-console/
├── src/
│   ├── routes/                        # (acima)
│   │
│   ├── server/                        # Server functions
│   │   ├── auth.ts                    # OIDC + Kanidm API auth
│   │   ├── session.ts                 # Session store (cookie-based)
│   │   └── kanidm-proxy.ts            # Proxy requests com bearer token
│   │
│   ├── lib/
│   │   ├── api/
│   │   │   ├── kanidm-client.ts       # HTTP client Kanidm REST API
│   │   │   ├── vault-client.ts        # HTTP client AliasVault API
│   │   │   └── types/
│   │   │       ├── kanidm.ts          # Types da API Kanidm
│   │   │       ├── vault.ts           # Types do AliasVault
│   │   │       └── shared.ts          # Types compartilhados
│   │   ├── auth/
│   │   │   ├── oidc-config.ts         # Config oidc-client-ts
│   │   │   ├── auth-context.tsx       # React context de auth
│   │   │   ├── permissions.ts         # Checagem de permissões por grupo
│   │   │   └── guards.ts             # Route guards reutilizáveis
│   │   ├── hooks/
│   │   │   ├── use-persons.ts         # TanStack Query — persons
│   │   │   ├── use-groups.ts          # TanStack Query — groups
│   │   │   ├── use-oauth2.ts          # TanStack Query — OAuth2
│   │   │   ├── use-service-accounts.ts
│   │   │   ├── use-vault.ts
│   │   │   ├── use-audit.ts
│   │   │   ├── use-system.ts
│   │   │   └── use-permissions.ts     # Hook de permissões
│   │   ├── utils/
│   │   │   ├── formatters.ts
│   │   │   ├── validators.ts          # Zod schemas
│   │   │   └── constants.ts           # Kanidm built-in groups, scopes
│   │   └── i18n/
│   │       ├── config.ts
│   │       ├── pt-BR.json
│   │       └── en.json
│   │
│   ├── components/
│   │   ├── ui/                        # Shadcn/ui
│   │   ├── layout/
│   │   │   ├── app-shell.tsx          # Sidebar + header + main
│   │   │   ├── sidebar.tsx
│   │   │   ├── header.tsx
│   │   │   ├── breadcrumb-auto.tsx
│   │   │   └── user-menu.tsx
│   │   ├── shared/
│   │   │   ├── entity-list.tsx        # Lista genérica
│   │   │   ├── entity-detail.tsx      # Layout detalhe com tabs
│   │   │   ├── confirm-dialog.tsx
│   │   │   ├── empty-state.tsx
│   │   │   ├── loading-skeleton.tsx
│   │   │   ├── error-boundary.tsx
│   │   │   ├── status-badge.tsx
│   │   │   ├── group-badge.tsx
│   │   │   ├── copy-button.tsx
│   │   │   ├── time-ago.tsx
│   │   │   ├── permission-gate.tsx
│   │   │   └── search-combobox.tsx
│   │   ├── identity/
│   │   │   ├── person-form.tsx
│   │   │   ├── person-card.tsx
│   │   │   ├── credential-status.tsx
│   │   │   ├── credential-reset-dialog.tsx
│   │   │   ├── group-assignment.tsx
│   │   │   └── import-wizard.tsx
│   │   ├── group/
│   │   │   ├── group-form.tsx
│   │   │   ├── member-list.tsx
│   │   │   ├── group-hierarchy.tsx
│   │   │   └── group-picker.tsx
│   │   ├── oauth2/
│   │   │   ├── client-form.tsx
│   │   │   ├── scope-map-editor.tsx
│   │   │   ├── secret-display.tsx
│   │   │   ├── redirect-uri-editor.tsx
│   │   │   └── oidc-test-panel.tsx
│   │   ├── vault/
│   │   │   ├── vault-status-card.tsx
│   │   │   └── email-domain-status.tsx
│   │   ├── audit/
│   │   │   ├── audit-log-table.tsx
│   │   │   ├── audit-filters.tsx
│   │   │   └── audit-detail-sheet.tsx
│   │   └── dashboard/
│   │       ├── stats-cards.tsx
│   │       ├── recent-activity.tsx
│   │       ├── system-health.tsx
│   │       └── quick-actions.tsx
│   │
│   ├── router.tsx                     # TanStack Router config
│   ├── app.tsx                        # Root com providers
│   └── start.ts                       # TanStack Start config
│
├── Dockerfile
├── package.json
├── tsconfig.json
├── tsr.config.json                    # TanStack Router file-based config
└── vite.config.ts

```

### 2.3 Configuração do Router

```typescript
// src/router.tsx
import { createRouter } from '@tanstack/react-router'
import { QueryClient } from '@tanstack/react-query'
import { routeTree } from './routeTree.gen'

export function createAppRouter() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,        // 30s — dados admin mudam com baixa frequência
        gcTime: 5 * 60_000,       // 5min garbage collection
        retry: 1,
        refetchOnWindowFocus: true,
      },
    },
  })

  return createRouter({
    routeTree,
    defaultPreload: 'intent',     // Preload on hover/focus
    defaultPreloadStaleTime: 0,
    context: {
      auth: undefined!,           // Injetado pelo AuthProvider
      queryClient,
    },
  })
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof createAppRouter>
  }
}
```

---

## 3. Autenticação e Autorização

### 3.1 Dual Session Architecture

O Console precisa de duas sessões distintas — Kanidm separa OAuth2 e API admin:

```
┌──────────────────────────────────────────────────────────┐
│                  ARCHGUARD CONSOLE                        │
│                                                           │
│  ┌─────────────────┐    ┌──────────────────────────────┐ │
│  │  OIDC Session    │    │  Kanidm API Session           │ │
│  │  ──────────────  │    │  ──────────────────────       │ │
│  │                  │    │                               │ │
│  │  Purpose:        │    │  Purpose:                     │ │
│  │  - Identity      │    │  - Admin operations           │ │
│  │  - Groups claim  │    │  - CRUD persons/groups        │ │
│  │  - UI display    │    │  - Manage OAuth2 clients      │ │
│  │                  │    │  - Reset credentials          │ │
│  │  Flow:           │    │                               │ │
│  │  OAuth2/OIDC     │    │  Flow:                        │ │
│  │  Auth Code+PKCE  │    │  POST /v1/auth multi-step     │ │
│  │                  │    │  → Bearer JWT                 │ │
│  │  Token:          │    │                               │ │
│  │  id_token +      │    │  Token:                       │ │
│  │  access_token    │    │  Kanidm session JWT           │ │
│  │                  │    │                               │ │
│  │  Storage:        │    │  Storage:                     │ │
│  │  httpOnly cookie │    │  httpOnly cookie               │ │
│  └─────────────────┘    └──────────────────────────────┘ │
└──────────────────────────────────────────────────────────┘
```

### 3.2 Fluxo de Login Completo

```
Usuário acessa /dashboard
    │
    ▼
_authed.tsx beforeLoad()
    │
    ├── Tem sessão válida? ──── SIM → renderiza dashboard
    │
    └── NÃO → redirect /login
                │
                ▼
        Login Page — "Entrar com ArchGuard ID"
                │
                ▼
        OIDC Authorization Code + PKCE
        → Kanidm /oauth2/authorise
                │
                ▼
        Kanidm login UI (multi-step)
        [username] → [password] → [MFA/passkey]
                │
                ▼
        Callback /callback?code=xxx&state=yyy
                │
                ▼
        Server function: troca code por tokens
        → Kanidm POST /oauth2/token
                │
                ▼
        Decodifica id_token → extrai groups, name, email
                │
                ▼
        Verifica se user é admin:
        groups contém "archguard_admins" ou
                      "idm_admins" ou
                      "{tenant}_admins" ?
                │
                ├── NÃO → /unauthorized
                │
                └── SIM → Cria sessão em httpOnly cookie
                          Redirect → /dashboard
```

### 3.3 Auth Guard — `_authed.tsx`

```typescript
// src/routes/_authed.tsx
import { createFileRoute, redirect, Outlet } from '@tanstack/react-router'
import { getSessionFn } from '../server/auth'
import { AppShell } from '../components/layout/app-shell'

export const Route = createFileRoute('/_authed')({
  beforeLoad: async ({ location }) => {
    const session = await getSessionFn()

    if (!session || !session.isAuthenticated) {
      throw redirect({
        to: '/login',
        search: { redirect: location.href },
      })
    }

    if (!session.isAdmin) {
      throw redirect({ to: '/unauthorized' })
    }

    return {
      auth: {
        user: session.user,
        groups: session.groups,
        permissions: session.permissions,
        kanidmToken: session.kanidmApiToken,
      },
    }
  },
  component: AuthedLayout,
})

function AuthedLayout() {
  const { auth } = Route.useRouteContext()
  return (
    <AppShell user={auth.user} permissions={auth.permissions}>
      <Outlet />
    </AppShell>
  )
}
```

### 3.4 Server Functions de Auth

```typescript
// src/server/auth.ts
import { createServerFn } from '@tanstack/react-start'
import { getCookie, setCookie, deleteCookie } from 'vinxi/http'

interface SessionData {
  isAuthenticated: boolean
  isAdmin: boolean
  user: {
    id: string
    name: string
    email: string
    displayName: string
  }
  groups: string[]
  permissions: Permission[]
  oidcTokens: {
    accessToken: string
    idToken: string
    refreshToken?: string
    expiresAt: number
  }
  kanidmApiToken: string
}

export const getSessionFn = createServerFn({ method: 'GET' })
  .handler(async (): Promise<SessionData | null> => {
    const sessionCookie = getCookie('archguard_session')
    if (!sessionCookie) return null

    try {
      const session = decryptSession(sessionCookie)

      // Verifica expiração do token OIDC
      if (session.oidcTokens.expiresAt < Date.now()) {
        const refreshed = await refreshOIDCToken(session.oidcTokens.refreshToken)
        if (!refreshed) return null
        session.oidcTokens = refreshed
        setCookie('archguard_session', encryptSession(session), {
          httpOnly: true,
          secure: true,
          sameSite: 'lax',
          maxAge: 86400,
        })
      }

      return session
    } catch {
      return null
    }
  })

export const loginCallbackFn = createServerFn({ method: 'POST' })
  .validator((data: { code: string; state: string; codeVerifier: string }) => data)
  .handler(async ({ data }) => {
    // 1. Troca authorization code por tokens
    const tokens = await exchangeCodeForTokens(data.code, data.codeVerifier)

    // 2. Decodifica id_token
    const claims = decodeJWT(tokens.id_token)
    const groups: string[] = claims.groups || []

    // 3. Verifica se é admin
    const adminGroups = [
      'archguard_admins', 'idm_admins',
      'idm_people_admins', 'idm_oauth2_admins',
    ]
    const isAdmin = groups.some(g =>
      adminGroups.includes(g) || g.endsWith('_admins')
    )

    // 4. Deriva permissões dos grupos
    const permissions = derivePermissions(groups)

    // 5. Cria sessão
    const session: SessionData = {
      isAuthenticated: true,
      isAdmin,
      user: {
        id: claims.sub,
        name: claims.preferred_username,
        email: claims.email,
        displayName: claims.name,
      },
      groups,
      permissions,
      oidcTokens: {
        accessToken: tokens.access_token,
        idToken: tokens.id_token,
        refreshToken: tokens.refresh_token,
        expiresAt: Date.now() + tokens.expires_in * 1000,
      },
      kanidmApiToken: process.env.ARCHGUARD_SA_TOKEN || '',
    }

    setCookie('archguard_session', encryptSession(session), {
      httpOnly: true,
      secure: true,
      sameSite: 'lax',
      maxAge: 86400,
    })

    return { success: true, redirect: '/dashboard' }
  })

export const logoutFn = createServerFn({ method: 'POST' })
  .handler(async () => {
    deleteCookie('archguard_session')
    return { success: true }
  })
```

### 3.5 Kanidm API Proxy — Service Account

O Console usa um **Service Account dedicado** para chamadas admin à API do Kanidm:

```
archguard-console-sa (service account)
  → membro de idm_admins
  → API token gerado no bootstrap
  → Todas operações admin passam por este token
  → Pro: simples, UX clean (login único)
  → Con: audit trail mostra "archguard-console-sa"
  → Mitigação: log no Console qual user real fez a ação
```

```typescript
// src/server/kanidm-proxy.ts
import { createServerFn } from '@tanstack/react-start'

const KANIDM_URL = process.env.ARCHGUARD_ID_URL || 'https://localhost:8443'
const KANIDM_SA_TOKEN = process.env.ARCHGUARD_SA_TOKEN!

export const kanidmApiFn = createServerFn({ method: 'POST' })
  .validator((data: {
    method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
    path: string
    body?: unknown
  }) => data)
  .handler(async ({ data }) => {
    const response = await fetch(`${KANIDM_URL}${data.path}`, {
      method: data.method,
      headers: {
        'Authorization': `Bearer ${KANIDM_SA_TOKEN}`,
        'Content-Type': 'application/json',
      },
      body: data.body ? JSON.stringify(data.body) : undefined,
    })

    if (!response.ok) {
      const error = await response.text()
      throw new Error(`Kanidm API ${response.status}: ${error}`)
    }

    const text = await response.text()
    return text ? JSON.parse(text) : null
  })
```

---

## 4. Modelo de Dados e Contratos de API

### 4.1 Types Core — Kanidm Entities

```typescript
// src/lib/api/types/kanidm.ts

// ══════════════════════════════════════════════
// PERSON
// ══════════════════════════════════════════════

/** Raw Kanidm response — attrs são sempre string[] */
export interface KanidmEntry {
  attrs: Record<string, string[]>
}

/** Tipo normalizado para uso no Console */
export interface Person {
  id: string                                // uuid
  username: string                          // name
  displayName: string                       // displayname
  legalName?: string                        // legalname
  emails: string[]                          // mail[]
  groups: string[]                          // memberof UUIDs
  groupNames: string[]                      // nomes resolvidos
  classes: string[]                         // class
  status: PersonStatus
  sshKeys: string[]
  accountExpiry?: Date
  accountValidFrom?: Date
  credentialStatus?: CredentialStatus
}

export type PersonStatus =
  | 'active'
  | 'expired'          // account_expire no passado
  | 'not_yet_valid'    // account_valid_from no futuro
  | 'locked'           // bloqueada por falhas de auth
  | 'disabled'         // desabilitada manualmente

export interface CredentialStatus {
  hasPassword: boolean
  hasTotp: boolean
  hasWebauthn: boolean
  hasPasskeys: boolean
  hasBackupCodes: boolean
  primaryMethod: 'password_mfa' | 'passkey' | 'password_only' | 'none'
}

// Payload para criar pessoa
export interface CreatePersonPayload {
  name: string                              // username
  displayname: string
  mail?: string[]
  legalname?: string
  groups?: string[]                         // IDs dos grupos para adicionar
}

// Payload para atualizar atributos
export interface UpdatePersonPayload {
  displayname?: string
  legalname?: string
  mail?: string[]
  loginshell?: string
  ssh_publickey?: string[]
  account_expire?: string                   // ISO 8601
  account_valid_from?: string
}

// ══════════════════════════════════════════════
// GROUP
// ══════════════════════════════════════════════

export interface Group {
  id: string
  name: string
  description?: string
  members: GroupMember[]
  memberOf: string[]
  memberCount: number
  isBuiltin: boolean                        // grupo nativo do Kanidm
  isTenant: boolean                         // grupo raiz de tenant
  classes: string[]
}

export interface GroupMember {
  id: string
  name: string
  type: 'person' | 'group' | 'service_account'
  displayName?: string
}

export interface CreateGroupPayload {
  name: string
  description?: string
  members?: string[]
}

// ══════════════════════════════════════════════
// SERVICE ACCOUNT
// ══════════════════════════════════════════════

export interface ServiceAccount {
  id: string
  name: string
  displayName: string
  description?: string
  groups: string[]
  groupNames: string[]
  apiTokens: ApiTokenInfo[]
  status: 'active' | 'expired' | 'disabled'
}

export interface ApiTokenInfo {
  tokenId: string
  label: string
  createdAt: Date
  expiresAt?: Date
}

export interface CreateServiceAccountPayload {
  name: string
  displayname: string
  description?: string
  groups?: string[]
}

// ══════════════════════════════════════════════
// OAUTH2 CLIENT
// ══════════════════════════════════════════════

export interface OAuth2Client {
  id: string
  name: string                               // client ID
  displayName: string
  type: 'basic' | 'public'
  landingUrl: string
  origins: string[]
  redirectUrls: string[]
  scopeMaps: ScopeMap[]
  supplementalScopeMaps: ScopeMap[]
  claimMaps: ClaimMap[]
  hasSecret: boolean
  isPkceEnabled: boolean
  classes: string[]
}

export interface ScopeMap {
  groupId: string
  groupName: string
  scopes: string[]
}

export interface ClaimMap {
  claimName: string
  groupId: string
  groupName: string
  values: string[]
}

export interface CreateOAuth2ClientPayload {
  name: string
  displayname: string
  origin_landing: string
  type: 'basic' | 'public'
}

// ══════════════════════════════════════════════
// SYSTEM / AUDIT
// ══════════════════════════════════════════════

export interface SystemStatus {
  kanidm: {
    status: 'ok' | 'error'
    version?: string
    domain?: string
    origin?: string
  }
  vault: {
    status: 'ok' | 'error' | 'unreachable'
    version?: string
  }
}

export interface AuditEvent {
  id: string
  timestamp: Date
  eventType: AuditEventType
  actor: string
  target?: string
  details: Record<string, unknown>
  sourceIp?: string
}

export type AuditEventType =
  | 'auth_success' | 'auth_failure'
  | 'person_created' | 'person_updated' | 'person_deleted'
  | 'group_created' | 'group_updated'
  | 'group_member_added' | 'group_member_removed'
  | 'oauth2_client_created' | 'oauth2_client_updated'
  | 'credential_reset'
  | 'account_locked' | 'account_unlocked'
  | 'token_generated' | 'token_revoked'
```

### 4.2 Normalizer — Kanidm Raw → Console Types

```typescript
// src/lib/api/normalizers.ts

import type { KanidmEntry, Person, Group, OAuth2Client, ServiceAccount } from './types/kanidm'

const BUILTIN_GROUPS = new Set([
  'idm_admins', 'idm_people_admins', 'idm_oauth2_admins',
  'idm_service_desk', 'idm_people_on_boarding',
  'idm_all_accounts', 'idm_all_persons',
  'system_admins', 'idm_hp_account_manage',
])

export function normalizePerson(raw: KanidmEntry): Person {
  const a = raw.attrs
  return {
    id: a.uuid?.[0] ?? '',
    username: a.name?.[0] ?? '',
    displayName: a.displayname?.[0] ?? '',
    legalName: a.legalname?.[0],
    emails: a.mail ?? [],
    groups: a.memberof ?? [],
    groupNames: [],                          // resolvido em batch separado
    classes: a.class ?? [],
    status: derivePersonStatus(a),
    sshKeys: a.ssh_publickey ?? [],
    accountExpiry: a.account_expire?.[0] ? new Date(a.account_expire[0]) : undefined,
    accountValidFrom: a.account_valid_from?.[0] ? new Date(a.account_valid_from[0]) : undefined,
  }
}

export function normalizeGroup(raw: KanidmEntry): Group {
  const a = raw.attrs
  const name = a.name?.[0] ?? ''
  return {
    id: a.uuid?.[0] ?? '',
    name,
    description: a.description?.[0],
    members: [],                             // resolvido com chamada separada
    memberOf: a.memberof ?? [],
    memberCount: a.member?.length ?? 0,
    isBuiltin: BUILTIN_GROUPS.has(name) || (a.class ?? []).includes('system_protected'),
    isTenant: !BUILTIN_GROUPS.has(name)
              && !name.endsWith('_admins')
              && !name.endsWith('_users')
              && !name.endsWith('_vendedores')
              && !name.endsWith('_gestores'),
    classes: a.class ?? [],
  }
}

export function normalizeOAuth2Client(raw: KanidmEntry): OAuth2Client {
  const a = raw.attrs
  const classes = a.class ?? []
  return {
    id: a.uuid?.[0] ?? '',
    name: a.name?.[0] ?? '',
    displayName: a.displayname?.[0] ?? '',
    type: classes.includes('oauth2_resource_server_basic') ? 'basic' : 'public',
    landingUrl: a.oauth2_rs_origin_landing?.[0] ?? '',
    origins: a.oauth2_rs_origin ?? [],
    redirectUrls: a.oauth2_rs_origin ?? [],
    scopeMaps: parseScopeMaps(a.oauth2_rs_scope_map ?? []),
    supplementalScopeMaps: parseScopeMaps(a.oauth2_rs_sup_scope_map ?? []),
    claimMaps: parseClaimMaps(a.oauth2_rs_claim_map ?? []),
    hasSecret: classes.includes('oauth2_resource_server_basic'),
    isPkceEnabled: true, // Kanidm: PKCE é obrigatório por padrão
    classes,
  }
}

function derivePersonStatus(attrs: Record<string, string[]>): PersonStatus {
  const now = new Date()
  if (attrs.account_expire?.[0] && new Date(attrs.account_expire[0]) < now) return 'expired'
  if (attrs.account_valid_from?.[0] && new Date(attrs.account_valid_from[0]) > now) return 'not_yet_valid'
  // locked status comes from credential status check
  return 'active'
}
```

### 4.3 Kanidm REST API — Client Completo

```typescript
// src/lib/api/kanidm-client.ts

import { kanidmApiFn } from '../../server/kanidm-proxy'
import { normalizePerson, normalizeGroup, normalizeOAuth2Client } from './normalizers'
import type * as T from './types/kanidm'

// Helper para chamadas
async function api(method: string, path: string, body?: unknown) {
  return kanidmApiFn({ data: { method: method as any, path, body } })
}

// ── PERSONS ─────────────────────────────────────

export const personApi = {
  list: async (): Promise<T.Person[]> => {
    const raw = await api('GET', '/v1/person')
    return (raw as any[]).map(normalizePerson)
  },

  get: async (id: string): Promise<T.Person> => {
    const raw = await api('GET', `/v1/person/${id}`)
    return normalizePerson(raw)
  },

  create: (payload: T.CreatePersonPayload) =>
    api('POST', '/v1/person', {
      attrs: {
        name: [payload.name],
        displayname: [payload.displayname],
        ...(payload.legalname && { legalname: [payload.legalname] }),
      },
    }),

  delete: (id: string) =>
    api('DELETE', `/v1/person/${id}`),

  setAttr: (id: string, attr: string, values: string[]) =>
    api('PUT', `/v1/person/${id}/_attr/${attr}`, values),

  appendAttr: (id: string, attr: string, values: string[]) =>
    api('POST', `/v1/person/${id}/_attr/${attr}`, values),

  deleteAttr: (id: string, attr: string) =>
    api('DELETE', `/v1/person/${id}/_attr/${attr}`),

  credentialStatus: (id: string) =>
    api('GET', `/v1/person/${id}/_credential/_status`),

  createResetToken: (id: string, ttl = 3600) =>
    api('GET', `/v1/person/${id}/_credential/_update_intent/${ttl}`),
}

// ── GROUPS ───────────────────────────────────────

export const groupApi = {
  list: async (): Promise<T.Group[]> => {
    const raw = await api('GET', '/v1/group')
    return (raw as any[]).map(normalizeGroup)
  },

  get: async (id: string): Promise<T.Group> => {
    const raw = await api('GET', `/v1/group/${id}`)
    return normalizeGroup(raw)
  },

  create: (payload: T.CreateGroupPayload) =>
    api('POST', '/v1/group', {
      attrs: {
        name: [payload.name],
        ...(payload.description && { description: [payload.description] }),
      },
    }),

  delete: (id: string) =>
    api('DELETE', `/v1/group/${id}`),

  getMembers: (id: string) =>
    api('GET', `/v1/group/${id}/_attr/member`),

  addMembers: (id: string, memberIds: string[]) =>
    api('POST', `/v1/group/${id}/_attr/member`, memberIds),

  removeMembers: (id: string, memberIds: string[]) =>
    api('DELETE', `/v1/group/${id}/_attr/member`, memberIds),
}

// ── OAUTH2 ──────────────────────────────────────

export const oauth2Api = {
  list: async (): Promise<T.OAuth2Client[]> => {
    const raw = await api('GET', '/v1/oauth2')
    return (raw as any[]).map(normalizeOAuth2Client)
  },

  get: async (id: string): Promise<T.OAuth2Client> => {
    const raw = await api('GET', `/v1/oauth2/${id}`)
    return normalizeOAuth2Client(raw)
  },

  createBasic: (payload: T.CreateOAuth2ClientPayload) =>
    api('POST', '/v1/oauth2/_basic', {
      attrs: {
        name: [payload.name],
        displayname: [payload.displayname],
        oauth2_rs_origin_landing: [payload.origin_landing],
      },
    }),

  createPublic: (payload: T.CreateOAuth2ClientPayload) =>
    api('POST', '/v1/oauth2/_public', {
      attrs: {
        name: [payload.name],
        displayname: [payload.displayname],
        oauth2_rs_origin_landing: [payload.origin_landing],
      },
    }),

  delete: (id: string) =>
    api('DELETE', `/v1/oauth2/${id}`),

  getSecret: (id: string) =>
    api('GET', `/v1/oauth2/${id}/_basic_secret`),

  setScopeMap: (id: string, groupId: string, scopes: string[]) =>
    api('POST', `/v1/oauth2/${id}/_scopemap/${groupId}`, scopes),

  deleteScopeMap: (id: string, groupId: string) =>
    api('DELETE', `/v1/oauth2/${id}/_scopemap/${groupId}`),

  setSupScopeMap: (id: string, groupId: string, scopes: string[]) =>
    api('POST', `/v1/oauth2/${id}/_sup_scopemap/${groupId}`, scopes),

  setClaimMap: (id: string, claimName: string, groupId: string, values: string[]) =>
    api('POST', `/v1/oauth2/${id}/_claimmap/${claimName}/${groupId}`, values),

  addRedirectUrl: (id: string, url: string) =>
    api('POST', `/v1/oauth2/${id}/_attr/oauth2_rs_origin`, [url]),

  enableLocalhostRedirects: (id: string) =>
    api('POST', `/v1/oauth2/${id}/_attr/oauth2_allow_localhost_redirect`, ['true']),

  preferShortUsername: (id: string) =>
    api('POST', `/v1/oauth2/${id}/_attr/oauth2_prefer_short_username`, ['true']),
}

// ── SERVICE ACCOUNTS ────────────────────────────

export const serviceAccountApi = {
  list: () => api('GET', '/v1/service_account'),
  get: (id: string) => api('GET', `/v1/service_account/${id}`),

  create: (payload: T.CreateServiceAccountPayload) =>
    api('POST', '/v1/service_account', {
      attrs: {
        name: [payload.name],
        displayname: [payload.displayname],
        ...(payload.description && { description: [payload.description] }),
      },
    }),

  delete: (id: string) =>
    api('DELETE', `/v1/service_account/${id}`),

  generateToken: (id: string, label: string, expiry?: string) =>
    api('POST', `/v1/service_account/${id}/_api_token`, { label, expiry }),

  revokeToken: (id: string, tokenId: string) =>
    api('DELETE', `/v1/service_account/${id}/_api_token/${tokenId}`),
}

// ── SYSTEM ──────────────────────────────────────

export const systemApi = {
  status: () => api('GET', '/status'),
  domain: () => api('GET', '/v1/domain'),
}
```

### 4.4 Zod Schemas — Validação de Forms

```typescript
// src/lib/utils/validators.ts
import { z } from 'zod'

// ── Person ──────────────────────────────────
export const createPersonSchema = z.object({
  name: z.string()
    .min(2, 'Mínimo 2 caracteres')
    .max(64, 'Máximo 64 caracteres')
    .regex(/^[a-z][a-z0-9._-]+$/, 'Apenas letras minúsculas, números, . _ -'),
  displayname: z.string().min(1, 'Obrigatório').max(128),
  legalname: z.string().max(128).optional(),
  mail: z.array(z.string().email('Email inválido')).min(1, 'Pelo menos um email'),
  groups: z.array(z.string()).optional(),
})
export type CreatePersonInput = z.infer<typeof createPersonSchema>

// ── Group ───────────────────────────────────
export const createGroupSchema = z.object({
  name: z.string()
    .min(2).max(64)
    .regex(/^[a-z][a-z0-9_-]+$/, 'Apenas letras minúsculas, números, _ -'),
  description: z.string().max(256).optional(),
  members: z.array(z.string()).optional(),
})
export type CreateGroupInput = z.infer<typeof createGroupSchema>

// ── OAuth2 Client ───────────────────────────
export const createOAuth2Schema = z.object({
  name: z.string()
    .min(2).max(64)
    .regex(/^[a-z][a-z0-9-]+$/, 'Apenas letras minúsculas, números e hífens'),
  displayname: z.string().min(1).max(128),
  origin_landing: z.string().url('URL inválida'),
  type: z.enum(['basic', 'public']),
  redirect_urls: z.array(z.string().url()).min(1, 'Pelo menos uma redirect URL'),
})
export type CreateOAuth2Input = z.infer<typeof createOAuth2Schema>

// ── Service Account ─────────────────────────
export const createServiceAccountSchema = z.object({
  name: z.string().min(2).max(64).regex(/^[a-z][a-z0-9_-]+$/),
  displayname: z.string().min(1).max(128),
  description: z.string().max(256).optional(),
  groups: z.array(z.string()).optional(),
})
export type CreateServiceAccountInput = z.infer<typeof createServiceAccountSchema>

// ── Search Params (URL state) ───────────────
export const searchParamsSchema = z.object({
  q: z.string().optional(),
  page: z.number().int().min(1).default(1),
  pageSize: z.number().int().min(10).max(100).default(25),
  sortBy: z.string().optional(),
  sortDir: z.enum(['asc', 'desc']).default('asc'),
  status: z.enum(['active', 'expired', 'locked', 'all']).default('all'),
  group: z.string().optional(),
})
export type SearchParams = z.infer<typeof searchParamsSchema>

// ── Audit Filters ───────────────────────────
export const auditFiltersSchema = z.object({
  period: z.enum(['1h', '24h', '7d', '30d', 'custom']).default('24h'),
  dateFrom: z.string().datetime().optional(),
  dateTo: z.string().datetime().optional(),
  eventType: z.string().optional(),
  actor: z.string().optional(),
  target: z.string().optional(),
  status: z.array(z.enum(['success', 'failure', 'alert'])).default(['success', 'failure', 'alert']),
})
export type AuditFilters = z.infer<typeof auditFiltersSchema>
```

---

## 5. Módulo: Dashboard

### 5.1 Wireframe

```
┌──────────────────────────────────────────────────────────────────┐
│  ArchGuard Console           🔍 Busca global...    👤 Edson ▾   │
├──────────────┬───────────────────────────────────────────────────┤
│              │                                                   │
│  📊 Dashboard│  Dashboard                                        │
│  👤 Identid. │                                                   │
│  🔧 Service  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────┐│
│  👥 Grupos   │  │  👤 42   │ │  👥 8    │ │  🔐 5    │ │ 🗝️ 3 ││
│  🔐 OAuth2   │  │ Persons  │ │ Grupos   │ │ OAuth2   │ │ Svc. ││
│  🗝️ Vault    │  │ +3 7 dias│ │ +1 7 dias│ │ clients  │ │ accts││
│  📋 Auditoria│  └──────────┘ └──────────┘ └──────────┘ └──────┘│
│  ⚙️ Config.  │                                                   │
│              │  ┌──────────────────────┐ ┌──────────────────────┐│
│              │  │ Atividade Recente    │ │ Saúde do Sistema     ││
│              │  │                      │ │                      ││
│              │  │ 🟢 14:32 João Silva  │ │ ArchGuard ID   🟢 OK││
│              │  │   Login VendaX       │ │ v1.4.1 · 99.9% up   ││
│              │  │                      │ │                      ││
│              │  │ 🟡 14:28 Maria Souza │ │ ArchGuard Vault 🟢 OK││
│              │  │   Reset credencial   │ │ v0.26 · 5 vaults    ││
│              │  │                      │ │                      ││
│              │  │ 🔴 14:15 admin       │ │ Gateway       🟢 OK ││
│              │  │   Auth falhou (3x)   │ │ nginx 1.27           ││
│              │  │                      │ │                      ││
│              │  │ 🟢 13:50 Edson       │ │ Último backup:       ││
│              │  │   Criou grupo        │ │ hoje 06:00 🟢        ││
│              │  └──────────────────────┘ └──────────────────────┘│
│              │                                                   │
│              │  Ações Rápidas                                    │
│              │  [+ Nova Pessoa] [+ Novo Grupo] [+ Novo Client]  │
│              │  [Gerar Reset Link] [Verificar Sistema]           │
│              │                                                   │
└──────────────┴───────────────────────────────────────────────────┘
```

### 5.2 Rota com Prefetch Paralelo

```typescript
// src/routes/_authed/dashboard.tsx
export const Route = createFileRoute('/_authed/dashboard')({
  loader: ({ context }) => {
    // Prefetch paralelo — não bloqueia render
    context.queryClient.ensureQueryData(dashboardQueries.stats())
    context.queryClient.ensureQueryData(dashboardQueries.recentActivity())
    context.queryClient.ensureQueryData(dashboardQueries.systemHealth())
  },
  component: DashboardPage,
})
```

### 5.3 Dados de cada Card

| Card | Fonte | Query |
|---|---|---|
| Persons count | `GET /v1/person` → `.length` | `personApi.list()` |
| Groups count | `GET /v1/group` → `.length` | `groupApi.list()` |
| OAuth2 clients | `GET /v1/oauth2` → `.length` | `oauth2Api.list()` |
| Service accounts | `GET /v1/service_account` → `.length` | `serviceAccountApi.list()` |
| System Health | `GET /status` + vault health | `systemApi.status()` |
| Recent Activity | Audit log (se disponível) ou polling | `auditApi.recent()` |

---

## 6. Módulo: Identidades (Persons)

### 6.1 Listagem — `/identities`

```
┌──────────────────────────────────────────────────────────────────┐
│  Identidades                                    [+ Nova Pessoa]  │
│                                                  [↑ Importar CSV]│
│                                                                   │
│  🔍 Buscar por nome, email...     Status: [Todos ▾]  Grupo: [▾] │
│                                                                   │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │ □  Nome              Email           Grupos  MFA    Status│  │
│  ├────────────────────────────────────────────────────────────┤  │
│  │ □  João Silva        joao@rio...     🏷️ 3    🔑     🟢    │  │
│  │ □  Maria Souza       maria@rio...    🏷️ 2    🔐     🟢    │  │
│  │ □  Carlos Pereira    carlos@...      🏷️ 1    ⚠️      🟡    │  │
│  │ □  Ana Costa         ana@outro...    🏷️ 4    🔑     🟢    │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                                   │
│  Mostrando 1-25 de 42          [← Anterior] [Próxima →]         │
│  Selecionados: 0  [Ações em lote ▾]                              │
│                                                                   │
│  MFA: 🔑 Passkey+TOTP  🔐 Password+TOTP  ⚠️ Só password         │
└──────────────────────────────────────────────────────────────────┘
```

**Rota com search params type-safe:**

```typescript
// src/routes/_authed/identities/index.tsx
export const Route = createFileRoute('/_authed/identities/')({
  validateSearch: searchParamsSchema,
  loaderDeps: ({ search }) => search,
  loader: async ({ context, deps }) => {
    await context.queryClient.ensureQueryData(personQueries.list(deps))
  },
  component: IdentitiesListPage,
})
```

**Ações em lote disponíveis:**
- Adicionar a grupo (selecionar grupo)
- Remover de grupo
- Gerar links de reset
- Bloquear contas
- Exportar selecionados (CSV)

### 6.2 Criação — `/identities/create` — Wizard 3 passos

```
PASSO 1: Dados básicos
─────────────────────

  Username *           ┌──────────────────────────────┐
                       │ joao.silva                    │
                       └──────────────────────────────┘
                       Apenas minúsculas, números, . _ -

  Nome de exibição *   ┌──────────────────────────────┐
                       │ João da Silva                 │
                       └──────────────────────────────┘

  Nome legal           ┌──────────────────────────────┐
                       │ João Carlos da Silva          │
                       └──────────────────────────────┘

  Emails *             ┌──────────────────────────────┐
                       │ joao@rioquality.com.br        │ [+ Adicionar]
                       └──────────────────────────────┘

                                   [Cancelar]  [Próximo →]
```

```
PASSO 2: Grupos
──────────────

  Grupos disponíveis              Grupos selecionados
  ┌─────────────────────┐        ┌─────────────────────┐
  │ 🔍 Buscar...        │        │                     │
  │                     │        │  ☑ rio_quality      │
  │  ☐ archguard_admins │   →    │  ☑ rio_quality_     │
  │  ☐ integralltech_   │   ←    │    vendedores       │
  │  ☐ outro_cliente_   │        │                     │
  │  ☐ vendax_users     │        │                     │
  └─────────────────────┘        └─────────────────────┘

                              [← Anterior]  [Próximo →]
```

```
PASSO 3: Revisão
───────────────

  ┌──────────────────────────────────────────────┐
  │  Confirmar criação                            │
  │                                               │
  │  Username:   joao.silva                       │
  │  Nome:       João da Silva                    │
  │  Legal:      João Carlos da Silva             │
  │  Email:      joao@rioquality.com.br           │
  │  Grupos:     rio_quality, rio_quality_vendas  │
  │                                               │
  │  ☑ Enviar link de configuração por email      │
  │  ☐ Gerar senha temporária                     │
  │                                               │
  └──────────────────────────────────────────────┘

                              [← Anterior]  [✓ Criar Pessoa]
```

**Fluxo de API ao clicar "Criar":**

```
1. POST /v1/person
   body: { attrs: { name: ["joao.silva"], displayname: ["João da Silva"] } }

2. PUT /v1/person/joao.silva/_attr/mail
   body: ["joao@rioquality.com.br"]

3. PUT /v1/person/joao.silva/_attr/legalname
   body: ["João Carlos da Silva"]

4. POST /v1/group/rio_quality/_attr/member
   body: ["<uuid-joao>"]

5. POST /v1/group/rio_quality_vendedores/_attr/member
   body: ["<uuid-joao>"]

6. GET /v1/person/joao.silva/_credential/_update_intent/3600
   → { token: "abc123..." }
   → Exibe link: https://auth.domain/ui/reset?token=abc123
```

### 6.3 Detalhe — `/identities/:personId`

```
┌──────────────────────────────────────────────────────────────────┐
│  ← Identidades                                                    │
│                                                                   │
│  ┌────┐  João da Silva                                           │
│  │ JS │  joao.silva · joao@rioquality.com.br                    │
│  └────┘  🟢 Ativo · Último login: há 2 horas                    │
│                                                                   │
│  [Visão Geral] [Grupos] [Credenciais] [Sessões] [Histórico]     │
│  ━━━━━━━━━━━━                                                     │
│                                                                   │
│  TAB: Visão Geral                                                │
│                                                                   │
│  Dados Pessoais                                [Editar]          │
│  ┌────────────────────────────────────────────────────────┐      │
│  │  Username       joao.silva                              │      │
│  │  Nome exibição  João da Silva                           │      │
│  │  Nome legal     João Carlos da Silva                    │      │
│  │  Emails         joao@rioquality.com.br                  │      │
│  │  UUID           a1b2c3d4-...                    [📋]   │      │
│  └────────────────────────────────────────────────────────┘      │
│                                                                   │
│  Credenciais Resumo                                              │
│  ┌────────────────────────────────────────────────────────┐      │
│  │  🔑 Passkey     Configurada (YubiKey 5)                 │      │
│  │  🔐 Password    Definida · Alterada há 15 dias          │      │
│  │  📱 TOTP        Ativo                                   │      │
│  │  🔢 Backup      5 códigos restantes                     │      │
│  └────────────────────────────────────────────────────────┘      │
│                                                                   │
│  Ações                                                            │
│  [Gerar Link Reset] [Bloquear Conta] [Remover Pessoa ⚠️]        │
└──────────────────────────────────────────────────────────────────┘
```

**Tab: Credenciais (detalhada)**

```
┌────────────────────────────────────────────────────────────┐
│  Método          Status      Detalhes           Ação       │
├────────────────────────────────────────────────────────────┤
│  🔑 Passkeys    1 ativa     YubiKey 5 NFC      [Detalhes] │
│  🔐 Password    Definida    Alterada há 15d    [Reset]    │
│  📱 TOTP        Ativo       1 dispositivo      [Detalhes] │
│  🔢 Backup      5 códigos   —                  [Regenerar]│
│  🔓 SSH Keys    2 chaves    ed25519, rsa       [Gerenciar]│
└────────────────────────────────────────────────────────────┘

  Gerar Link de Reset
  Validade:  [1 hora ▾]    [Gerar Link]

  ┌──────────────────────────────────────────────────┐
  │  🔗 Link gerado:                                 │
  │  https://auth.domain/ui/reset?token=eyJ...       │
  │                                      [📋 Copiar] │
  │  Expira em: 1 hora                               │
  │  ⚠️ Este link dá acesso total às credenciais     │
  └──────────────────────────────────────────────────┘
```

**Tab: Grupos**

```
  Membro de:
  ┌────────────────────────────────────────────────────────┐
  │  🏷️ rio_quality                              [Remover] │
  │  🏷️ rio_quality_vendedores                   [Remover] │
  │  🏷️ vendax_users                             [Remover] │
  └────────────────────────────────────────────────────────┘

  [+ Adicionar a grupo]

  ┌──────────────────────────────────────────────────┐
  │  🔍 Buscar grupo...                              │
  │  rio_quality_gestores              [+ Adicionar] │
  │  archguard_admins                  [+ Adicionar] │
  └──────────────────────────────────────────────────┘
```

### 6.4 Import CSV — `/identities/import`

```
PASSO 1: Upload
  Arraste um arquivo CSV ou [Selecionar arquivo]
  [Baixar template CSV]

PASSO 2: Mapeamento
  Coluna CSV        →  Campo ArchGuard
  username          →  name (username)    ✅ Obrigatório
  display_name      →  displayname        ✅ Obrigatório
  email             →  mail               ✅ Obrigatório
  groups            →  groups             ☐ Opcional

PASSO 3: Validação
  ✅ 28 válidas · ⚠️ 3 avisos · ❌ 1 erro
  [Ver detalhes ▾]

PASSO 4: Progresso
  ████████████████░░░░░░  54%  (15/28)
  ✅ joao.silva — criado + 2 grupos
  ⏳ carlos.pereira — processando...
```

---

## 7. Módulo: Service Accounts

### 7.1 Listagem — `/service-accounts`

```
┌──────────────────────────────────────────────────────────────────┐
│  Service Accounts                            [+ Nova Account]    │
│                                                                   │
│  🔍 Buscar...                                                    │
│                                                                   │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │  Nome               Descrição           Tokens   Status   │  │
│  ├────────────────────────────────────────────────────────────┤  │
│  │  vendax-backend     VendaX API access   2        🟢       │  │
│  │  archguard-console  Console admin       1        🟢       │  │
│  │  mentors-sync       Mentors iPaaS       1        🟡 exp.  │  │
│  └────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

### 7.2 Detalhe com Token Management

```
┌──────────────────────────────────────────────────────────────────┐
│  ← Service Accounts                                              │
│                                                                   │
│  vendax-backend · Service Account · 🟢 Ativo                    │
│                                                                   │
│  API Tokens                                                       │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │  Label           Criado       Expira       Ação            │  │
│  ├────────────────────────────────────────────────────────────┤  │
│  │  production-v2   2 sem atrás  em 88 dias   [Revogar]      │  │
│  │  staging         1 mês atrás  em 58 dias   [Revogar]      │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                                   │
│  [+ Gerar Novo Token]                                            │
│                                                                   │
│  ┌──────────────────────────────────────────────────┐            │
│  │  ⚠️ Este token só será exibido UMA VEZ:          │            │
│  │                                                   │            │
│  │  eyJhbGciOiJFUzI1NiIs...                         │            │
│  │                                      [📋 Copiar]  │            │
│  │                                                   │            │
│  │  Label: production-v3 · Expira: 90 dias          │            │
│  └──────────────────────────────────────────────────┘            │
│                                                                   │
│  Grupos: vendax_service, archguard_users                         │
│  Ações: [Remover Account ⚠️]                                     │
└──────────────────────────────────────────────────────────────────┘
```

---

## 8. Módulo: Grupos

### 8.1 Listagem — `/groups`

```
┌──────────────────────────────────────────────────────────────────┐
│  Grupos                                         [+ Novo Grupo]   │
│                                                                   │
│  [☰ Lista] [🌳 Árvore]     🔍 Buscar...      [☐ Ocultar builtin]│
│                                                                   │
│  LISTA:                                                          │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │  Nome                    Membros  Tipo       Descrição     │  │
│  ├────────────────────────────────────────────────────────────┤  │
│  │  rio_quality             15       🏢 Tenant   Grupo raiz   │  │
│  │  rio_quality_admins      2        👑 Admin    TI Rio...    │  │
│  │  rio_quality_vendedores  10       👤 Users    Equipe...    │  │
│  │  rio_quality_gestores    3        👤 Users    Gerentes     │  │
│  │  ─── builtin ────────────────────────────────────────────  │  │
│  │  idm_admins              1        🔒 System   Kanidm admin │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                                   │
│  ÁRVORE:                                                         │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │  archguard_admins (2)                                      │  │
│  │  ├── rio_quality (15)                                      │  │
│  │  │   ├── rio_quality_admins (2)                            │  │
│  │  │   ├── rio_quality_vendedores (10)                       │  │
│  │  │   └── rio_quality_gestores (3)                          │  │
│  │  ├── outro_cliente (8)                                     │  │
│  │  │   ├── outro_cliente_admins (1)                          │  │
│  │  │   └── outro_cliente_users (7)                           │  │
│  │  └── integralltech_devs (4)                                │  │
│  └────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

### 8.2 Detalhe — `/groups/:groupId`

```
┌──────────────────────────────────────────────────────────────────┐
│  ← Grupos                                                        │
│                                                                   │
│  rio_quality_vendedores                                          │
│  Equipe de vendas da Rio Quality · 10 membros                    │
│                                                                   │
│  [Detalhes] [Membros] [OAuth2 Scopes]                            │
│  ━━━━━━━━━━━━━━━━━━━━                                             │
│                                                                   │
│  TAB: Membros                                                    │
│                                                                   │
│  🔍 Buscar membros...                        [+ Adicionar]      │
│                                                                   │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │  👤 João Silva       joao.silva       [Remover]           │  │
│  │  👤 Maria Souza      maria.souza      [Remover]           │  │
│  │  👤 Carlos Pereira   carlos.pereira   [Remover]           │  │
│  │  ... (+7 mais)                                             │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                                   │
│  Adicionar membro:                                               │
│  ┌──────────────────────────────────────────────────┐            │
│  │  🔍 Buscar pessoa ou grupo...                    │            │
│  │  👤 Pedro Oliveira  pedro.oliveira  [+ Adicionar]│            │
│  │  👥 rio_quality_gestores (grupo)    [+ Adicionar]│            │
│  └──────────────────────────────────────────────────┘            │
│                                                                   │
│  TAB: OAuth2 Scopes                                              │
│  ┌────────────────────────────────────────────────────────┐      │
│  │  Client              →  Scopes atribuídos a este grupo │      │
│  ├────────────────────────────────────────────────────────┤      │
│  │  vendax-rioquality   →  openid, email, profile, groups │      │
│  │  powerbi-embedded    →  openid, email                  │      │
│  └────────────────────────────────────────────────────────┘      │
│                                                                   │
│  Membro de (grupos pai):  🏷️ rio_quality                         │
│  Ações: [Remover Grupo ⚠️]                                       │
└──────────────────────────────────────────────────────────────────┘
```

---

## 9. Módulo: OAuth2 / SSO

### 9.1 Listagem — `/oauth2`

```
┌──────────────────────────────────────────────────────────────────┐
│  OAuth2 / SSO Clients                        [+ Novo Client]    │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │  🔐 vendax-rioquality                                    │    │
│  │  VendaX - Rio Quality · Basic client                     │    │
│  │  https://vendax.integralltech.com.br                      │    │
│  │  Scopes: rio_quality → openid email profile groups       │    │
│  │  Última auth: há 5 min                                    │    │
│  ├──────────────────────────────────────────────────────────┤    │
│  │  🔓 archguard-console                                    │    │
│  │  ArchGuard Console · Public client (PKCE)                │    │
│  │  https://console.archguard.local                          │    │
│  │  Scopes: archguard_admins → openid email profile groups  │    │
│  ├──────────────────────────────────────────────────────────┤    │
│  │  🔓 vendax-mobile                                        │    │
│  │  VendaX Mobile App · Public client (PKCE)                │    │
│  │  com.integralltech.vendax://                              │    │
│  │  Localhost redirects: enabled                             │    │
│  └──────────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────────┘
```

### 9.2 Criação — Wizard

```
PASSO 1: Tipo de Client
────────────────────────

  ┌───────────────────────────┐  ┌───────────────────────────┐
  │  🔐 Basic (Confidential)  │  │  🔓 Public (PKCE-only)    │
  │                            │  │                           │
  │  Para backends e serviços  │  │  Para SPAs e apps mobile  │
  │  que guardam secret de     │  │  que não guardam secrets  │
  │  forma segura.             │  │  de forma segura.         │
  │                            │  │                           │
  │  • Spring Boot backends    │  │  • React/Vue/Angular SPAs │
  │  • Node.js servers         │  │  • Flutter/React Native   │
  │  • Python APIs             │  │  • Desktop apps           │
  │                            │  │                           │
  │  [Selecionar]              │  │  [Selecionar]             │
  └───────────────────────────┘  └───────────────────────────┘
```

```
PASSO 2: Configuração
─────────────────────

  Client ID *         ┌────────────────────────────────┐
                      │ vendax-mobile                   │
                      └────────────────────────────────┘

  Nome de exibição *  ┌────────────────────────────────┐
                      │ VendaX Mobile App               │
                      └────────────────────────────────┘

  Landing Page URL *  ┌────────────────────────────────┐
                      │ https://vendax.integralltech... │
                      └────────────────────────────────┘

  Redirect URLs *
  ┌──────────────────────────────────────────────┐
  │ com.integralltech.vendax://callback      [✕] │
  │ http://localhost:8080/callback           [✕] │
  └──────────────────────────────────────────────┘
  [+ Adicionar URL]

  ☑ Permitir redirects para localhost (dev)

  Scope Mapping
  ┌────────────────────────────────────────────────┐
  │  Grupo               →  Scopes                 │
  │  ┌────────────────┐     ☑ openid               │
  │  │ rio_quality  ▾ │     ☑ email                │
  │  └────────────────┘     ☑ profile              │
  │                          ☑ groups               │
  │                          ☐ ssh_publickeys       │
  │  [+ Adicionar scope map]                       │
  └────────────────────────────────────────────────┘

                                    [Criar Client]
```

### 9.3 Detalhe — `/oauth2/:clientId`

```
┌──────────────────────────────────────────────────────────────────┐
│  ← OAuth2 Clients                                                │
│                                                                   │
│  vendax-rioquality                                               │
│  VendaX - Rio Quality · 🔐 Basic client                         │
│                                                                   │
│  [Configuração] [Scopes] [Integração]                            │
│  ━━━━━━━━━━━━━━━━━━━━━                                            │
│                                                                   │
│  TAB: Configuração                                               │
│                                                                   │
│  Detalhes                                          [Editar]      │
│  ┌────────────────────────────────────────────────────────┐      │
│  │  Client ID      vendax-rioquality                      │      │
│  │  Tipo            Basic (confidential)                   │      │
│  │  Landing URL     https://vendax.integralltech.com.br    │      │
│  │  PKCE            S256 (obrigatório)                     │      │
│  │  Token signing   ES256                                  │      │
│  └────────────────────────────────────────────────────────┘      │
│                                                                   │
│  Client Secret                                                    │
│  ┌────────────────────────────────────────────────────────┐      │
│  │  ●●●●●●●●●●●●●●●●●●●●       [👁️ Mostrar] [📋 Copiar] │      │
│  │                                [🔄 Rotacionar Secret]  │      │
│  └────────────────────────────────────────────────────────┘      │
│  ⚠️ Rotacionar invalidará todos tokens ativos                    │
│                                                                   │
│  Redirect URLs                                                    │
│  ┌────────────────────────────────────────────────────────┐      │
│  │  https://vendax.integralltech.com.br/oauth2/cb    [✕] │      │
│  │  [+ Adicionar URL]                                     │      │
│  └────────────────────────────────────────────────────────┘      │
│                                                                   │
│  TAB: Integração (snippets para copiar)                          │
│                                                                   │
│  Discovery URL:                                    [📋 Copiar]   │
│  https://auth.domain/oauth2/openid/vendax-rioquality/             │
│  .well-known/openid-configuration                                │
│                                                                   │
│  Spring Boot application.yml:                 [📋 Copiar bloco]  │
│  ┌────────────────────────────────────────────────────┐          │
│  │  spring:                                           │          │
│  │    security:                                       │          │
│  │      oauth2:                                       │          │
│  │        client:                                     │          │
│  │          registration:                             │          │
│  │            archguard:                              │          │
│  │              client-id: vendax-rioquality           │          │
│  │              client-secret: ${SECRET}               │          │
│  │              scope: openid,profile,email,groups     │          │
│  │        provider:                                   │          │
│  │          archguard:                                │          │
│  │            issuer-uri: https://auth.domain/...     │          │
│  └────────────────────────────────────────────────────┘          │
│                                                                   │
│  React / oidc-client-ts:                       [📋 Copiar bloco] │
│  Flutter / flutter_appauth:                    [📋 Copiar bloco] │
│                                                                   │
│  [🧪 Testar Fluxo OIDC]  [Remover Client ⚠️]                    │
└──────────────────────────────────────────────────────────────────┘
```

---

## 10. Módulo: Vault

### 10.1 Tela — `/vault`

```
┌──────────────────────────────────────────────────────────────────┐
│  ArchGuard Vault — Powered by AliasVault                         │
│                                                                   │
│  Status do Serviço                                               │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │  🟢 Online · v0.26.0                                     │    │
│  │  URL: https://vault.archguard.local                       │    │
│  │  Uptime: 14 dias                                          │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                   │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐           │
│  │ 5 Vaults │ │ 47 Senhas│ │ 12 Alias │ │ 3 Domín. │           │
│  │ ativos   │ │ armazen. │ │ email    │ │ email    │           │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘           │
│                                                                   │
│  Email Server (SMTP)                                             │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │  🟢 archguard.local      MX ativo · 12 aliases          │    │
│  │  🟡 integrall.vault      DNS pendente                    │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                   │
│  [Abrir Vault Admin ↗️] [Configurar Domínio] [Docs AliasVault ↗️]│
│                                                                   │
│  ℹ️ O Vault usa criptografia zero-knowledge.                     │
│  O servidor nunca acessa dados dos cofres.                       │
│  Gestão de dados é feita pelo usuário no Vault UI.               │
└──────────────────────────────────────────────────────────────────┘
```

---

## 11. Módulo: Auditoria

### 11.1 Tela — `/audit`

```
┌──────────────────────────────────────────────────────────────────┐
│  Auditoria                                          [⬇️ Exportar]│
│                                                                   │
│  Período: [Últimas 24h ▾]  Tipo: [Todos ▾]  Ator: [Todos ▾]    │
│  Status: ☑ Sucesso ☑ Falha ☑ Alerta  Alvo: [🔍 Buscar...]      │
│                                                                   │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │ Hora      Tipo            Ator        Alvo       ⓘ       │  │
│  ├────────────────────────────────────────────────────────────┤  │
│  │ 14:32:05  🟢 auth_ok      joao.silva  vendax     [→]     │  │
│  │ 14:28:11  🟡 cred_reset   admin       maria      [→]     │  │
│  │ 14:15:02  🔴 auth_fail    ?           admin      [→]     │  │
│  │ 14:15:01  🔴 auth_fail    ?           admin      [→]     │  │
│  │ 14:14:59  🔴 auth_fail    ?           admin      [→]     │  │
│  │ 13:50:44  🟢 grp_create   edson       rio_q..    [→]     │  │
│  │ 13:48:12  🟢 person_add   edson       joao       [→]     │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                                   │
│  ⚠️ Alerta: 3 tentativas falhas em "admin" de 192.168.1.5       │
│  [Bloquear IP] [Ignorar]                                         │
│                                                                   │
│  Ao clicar [→], abre sheet lateral:                              │
│  ┌──────────────────────────────────────────┐                    │
│  │  Evento: auth_failure                     │                    │
│  │  Timestamp: 2026-02-16T14:15:02Z         │                    │
│  │  Ator: desconhecido                       │                    │
│  │  Alvo: admin                              │                    │
│  │  IP: 192.168.1.5                          │                    │
│  │  User-Agent: curl/8.1.2                   │                    │
│  │  Motivo: invalid_credential               │                    │
│  └──────────────────────────────────────────┘                    │
└──────────────────────────────────────────────────────────────────┘
```

---

## 12. Módulo: Configurações

### 12.1 Sub-rotas

| Rota | Conteúdo |
|---|---|
| `/settings` | Domínio, branding, informações gerais |
| `/settings/security` | Políticas de MFA, sessão, lockout |
| `/settings/backup` | Backup & restore, agendamento |
| `/settings/system` | Health detalhado, versões, info |

### 12.2 Wireframe — `/settings/security`

```
┌──────────────────────────────────────────────────────────────────┐
│  Configurações > Segurança                                       │
│                                                                   │
│  Políticas de Autenticação                                       │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │  MFA obrigatório para admins           [🔘 ON ]         │    │
│  │  Grupos: archguard_admins, idm_admins                    │    │
│  │                                                          │    │
│  │  MFA obrigatório para todos            [  OFF 🔘]       │    │
│  │  ⚠️ Pode impedir login de usuários sem MFA              │    │
│  │                                                          │    │
│  │  Expiração de sessão: [8 horas ▾]                       │    │
│  │  Máx tentativas login: [5 ▾] em [5 min ▾]               │    │
│  │  Ação: [Bloquear 15 min ▾]                              │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                   │
│  Contas Break-Glass                                              │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │  ⚠️ Contas de emergência — apenas quando admin           │    │
│  │  principal não consegue acessar.                         │    │
│  │                                                          │    │
│  │  admin       [Rotacionar Senha]   Último uso: nunca      │    │
│  │  idm_admin   [Rotacionar Senha]   Último uso: 30d        │    │
│  └──────────────────────────────────────────────────────────┘    │
│                                                                   │
│                                              [Salvar Alterações]  │
└──────────────────────────────────────────────────────────────────┘
```

---

## 13. Regras de Negócio e Permissões

### 13.1 Sistema de Permissões

```typescript
// src/lib/auth/permissions.ts

export type Permission =
  // Identidades
  | 'persons:read' | 'persons:create' | 'persons:update'
  | 'persons:delete' | 'persons:credentials' | 'persons:import'
  // Grupos
  | 'groups:read' | 'groups:create' | 'groups:update'
  | 'groups:delete' | 'groups:members'
  // OAuth2
  | 'oauth2:read' | 'oauth2:create' | 'oauth2:update'
  | 'oauth2:delete' | 'oauth2:secrets'
  // Service Accounts
  | 'service_accounts:read' | 'service_accounts:create'
  | 'service_accounts:delete' | 'service_accounts:tokens'
  // Vault
  | 'vault:read' | 'vault:admin'
  // Auditoria
  | 'audit:read' | 'audit:export'
  // Sistema
  | 'settings:read' | 'settings:update' | 'system:admin'
```

### 13.2 Mapeamento Grupo → Permissões

```typescript
const GROUP_PERMISSIONS: Record<string, Permission[]> = {
  // Kanidm built-in
  'idm_admins': ['system:admin'],
  'idm_people_admins': [
    'persons:read', 'persons:create', 'persons:update', 'persons:delete',
    'persons:credentials', 'persons:import',
    'groups:read', 'groups:create', 'groups:update', 'groups:members',
  ],
  'idm_oauth2_admins': [
    'oauth2:read', 'oauth2:create', 'oauth2:update',
    'oauth2:delete', 'oauth2:secrets',
  ],
  'idm_service_desk': [
    'persons:read', 'persons:credentials', 'groups:read',
  ],
  'idm_people_on_boarding': [
    'persons:read', 'persons:create', 'persons:import',
    'groups:read', 'groups:members',
  ],
  // ArchGuard
  'archguard_admins': ['system:admin'],
}

// Tenant admins pattern: *_admins → escopo limitado ao tenant
export function derivePermissions(groups: string[]): Permission[] {
  const perms = new Set<Permission>()

  for (const group of groups) {
    const direct = GROUP_PERMISSIONS[group]
    if (direct) {
      direct.forEach(p => perms.add(p))
    }

    // Tenant admin: {tenant}_admins
    if (group.endsWith('_admins') && !GROUP_PERMISSIONS[group]) {
      ['persons:read', 'persons:create', 'persons:update',
       'persons:credentials', 'groups:read', 'groups:members',
       'audit:read'].forEach(p => perms.add(p as Permission))
    }
  }

  if (perms.has('system:admin')) return ALL_PERMISSIONS
  return Array.from(perms)
}
```

### 13.3 Hook de Permissões

```typescript
// src/lib/hooks/use-permissions.ts
export function usePermissions() {
  const { auth } = useRouteContext({ from: '/_authed' })

  return {
    groups: auth.groups,
    permissions: auth.permissions,
    can: (perm: Permission | Permission[]) =>
      hasPermission(auth.permissions, perm),
    canAny: (perms: Permission[]) =>
      hasAnyPermission(auth.permissions, perms),
    isSystemAdmin: auth.permissions.includes('system:admin'),
    isTenantAdmin: auth.groups.some(g =>
      g.endsWith('_admins') && !['archguard_admins', 'idm_admins'].includes(g)
    ),
    tenants: auth.groups
      .filter(g => g.endsWith('_admins'))
      .map(g => g.replace('_admins', '')),
  }
}
```

### 13.4 Guards de Rota e UI

```typescript
// Route guard — usado em beforeLoad
export function requirePermission(...perms: Permission[]) {
  return ({ context }: { context: { auth: AuthContext } }) => {
    const has = perms.every(p =>
      context.auth.permissions.includes(p) ||
      context.auth.permissions.includes('system:admin')
    )
    if (!has) {
      throw redirect({ to: '/dashboard', search: { error: 'forbidden' } })
    }
  }
}

// Uso:
// export const Route = createFileRoute('/_authed/oauth2/create')({
//   beforeLoad: requirePermission('oauth2:create'),
//   component: CreateOAuth2ClientPage,
// })
```

```typescript
// UI gate — esconde elementos sem permissão
interface PermissionGateProps {
  require: Permission | Permission[]
  any?: boolean
  fallback?: React.ReactNode
  children: React.ReactNode
}

export function PermissionGate({ require, any, fallback, children }: PermissionGateProps) {
  const { can, canAny } = usePermissions()
  const perms = Array.isArray(require) ? require : [require]
  const allowed = any ? canAny(perms) : can(require)
  return allowed ? <>{children}</> : <>{fallback}</>
}

// Uso:
// <PermissionGate require="oauth2:secrets">
//   <Button>Mostrar Secret</Button>
// </PermissionGate>
```

### 13.5 Regras de Negócio por Módulo

| Módulo | Regra | Enforcement |
|---|---|---|
| **Identidades** | Tenant admin só vê persons do seu tenant | Client filter + API scope |
| **Identidades** | Não pode deletar a si mesmo | Client + API rejeita |
| **Identidades** | Não pode remover último admin de archguard_admins | Client validation |
| **Grupos** | Grupos builtin são read-only | UI desabilita, class `system_protected` |
| **Grupos** | Não pode criar grupo com nome builtin | Zod + API rejeita |
| **OAuth2** | archguard-console client não pode ser deletado | UI esconde botão |
| **OAuth2** | Rotação de secret requer confirmação dupla | Confirm dialog + digitar nome |
| **Service Accts** | Token só exibido uma vez na criação | Dialog não-persistente |
| **Audit** | Tenant admin só vê eventos do seu tenant | Server filter |
| **Settings** | Apenas system:admin pode alterar | Route guard + UI gate |
| **Cred Reset** | Link expira conforme TTL (padrão 1h) | Kanidm enforces |
| **Import CSV** | Limite 500 persons por import | Client validation |
| **Import CSV** | Valida unicidade de username antes | Pre-check batch |

### 13.6 Multi-tenancy — Filtragem de Dados

Kanidm não tem scoped queries nativo — filtragem é feita client-side:

```typescript
// src/lib/hooks/use-persons.ts
export function usePersons(params?: SearchParams) {
  const { isSystemAdmin, tenants } = usePermissions()
  const query = useQuery(personQueries.list(params))

  const filteredData = useMemo(() => {
    if (!query.data) return []
    if (isSystemAdmin) return query.data

    // Tenant admin: só vê persons nos grupos do tenant
    return query.data.filter(person =>
      person.groupNames.some(g =>
        tenants.some(t => g.startsWith(t))
      )
    )
  }, [query.data, isSystemAdmin, tenants])

  return { ...query, data: filteredData }
}
```

---

## 14. State Management

### 14.1 Estratégia

| Tipo de Estado | Solução | Justificativa |
|---|---|---|
| Server state (Kanidm data) | TanStack Query v5 | Cache, invalidação, mutations, loading/error |
| URL state (filtros, paginação) | TanStack Router search params | Type-safe, shareable, SSR |
| Auth state | Route context + httpOnly cookies | Seguro, não exposto ao client |
| UI local (modals, forms) | React useState/useReducer | Simples |
| UI global (sidebar, theme) | Zustand | Persistência localStorage, leve |

### 14.2 Query Key Convention

```typescript
const queryKeys = {
  persons: {
    all:         ['persons'],
    list:        (p?: SearchParams) => ['persons', 'list', p],
    detail:      (id: string) => ['persons', 'detail', id],
    credentials: (id: string) => ['persons', 'credentials', id],
  },
  groups: {
    all:     ['groups'],
    list:    (p?: SearchParams) => ['groups', 'list', p],
    detail:  (id: string) => ['groups', 'detail', id],
    members: (id: string) => ['groups', 'members', id],
  },
  oauth2: {
    all:    ['oauth2'],
    list:   () => ['oauth2', 'list'],
    detail: (id: string) => ['oauth2', 'detail', id],
    secret: (id: string) => ['oauth2', 'secret', id],
  },
  serviceAccounts: {
    all:    ['service-accounts'],
    list:   () => ['service-accounts', 'list'],
    detail: (id: string) => ['service-accounts', 'detail', id],
  },
  system:    { status: ['system', 'status'], domain: ['system', 'domain'] },
  audit:     { list: (f: AuditFilters) => ['audit', 'list', f] },
  vault:     { status: ['vault', 'status'] },
  dashboard: {
    stats:    ['dashboard', 'stats'],
    activity: ['dashboard', 'activity'],
    health:   ['dashboard', 'health'],
  },
}
```

### 14.3 Mutation com Optimistic Update

```typescript
// Exemplo: Adicionar membro a grupo
export function useAddGroupMember(groupId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (memberId: string) =>
      groupApi.addMembers(groupId, [memberId]),

    onMutate: async (memberId) => {
      await queryClient.cancelQueries({
        queryKey: queryKeys.groups.members(groupId)
      })
      const previous = queryClient.getQueryData(
        queryKeys.groups.members(groupId)
      )
      // Optimistic insert
      queryClient.setQueryData(
        queryKeys.groups.members(groupId),
        (old: GroupMember[] = []) => [
          ...old,
          { id: memberId, name: '...', type: 'person' as const },
        ]
      )
      return { previous }
    },

    onError: (_err, _memberId, context) => {
      if (context?.previous) {
        queryClient.setQueryData(
          queryKeys.groups.members(groupId),
          context.previous
        )
      }
    },

    onSettled: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.groups.members(groupId)
      })
    },
  })
}
```

### 14.4 TanStack Query Hooks — Padrão

```typescript
// src/lib/hooks/use-persons.ts
export const personQueries = {
  list: (params?: SearchParams) => ({
    queryKey: queryKeys.persons.list(params),
    queryFn: () => personApi.list(),
  }),
  detail: (id: string) => ({
    queryKey: queryKeys.persons.detail(id),
    queryFn: () => personApi.get(id),
  }),
  credentials: (id: string) => ({
    queryKey: queryKeys.persons.credentials(id),
    queryFn: () => personApi.credentialStatus(id),
  }),
}

export function useCreatePerson() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (input: CreatePersonInput) => {
      // 1. Criar pessoa
      const person = await personApi.create({
        name: input.name,
        displayname: input.displayname,
        legalname: input.legalname,
      })
      // 2. Definir emails
      if (input.mail?.length) {
        await personApi.setAttr(input.name, 'mail', input.mail)
      }
      // 3. Adicionar a grupos
      if (input.groups?.length) {
        for (const groupId of input.groups) {
          await groupApi.addMembers(groupId, [person.id])
        }
      }
      return person
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.persons.all })
      queryClient.invalidateQueries({ queryKey: queryKeys.groups.all })
    },
  })
}

export function useResetCredentials() {
  return useMutation({
    mutationFn: ({ personId, ttl }: { personId: string; ttl?: number }) =>
      personApi.createResetToken(personId, ttl),
  })
}
```

---

## 15. Componentes Compartilhados

### 15.1 EntityList — Lista Genérica

```typescript
// src/components/shared/entity-list.tsx
interface EntityListProps<T> {
  data: T[]
  columns: ColumnDef<T>[]
  isLoading: boolean
  searchPlaceholder?: string
  filterComponent?: React.ReactNode
  actions?: React.ReactNode              // Botões header (Criar, Importar)
  bulkActions?: BulkAction<T>[]          // Ações em lote
  emptyState?: {
    title: string
    description: string
    action?: { label: string; to: string }
  }
  onSearch?: (query: string) => void
  pagination?: {
    page: number
    pageSize: number
    total: number
    onPageChange: (page: number) => void
  }
}

// Usado em todos os módulos de listagem:
// /identities, /groups, /oauth2, /service-accounts, /audit
```

### 15.2 Sidebar Navigation

```typescript
// src/components/layout/sidebar.tsx
const navigation = [
  { label: 'Dashboard',        to: '/dashboard',        icon: LayoutDashboard, permission: null },
  { label: 'Identidades',      to: '/identities',       icon: Users,           permission: 'persons:read' },
  { label: 'Service Accounts', to: '/service-accounts',  icon: Bot,             permission: 'service_accounts:read' },
  { label: 'Grupos',           to: '/groups',            icon: UsersRound,      permission: 'groups:read' },
  { label: 'OAuth2 / SSO',     to: '/oauth2',            icon: KeyRound,        permission: 'oauth2:read' },
  { label: 'Vault',            to: '/vault',             icon: ShieldCheck,     permission: 'vault:read' },
  { label: 'Auditoria',        to: '/audit',             icon: FileText,        permission: 'audit:read' },
  { label: 'Configurações',    to: '/settings',          icon: Settings,        permission: 'settings:read' },
]

export function Sidebar() {
  const { can } = usePermissions()

  return (
    <aside className="w-64 border-r bg-background h-full">
      <div className="p-4 border-b">
        <h1 className="text-lg font-bold flex items-center gap-2">
          🛡️ ArchGuard
        </h1>
        <p className="text-xs text-muted-foreground">Console</p>
      </div>

      <nav className="p-2 space-y-1">
        {navigation
          .filter(item => !item.permission || can(item.permission))
          .map(item => (
            <Link
              key={item.to}
              to={item.to}
              className="flex items-center gap-3 px-3 py-2 rounded-md text-sm
                         hover:bg-accent transition-colors"
              activeProps={{ className: 'bg-accent font-medium' }}
            >
              <item.icon className="h-4 w-4" />
              {item.label}
            </Link>
          ))}
      </nav>
    </aside>
  )
}
```

### 15.3 Header com Breadcrumb Automático

```typescript
// src/components/layout/header.tsx
export function Header({ user }: { user: SessionUser }) {
  const matches = useMatches()

  // Gera breadcrumb automaticamente das rotas matched
  const breadcrumbs = matches
    .filter(m => m.pathname !== '/')
    .map(m => ({
      label: routeLabels[m.routeId] ?? m.pathname.split('/').pop(),
      to: m.pathname,
    }))

  return (
    <header className="h-14 border-b px-6 flex items-center justify-between">
      <Breadcrumb>
        {breadcrumbs.map((crumb, i) => (
          <BreadcrumbItem key={crumb.to}>
            {i < breadcrumbs.length - 1 ? (
              <BreadcrumbLink asChild>
                <Link to={crumb.to}>{crumb.label}</Link>
              </BreadcrumbLink>
            ) : (
              <BreadcrumbPage>{crumb.label}</BreadcrumbPage>
            )}
          </BreadcrumbItem>
        ))}
      </Breadcrumb>

      <div className="flex items-center gap-4">
        <CommandSearch />
        <UserMenu user={user} />
      </div>
    </header>
  )
}
```

### 15.4 Busca Global (Command Palette)

```typescript
// src/components/layout/command-search.tsx
// Acessível via Ctrl+K / Cmd+K
// Busca em: Persons, Groups, OAuth2 Clients, Pages
// Usa TanStack Query para busca instant com cache

export function CommandSearch() {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const navigate = useNavigate()

  // Busca paralela em todas entidades
  const persons = useQuery({
    queryKey: ['search', 'persons', query],
    queryFn: () => personApi.list(), // filtra client-side para MVP
    enabled: open && query.length > 1,
  })
  const groups = useQuery({
    queryKey: ['search', 'groups', query],
    queryFn: () => groupApi.list(),
    enabled: open && query.length > 1,
  })

  // Keyboard shortcut
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault()
        setOpen(true)
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [])

  return (
    <CommandDialog open={open} onOpenChange={setOpen}>
      <CommandInput placeholder="Buscar pessoas, grupos, apps..." value={query} onValueChange={setQuery} />
      <CommandList>
        <CommandGroup heading="Pessoas">
          {/* filtered persons */}
        </CommandGroup>
        <CommandGroup heading="Grupos">
          {/* filtered groups */}
        </CommandGroup>
        <CommandGroup heading="Páginas">
          <CommandItem onSelect={() => navigate({ to: '/identities/create' })}>
            + Nova Pessoa
          </CommandItem>
          <CommandItem onSelect={() => navigate({ to: '/groups/create' })}>
            + Novo Grupo
          </CommandItem>
        </CommandGroup>
      </CommandList>
    </CommandDialog>
  )
}
```

### 15.5 ConfirmDialog — Confirmação com Digitação

```typescript
// Para ações destrutivas que requerem digitação do nome da entidade

interface ConfirmDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description: string
  confirmText: string              // texto que o usuário precisa digitar
  destructive?: boolean
  onConfirm: () => void
  isLoading?: boolean
}

// Usado para: Deletar pessoa, deletar grupo, rotacionar secret, remover OAuth2 client
```

---

## 16. Tratamento de Erros

### 16.1 Camadas de Erro

```
┌──────────────────────────────────────────────────┐
│  LAYER 1: Route Error Boundary                    │
│  Captura erros de loader/beforeLoad              │
│  → Exibe página de erro com retry                │
├──────────────────────────────────────────────────┤
│  LAYER 2: TanStack Query Error                    │
│  Captura erros de API calls                      │
│  → Toast notification + retry automático (1x)    │
├──────────────────────────────────────────────────┤
│  LAYER 3: Mutation Error                          │
│  Captura erros de operações de escrita           │
│  → Toast com mensagem específica do Kanidm       │
├──────────────────────────────────────────────────┤
│  LAYER 4: Form Validation                         │
│  Zod + TanStack Form                             │
│  → Inline field errors                           │
└──────────────────────────────────────────────────┘
```

### 16.2 Mapeamento de Erros Kanidm

```typescript
// src/lib/utils/error-mapper.ts

const KANIDM_ERROR_MAP: Record<string, string> = {
  'duplicate_value':    'Este valor já existe no sistema.',
  'no_matching_entries': 'Nenhum registro encontrado.',
  'access_denied':      'Sem permissão para esta operação.',
  'invalid_attribute':  'Atributo inválido.',
  'missing_attribute':  'Atributo obrigatório não fornecido.',
  'session_expired':    'Sessão expirada. Faça login novamente.',
  'account_locked':     'Conta bloqueada por excesso de tentativas.',
  'credential_invalid': 'Credenciais inválidas.',
  'schema_violation':   'Os dados não seguem o formato esperado.',
}

export function mapKanidmError(error: unknown): string {
  if (error instanceof Error) {
    // Kanidm retorna erros no formato "Kanidm API 400: {...}"
    const match = error.message.match(/Kanidm API (\d+): (.+)/)
    if (match) {
      const [, status, body] = match
      try {
        const parsed = JSON.parse(body)
        return KANIDM_ERROR_MAP[parsed.error] ?? parsed.error ?? `Erro ${status}`
      } catch {
        return body
      }
    }
    return error.message
  }
  return 'Erro desconhecido'
}
```

### 16.3 Toast Notifications

```typescript
// Padrão para mutations:
const createPerson = useCreatePerson()

const handleSubmit = async (data: CreatePersonInput) => {
  try {
    await createPerson.mutateAsync(data)
    toast.success('Pessoa criada com sucesso!')
    navigate({ to: '/identities' })
  } catch (error) {
    toast.error(mapKanidmError(error))
  }
}
```

---

## 17. Internacionalização

### 17.1 Configuração

```typescript
// src/lib/i18n/config.ts
import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import ptBR from './pt-BR.json'
import en from './en.json'

i18n.use(initReactI18next).init({
  resources: {
    'pt-BR': { translation: ptBR },
    en: { translation: en },
  },
  lng: 'pt-BR',                    // Padrão brasileiro
  fallbackLng: 'en',
  interpolation: { escapeValue: false },
})

export default i18n
```

### 17.2 Estrutura de Traduções

```json
// pt-BR.json (excerpt)
{
  "nav": {
    "dashboard": "Dashboard",
    "identities": "Identidades",
    "serviceAccounts": "Service Accounts",
    "groups": "Grupos",
    "oauth2": "OAuth2 / SSO",
    "vault": "Vault",
    "audit": "Auditoria",
    "settings": "Configurações"
  },
  "identities": {
    "title": "Identidades",
    "create": "Nova Pessoa",
    "import": "Importar CSV",
    "search": "Buscar por nome, email...",
    "columns": {
      "name": "Nome",
      "email": "Email",
      "groups": "Grupos",
      "mfa": "MFA",
      "status": "Status"
    },
    "form": {
      "username": "Username",
      "usernameHelp": "Apenas letras minúsculas, números, . _ -",
      "displayName": "Nome de exibição",
      "legalName": "Nome legal",
      "emails": "Emails",
      "addEmail": "Adicionar email"
    },
    "status": {
      "active": "Ativo",
      "expired": "Expirado",
      "locked": "Bloqueado",
      "notYetValid": "Não ativo ainda"
    },
    "credential": {
      "passkey": "Passkey",
      "password": "Password",
      "totp": "TOTP",
      "backup": "Códigos backup",
      "sshKeys": "Chaves SSH",
      "resetLink": "Gerar Link de Reset",
      "resetTtl": "Validade",
      "resetWarning": "Este link dá acesso total às credenciais"
    },
    "actions": {
      "block": "Bloquear Conta",
      "delete": "Remover Pessoa",
      "resetCredentials": "Gerar Link Reset"
    }
  },
  "groups": {
    "title": "Grupos",
    "create": "Novo Grupo",
    "members": "Membros",
    "addMember": "Adicionar membro",
    "removeMember": "Remover",
    "builtin": "Sistema",
    "tenant": "Tenant",
    "treeView": "Árvore",
    "listView": "Lista"
  },
  "oauth2": {
    "title": "OAuth2 / SSO Clients",
    "create": "Novo Client",
    "types": {
      "basic": "Basic (Confidential)",
      "public": "Public (PKCE-only)"
    },
    "secret": "Client Secret",
    "rotateSecret": "Rotacionar Secret",
    "rotateWarning": "Rotacionar invalidará todos tokens ativos",
    "scopeMap": "Scope Mapping",
    "redirectUrls": "Redirect URLs",
    "integration": "Integração",
    "testFlow": "Testar Fluxo OIDC"
  },
  "common": {
    "save": "Salvar",
    "cancel": "Cancelar",
    "create": "Criar",
    "edit": "Editar",
    "delete": "Remover",
    "confirm": "Confirmar",
    "next": "Próximo",
    "previous": "Anterior",
    "search": "Buscar...",
    "loading": "Carregando...",
    "noResults": "Nenhum resultado",
    "copy": "Copiar",
    "copied": "Copiado!",
    "showAll": "Ver todos",
    "export": "Exportar",
    "import": "Importar",
    "actions": "Ações",
    "showing": "Mostrando {{from}}-{{to}} de {{total}}",
    "selected": "{{count}} selecionados"
  },
  "errors": {
    "generic": "Algo deu errado. Tente novamente.",
    "notFound": "Recurso não encontrado.",
    "forbidden": "Sem permissão para esta operação.",
    "sessionExpired": "Sessão expirada. Faça login novamente.",
    "networkError": "Erro de conexão. Verifique sua rede."
  }
}
```

---

## Apêndice: Resumo de Implementação por Fase

### Fase 1 — Fundação (Semanas 1–2)

- [ ] `npm create @tanstack/start` + configuração do projeto
- [ ] Shadcn/ui + Tailwind + Lucide setup
- [ ] OIDC flow completo (login → callback → session → logout)
- [ ] Server function proxy para Kanidm API
- [ ] `_authed.tsx` guard com derivação de permissões
- [ ] AppShell (sidebar + header + breadcrumb)
- [ ] Busca global (Cmd+K)
- [ ] Dashboard com stats e health check

### Fase 2 — CRUD Core (Semanas 3–5)

- [ ] Persons: listagem + criação wizard + detalhe + tabs
- [ ] Groups: listagem (lista+árvore) + criação + membros
- [ ] OAuth2: listagem + criação wizard + detalhe + scope editor
- [ ] Service Accounts: listagem + criação + token management
- [ ] Credential reset flow
- [ ] Permission gates em todas as rotas/componentes

### Fase 3 — Avançado (Semanas 6–8)

- [ ] Import CSV wizard completo
- [ ] Audit log com filtros e sheet de detalhe
- [ ] Settings (security policies, backup)
- [ ] Vault status panel
- [ ] Internationalization (pt-BR + en)
- [ ] Ações em lote (bulk operations)
- [ ] OIDC test panel no OAuth2 detail
- [ ] Integration snippets (Spring, React, Flutter)

### Fase 4 — Polish (Semanas 9–10)

- [ ] Error handling completo + error mapper Kanidm
- [ ] Loading states + skeleton loaders
- [ ] Optimistic updates em mutations críticas
- [ ] Testes unitários (hooks, validators, normalizers)
- [ ] Testes de integração (flows principais)
- [ ] Dockerfile + Docker Compose integration
- [ ] Documentação (README, admin guide)
- [ ] Deploy em produção com Rio Quality

---

*ArchGuard Console — Identity & Secrets Management Interface. Built with TanStack Start by IntegrAllTech.*
