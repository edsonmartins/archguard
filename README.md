# ArchGuard — Corpus de governança (SDD)

Plano de controle de identidade da plataforma **ArchGate** (PAM), derivado de fork do
**Casdoor** (Apache License 2.0).

> **Estado:** especificação completa, pré-implementação. Nenhum código escrito.
> **Método:** CONSTITUTION → ADR → RFC → pacote OpenSpec → implementação (I-9.1).

---

## Índice

### Constituição
[`CONSTITUTION.md`](CONSTITUTION.md) — invariantes que nenhum ADR, RFC ou pacote pode violar.
Seções 2 (licença), 3 (soberania) e 4 (segurança) são pétreas na v1.

### ADRs — decisões arquiteturais (`docs/adr/`)

| # | Decisão | Eixo |
|---|---|---|
| [0001](docs/adr/ADR-0001-fork-casdoor-como-base.md) | Fork direto do Casdoor como base | Fundacional |
| [0002](docs/adr/ADR-0002-licenciamento-e-atribuicao.md) | Licenciamento, atribuição e higiene de dependências | Jurídico |
| [0003](docs/adr/ADR-0003-sincronizacao-com-upstream.md) | Sincronização seletiva por cherry-pick | Governança do fork |
| [0004](docs/adr/ADR-0004-console-administrativo-archbase.md) | Console próprio em React 19 + Mantine v9 + Archbase | Frontend |
| [0005](docs/adr/ADR-0005-separacao-authn-authz-openfga.md) | Separação AuthN/AuthZ; OpenFGA como PDP | Autorização |
| [0006](docs/adr/ADR-0006-multitenancy-b2b.md) | Usuário em múltiplas organizações | Modelo de dados |
| [0007](docs/adr/ADR-0007-trilha-auditoria-imutavel.md) | Trilha imutável e tamper-evident | Auditoria |
| [0008](docs/adr/ADR-0008-eliminacao-master-password-e-break-glass.md) | Fim da senha-mestra; break-glass formal | Segurança |
| [0009](docs/adr/ADR-0009-postgresql-unico-backend.md) | PostgreSQL 15+ como único backend | Persistência |
| [0010](docs/adr/ADR-0010-mfa-obrigatorio-e-step-up.md) | MFA obrigatório, WebAuthn-first, step-up | Autenticação |
| [0011](docs/adr/ADR-0011-federacao-oidc-componentes-archgate.md) | Federação OIDC com componentes do ArchGate | Integração |
| [0012](docs/adr/ADR-0012-gestao-de-chaves-e-segredos.md) | Custódia de chaves no OpenBao | Criptografia |
| [0013](docs/adr/ADR-0013-observabilidade-opentelemetry.md) | OpenTelemetry → VictoriaMetrics/Loki/Tempo | Observabilidade |
| [0014](docs/adr/ADR-0014-lgpd-retencao-e-crypto-shredding.md) | LGPD: retenção e crypto-shredding | Conformidade |
| [0015](docs/adr/ADR-0015-rebranding-e-reducao-de-escopo.md) | Rebranding e redução de superfície | Escopo |
| [0016](docs/adr/ADR-0016-manutencao-do-framework-beego.md) | Manter Beego isolado atrás de fronteiras | Dívida técnica |
| [0017](docs/adr/ADR-0017-perfis-de-implantacao-e-custodia-minima.md) | Perfis `dev`/`pilot`/`production` e custódia mínima — **emenda a I-1.3** | Implantação |
| [0018](docs/adr/ADR-0018-forja-e-infraestrutura-de-ci.md) | Forja e CI: **GitHub privado** (GitLab CE rejeitado com motivo) — prevenção p/ papéis de trabalho + detecção de contorno admin (**submetido à ratificação**; aguarda 1 assinatura) | Infraestrutura |
| [0019](docs/adr/ADR-0019-matriz-de-licencas-mpl-e-remocao-ldap-server.md) | MPL linkado e remoção do servidor LDAP — **emenda a I-2.2 (pétreo)** · *aguarda 2 assinaturas* | Jurídico |

### RFCs — desenhos técnicos (`docs/rfc/`)

| # | Documento |
|---|---|
| [0001](docs/rfc/RFC-0001-arquitetura-de-referencia.md) | Arquitetura de referência (componentes, fluxos, degradação, RNFs) |
| [0002](docs/rfc/RFC-0002-modelo-de-dados-identidade-multitenancy.md) | Modelo de dados de identidade e multi-tenancy |
| [0003](docs/rfc/RFC-0003-subsistema-de-auditoria-imutavel.md) | Subsistema de auditoria imutável |
| [0004](docs/rfc/RFC-0004-modelo-de-autorizacao-openfga.md) | Modelo de autorização e integração com OpenFGA |
| [0005](docs/rfc/RFC-0005-archguard-console-arquitetura-frontend.md) | ArchGuard Console: arquitetura frontend |
| [0006](docs/rfc/RFC-0006-contratos-de-federacao-oidc-archgate.md) | Contratos de federação OIDC do ArchGate |
| [0007](docs/rfc/RFC-0007-migracao-e-coexistencia.md) | Migração, coexistência e sincronismo com diretórios |

### Pacotes OpenSpec (`openspec/changes/`)

Cada pacote contém `proposal.md`, `design.md`, `tasks.md` e `specs/<capability>/spec.md`
com critérios de aceite em **WHEN/THEN**.

