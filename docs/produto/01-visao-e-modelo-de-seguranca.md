# ArchGuard — Visão e Modelo de Segurança

> Documento de produto (âncora). Público: avaliadores de segurança, integradores, operadores e
> tomadores de decisão. Escrito para ser **honesto sobre o que está em produção hoje** vs. o que é
> projetado/pendente — num produto de segurança, uma documentação que descreve garantias inexistentes
> é passivo, não ativo.
>
> Status: **rascunho vivo** (2026-08-02). Fonte da verdade do comportamento é o código +
> `CONSTITUTION.md` + os ADR/RFC citados. Onde este documento e o código divergirem, o código vence e
> este documento deve ser corrigido.

## 1. O que é o ArchGuard

O ArchGuard é o **plano de controle de identidade** da plataforma **ArchGate** (PAM — Privileged
Access Management) da IntegrAllTech. Ele é a autoridade de **quem é** e **quem pode**: identidade,
credenciais, MFA, sessão, emissão/revogação de tokens, federação (OIDC) e trilha de auditoria.

É um **fork governado do Casdoor** (Go + Beego + XORM, Apache-2.0), com um núcleo novo (`internal/`)
construído sob disciplina própria (ver §6). O código herdado do Casdoor continua na árvore; o valor
do ArchGuard está no núcleo governado — e este documento distingue os dois com clareza.

### Fronteiras — o que o ArchGuard faz e não faz

| Faz (é autoridade) | **Não** faz (é de outro componente) |
|---|---|
| Identidade e multi-tenancy | Proxy de protocolo → **Warpgate** |
| Credenciais, MFA, step-up | Gravação de sessão → **Apache Guacamole** |
| Sessão, emissão/revogação de token | Rede de acesso → **NetBird** |
| Autorização granular (PDP) | Brokering/custódia de credencial de recurso → **OpenBao** |
| Federação OIDC, trilha de auditoria | |

Um ArchGuard que tentasse fazer proxy ou gravar sessão estaria fora de escopo por design. Ele
**decide e prova**; os componentes de dados executam.

## 2. Modelo de segurança — as garantias

O modelo é ancorado em **oito invariantes que quebram o build** (`test/invariants/`). Não são "boas
práticas": são condições de rejeição automática, verificadas por uma suíte que já pegou defeitos reais
em desenvolvimento. Para um avaliador, são as **promessas verificáveis** do produto.

| # | Garantia ao cliente | Mecanismo |
|---|---|---|
| **INV-1** | Ninguém autentica com credencial que não é sua. **Senha-mestra não existe.** | Sem caminho de bypass; auth só com a credencial do próprio dono |
| **INV-2** | A trilha de auditoria é **imutável** — nenhum `UPDATE`/`DELETE`, em nenhuma camada | Trigger anti-mutação no banco + `REVOKE` de privilégio + hash chain |
| **INV-3** | O domínio é **puro** — não acopla framework nem ORM | Regra de dependência de pacote (arquitetura hexagonal) |
| **INV-4** | Higiene de licença — **nada de AGPL/GPL/SSPL/BUSL** na árvore | SBOM + license gate no CI (ADR-0002/0019) |
| **INV-5** | **Isolamento entre tenants** — nenhuma query sem predicado de tenant | RLS no PostgreSQL + predicado explícito + teste estático |
| **INV-6** | **Não existe fail-open.** Falha de PDP, cofre ou auditoria ⇒ **negação** | Fail-closed por construção (ADR-0005) |
| **INV-7** | Segredos e chaves privadas **nunca** no banco nem em log/telemetria | Custódia (OpenBao/keystore selado) + redação de telemetria |
| **INV-8** | Toda operação declara seu **nível de garantia** (L1/L2/L3) | Catálogo de operações + gate no pipeline (ADR-0010) |

### 2.1 Fail-closed como postura padrão (INV-6)

Se o motor de decisão (PDP), o cofre ou a auditoria falham, a resposta é **negar** — nunca liberar. A
distinção entre `denied` (uma decisão) e `error` (uma falha) é preservada em toda a stack, porque a
auditoria depende dela. Num PAM, essa é a postura correta: preferir bloquear a operar cego.

### 2.2 Multi-tenancy com isolamento no banco (INV-5)

Cada organização é uma **fronteira de isolamento** com `id` estável e **Row-Level Security** no
PostgreSQL chaveada por `organization_id`. O isolamento não depende do código de aplicação lembrar de
filtrar: é imposto pelo banco e verificado por invariante. Leituras que cruzam tenants passam por uma
porta dedicada (`GlobalAuthorizer`), que é **fail-closed em perfil conforme** e carrega principal +
motivo, auditados de forma durável (ADR-0022).

### 2.3 Autorização granular — ReBAC com fonte da verdade no domínio

