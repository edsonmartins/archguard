# Prompts de execução — Claude Code · ArchGuard

Três prompts: **arranque** (uma vez), **sessão de pacote** (reutilizável) e **triagem de
upstream** (recorrente). Cole o texto dentro do bloco, sem os cabeçalhos deste arquivo.

---

## A. Prompt de arranque — executar uma única vez

> **Pré-requisito:** o repositório deve conter `CONSTITUTION.md`, `CLAUDE.md`, `docs/adr/`,
> `docs/rfc/` e `openspec/changes/`, com o fork do Casdoor já clonado como base.

```
Você vai implementar o ArchGuard: o plano de controle de identidade da plataforma ArchGate
(PAM) da IntegrAllTech, construído como fork direto do Casdoor (Go + Beego + XORM,
Apache License 2.0).

Este é software de segurança. Um defeito aqui é acesso privilegiado indevido à produção de um
cliente, ou uma trilha de auditoria que não prova nada. Trate cada decisão com esse peso.

## Antes de qualquer código

Leia, nesta ordem, e me devolva um resumo do que entendeu:

1. CLAUDE.md — como trabalhar neste repositório (prevalece sobre instruções de sessão)
2. CONSTITUTION.md — invariantes; autoridade máxima, seções 2, 3 e 4 são pétreas
3. README.md — índice do corpus, grafo de dependências e fases
4. docs/adr/ADR-0001, ADR-0002, ADR-0003, ADR-0009, ADR-0015, ADR-0016
5. docs/rfc/RFC-0001 (arquitetura de referência)
6. openspec/changes/001-bootstrap-fork/ — proposal.md, design.md, specs/fork-baseline/spec.md,
   tasks.md

No resumo, responda explicitamente:
- Quais são os 8 invariantes que quebram o build e por que cada um existe
- Por que a estratégia é cherry-pick seletivo e nunca merge de branch do upstream
- Qual é o papel da suíte de invariantes na estratégia de fork
- O que está fora do escopo do ArchGuard (o que pertence a Warpgate/Guacamole/NetBird/OpenBao)

Se encontrar qualquer contradição entre CONSTITUTION, ADRs, RFCs e o pacote 001, PARE e liste
as contradições. Não escolha um lado. Contradição em corpus de governança é defeito a corrigir,
não ambiguidade a resolver por conta própria.

## Depois do resumo aprovado

Execute o pacote 001-bootstrap-fork, uma tarefa por vez, na ordem de tasks.md.

Ordem de prioridade dentro do pacote — respeite mesmo que pareça ineficiente:

  Bloco 1 (T-001 a T-006): fork point, licença, atribuição, DIVERGENCE.md
  Bloco 2 (T-018, T-019):  suíte de invariantes e CI ANTES das remoções
  Bloco 3 (T-007 a T-014): remoções de escopo e PostgreSQL único
  Bloco 4 (T-015 a T-017): fronteiras de framework e rebranding
  Bloco 5 (T-020 a T-024): imagem, stack, smoke test, watcher, docs

A antecipação dos blocos 2 sobre 3 é deliberada: a suíte de invariantes precisa existir ANTES
das remoções, para que a remoção da senha-mestra (T-011) seja verificada por teste no momento
em que acontece — e não meses depois.

## Para cada tarefa

1. Leia os cenários WHEN/THEN de specs/fork-baseline/spec.md relacionados à tarefa
2. Apresente um plano curto ANTES de codar. Aguarde minha confirmação se a tarefa tocar:
   invariantes, esquema de banco, criptografia ou fluxo de autenticação
3. Escreva primeiro o teste derivado dos cenários
4. Implemente o mínimo que satisfaz os cenários
5. Rode o gate completo: make lint && make test && make invariants && make deps-check &&
   make sbom && make build
6. Só com o gate verde: marque [x] em tasks.md e faça UM commit da tarefa

Nunca marque [x] sem gate verde. Nunca implemente duas tarefas no mesmo commit. Nunca altere um
teste de invariante para fazê-lo passar — se acredita que o teste está errado, pare e reporte.

## Tarefa especial: T-001

T-001 exige verificar na FONTE PRIMÁRIA (aba Insights/Releases do repositório upstream) a
release corrente, a licença vigente no arquivo LICENSE e a base de mantenedores. Há divergência
conhecida entre fontes secundárias sobre a última tag publicada. Registre a evidência em
docs/upstream/FORK_POINT.md com SHA completo, tag, data e hash de verificação da árvore.

Não congele o fork point sem essa evidência. O fork point é ativo de valor: o grant Apache 2.0
sobre o código publicado é irrevogável, e é ele que protege o projeto de um eventual
relicenciamento futuro do upstream.

## Pare e me consulte se

- A spec não cobre um caso que você encontrou no código herdado
- Uma remoção de escopo quebrar acoplamento interno não óbvio do upstream
- A implementação correta exigir violar um invariante
- For necessária qualquer dependência nova (verifique a matriz de licenças do ADR-0002 e
  pergunte antes de adicionar)
- O LICENSE do upstream divergir do esperado (Apache 2.0)

Comece pela leitura e pelo resumo. Não escreva código ainda.
```

