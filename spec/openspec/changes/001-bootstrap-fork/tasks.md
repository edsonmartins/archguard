# Tasks — 001 · Bootstrap do fork ArchGuard

> Granularidade de sessão (I-9.3). "Pronto" = gate de verificação do pacote (I-9.4).
>
> **A ordem por blocos abaixo é normativa.** A antecipação do Bloco 2 sobre o Bloco 3 é
> deliberada: a suíte de invariantes precisa existir ANTES das remoções, para que a remoção da
> senha-mestra (T-011) seja verificada por teste no momento em que acontece.
>
> **Gate parcial nos Blocos 0 e 1 é esperado e aceito**: `invariants`, `deps-check` e `sbom`
> nascem como stubs que falham com mensagem clara (T-000c) e só são implementados de fato em
> T-018/T-019. Não implemente os alvos fora de ordem para "deixar verde".

## Bloco 0 — Preparação do ambiente

- [x] **T-000a** Inicializar repositório git e clonar o Casdoor como base do fork.
- [ ] **T-000b** Mover o corpus de governança de `spec/` para a raiz, conforme os caminhos do
      CLAUDE.md.
- [ ] **T-000c** Criar o Makefile com os alvos do gate (`lint`, `test`, `invariants`,
      `deps-check`, `sbom`, `build`); `invariants`, `deps-check` e `sbom` como stubs que
      falham com mensagem clara até T-018/T-019.

## Bloco 1 — Fork point, licença e atribuição

- [ ] **T-001** Verificar na fonte primária (repositório upstream) release corrente, licença
      vigente e base de mantenedores; registrar evidência.
- [ ] **T-002** Criar fork, congelar fork point e escrever `docs/upstream/FORK_POINT.md`.
- [ ] **T-003** Configurar `vendor/upstream` como espelho somente-leitura e proteger `main`.
- [ ] **T-004** Preservar `LICENSE`; redigir bloco de atribuição no `NOTICE` (ADR-0002).
- [ ] **T-005** Definir política de cabeçalhos de copyright e aplicar em arquivos novos.
- [ ] **T-006** Inicializar `docs/upstream/DIVERGENCE.md`.

## Bloco 2 — Suíte de invariantes e CI (ANTES das remoções)

- [ ] **T-018** Implementar suíte de invariantes (4 testes do design).
- [ ] **T-019** Pipeline CI completo com SBOM e license gate bloqueante.

## Bloco 3 — Remoções de escopo e PostgreSQL único

- [ ] **T-007** Mapear dependências internas dos módulos fora de escopo (relatório).
- [ ] **T-008** Remover módulos de pagamento/produto/assinatura.
- [ ] **T-009** Remover funcionalidades de agentes IA/MCP.
- [ ] **T-010** Reduzir provedores ao catálogo curado (ADR-0015, §3).
- [ ] **T-011** **Remover a senha-mestra do código** e a coluna correspondente via migration.
- [ ] **T-012** Remover dialetos de banco não-PostgreSQL.
- [ ] **T-013** Implantar migrations versionadas com travamento de execução concorrente.
- [ ] **T-014** Criar papéis de banco segregados (aplicação/migração/leitura).

## Bloco 4 — Fronteiras de framework e rebranding

- [ ] **T-015** Criar `internal/domain/**` e a regra de dependência no CI (ADR-0016).
- [ ] **T-016** Introduzir camada de persistência `pgx` para código novo.
- [ ] **T-017** Rebranding: identificadores, cabeçalhos HTTP, assets, strings.

## Bloco 5 — Imagem, stack, perfis, smoke test, watcher, docs

- [ ] **T-020** Imagem de container reprodutível e assinada; usuário não-root.
- [ ] **T-020a** Implementar seleção obrigatória de perfil de implantação
      (`dev`/`pilot`/`production`); ausência de perfil ⇒ erro fatal (ADR-0017, §1).
- [ ] **T-020b** Implementar keystore local selado do perfil `dev`: chave cifrada (AEAD) fora
      do banco, material de selagem no boot, sem geração automática silenciosa (ADR-0017, §3).
- [ ] **T-020c** Implementar travas do perfil `dev`: aviso de inicialização,
      `compliance: non_conformant` no health check, negação de operações L3 e recusa de boot
      sob indício de exposição pública (ADR-0017, §4).
- [ ] **T-021** Stack Docker Swarm + Traefik mínima (core + PostgreSQL), TLS obrigatório.
- [ ] **T-022** Smoke test ponta a ponta: subir com perfil `dev` declarado, autenticar,
      emitir token OIDC.
- [ ] **T-023** Watcher de upstream (diff semanal + fila de triagem) — ADR-0003.
- [ ] **T-024** README de desenvolvimento e runbook mínimo de operação.

## Gate de verificação
CI verde ponta a ponta; suíte de invariantes falhando corretamente quando um comportamento
proibido é injetado (teste do teste); smoke test aprovado; `NOTICE` revisado; due diligence de
licença iniciada com o jurídico.
