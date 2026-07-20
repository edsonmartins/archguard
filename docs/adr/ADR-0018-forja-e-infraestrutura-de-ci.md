# ADR-0018 — Forja de código e infraestrutura de CI

- **Status:** **Proposto — submetido à ratificação** (bloqueia T-019 e T-003)
- **Data:** 2026-07-20
- **Invariantes tocados:** I-9.4 (gate como autoridade de "pronto"), I-8.1 (proteção da linha
  `main`), I-3.1 (soberania — cenário self-hosted de primeira classe), ADR-0008 (tier admin
  como break-glass), ADR-0007/RFC-0003 (detecção de proveniência análoga à cadeia de auditoria)
- **Escopo:** infraestrutura. A forja **não** é dependência de árvore de build — a matriz de
  licenças do ADR-0002 não se aplica aqui (ver ADR-0002 §3a).

## Contexto

O repositório do ArchGuard existe apenas localmente. Três tarefas do pacote 001 dependem de
uma forja:

- **T-003** — proteção de `main`: o ADR-0003 exige que `vendor/upstream` seja espelho
  somente-leitura e que `main` receba mudanças apenas por fluxo controlado.
- **T-019** — pipeline de CI com gate **bloqueante** (lint, testes, invariantes, regra de
  dependência, SBOM, license gate). O CONSTITUTION I-9.4 faz do gate a única autoridade sobre
  "pronto"; o CI é onde essa autoridade vira mecanismo.
- **T-020** — registry de imagens de container.

**Critério decisivo (corrigido em 2026-07-20):** exigir "merge mecanicamente impossível
inclusive para admin" é **incoerente como requisito absoluto** — nenhuma forja pode vincular
o próprio superadmin (GitHub org owner altera o ruleset, Gitea admin idem). Quem controla a
forja controla as regras; isso é propriedade estrutural, não limitação do GitLab CE. O
critério correto tem duas metades:

1. O gate é **mecanicamente impossível de contornar para todo papel de trabalho humano**
   (Developer, Maintainer): push a `main` = "No one", merge exige pipeline verde.
2. **Todo contorno exercido pelo tier administrativo é detectável e alertado** — nunca
   silencioso.

A prevenção resolve a metade 1; a **detecção** resolve a metade 2. É o mesmo padrão que o
projeto já adotou duas vezes: ADR-0007 não impede o DBA de editar a auditoria (hash-chain
torna inegável); ADR-0019 não proíbe MPL linkado (detecta a transição para modificado). Aqui:
não se impede o push do admin — detecta-se e alerta. Coerência de produto: privilégio
administrativo é break-glass auditado (ADR-0008), não conta de trabalho; o ArchGuard aplica ao
próprio SDLC o modelo que vende.

Critérios secundários: registry de container embutido (T-020); execução de runners
self-hosted; maturidade do CI para sustentar gate de segurança; soberania (I-3.1 — o fonte de
um produto PAM é ativo sensível); custo operacional para uma squad pequena.

## Decisão (proposta)

**GitLab CE (Community Edition) self-hosted**, operado pela IntegrAllTech, com:

- **Branch protection** em `main`: push direto proibido; merge request obrigatório; *required
  status checks* (pipeline do gate completo) bloqueantes; força-push proibido.
- `vendor/upstream` como branch protegida somente-leitura (push restrito ao fluxo de espelho).
- **GitLab CI** executando o gate do CLAUDE.md §5 em todo MR e em `main`; nenhuma etapa
  pulável por flag (ADR-0002 §4).
- **Container Registry embutido** para as imagens do T-020.
- **Runners self-hosted** dedicados, sem executores compartilhados de terceiros.

## Alternativas consideradas

