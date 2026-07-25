# Proposal — 011 · Control Plane API (composition root + exposição pública)

## Por quê

Os pacotes 002–007 e 009 entregaram a arquitetura hexagonal do ArchGuard — identidade
multi-tenant, trilha imutável, acesso privilegiado, MFA, federação OIDC, autorização e
sincronismo de diretório — em `internal/domain/**`, `internal/adapters/**` e `internal/http/**`,
com cobertura de teste. Mas **nada disso está ligado ao servidor em execução**: o binário do
Beego só linka `internal/{domain,deploy,keystore,migrate}`. Os ~65 stores `postgres` e **toda a
camada `internal/http`** são código órfão — compilam, têm teste, e nenhuma rota os invoca. A API
viva continua sendo a do Casdoor legado (XORM).

O pacote 008 (console) consome **exclusivamente API pública versionada descrita em OpenAPI**
(ADR-0004, RFC-0005). Essa API não existe. O ADR-0004 antecipou exatamente esta consequência:
*"Toda lacuna da API pública vira trabalho de backend antes de virar tela."* Este pacote **é**
esse trabalho de backend — o *composition root* que torna a arquitetura já construída **viva** e
a expõe como contrato para o 008.

## O que muda

- **Composition root no repo principal**: uma camada de boot (adapter/infra, não domínio) que
  inicializa um `*pgxpool.Pool`, constrói os stores `postgres` **exigindo contexto de tenant**,
  e seleciona adapters por perfil de implantação (`internal/deploy`).
- **Montagem dos handlers `internal/http` no Beego** sob uma API pública versionada `/api/v1`,
  reusando o que já está implementado e testado — **sem reescrever** como controller novo.
- **Enforcement no wiring**: todo endpoint de domínio resolve tenant (INV-5), declara nível de
  garantia e responde erro de garantia insuficiente quando aplicável (INV-8), e nega quando uma
  dependência crítica (PDP, cofre, auditoria) está indisponível (INV-6, fail-closed).
- **Contrato OpenAPI** da API nova, versionado e verificável no CI — fonte da verdade única do
  cliente gerado do 008 (T-002).
- **Seleção de adapter por perfil**: dev = keystore local / provisional; perfil conforme exige
  o backend real (OpenBao) — cujo *endpoint/infra* permanece config do `archguard-devops`, mas
  o **seam de seleção** mora aqui.

## O que não muda

- **Nenhuma regra de domínio nova.** É wiring de código já existente e testado.
- **`internal/domain/**` permanece livre de framework/ORM** (INV-3): o composition root é a
  camada de infra, autorizada a importar Beego e pgx; o domínio, não.
- **Nenhum endpoint "só para a UI"** (I-7.6): a API é pública, versionada e documentada.
- **Sem fail-open, sem senha-mestra, sem custódia dev em produção** (INV-1, INV-6, INV-7).

## Impacto

- **Depende de:** 002, 003, 004, 005, 006, 007, 009 (todos completos).
- **Desbloqueia:** 008 (console) — que passa a ter API real e OpenAPI contra o qual gerar
  cliente; e 010 (observability) — instrumenta os caminhos que passam a existir em runtime.
- **Risco:** o wiring não pode enfraquecer invariante. Cada endpoint montado é avaliado contra
  INV-5 (predicado de tenant), INV-6 (fail-closed) e INV-8 (garantia). Mitigação: suíte de
  invariantes estendida para cobrir a camada montada, não só o domínio isolado.
