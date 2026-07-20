# Resposta ao Claude Code — resolução da contradição e liberação do pacote 001

Cole o bloco abaixo na sessão em aberto.

```
Resumo aprovado. A leitura está correta em todos os pontos, inclusive na compreensão de por que
o Bloco 2 antecede o Bloco 3.

## 1. Contradição — procede. Corrigida no corpus.

Você identificou uma falha real de governança, não uma ambiguidade. A raiz: I-1.3 confundia
"autossuficiência de runtime" (resiliência sob falha) com "configuração suportada em produção"
(compromisso comercial). Isso colidia com I-4.3, que é pétreo.

Resolução emitida como **ADR-0017 — Perfis de implantação e custódia de chaves no perfil
mínimo**, que é formalmente uma **emenda constitucional** (I-10.1). Atualize seu working set: o
corpus mudou em seis arquivos.

Decisão em resumo:

- Três perfis nomeados e obrigatórios: `dev` (core + PostgreSQL), `pilot` (+ OpenBao),
  `production` (+ OpenBao HA, OpenFGA, OTLP). Perfil não declarado = erro fatal de
  inicialização.
- I-1.3 emendado: autossuficiência passa a descrever **continuidade sob falha** — serviço
  adjacente indisponível não derruba o plano de autenticação. A configuração suportada
  comercialmente é `production`.
- **I-4.3 preservado sem alteração.** No perfil `dev`, a chave vive em keystore local selado:
  cifrada, fora do banco, com material de selagem fornecido no boot e nunca persistido junto
  nem no banco. Sem material de selagem, o processo não inicia — não há geração automática
  silenciosa.
- Quatro travas no `dev`: aviso de inicialização, marca de não conformidade no health check,
  **negação de operações L3** e recusa de inicialização sob indício de exposição pública.

Respondendo diretamente à sua pergunta: a chave que assina o token do smoke test vive no
keystore local selado, e o smoke test agora declara o perfil `dev` explicitamente.

Arquivos alterados:
- `CONSTITUTION.md` — I-1.3 emendado + Anexo B (registro de emendas)
- `docs/adr/ADR-0017-perfis-de-implantacao-e-custodia-minima.md` — novo
- `docs/rfc/RFC-0001-arquitetura-de-referencia.md` — §4, §6, §7
- `docs/adr/ADR-0012-gestao-de-chaves-e-segredos.md` — modo degradado
- `openspec/changes/001-bootstrap-fork/specs/fork-baseline/spec.md` — 3 requirements novos
- `openspec/changes/001-bootstrap-fork/tasks.md` — ordem de blocos + Bloco 0 + T-020a/b/c

## 2. Observação sobre a ordem dos blocos — procede. Corrigida.

Você está certo: a reordenação estava só no prompt, e o CLAUDE.md prevalece sobre instrução de
sessão. Isso deixaria a regra e o registro de estado incoerentes. A ordem por blocos agora é
**normativa dentro do tasks.md** do pacote 001. Siga o tasks.md; não há mais divergência.

## 3. Pré-requisitos de ambiente — reconhecidos, viraram Bloco 0.

Correto: o corpus está em `spec/` e não há repositório git. Isso agora é trabalho especificado,
não pressuposto:

- T-000a — inicializar git e clonar o Casdoor como base do fork
- T-000b — mover o corpus de `spec/` para a raiz, conforme os caminhos do CLAUDE.md
- T-000c — criar o Makefile com os alvos do gate

Sobre T-000c, uma expectativa explícita: `invariants`, `deps-check` e `sbom` começam como stub
que falha com mensagem clara e só são implementados de fato em T-018/T-019. **Gate parcial nas
tarefas do Bloco 1 é esperado e aceito** — não trate isso como impedimento, e não implemente os
alvos fora de ordem para "deixar verde".

## 4. Prossiga

1. Releia os seis arquivos alterados (comece por ADR-0017).
2. Confirme em uma linha que a contradição está resolvida do seu ponto de vista — se a
   resolução ainda deixar lacuna, quero saber agora.
3. Execute o Bloco 0 (T-000a → T-000c), apresentando o plano antes de cada tarefa.
4. Siga para T-001: verificação em fonte primária da release, licença e mantenedores do
   upstream antes de congelar o fork point. Não congele sem a evidência registrada em
   `docs/upstream/FORK_POINT.md`.

Mantenha o mesmo padrão de leitura crítica. Detectar essa contradição antes da primeira linha
de código custou um ADR; descobri-la no M6, com clientes em piloto e chaves emitidas, custaria
migração de material criptográfico em produção.
```
