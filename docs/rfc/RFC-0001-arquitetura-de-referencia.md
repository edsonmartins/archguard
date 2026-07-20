# RFC-0001 — Arquitetura de referência do ArchGuard

- **Status:** Proposto
- **Data:** 2026-07-19
- **ADRs relacionados:** ADR-0001, ADR-0005, ADR-0009, ADR-0011, ADR-0012, ADR-0013, ADR-0016

## 1. Objetivo

Definir a arquitetura de referência do ArchGuard como plano de controle de identidade do
ArchGate: componentes, fronteiras, fluxos principais, topologia de implantação e modos de
degradação.

## 2. Escopo

**No escopo:** identidade, credenciais, MFA e step-up, sessões, emissão e revogação de tokens,
federação com IdPs corporativos, multi-tenancy, trilha de auditoria, integração com PDP e
cofre, API pública e console.

**Fora do escopo:** proxy de protocolo, gravação de sessão, brokering de credencial de ativo
alvo, roteamento de rede — responsabilidades de Warpgate, Guacamole, OpenBao e NetBird.

## 3. Visão de componentes

```
                         ┌──────────────────────────────┐
                         │   ArchGuard Console (SPA)    │
                         │ React 19 · Mantine v9 · Archbase │
                         └───────────────┬──────────────┘
                                         │ HTTPS · API pública v1 (OpenAPI)
┌────────────────────────────────────────▼─────────────────────────────────────┐
│                            ArchGuard Core (Go — fork)                        │
│                                                                              │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐  │
│  │ Protocolos │ │ Identidade │ │ Sessão &   │ │ Auditoria  │ │ Federação  │  │
│  │ OIDC/OAuth2│ │ Users·Orgs │ │ MFA·StepUp │ │ append-only│ │ LDAP·SAML  │  │
│  │ SAML·LDAP  │ │ Memberships│ │ Break-glass│ │ hash-chain │ │ SCIM·sync  │  │
│  │ RADIUS·CAS │ │ Grupos     │ │            │ │            │ │            │  │
│  └────────────┘ └────────────┘ └────────────┘ └────────────┘ └────────────┘  │
│         │              │              │              │              │        │
│  ┌──────▼──────────────▼──────────────▼──────────────▼──────────────▼─────┐  │
│  │  Portas (interfaces): PolicyDecisionPoint · KeyCustodian · AuditSink ·  │  │
│  │  DirectoryConnector · Notifier · Repository(tenant-scoped)             │  │
│  └──────┬───────────────────┬──────────────────┬────────────────┬─────────┘  │
└─────────┼───────────────────┼──────────────────┼────────────────┼────────────┘
          │                   │                  │                │
   ┌──────▼──────┐    ┌───────▼──────┐   ┌───────▼──────┐  ┌──────▼───────┐
   │ PostgreSQL  │    │  OpenFGA     │   │  OpenBao     │  │ Coletor OTLP │
   │ 15+ (RLS)   │    │  (PDP)       │   │ (chaves)     │  │ (telemetria) │
   └─────────────┘    └──────────────┘   └──────────────┘  └──────────────┘

   Consumidores (OIDC): Warpgate · Apache Guacamole · NetBird · OpenBao ·
                        Proxy Oracle JDBC (Java) · demais produtos IntegrAllTech
```

## 4. Fronteiras arquiteturais

| Porta | Implementação v1 | Obrigatória? | Degradação |
|---|---|---|---|
| `Repository` | PostgreSQL 15+ (pgx para código novo; XORM legado) | **Sim** | Indisponível ⇒ serviço fora |
| `KeyCustodian` | OpenBao (HTTP) | **Sim** em `pilot`/`production`; keystore local selado em `dev` (ADR-0017) | Cache curto; expirado ⇒ emissão degrada e L3 falha fechado |
| `PolicyDecisionPoint` | OpenFGA | Não | Decisões privilegiadas **negam** (fail-closed) |
| `AuditSink` | Trilha própria + export OTLP | **Sim** | Falha ⇒ operação privilegiada negada |
| `DirectoryConnector` | LDAP/AD, SCIM, OIDC/SAML | Não | Sincronismo pausa; login local segue |
| `Notifier` | SMTP / webhook | Break-glass: sim | Sem canal ⇒ break-glass negado |

Regra estrutural (ADR-0016): pacotes de domínio não importam framework web nem ORM.

## 5. Fluxos principais