| Opção | Por que não |
|---|---|
| **Gitea self-hosted** | Mais leve de operar, porém o CI (Gitea Actions) é imaturo demais para sustentar um gate de segurança bloqueante como mecanismo central; ecossistema de runners e cache instável. Reavaliável em rebase de major se o CI amadurecer |
| **GitHub privado** | Caminho mais rápido e melhor ecossistema de Actions; porém coloca o código-fonte de um produto PAM sob fornecedor estrangeiro, fora do perímetro da IntegrAllTech. **Defensável como escolha consciente; inaceitável como default.** Se os custos operacionais do self-hosted se provarem impeditivos, a migração exige revisão deste ADR com essa troca explicitada |
| **GitLab.com (SaaS)** | Mesma semântica de proteção, sem custo de operação; mas mesma objeção de soberania do GitHub privado, com ecossistema menor |

## Custo operacional assumido (self-hosted)

- Provisionamento: 1 VM/host para GitLab CE + 1–2 runners (Docker), dentro da infraestrutura
  existente da IntegrAllTech.
- Operação recorrente estimada: **2–4 h/mês** — atualizações mensais de segurança do GitLab
  (janela de manutenção), monitoramento de disco/backup, upgrade de runners.
- Este custo é aceito como o preço da soberania sobre o fonte (I-3.1). Estouro recorrente
  desse orçamento é gatilho de reavaliação (ver Reversibilidade).

## Backup e DR do repositório

- **Backup nativo diário** do GitLab (repositórios + metadados de MR/issues + registry) com
  retenção de 30 dias, cópia **offsite** cifrada.
- **`git bundle` diário** de `main` e `vendor/upstream` para storage independente do GitLab —
  o histórico git sobrevive mesmo à perda total da forja e dos seus backups.
- **Teste de restauração trimestral** documentado em runbook (T-024 cobre o runbook inicial).
- RPO ≤ 24 h; RTO ≤ 1 dia útil para a forja (o desenvolvimento local segue possível nesse
  intervalo — git é distribuído; a perda máxima é de metadados de MR/CI).

## Reversibilidade

Cara após acumular histórico de CI e releases (pipelines, registry, MRs) — por isso a decisão
é tomada por ADR antes do T-019. Gatilhos de reavaliação: custo operacional recorrente acima
do orçado; vulnerabilidade estrutural no GitLab CE sem correção tempestiva; mudança de
licenciamento do GitLab CE que afete o uso. Migração de saída: espelho git é trivial;
pipelines exigem reescrita (contida — o gate é um Makefile, o CI apenas o invoca).

## Teste de aceitação *(fixado em 2026-07-20; dividido em duas partes em 2026-07-20)*

Forja não se ratifica por documentação de fornecedor; ratifica-se por **demonstração**. O
teste responde a duas perguntas distintas, com gates distintos:

**(i) A ferramenta consegue tornar o merge mecanicamente impossível?** — provada em
**instância descartável**. É esta evidência que **ratifica o ADR-0018**:

1. Um MR aberto com o gate de invariantes **vermelho**;
2. Prova de que o merge é **mecanicamente impossível** — inclusive para quem tem permissão de
   mantenedor;
3. Prova da ausência de qualquer caminho de contorno: override de mantenedor, force-push e
   bypass de proteção desabilitados ou inexistentes.

Qualquer caminho de contorno reprova a escolha, **seja qual for a ferramenta**: sem isso o
ADR-0003 é convenção, não controle.

**(ii) A NOSSA configuração na forja definitiva bloqueia de fato?** — só se prova na
instância definitiva, após o provisionamento. É esta evidência que **fecha o T-003**, que
tem gate próprio: repetição das provas 1–3 na configuração real, anexada ao commit do T-003.

Produzir a evidência apenas na forja definitiva foi rejeitado como via única: ratificar o ADR
depois de já ter migrado inverteria a ordem e esvaziaria o teste.

**Prova pendente — Maintainer não-admin (CONDIÇÃO BLOQUEANTE do T-003).** A inferência por
monotonicidade de papéis **não** é aceita como prova final: todo o argumento de segurança
repousa em "os papéis que humanos usam estão bloqueados", e o padrão fixado é demonstração,
não documentação. Motivo instrumental pelo qual não foi fechada no PoC: falha de formato de
token do usuário de teste na instância descartável — **não** resultado de segurança. Critério
de aceite bloqueante do T-003, evidência anexada: um Maintainer não-admin **não** consegue
(a) push a `main`, (b) merge com gate vermelho, (c) force-push. Se falhar na forja definitiva,
o T-003 não fecha e este ADR volta a Proposto.

