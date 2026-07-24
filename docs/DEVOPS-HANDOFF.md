# Handoff para o `archguard-devops`

Este documento lista o que o repositório **archguard** (plano de controle, Go) já
entrega e **o que resta para o GA**, que vive todo no projeto de implantação
`archguard-devops`: montar a infraestrutura, ligar os adapters reais no boot e
fechar o bloco de observabilidade/compliance operacional.

> **Como o backend foi construído:** cada capacidade tem um **porto** no domínio
> (`internal/domain/**`) e uma implementação **provisória** (dev/test) que o boot de
> produção substitui pela implementação real. Nada no domínio precisa mudar — o
> handoff é DI (injeção de dependência) + infraestrutura.

---

## 1. Estado do repositório principal (pacotes 001–010)

| Pacote | Estado | Observação |
|---|---|---|
| 001 bootstrap-fork | ✅ completo | |
| 002 identity-multitenancy | ✅ completo | isolamento em duas barreiras (repos com escopo + RLS FORCE) |
| 003 immutable-audit-trail | ✅ completo | cadeia por hash, selagem Ed25519, verificador |
| 004 privileged-access | ✅ completo | delegação + break-glass |
| 005 mfa-step-up | ✅ completo | fator por identidade, AAL, catálogo de operações (INV-8) |
| 006 oidc-federation | ✅ completo | claims v1, PKCE, rotação de refresh, JWKS |
| 007 authz-openfga | ✅ completo (20/20) | PDP como avaliador Go puro; OpenFGA = projeção opcional |
| 008 admin-console | ⏸️ não iniciado | frontend React (decisão pendente: in-tree vs projeto separado) |
| 009 directory-sync-federation | ✅ completo (21/21) | LDAP/SCIM/SAML/OIDC de entrada + JIT dedup |
| 010 observability-compliance | ✅ **núcleo Go** (17/25) | 8 restantes = infra deste handoff |

**`go test ./...` local** encosta em testes **legados do upstream** (`xlsx`, `object`)
que exigem fixture/MySQL — pré-existentes, fora do escopo. A CI oficial roda
`go test -tags skipCi` **com serviço de DB**. Validar o trabalho pelos pacotes-alvo
(`internal/...` + `test/invariants`).

---

## 2. Stack de infraestrutura a montar (CLAUDE.md §9 / RFC-0001)

| Camada | Componente | Uso pelo código |
|---|---|---|
| Banco | **PostgreSQL 15+** (RLS) | migrations em `internal/migrate`; papéis segregados em `deploy/postgres/roles.sql` (app **sem** BYPASSRLS, **sem** UPDATE/DELETE na auditoria) |
| Cofre | **OpenBao** (HA) via HTTP | `internal/adapters/openbao` (KV v2 + transit); **nunca linkado** (MPL-2.0) |
| PDP | **OpenFGA** (opcional) | projeção das tuplas do `authz_tuple`; ou usar o avaliador Go embutido |
| Métricas | **VictoriaMetrics** → Grafana | export OTLP (T-001/006) |
| Logs | **Loki** | cópia de auditoria marcada + logs operacionais (T-003/009) |
| Traces | **Tempo** | tracing distribuído (T-002) |
| Deploy | **Docker Swarm + Traefik**, **TLS obrigatório** | |

RPO/RTO alvo: ≤ 5 min / ≤ 30 min. Escala inicial por instância: 50k identidades,
200 organizações, 100 req/s.

---

## 3. Adapters provisórios → produção (a substituição de DI no boot)

Cada linha é uma troca no wiring de inicialização (perfil `production`). Todos os
provisórios estão marcados **não suportados em produção** e o `/health` já reporta
**não conforme** quando a custódia é local (pacote 010, T-016).

