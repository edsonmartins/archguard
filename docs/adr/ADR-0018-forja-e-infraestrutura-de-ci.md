# ADR-0018 — Forja de código e infraestrutura de CI

- **Status:** **Proposto — submetido à ratificação** (bloqueia T-019b e T-003)
- **Data:** 2026-07-20
- **Invariantes tocados:** I-9.4 (gate como autoridade de "pronto"), I-8.1 (proteção da linha
  `main`), ADR-0008 (tier admin como break-glass), ADR-0007/RFC-0003 (detecção de proveniência
  análoga à cadeia de auditoria)
- **Escopo:** infraestrutura. A forja **não** é dependência de árvore de build — a matriz de
  licenças do ADR-0002 não se aplica aqui (ver ADR-0002 §3a).

## Contexto

O repositório do ArchGuard existe apenas localmente. Três tarefas do pacote 001 dependem de
uma forja:

- **T-003** — proteção de `main`: o ADR-0003 exige que `vendor/upstream` seja espelho
  somente-leitura e que `main` receba mudanças apenas por fluxo controlado.
- **T-019b** — o gate de CI (lint, testes, invariantes, regra de dependência, SBOM, license
  gate) vira **status check obrigatório**. O CONSTITUTION I-9.4 faz do gate a única autoridade
  sobre "pronto"; a forja é onde essa autoridade vira mecanismo.
- **T-020** — registry de imagens de container.

**Critério decisivo:** exigir "merge mecanicamente impossível inclusive para admin" é
**incoerente como requisito absoluto** — nenhuma forja pode vincular o próprio superadmin
(GitHub org owner altera o ruleset, GitLab admin idem, Gitea admin idem). Quem controla a forja
controla as regras; isso é **propriedade estrutural de forja**, não defeito de um produto. O
critério correto tem duas metades:

1. O gate é **mecanicamente impossível de contornar para todo papel de trabalho humano**: push
   direto a `main` proibido, merge exige status check verde.
2. **Todo contorno exercido pelo tier administrativo é detectável e alertado** — nunca
   silencioso.

A prevenção resolve a metade 1; a **detecção** resolve a metade 2. É o mesmo padrão que o
projeto já adotou duas vezes: ADR-0007 não impede o DBA de editar a auditoria (hash-chain torna
inegável); ADR-0019 não proíbe MPL linkado (detecta a transição para modificado). Aqui: não se
impede o push do admin — detecta-se e alerta. Coerência de produto: privilégio administrativo é
break-glass auditado (ADR-0008), não conta de trabalho; o ArchGuard aplica ao próprio SDLC o
modelo que vende.

## Decisão

**GitHub privado (organização IntegrAllTech).**

**Motivo decisivo, e é o único que importa:** a forja **já está provisionada** e a equipe **já
tem musculatura operacional** nela. Provisionar infraestrutura nova só para satisfazer um ADR
inverte a ordem — o ADR registra a decisão, não se justifica a si mesmo.

Configuração (detalhada nos "Controles do T-003"):
- **Ruleset** em `main`: PR obrigatório, aprovações mínimas, *required status checks*
  bloqueantes apontando para o job do gate, `Require branches to be up to date`, force-push e
  deleção bloqueados, `Require signed commits`.
- **GitHub Actions** executando o gate do CLAUDE.md §5 em todo PR e em `main`.
- **GitHub Container Registry (GHCR)** para as imagens do T-020.
- Papéis de trabalho humano: **Write**. Admin da organização **não** é conta de trabalho.

## Correção do raciocínio anterior (a análise errada é parte da decisão)

A recomendação anterior deste ADR era **GitLab CE self-hosted**. Está **rejeitada**. O registro
do erro fica porque a análise equivocada é parte da decisão:

- **"Fornecedor estrangeiro" NÃO decorre dos invariantes.** I-3.1/I-3.2 governam **dados de
  cliente** (identidades, credenciais, trilha de auditoria). Código-fonte **não** é dado de
  cliente, e este derivado vem de um projeto Apache-2.0 **já público**. Confidencialidade de IP
  e soberania de dados são preocupações distintas — foram indevidamente coladas na versão
  anterior. A soberania de dados (I-3.1) é satisfeita pelo **deployment on-premises do produto**
  no cliente, não pela localização da forja de desenvolvimento.
- **No critério decisivo (status check bloqueante), GitHub rulesets equivalem ao GitLab CE.** O
  bypass de admin existe igualmente — o PoC provou que é propriedade estrutural de forja, não
  defeito de produto.