### Evidência (i) — demonstração em instância descartável (2026-07-20)

Ambiente: `gitlab/gitlab-ce` (arm64) em runtime de container local; projeto com `main`
protegido (`push_access_level=No one`, `merge_access_level=Maintainer`,
`allow_force_push=false`) e `only_allow_merge_if_pipeline_succeeds=true`. MR
`poc/violation → main` reintroduzindo a senha-mestra, com status de gate **`failed`** no
HEAD. A suíte de invariantes foi verificada localmente falhando de verdade nessa branch
(INV-1 FAIL).

| # | Ação | Ator | Resultado | Veredito |
|---|---|---|---|---|
| 1 | Merge do MR via API, gate vermelho | root (**admin + owner**) | **HTTP 405** *Method Not Allowed* | ✅ bloqueado |
| 2 | Merge via API com flags `skip_ci=true`, `merge_when_pipeline_succeeds=false` | root | **HTTP 405** em todas | ✅ sem bypass por flag |
| 3 | Force-push **não-fast-forward** a `main` | root (admin+owner) | *pre-receive hook declined* | ✅ bloqueado |
| 4 | Push **fast-forward** direto a `main` (push=No one) | root (**admin + owner**) | **SUCESSO** — `main` avançou para o commit da violação | ❌ **bypass** |

**Resultado 4 é uma ressalva material, não uma aprovação limpa.** Um **administrador de
instância que também é owner** do projeto contorna o portão por push fast-forward direto,
mesmo com push restrito a "No one". A recusa de force-push (não-ff) e a recusa de merge por
API valem inclusive para o root; o que o GitLab CE **não** impede mecanicamente é o push de
um admin/owner de instância — comportamento documentado do CE (administradores têm poderes
amplos; *security policies* que restringem admins são recurso Ultimate).

**Leitura para a decisão:**

- O controle é **mecânico para os papéis que humanos usam no dia a dia** (Developer /
  Maintainer): push a `main` é "No one", merge exige pipeline verde. *(O teste com um
  Maintainer não-admin não foi fechado empiricamente nesta rodada — falha de formato de token
  no PoC, não resultado de segurança; a inferência acima decorre da monotonicidade de papéis
  do GitLab e de `push_access_level=No one`.)*
- O bypass existe **apenas para admin-de-instância + owner** — tier que, pela própria
  filosofia do ArchGuard, deve ser **minimizado e tratado como break-glass auditado**
  (ADR-0008), não concedido a contas de trabalho.

**Consequência para este ADR (decidida em 2026-07-20):** a escolha do GitLab CE **é mantida**
sob o critério corrigido — o resultado 4 recai inteiramente no tier admin/owner, que a regra
organizacional abaixo retira das contas de trabalho e a camada de detecção torna não
silencioso. A prevenção da metade 1 do critério está demonstrada (linhas 1–3); a detecção da
metade 2 é o que o T-003 implementa.

> Instância descartável destruída após a coleta; nenhum artefato do PoC entra em `main`.

## Controles a implementar no T-003

**Regra organizacional (configuração + documento):**
- Nenhuma conta humana de trabalho é admin de instância nem owner do repositório.
- Papéis de trabalho: Developer e Maintainer, com push a `main` = "No one" e merge exigindo
  pipeline verde.
- Credencial admin é **break-glass**: uso justificado, temporário e auditado — aplicação do
  ADR-0008, não exceção a ele.

**Detecção (o controle que substitui a prevenção impossível):**
- **(a) Verificador de proveniência de `main`** — job agendado + on-push que confirma que todo
  commit alcançável de `main` provém de MR mergeado com gate verde. Commit sem MR
  correspondente ⇒ **alerta de severidade máxima**. É o verificador de cadeia do RFC-0003
  aplicado ao histórico do repositório, não à trilha de eventos.
