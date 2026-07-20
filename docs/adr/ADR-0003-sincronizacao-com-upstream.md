# ADR-0003 — Estratégia de sincronização com o upstream

- **Status:** Aceito
- **Data:** 2026-07-19
- **Invariantes tocados:** I-8.1 a I-8.4

## Contexto

O upstream (Casdoor) opera com cadência altíssima de micro-releases via *semantic-release*,
com dezenas de tags por mês. O ArchGuard divergirá estruturalmente em quatro eixos:
multi-tenancy (ADR-0006), auditoria imutável (ADR-0007), controles de privilégio (ADR-0008) e
console (ADR-0004). Merge contínuo de branch é inviável: produziria conflito permanente
justamente nos arquivos mais divergentes e reintroduziria, por acidente, comportamentos que a
constituição proíbe (ex.: master password).

Simultaneamente, ignorar o upstream é inaceitável: correções de segurança em código de
identidade são críticas.

## Decisão

**Sincronização seletiva por cherry-pick, com triagem por classe de mudança. Merge de branch
upstream é proibido.**

### Topologia de branches

```
upstream/master ──(read-only mirror)──> vendor/upstream
                                             │  (cherry-pick seletivo)
                                             ▼
main ────────────────────────────────► release/x.y
  ▲
  └── feature/*  (pacotes OpenSpec)
```

- `vendor/upstream`: espelho somente-leitura, nunca merged em `main`.
- `main`: linha do ArchGuard. Recebe commits do upstream **exclusivamente** por cherry-pick
  com trailer `Upstream-Commit: <sha>`.

### Triagem por classe

| Classe | SLA de triagem | Ação padrão |
|---|---|---|
| **Segurança** (CVE, correção de auth, fix criptográfico) | **72 h** da divulgação | Cherry-pick obrigatório ou mitigação documentada |
| **Correção de bug** em subsistema não divergente | 30 dias | Cherry-pick se aplicável sem conflito estrutural |
| **Feature** alinhada ao escopo PAM | Avaliação trimestral | ADR/RFC próprio antes de importar |
| **Feature** fora de escopo (agentes IA/MCP, provedores irrelevantes) | — | **Não importar** |
| **Refactor amplo do upstream** | — | Não importar; avaliar apenas em rebase de major |

### Instrumentação
- **Registro de divergência** (`docs/upstream/DIVERGENCE.md`): arquivos e subsistemas com
  divergência estrutural, e por quê. Serve de mapa de risco de conflito.
- **Watcher automatizado**: diff semanal entre `vendor/upstream` e o último ponto sincronizado,
  classificando commits por caminho de arquivo e emitindo fila de triagem.
- **Teste de regressão de invariante**: suíte que falha se um cherry-pick reintroduzir
  comportamento proibido (master password utilizável, endpoint que edita auditoria, query sem
  predicado de tenant). Executada em todo cherry-pick.

### Rebase de major
No máximo **um por ano**, tratado como pacote OpenSpec dedicado, com orçamento próprio e gate
de verificação completo.

## Consequências

- Trabalho recorrente de triagem (estimado 2–4 h/semana).
- Divergência crescente é **aceita e planejada**, não combatida.
- A suíte de invariantes é a defesa real contra regressão por importação — sua ausência
  invalidaria esta estratégia.

## Riscos

- **Correção de segurança em código profundamente divergente**: o cherry-pick pode não
  aplicar. Mitigação: mitigação própria documentada no mesmo SLA de 72 h, com ADR se alterar
  arquitetura.
- **Erosão da triagem** por pressão de entrega. Mitigação: fila de triagem é item fixo de
  planejamento, não trabalho de fundo.