| Porto (domínio) | Provisório (dev) | Produção (ligar no boot) |
|---|---|---|
| `SecretStore` | `secretstore.Provisional` (keystore selado) | `openbao.NewKVSecretStore(client, "secret")` |
| `VaultSigner` | — (Signer local do 006) | `openbao.NewTransitSigner(client, "transit")` |
| `Sealer` (auditoria) | `auditseal.Provisional` (Ed25519 in-process) | `openbao.NewTransitSealer(client, "transit", "audit-seal")` |
| `KeyCustodian` / `SubjectCipher` | `keycustodian.Provisional` (HMAC + AES in-process) | backend OpenBao (HMAC via transit + chaves por titular no cofre) |
| `Alerter` | `alerting` provisório (log/memória) | integração com alertmanager (T-008) |
| `AuthzMetrics` | `metrics.MemoryAuthzMetrics` | exportador OTel (T-001) via `metrics.MeasuredPDP` |
| `GlobalAuthorizer` (007) | `ProfileAuthorizer` (só perfil Dev) | OpenFGA PDP (ou o avaliador Go + política real) |
| `PolicyDecisionPoint` | avaliador Go sobre `authz_tuple` | idem ou `OpenFGA` atrás do mesmo porto |

### 3.1 Endurecimento obrigatório: chave built-in de token (INV-7)

O fork herdou do Casdoor o par default `object/token_jwt_key.{key,pem}` (chave
privada RSA **committada** — placeholder de dev conhecido publicamente, não um
segredo vazado). O `object/init.go:initBuiltInCert()` a semeia no cert `cert-built-in`
e `SealCerts()` a move para o keystore no boot; `CertPrivateKeyPEM` (`object/
deployment.go`) resolve do keystore quando o cert é `keystore:<id>`, senão devolve a
chave crua. Em **produção**, o ADR-0017 já cobre isso (keystore/cofre; perfil dev não
conforme; L3 negado em dev).

**Pendente (fazer no devops, com boot verificável):** um plano de PAM **NÃO** deve
versionar chave privada default alguma (INV-7). Trocar o seed por **geração no boot**
(chave nova por deployment, selada no keystore/cofre — nunca persistida no repo) e
**remover** `object/token_jwt_key.key`/`.pem` do versionamento (`.gitignore`),
mantendo `readTokenFromFile()` a retornar vazio quando ausente e o dev obtendo a chave
do keystore. Requer ensaio de boot (assina/verifica token) — por isso não foi feito no
repo principal sem DB/keystore. Enquanto não feito: **override obrigatório do cert
default em qualquer instalação não-dev**.

---

## 4. Cola de boot por pacote (o que ligar no `main.go`/roteador Beego)

### 4.1 Login real / ponte de sessão (006)
A ponte de sessão está construída e testada em duas camadas
(`postgres.SessionBridge` + `http.BridgingResolver`). Falta a cola de fronteira:

1. **Adapter Beego `LegacyBinding`** (~20 linhas): ler/gravar `identity.ID` +
   `auth_session.ID` na sessão Beego legada.
2. **Hook no login legado**: ao logar com sucesso, resolver a identidade
   (`username → email → IdentityStore.FindByEmail` com o `KeyCustodian`), chamar
   `SessionBridge.EstablishSession(identity, AAL provado, métodos, now)` e gravar
   `identity.ID`/`session.ID` na sessão Beego.
3. **Montar o `OIDCServer`** (`internal/http`) no roteador Beego (uma linha:
   `web.Handler("/", server.Handler())`).
4. **DI no boot**: `Signer` do cofre (transit), `pool` pgx, `BridgingResolver`,
   config de issuer/endpoints, registry de clientes.

### 4.2 Autorização granular (007) — pendências M4
- **Fontes de dados** de `asset`/`asset_group`/atribuição-de-papel/filiação-a-grupo
  (RFC-0004 §9, questões abertas: cadastro vs importação; granularidade host/serviço/
  conta). Hoje são domínio puro + mecanismos testados sobre conjuntos fornecidos.
- **Enqueue na mutação**: chamar a projeção (`domain.Project*`) → `AuthzOutbox` nas
  transações dos fluxos 002/004 que mudam autorização.
- **Binding `GrantTarget → assetRef`** canônico.
- **Scheduler** do `TuplePublisher` (drena outbox) e do `AuthzReconciler`.
- (Opcional) **OpenFGA** como adapter de projeção atrás de `PolicyDecisionPoint`.

### 4.3 Diretório/federação de entrada (009)
- **Scheduler** do sync incremental (`ldapsync.Syncer`), com credenciais resolvidas
  do cofre (`DirectoryConnectorProvisioner.ResolveCredential`).
- **Montar endpoints SCIM** (`http.SCIMUserHandler`/`SCIMGroupHandler`) no roteador,
  com `OrgResolver` (tenant do path/token) e `DirectoryProvisioner` como provisioner.