### 5.1 Login com seleção de tenant
1. Autenticação primária (senha/passkey/IdP federado).
2. Resolução de memberships ativos da identidade (ADR-0006).
3. Seleção de tenant ativo (automática se único; explícita e auditada se múltiplos).
4. Avaliação da política de MFA **do tenant ativo** (mais restritiva vence — ADR-0010).
5. Emissão de token com `org`, `acr`, `amr`, `sid` (RFC-0006).
6. Eventos de auditoria: autenticação, seleção de tenant, resultado de MFA.

### 5.2 Abertura de sessão privilegiada
1. Componente do ArchGate redireciona ao ArchGuard (OIDC).
2. Operação classificada **L3** ⇒ step-up WebAuthn imediato.
3. Consulta ao PDP (OpenFGA) para o par (sujeito, ativo, ação, contexto).
4. Negação ⇒ evento auditado com justificativa da decisão. Aprovação ⇒ token de escopo mínimo
   com identificador de correlação de sessão.
5. O componente abre a sessão e registra sua própria trilha, correlacionada pelo mesmo
   identificador (ADR-0011).

### 5.3 Break-glass
Solicitação com justificativa ⇒ step-up ⇒ notificação imediata ⇒ aprovação de N pares ⇒
concessão temporária com expiração automática ⇒ revogação de sessões derivadas ⇒ revisão
pós-uso. Falha de auditoria ou de notificação ⇒ **negado** (ADR-0008).

## 6. Topologia de implantação

**Produção (Docker Swarm + Traefik):** N réplicas stateless do core atrás do Traefik (TLS
terminado no ingress, HTTPS obrigatório para WebAuthn), PostgreSQL com réplica e PITR,
OpenBao em HA, OpenFGA replicado, coletor OTLP.

**Estado:** o core é stateless; estado de sessão em banco (com cache), permitindo perda de
qualquer réplica sem perda de sessão.

**Perfis de implantação (normativo: ADR-0017).** O perfil é configuração explícita e
obrigatória; ausência de declaração é erro fatal de inicialização.

| Perfil | Composição | Custódia | Uso |
|---|---|---|---|
| `dev` | Core + PostgreSQL | Keystore local selado | Desenvolvimento, CI, smoke test, demonstração. **Não suportado em produção**; operações L3 negadas; marcado como não conforme no health check |
| `pilot` | Core + PostgreSQL + OpenBao | OpenBao | Piloto e homologação. Sem OpenFGA, decisões privilegiadas granulares ficam indisponíveis (negadas), não permissivas |
| `production` | Core + PostgreSQL + OpenBao (HA) + OpenFGA + coletor OTLP | OpenBao (HA) | **Única configuração suportada comercialmente** |

## 7. Modos de degradação (resumo normativo)

| Falha | Comportamento exigido |
|---|---|
| PDP indisponível | AuthN normal; decisões privilegiadas **negadas**; alerta |
| Cofre indisponível | Cache curto sustenta assinatura; expirado ⇒ emissão degrada; L3 negado |
| Auditoria indisponível | **Operações privilegiadas negadas** (I-5.4) |
| Coletor OTLP indisponível | Nenhum impacto funcional |
| IdP externo indisponível | Login federado falha; login local segue |
| ArchGuard indisponível | Sessões existentes nos componentes sobrevivem até expirar; **novos acessos não são concedidos** |
| Perfil `dev` em uso | Operações L3 **negadas**; instalação marcada como não conforme; recusa de inicialização sob indício de exposição pública (ADR-0017) |

## 8. Requisitos não funcionais (metas iniciais)

| Métrica | Meta |
|---|---|
| Latência p95 de emissão de token | < 150 ms (sem step-up) |
| Latência p95 de decisão do PDP | < 50 ms |
| Disponibilidade do plano de autenticação | 99,9% mensal |
| RPO / RTO | ≤ 5 min / ≤ 30 min |
| Escala inicial por instância | 50k identidades, 200 organizações, 100 req/s sustentados |

Metas são hipóteses a validar em teste de carga no M2; divergência gera revisão deste RFC.

## 9. Riscos arquiteturais

| Risco | Mitigação |
|---|---|
| ArchGuard vira ponto único de falha do ArchGate | HA; sessões sobrevivem à indisponibilidade; runbook |
| Divergência do fork encarece rebase | ADR-0003 + `DIVERGENCE.md` + suíte de invariantes |
| Sincronizador do PDP diverge do banco | Outbox transacional + reconciliação periódica (RFC-0004) |
| Latência de auditoria síncrona no hot path | Escrita otimizada e particionada; medição contínua (ADR-0013) |
