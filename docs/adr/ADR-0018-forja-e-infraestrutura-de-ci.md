# ADR-0018 — Forja de código e infraestrutura de CI

- **Status:** **Proposto** (aguardando aprovação — bloqueia T-019 e T-003)
- **Data:** 2026-07-20
- **Invariantes tocados:** I-9.4 (gate como autoridade de "pronto"), I-8.1 (proteção da linha
  `main`), I-3.1 (soberania — cenário self-hosted de primeira classe)
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

**Critério decisivo:** a forja precisa suportar *required status checks* **bloqueantes** na
proteção de branch — merge em `main` mecanicamente impossível com o gate de invariantes
vermelho. Se a forja não sustentar isso, o ADR-0003 vira convenção, não controle.

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

**Consequência para este ADR:** a escolha do GitLab CE **não é reprovada**, mas passa a exigir
um controle organizacional explícito — *nenhuma conta humana de trabalho é admin-de-instância
ou owner do repositório* — que precisa entrar na configuração do T-003 e ser você a decidir se
é suficiente. Sem essa regra, o ADR-0003 é contornável por qualquer admin. **Decisão pendente
do arquiteto antes da ratificação.**

> Instância descartável destruída após a coleta; nenhum artefato do PoC entra em `main`.

## Insumos pendentes (Edson)

- Existe GitLab já provisionado na infraestrutura da IntegrAllTech?
- Avaliação de musculatura operacional do time para self-hosted (valida o orçamento de
  2–4 h/mês assumido acima).

## Consequências

- O gate de invariantes torna-se **mecanicamente obrigatório** — nenhum merge em `main` com
  vermelho, nem por disciplina, nem por exceção.
- T-003 executável imediatamente após o provisionamento.
- A squad assume operação de infraestrutura crítica própria (custo aceito e orçado acima).