- **(b) Audit events** do GitLab para ações do tier admin ⇒ alerta em canal de segurança.
- **(c) Commits assinados** em `main`, com verificação de assinatura no mesmo job — sem isso a
  proveniência é afirmação, não prova.
- **(d)** Alerta **imediato** em canal de segurança, não e-mail de relatório.

**Admin Mode (mitigação de primeira linha):** o `admin_mode` (application setting) **está
disponível no GitLab CE** (introduzido no GitLab 12.x) e exige reautenticação explícita para
exercer poderes administrativos — é step-up (ADR-0010) aplicado à forja. **Habilitar** e
evidenciar empiricamente na forja definitiva (T-003) — evidência **obrigatória**, ao lado da
prova do Maintainer: habilitado, funcionando, e o toggle demonstrado. Reduz o vetor do "admin
distraído" (abaixo) ao forçar um ato consciente antes de qualquer ação privilegiada.

## Risco aceito

O tier admin/owner do GitLab CE contorna a proteção de branch por push fast-forward direto a
`main`. **Não é mitigável por configuração no CE**, e nenhuma forja self-hosted mainstream
barra o próprio superadmin sem tier pago. O risco é **aceito** e compensado por: (i)
segregação organizacional — nenhuma conta humana de trabalho no tier admin; (ii) detecção de
proveniência de `main` com alerta; (iii) auditoria de ações administrativas; (iv) Admin Mode.
Reavaliar se houver aquisição de tier pago ou mudança de forja.

**O vetor dominante não é o admin malicioso — é o admin distraído:** trabalho de rotina com
conta privilegiada e push por memória muscular. É ordens de magnitude mais provável que
sabotagem, e é exatamente o que a segregação organizacional previne (e o Admin Mode dificulta).

## Insumos pendentes (Edson)

- Existe GitLab já provisionado na infraestrutura da IntegrAllTech?
- Avaliação de musculatura operacional do time para self-hosted (valida o orçamento de
  2–4 h/mês assumido acima).

## Consequências

- O gate de invariantes torna-se **mecanicamente obrigatório** para os papéis de trabalho —
  nenhum merge em `main` com vermelho, nem por disciplina, nem por exceção.
- Contorno pelo tier admin/owner é **possível mas detectável e alertado** (não silencioso).
- T-003 executável imediatamente após o provisionamento, com aceite bloqueante do teste do
  Maintainer.
- A squad assume operação de infraestrutura crítica própria (custo aceito e orçado acima).

## Custo operacional e aval de sócio

Se o caminho escolhido for GitLab CE self-hosted novo (não uma instância já existente), há
**custo operacional recorrente**: 1 host para GitLab CE + 1–2 runners, storage com backup
offsite cifrado, e ~2–4 h/mês de manutenção (upgrades mensais de segurança, monitoramento,
teste de restauração). Ordem de grandeza: uma VM média + storage — dentro da infraestrutura
existente da IntegrAllTech.

**O aval de custo é do sócio (Neimar), distinto da ratificação técnica (Edson):** o Neimar tem
legitimidade sobre o **custo** (máquina, storage, backup, manutenção), não sobre a
arquitetura. Se o caminho for reutilizar GitLab já provisionado, o custo marginal é próximo de
zero e este aval é dispensável.

## Ratificação

Decisão de infraestrutura, cara de reverter após histórico de CI e releases. Ratificação:

| Papel | Nome | Escopo | Data | Ratificação |
|---|---|---|---|---|
| Arquiteto de Software e Soluções | Edson Martins | Técnico (arquitetura) | ______ | ☐ |
| Sócio-fundador | Neimar Chagas | Custo operacional (só se forja nova self-hosted) | ______ | ☐ |

Condicionada à resposta dos **insumos pendentes** acima (GitLab existente? musculatura
operacional). A prova (ii) e o **teste do Maintainer** são aceite bloqueante do T-003, não
da ratificação deste ADR.