- **Erro de processo:** apliquei a uma decisão **reversível** o rigor devido a decisões
  irreversíveis. Trocar de forja é barato (histórico git é portátil; a config de CI são poucos
  arquivos e o gate é um Makefile que a forja apenas invoca). Não merecia PoC nem bloqueio de
  duas tarefas. O fork point (ADR-0002) é irreversível e mereceu esse rigor; a forja não.

O que **se aproveita** do PoC de GitLab CE: a evidência de que **nenhuma forja mainstream barra
o próprio superadmin sem tier pago**. Ela sustenta a seção "Risco aceito" no GitHub por
**demonstração**, não por presunção — é o resultado durável do experimento.

## Alternativas consideradas

| Opção | Status | Motivo |
|---|---|---|
| **GitHub privado (org IntegrAllTech)** | **ESCOLHIDA** | Já provisionada, equipe com musculatura operacional; rulesets sustentam status check bloqueante; GHCR embutido para T-020; Actions maduro |
| **GitLab CE self-hosted** | **REJEITADA** (era a recomendação anterior) | Exigiria **provisionar e operar infraestrutura nova** (host, storage, backup, ~2–4 h/mês) para uma capacidade que a forja já em uso oferece. A objeção anterior de "soberania" não decorre dos invariantes (ver correção acima). Semântica de proteção equivalente ao GitHub — nenhuma vantagem que compense o custo operacional novo |
| **Gitea self-hosted** | Rejeitada | CI (Gitea Actions) imaturo demais para sustentar gate de segurança bloqueante; mesmo custo de infra nova |
| **GitLab.com (SaaS)** | Rejeitada | Sem vantagem sobre o GitHub já em uso; migração sem motivo |

## Teste de aceitação — duas perguntas, dois gates

**(i) A ferramenta consegue tornar o merge impossível para papéis de trabalho, e é o contorno
de admin uma propriedade estrutural?** — **respondida** pelo PoC de GitLab CE (evidência
abaixo). O resultado é agnóstico de forja: prevenção mecânica funciona para papéis de trabalho;
o superadmin contorna em qualquer forja mainstream. Isso **ratifica o desenho** (prevenção +
detecção), não uma ferramenta específica.

**(ii) A NOSSA configuração no GitHub bloqueia de fato?** — só se prova na organização
definitiva. É esta evidência que **fecha o T-003**, com gate próprio (provas empíricas abaixo).

**Prova pendente — conta Write não-admin (CONDIÇÃO BLOQUEANTE do T-003).** A inferência por
monotonicidade de papéis **não** é aceita como prova final: todo o argumento de segurança
repousa em "os papéis que humanos usam estão bloqueados", e o padrão é demonstração. No GitHub:
uma conta com papel **Write** não consegue (a) push direto a `main`, (b) merge com check
vermelho, (c) force-push. Evidência anexada ao commit do T-003; se falhar, o T-003 não fecha e
este ADR volta a Proposto.

### Evidência do PoC — GitLab CE em instância descartável (2026-07-20)

Conduzido enquanto a recomendação ainda era GitLab CE. Mantido porque seu resultado é durável e
**agnóstico de forja**: demonstra que o contorno de admin é estrutural. Ambiente:
`gitlab/gitlab-ce` (arm64) em runtime de container local; `main` protegido
(`push_access_level=No one`, `merge_access_level=Maintainer`, `allow_force_push=false`,
`only_allow_merge_if_pipeline_succeeds=true`); MR reintroduzindo a senha-mestra com status de
gate `failed`. A suíte de invariantes foi verificada falhando de verdade nessa branch (INV-1
FAIL).

| # | Ação | Ator | Resultado | Veredito |
|---|---|---|---|---|
| 1 | Merge do MR via API, gate vermelho | root (**admin + owner**) | **HTTP 405** *Method Not Allowed* | ✅ bloqueado |
| 2 | Merge com flags `skip_ci=true`, `merge_when_pipeline_succeeds=false` | root | **HTTP 405** em todas | ✅ sem bypass por flag |
| 3 | Force-push **não-fast-forward** a `main` | root (admin+owner) | *pre-receive hook declined* | ✅ bloqueado |
| 4 | Push **fast-forward** direto a `main` (push=No one) | root (**admin + owner**) | **SUCESSO** — `main` avançou para a violação | ❌ **bypass** |

O resultado 4 é o achado durável: um administrador contorna a proteção de branch por push
direto. É **comportamento estrutural de forja** (no GitHub, o análogo é o org owner alterar o
ruleset ou constar em `bypass actors`), não defeito do GitLab CE. As recusas de merge (405) e de
force-push valem inclusive para o superadmin; o que nenhuma forja mainstream impede sem tier
pago é o push do próprio superadmin.