---

## B. Template de sessão — pacotes 002 a 010

Substitua `<NNN-nome>`, `<capability>` e a lista de ADRs/RFCs conforme o README.

```
Sessão de implementação do pacote <NNN-nome> do ArchGuard.

## Contexto obrigatório

Leia antes de qualquer código:
1. CLAUDE.md e CONSTITUTION.md
2. docs/adr/<ADRs citados no proposal.md do pacote>
3. docs/rfc/<RFCs citados>
4. openspec/changes/<NNN-nome>/proposal.md, design.md, specs/<capability>/spec.md, tasks.md

Verifique que os pacotes dos quais este depende (ver grafo no README.md) estão com o gate
verde. Se não estiverem, pare e reporte — não comece um pacote sobre base vermelha.

## Execução

Uma tarefa por vez, na ordem de tasks.md, com o fluxo de 6 passos do CLAUDE.md §4.
Gate completo verde antes de marcar [x]. Um commit por tarefa.

## Cobertura de cenários

Ao final, produza um mapa: cada Requirement e cada Scenario da spec.md → teste que o cobre.
Cenário sem teste correspondente é tarefa incompleta, ainda que o código exista.

## Registro de divergência

Toda divergência estrutural criada em relação ao upstream deve entrar em
docs/upstream/DIVERGENCE.md com o subsistema afetado e o motivo. Sem isso, a triagem futura de
cherry-pick fica cega.

## Pare e me consulte se

- Encontrar contradição entre ADR, RFC e spec
- A tarefa tocar uma das "questões em aberto" registradas nos RFCs — elas estão marcadas como
  em aberto justamente porque a resposta não foi decidida. Não invente a resposta
- Precisar divergir do esquema definido no RFC-0002 ou RFC-0003
- Precisar de dependência nova

Comece confirmando o contexto lido e o plano da primeira tarefa.
```

---

## C. Prompt de triagem de upstream — recorrente (semanal)

```
Triagem semanal de upstream do ArchGuard, conforme ADR-0003.

1. Atualize o espelho vendor/upstream (somente-leitura; NUNCA faça merge em main)
2. Liste os commits novos desde o último ponto sincronizado registrado em
   docs/upstream/LAST_SYNC.md
3. Classifique cada commit por caminho de arquivo e natureza:
   - SEGURANÇA (CVE, correção de auth, fix criptográfico)  → SLA de triagem: 72 h
   - CORREÇÃO em subsistema não divergente                 → avaliar cherry-pick
   - FEATURE alinhada a PAM                                → exige ADR/RFC antes de importar
   - FEATURE fora de escopo (IA/MCP, provedores não curados)→ descartar
   - REFACTOR amplo                                        → descartar
4. Cruze cada commit com docs/upstream/DIVERGENCE.md. Commit que toque subsistema divergente
   exige revisão manual — não aplique automaticamente
5. Para os aprovados: cherry-pick com o trailer Upstream-Commit: <sha>, gate completo verde, e
   a suíte de invariantes obrigatoriamente executada
6. Se um cherry-pick de SEGURANÇA não aplicar por divergência estrutural: pare, reporte, e
   proponha mitigação própria dentro do mesmo SLA de 72 h

Verifique também se o arquivo LICENSE do upstream mudou. Mudança de licença é incidente de
governança com triagem em 48 h — reporte imediatamente e não importe nada até a decisão.

Entregue: tabela de commits classificados, o que foi aplicado, o que foi descartado e por quê,
e LAST_SYNC.md atualizado.
```

---

## Notas de uso

- **`CLAUDE.md` faz o trabalho pesado.** Ele é lido automaticamente a cada sessão; os prompts
  acima apenas orientam o recorte. Não duplique regras nos prompts — se uma regra é permanente,
  o lugar dela é o `CLAUDE.md`.
- **O bloco "Pare e me consulte" não é formalidade.** Em software de segurança, o modo de falha
  caro é o agente decidindo sozinho num ponto que a spec deixou em aberto de propósito.
- **Sessão longa degrada.** Prefira uma sessão por bloco de tarefas a uma sessão por pacote
  inteiro; ao reiniciar, o `CLAUDE.md` e o `tasks.md` marcado reconstroem o estado.
- **Antes do M1**, resolva os três bloqueios do README: verificação em fonte primária do
  upstream, due diligence jurídica e inventário do PoC Kanidm.
