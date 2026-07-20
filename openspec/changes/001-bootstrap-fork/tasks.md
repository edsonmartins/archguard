# Tasks — 001 · Bootstrap do fork ArchGuard

> Granularidade de sessão (I-9.3). "Pronto" = gate de verificação do pacote (I-9.4).

## Ordem de execução (normativa)

Executar por blocos, **nesta ordem** — que difere da numeração sequencial:

| Bloco | Tarefas | Motivo |
|---|---|---|
| 1 | T-000 a T-006 | Ambiente, fork point, licença, atribuição |
| **2** | **T-018, T-019** | **Suíte de invariantes e CI ANTES das remoções**, para que a remoção da senha-mestra (T-011) nasça verificada por teste no momento em que acontece |
| 3 | T-007 a T-014 | Remoções de escopo e PostgreSQL único |
| 4 | T-015 a T-017 | Fronteiras de framework e rebranding |
| 5 | T-020 a T-025 | Perfis, imagem, stack, smoke test, watcher, docs |

## Bloco 0 — Preparação do ambiente

- [x] **T-000a** Inicializar repositório git; clonar o Casdoor como base do fork.
- [x] **T-000b** Mover o corpus de governança de `spec/` para a raiz, conforme os caminhos
      referenciados no `CLAUDE.md` (`CONSTITUTION.md`, `docs/adr/`, `docs/rfc/`,
      `openspec/changes/`).
- [x] **T-000c** Criar `Makefile` com os alvos do gate (`lint`, `test`, `invariants`,
      `deps-check`, `sbom`, `build`). Os alvos `invariants`, `deps-check` e `sbom` começam como
      *stub* que falha com mensagem explícita, e são implementados de fato em T-018/T-019 —
      gate parcial nas tarefas do Bloco 1 é esperado e aceito.

- [x] **T-001** Verificar na fonte primária (repositório upstream) release corrente, licença
      vigente e base de mantenedores; registrar evidência.
- [x] **T-002** Criar fork, congelar fork point e escrever `docs/upstream/FORK_POINT.md`.
- [ ] **T-003** Configurar `vendor/upstream` como espelho somente-leitura e proteger `main`.
- [ ] **T-004** Preservar `LICENSE`; redigir bloco de atribuição no `NOTICE` (ADR-0002).
- [ ] **T-005** Definir política de cabeçalhos de copyright e aplicar em arquivos novos.
- [ ] **T-006** Inicializar `docs/upstream/DIVERGENCE.md`.
- [ ] **T-007** Mapear dependências internas dos módulos fora de escopo (relatório).
- [ ] **T-008** Remover módulos de pagamento/produto/assinatura.
- [ ] **T-009** Remover funcionalidades de agentes IA/MCP.
- [ ] **T-010** Reduzir provedores ao catálogo curado (ADR-0015, §3).
- [ ] **T-011** **Remover a senha-mestra do código** e a coluna correspondente via migration.
- [ ] **T-012** Remover dialetos de banco não-PostgreSQL.
- [ ] **T-013** Implantar migrations versionadas com travamento de execução concorrente.
- [ ] **T-014** Criar papéis de banco segregados (aplicação/migração/leitura).
- [ ] **T-015** Criar `internal/domain/**` e a regra de dependência no CI (ADR-0016).
- [ ] **T-016** Introduzir camada de persistência `pgx` para código novo.
- [ ] **T-017** Rebranding: identificadores, cabeçalhos HTTP, assets, strings.
- [ ] **T-018** Implementar suíte de invariantes (4 testes do design).
- [ ] **T-019** Pipeline CI completo com SBOM e license gate bloqueante.
- [ ] **T-020a** Implementar perfis de implantação `dev`/`pilot`/`production` com declaração
      obrigatória e reporte no health check (ADR-0017).
- [ ] **T-020b** Implementar keystore local selado do perfil `dev` (chave cifrada fora do banco;
      material de selagem no boot; recusa de inicialização sem material).
- [ ] **T-020c** Implementar travas do perfil `dev`: aviso, marca de não conformidade, negação
      de L3 e recusa sob indício de exposição pública.
- [ ] **T-020** Imagem de container reprodutível e assinada; usuário não-root.
- [ ] **T-021** Stack Docker Swarm + Traefik mínima (core + PostgreSQL), TLS obrigatório.
- [ ] **T-022** Smoke test ponta a ponta no perfil `dev`: subir, autenticar, emitir token OIDC,
      validar JWKS e verificar estabilidade do JWKS entre reinícios.
- [ ] **T-023** Watcher de upstream (diff semanal + fila de triagem) — ADR-0003.
- [ ] **T-024** README de desenvolvimento e runbook mínimo de operação.

- [ ] **T-025** Registrar em `DIVERGENCE.md` as remoções e a introdução de perfis.

## Gate de verificação
CI verde ponta a ponta; suíte de invariantes falhando corretamente quando um comportamento
proibido é injetado (teste do teste); smoke test aprovado no perfil `dev` com JWKS estável entre
reinícios; nenhuma chave privada no banco em qualquer perfil; `NOTICE` revisado; due diligence
de licença iniciada com o jurídico.