> Instância descartável destruída após a coleta; nenhum artefato do PoC entra em `main`.

## Controles a implementar no T-003 (GitHub)

**Prevenção (papéis de trabalho humano):**
- **Ruleset** em `main`: PR obrigatório; aprovações mínimas; *required status checks*
  **bloqueantes** apontando para o job do gate; `Require branches to be up to date`.
- **Bloquear force-push e deleção** de `main`.
- **`Require signed commits`** no ruleset (cobre o item de commits assinados).
- Papéis de trabalho = **Write**. Admin da organização **não** é conta de trabalho — mesma
  regra organizacional (break-glass, ADR-0008).
- `vendor/upstream` protegido como somente-leitura (push restrito ao fluxo de espelho).

**Detecção (agnóstica de forja):**
- **(a) Verificador de proveniência de `main`** — job agendado + on-push que confirma que todo
  commit alcançável de `main` provém de PR mergeado com gate verde. Commit sem PR
  correspondente ⇒ **alerta de severidade máxima**. É o verificador de cadeia do RFC-0003
  aplicado ao histórico do repositório, não à trilha de eventos.
- **(b) Audit log da organização** para ações administrativas **e alterações de ruleset** ⇒
  alerta imediato em canal de segurança.
- **(c) `bypass actors` do ruleset VAZIO e monitorado** — é o análogo direto do bypass de
  admin. Lista vazia; qualquer alteração nela é **evento de segurança**, não configuração de
  rotina.
- **(d)** Alerta **imediato** em canal de segurança, não e-mail de relatório.

## Risco aceito

O admin da organização pode alterar ou contornar o ruleset (ex.: adicionar-se a `bypass
actors`, ou push direto por poder administrativo). **Não é mitigável por configuração, em
nenhuma forja mainstream sem tier pago** — demonstrado no PoC de GitLab CE (resultado 4). O
risco é **aceito** e compensado por: (i) segregação organizacional — nenhuma conta humana de
trabalho no tier admin; (ii) `bypass actors` vazio e monitorado; (iii) verificador de
proveniência de `main` com alerta; (iv) audit log com alerta imediato. Reavaliar se houver
aquisição de tier pago (GitHub Enterprise) ou mudança de forja.

**O vetor dominante não é o admin malicioso — é o admin distraído:** trabalho de rotina com
conta privilegiada e push por memória muscular. É ordens de magnitude mais provável que
sabotagem, e é exatamente o que a segregação organizacional previne.

## Critérios de aceite bloqueantes do T-003

1. Conta com papel **Write** não consegue: push direto a `main`, merge com check vermelho,
   force-push. **Evidência empírica anexada** (a prova pendente do PoC executa-se aqui).
2. **Alteração de ruleset** gera evento no audit log e dispara alerta — demonstrado.
3. **`bypass actors` vazio**, verificado.

Falhou qualquer um ⇒ o T-003 não fecha.

## Backup e DR do repositório

- Histórico git é distribuído: cada clone é uma cópia. **`git bundle` diário** de `main` e
  `vendor/upstream` para storage independente do GitHub.
- Metadados de PR/CI e releases dependem do GitHub; export periódico via API para o mesmo
  storage.
- RPO ≤ 24 h; o desenvolvimento local segue possível durante indisponibilidade da forja.

## Reversibilidade

**Barata** (esta é a correção central de processo): o histórico git é portátil e o gate é um
Makefile que qualquer forja invoca. Migrar de forja custa reescrever os workflows de CI (poucos
arquivos) e reconfigurar proteção de branch — não há lock-in estrutural. Foi por subestimar
essa reversibilidade ao contrário (tratá-la como cara) que a decisão anterior recebeu rigor
excessivo.

## Ratificação

Assinatura única do arquiteto — decisão técnica de infraestrutura, não toca invariante pétreo.
**A ressalva de custo operacional caiu:** não há provisionamento novo (a forja já está em uso),
logo não há decisão de custo para o sócio.

| Papel | Nome | Data | Ratificação |
|---|---|---|---|
| Arquiteto de Software e Soluções | Edson Martins | ______ | ☐ |

As provas do item "Critérios de aceite bloqueantes" são aceite do **T-003**, não da ratificação
deste ADR.

## Consequências

- O gate torna-se **mecanicamente obrigatório** para os papéis de trabalho — nenhum merge em
  `main` com check vermelho, nem por disciplina, nem por exceção.
- Contorno pelo tier admin é **possível mas detectável e alertado** (não silencioso).
- T-003 executável imediatamente (forja já provisionada), com aceite bloqueante das três provas.
- Sem custo de infraestrutura nova; sem operação de forja própria a assumir.