| # | Pacote | Capability | Tarefas |
|---|---|---|---|
| 001 | [Bootstrap do fork](openspec/changes/001-bootstrap-fork/) | `fork-baseline` | 31 |
| 002 | [Identidade e multi-tenancy](openspec/changes/002-identity-multitenancy/) | `identity-multitenancy` | 20 |
| 003 | [Trilha de auditoria imutável](openspec/changes/003-immutable-audit-trail/) | `audit-trail` | 21 |
| 004 | [Acesso privilegiado](openspec/changes/004-privileged-access-controls/) | `privileged-access` | 20 |
| 005 | [MFA e step-up](openspec/changes/005-mfa-step-up/) | `authentication-assurance` | 20 |
| 006 | [Federação OIDC](openspec/changes/006-oidc-federation-archgate/) | `oidc-federation` | 20 |
| 007 | [Autorização granular](openspec/changes/007-authz-openfga/) | `fine-grained-authz` | 20 |
| 008 | [Console administrativo](openspec/changes/008-admin-console/) | `admin-console` | 26 |
| 009 | [Sincronismo e federação de entrada](openspec/changes/009-directory-sync-federation/) | `directory-sync` | 21 |
| 010 | [Observabilidade e conformidade](openspec/changes/010-observability-compliance/) | `observability-compliance` | 25 |

**Total: 224 tarefas** em granularidade de sessão.

---

## Grafo de dependências

```
001 bootstrap-fork
 ├──► 002 identity-multitenancy
 │     ├──► 003 immutable-audit-trail
 │     │     ├──► 004 privileged-access ◄── 005 mfa-step-up
 │     │     │     └──► 007 authz-openfga
 │     │     ├──► 006 oidc-federation  ◄── 005
 │     │     └──► 010 observability-compliance
 │     └──► 009 directory-sync  ◄── 005, 006
 └──► 008 admin-console  ◄── 002,003,004,005,007
```

Caminho crítico: **001 → 002 → 003 → 005 → 004 → 007**.

## Fases sugeridas

| Fase | Pacotes | Marco |
|---|---|---|
| **M1 — Base governada** | 001 | Fork congelado, CI com invariantes, stack mínima sobe |
| **M2 — Núcleo de identidade** | 002, 003 | Multi-tenancy B2B + trilha verificável |
| **M3 — Privilégio** | 005, 004 | MFA obrigatório, step-up, break-glass; **fim de todo backdoor** |
| **M4 — Integração** | 006, 007 | ArchGate federado com autorização granular |
| **M5 — Produto** | 008, 009 | Console próprio + sincronismo corporativo |
| **M6 — GA** | 010 | Custódia real, LGPD, observabilidade; pronto para venda |

## Emendas aplicadas

| Data | Alteração | ADR |
|---|---|---|
| 2026-07-20 | I-1.3 emendado: autossuficiência passa a descrever continuidade sob falha, não topologia suportada. Perfis `dev`/`pilot`/`production` normatizados. I-4.3 (pétreo) preservado | [ADR-0017](docs/adr/ADR-0017-perfis-de-implantacao-e-custodia-minima.md) |
| 2026-07-20 | **PROPOSTO** — I-2.2 corrigido: copyleft por arquivo (MPL) permitido linkado quando não modificado; servidor LDAP embutido removido (GPL). **Seção pétrea: exige ratificação de Edson + Neimar** | [ADR-0019](docs/adr/ADR-0019-matriz-de-licencas-mpl-e-remocao-ldap-server.md) |

## Pendências bloqueantes antes do M1

1. **Verificar na fonte primária** (aba Insights/Releases do repositório upstream) a release
   corrente, a licença vigente e a base de mantenedores — há divergência entre fontes
   secundárias sobre a última tag publicada. Registrar evidência antes de congelar o fork
   point.
2. **Due diligence jurídica** com advogado especializado: obrigações Apache 2.0 do fork,
   dependências MPL-2.0 em processo separado, licenças transitivas. Os ADRs 0002 e 0014 são
   panorama técnico, **não parecer legal**.
3. **Inventário do PoC Kanidm** (volume e qualidade dos dados de identidade) para dimensionar
   o RFC-0007.
4. **Ratificação do ADR-0019** (Edson + Neimar): até as duas assinaturas, as três dependências
   MPL-2.0 sobreviventes (`golang-lru` via Beego, `go-uuid` via gokrb5, `layeh.com/radius`)
   são **pendência conhecida documentada** — não violação a corrigir, não permissão a exercer.
   Bloqueia o fechamento do T-018 (INV-4). **Ao ratificar**: aplicar a emenda ao I-2.2
   (CONSTITUTION + Anexo B), a matriz de três estados no ADR-0002 e a linha do INV-4 no
   CLAUDE.md §3 (texto já fixado na decisão de 2026-07-20).
5. **Ratificação do ADR-0018** (Edson): forja = **GitHub privado** (já em uso). PoC descartável
   demonstrou que o contorno do tier admin é **estrutural em qualquer forja** — sustenta o
   "risco aceito" por demonstração. Bloqueia T-003 e T-019b (T-019a e o Bloco 3 seguem
   liberados). **Aceite bloqueante do T-003:** conta Write não faz push/merge-vermelho/
   force-push; alteração de ruleset alerta; `bypass actors` vazio.

## Questões em aberto (registradas nos RFCs)

- Fonte de tempo confiável: NTP autenticado basta ou haverá exigência de carimbo RFC 3161?
- Ativos são cadastrados no ArchGuard ou importados de Warpgate/NetBird, e em qual
  granularidade (host, serviço, conta-alvo)?
- Coexistência de dois IdPs por componente durante a virada é aceitável, ou a troca deve ser
  atômica por componente?
- Formato de exportação de auditoria: adotar OCSF para integração com SIEMs?