- **Endpoints ACS de federação** (SAML/OIDC) montados, com DI de
  `samlfed.Validator` (metadados do IdP, cert) e `oidcfed.Verifier` (JWKS do OP,
  issuer, client id); no sucesso, `DirectoryProvisioner.ProvisionFederated` (JIT).
- **Enqueue de concessão** quando o ativo canônico existir (liga com 007 M4).

### 4.4 Observabilidade/custódia (010)
- Ligar os adapters reais do cofre (seção 3) e o exportador OTel (seção 5).

---

## 5. Bloco de infra pendente do pacote 010 (8 tarefas → devops)

Todos os **seams já existem** no código; o devops liga os exportadores/config.

| Tarefa | O que fazer | Seam pronto |
|---|---|---|
| **T-001** | Instrumentar métricas OTel nos caminhos críticos + export OTLP | `domain.SLICatalog`, `metrics.MeasuredPDP`, `AuthzMetrics` |
| **T-002** | Tracing distribuído com propagação de contexto | OTel SDK já no `go.mod` (transitivo) |
| **T-003** | Logs estruturados com `trace_id` + **filtro de redação** no pipeline | `domain.RedactValue/RedactAttr` (T-004) |
| **T-007** | Dashboards Grafana versionados (SLIs) | catálogo de SLIs (T-006) |
| **T-008** | Alertas obrigatórios (alertmanager): falha de gravação/verificação de auditoria (**severidade máxima**), cofre/PDP indisponível, pico de falhas de auth, break-glass (informativo imediato) | `domain.Alerter`, `Severity` |
| **T-009** | Export de **cópia** da auditoria para Loki (marcada como cópia; original nunca sai da trilha) | trilha imutável (003) |
| **T-022** | Runbooks: DR do cofre, rotação de chaves, incidente, notificação ANPD/titulares | políticas de rotação (T-013/014), crypto-shredding (T-019) |
| **T-025** | Documentação de apoio ao RIPD (papéis controlador/operador LGPD) | classificação LGPD nas migrations (0006 + gate T-017) |

**Higiene obrigatória** (spec / I-3.2): nenhum segredo/token/PII em claro em nenhum
sinal — aplicar `domain.RedactAttr` na saída de logs/traces; usuário referenciado por
**pseudônimo** (`identity.Subject`), nunca e-mail. O teste `TestINV7Telemetry*`
(make invariants) já quebra o build se a redação enfraquecer.

---

## 6. Ordem sugerida no `archguard-devops`

1. **Postgres** (RLS + `roles.sql`) → migrations aplicadas (`internal/migrate`).
2. **OpenBao** (transit + KV + chaves por titular) → trocar os 4 adapters de custódia
   (seção 3); `/health` deve reportar **conformant**.
3. **Boot de login real** (seção 4.1) → destrava `/authorize` em homolog.
4. **OpenFGA/projeção** + scheduler do publisher/reconciler (007, seção 4.2).
5. **Schedulers/endpoints de diretório e federação** (009, seção 4.3).
6. **OTel + VictoriaMetrics/Loki/Tempo/Grafana + alertmanager** (010, seção 5).
7. **Runbooks + RIPD** (T-022/T-025).
8. **Docker Swarm + Traefik + TLS**; ensaiar DR/rotação.

---

## 7. Invariantes que acompanham cada peça (não relaxar)

- **INV-4**: OpenBao/OpenFGA sempre via HTTP, **nunca linkados** (fronteira MPL/GPL).
- **INV-6**: fail-closed — cofre/PDP/auditoria indisponível ⇒ **negação**; L3 nunca
  degrada (T-015).
- **INV-7**: segredos/chaves nunca no banco/log/telemetria — só referências ao cofre;
  redação obrigatória na telemetria.
- **INV-2**: auditoria append-only (papel de app sem UPDATE/DELETE; triggers).
- **I-3.3**: campo pessoal novo em migration exige classificação LGPD (gate T-017).

O corpus de governança (`CONSTITUTION.md`, `docs/adr/`, `docs/rfc/`, `openspec/`) e a
suíte `test/invariants/` são a autoridade — o `archguard-devops` consome os artefatos
(imagem, migrations, `roles.sql`, SBOM) sem reabrir decisões.