O acesso privilegiado é decidido por um **PDP** (Policy Decision Point) sobre um grafo de relações
estilo Zanzibar (`operator`/`auditor`/`owner`/`member`/`has_active_grant`, com herança por hierarquia
de ativos e por grupo). O grafo é **projeção derivada** da fonte da verdade (o modelo de identidade no
PostgreSQL), sincronizada por um **outbox transacional → publisher**, e **auto-curada** por um
reconciler periódico com política assimétrica (remove o obsoleto automaticamente; amplia só sob
revisão). Nenhuma decisão privilegiada usa cache. (Pacote 007, RFC-0004, ADR-0005.)

Origens de acesso reconhecidas e auditáveis: **direto** (owner/operator no ativo), **herdado** (de um
grupo de ativos ancestral), **via grupo** (o membro herda do grupo) e **concessão** (um grant
privilegiado vigente, com janela de validade que expira no próprio grafo).

### 2.4 Níveis de garantia por operação (INV-8, ADR-0010)

Toda operação da API declara **L1/L2/L3**. O pipeline resolve a sessão e **impõe** o nível: uma
operação L3 exige step-up reforçado (WebAuthn) antes de executar. Esconder um botão no frontend não é
controle de acesso — o controle é a API versionada, com o nível declarado e imposto no servidor.

### 2.5 Trilha de auditoria imutável (INV-2)

A auditoria é **append-only**, particionada, encadeada por hash **por tenant**, com três barreiras
independentes: trigger no banco, `REVOKE` de privilégio e a cadeia de hash que torna adulteração
detectável. Uma operação cujo evento de auditoria não pôde ser gravado **não acontece** (I-5.4).

## 3. Estado de maturidade (honesto)

Distinguir **fundação** de **produto endurecido** é parte da honestidade de um produto de segurança.

### O que está no ar e validado em produção
- Plano de controle `/api/v1` (identidade, sessão, tenants, saúde, concessões, auditoria).
- **Autorização granular completa** (pacote 007): projeção, publisher, reconciler, ciclo de vida,
  cascade, revisão de acesso com origem — validada ao vivo no piloto.
- Multi-tenancy com RLS; trilha de auditoria imutável; `GlobalAuthorizer` fail-closed (ADR-0022).
- Auto-atualização com sessão persistente (Redis).

### O que é projetado/pendente (não confie como se estivesse pronto)
- **Contrato OIDC v1 (claims `org/mid/acr/sid/pcid`, rotação de refresh, detecção de reuso):**
  implementado e testado, **mas não montado** — quem serve OAuth hoje é o Casdoor legado (claims
  stock `name`+`groups`). Montagem projetada e adiada em **ADR-0023**.
- **Postura de produção:** o piloto roda com **custódia de chave em modo *dev*** (keystore local
  selado); a custódia conforme (OpenBao) é suportada mas não ativa no piloto.
- **Observabilidade (pacote 010):** desenhada, não executada.
- **Fluxos L3 ponta a ponta** (passkey do admin, verificar cadeia, break-glass) não todos exercitados
  em produção; **escala** não validada sob carga.

O roadmap de endurecimento (de fundação a produto) é mantido à parte; este documento apenas declara o
estado com honestidade.

## 4. Superfície herdada do Casdoor (o que isto NÃO garante)

O ArchGuard herda uma superfície ampla do Casdoor (Beego/XORM) que **não** está sob as garantias do
núcleo governado e é, hoje, quem serve OAuth. Um avaliador deve tratar a superfície legada como
**não-auditada** até o encolhimento planejado pós-montagem do OIDC v1. A divergência estrutural é
rastreada em `docs/upstream/DIVERGENCE.md` (rastreamento em evolução).

## 5. Stack de confiança

PostgreSQL 15+ (com RLS) como fonte da verdade; PDP sobre grafo de relações; OpenBao para custódia
(MPL-2.0, **nunca linkado** — via HTTP); OpenTelemetry para observabilidade; deploy em Docker Swarm +
Traefik com **TLS obrigatório**.

## 6. Governança (por que confiar no processo, não só no código)

O ArchGuard é desenvolvido sob **Spec-Driven Development**: uma constituição de invariantes
(`CONSTITUTION.md`), decisões arquiteturais rastreáveis (ADR), especificações de comportamento (RFC e
OpenSpec com cenários WHEN/THEN como critério de aceite), e a suíte de invariantes que **quebra o
build** quando uma garantia é violada. Cada tarefa passa por um gate completo (lint, testes,
invariantes, dependências, SBOM/licenças, build) antes de ser considerada pronta. Para um produto de
segurança, o processo é parte do produto.

---

*Próximos documentos de produto: Guia do Operador/Runbooks, Referência de Integração/API, Guia do
Administrador. Ver o índice em `docs/produto/`.*
