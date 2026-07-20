# Tasks — 001 · Bootstrap do fork ArchGuard

> Granularidade de sessão (I-9.3). "Pronto" = gate de verificação do pacote (I-9.4).

## Ordem de execução (normativa)

Executar por blocos, **nesta ordem** — que difere da numeração sequencial:

| Bloco | Tarefas | Motivo |
|---|---|---|
| 1 | T-000a/b/c, T-001, T-002, **T-005, T-004** (antecipadas, nesta ordem — decisão de 2026-07-20) | Ambiente e fork point; política de copyright ANTES de arquivos novos; NOTICE com fork point fresco |
| **2** | **T-018, T-019** | **Suíte de invariantes e CI ANTES das remoções**, para que a remoção da senha-mestra (T-011) nasça verificada por teste no momento em que acontece. **T-019 bloqueada até aprovação do ADR-0018 (forja)** |
| 1b | T-003, T-006 | T-003 bloqueada pela decisão de forja: executar imediatamente após ADR-0018 aprovado e forja provisionada. T-006 materialmente satisfeita em T-002 (DIVERGENCE.md criado) — formalizar |
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
- [ ] **T-003** Configurar `vendor/upstream` como espelho somente-leitura e proteger `main`
      no **GitHub** (bloqueada até ADR-0018 ratificado). Conforme ADR-0018: ruleset em `main`
      (PR obrigatório, aprovações mínimas, required status checks bloqueantes, up-to-date,
      force-push/deleção bloqueados, `Require signed commits`); papéis de trabalho = Write,
      admin da org não é conta de trabalho; detecção (verificador de proveniência de `main`
      com alerta máximo; audit log da org p/ ações admin e alterações de ruleset; `bypass
      actors` VAZIO e monitorado; alerta imediato). **Aceite bloqueante:** (1) conta Write não
      consegue push direto a `main`, merge com check vermelho, nem force-push — evidência
      anexada; (2) alteração de ruleset gera evento no audit log com alerta — demonstrado; (3)
      `bypass actors` vazio, verificado.
- [x] **T-004** Preservar `LICENSE`; redigir bloco de atribuição no `NOTICE` (ADR-0002).
- [x] **T-005** Definir política de cabeçalhos de copyright e aplicar em arquivos novos.
- [ ] **T-006** Inicializar `docs/upstream/DIVERGENCE.md`.
- [ ] **T-007** Mapear dependências internas dos módulos fora de escopo (relatório).
- [x] **T-008** Remover módulos de pagamento/produto/assinatura. *(Código: object/controllers
      de pagamento + `pp/` removidos; gating de subscription no auth, `GetPaymentProvider`,
      `User.Cart` e seeds/dump removidos; 38 rotas fora; `go-cleanhttp` sai do baseline. Gate
      verde.)* **Esquema pendente:** colunas órfãs `cart`/`balance`/`balanceCredit`/
      `balanceCurrency` só saem em migration explícita pós-T-013 (nunca auto-sync) — ver T-013.
- [x] **T-009** Remover funcionalidades de agentes IA/MCP. *(Servidor MCP `/api/mcp`, registro
      de servidores MCP (`Server`/`Tool`), entidade `Agent`, provider Agent/OpenClaw (transcrição
      + telemetria OTLP), scanner de intranet MCP. Fronteira preservada: OAuth DCR (RFC 7591),
      `Entry`/log-collector geral, Security Scan, RADIUS ficam. SDK MCP + otlp saem do grafo.
      Redução de superfície, não de licença. Gate verde.)*
- [x] **T-010** Reduzir provedores ao catálogo curado (ADR-0015, §3). *(3 partes: notificações
      + eleição FTL freetype; idp ao catálogo (Entra/AD, Google, Okta, GitHub, GitLab, Custom,
      Casdoor) + remove goth + fluxos WeChat; faceId/idv/captcha-aliyun (KYC não-PAM). Esvaziou
      do baseline: mautrix, go.mau.fi, freetype, mrjones, 6 alibaba → baseline 15→5. Decisão do
      usuário: manter GitHub/GitLab. Gate verde.)* **Esquema pendente T-013:** colunas órfãs
      `face_ids` + provedores sociais removidos.
- [x] **T-010a** Remover o servidor LDAP embutido e a dependência `goldap` (GPL-2.0) —
      ADR-0019 Parte III / ADR-0015 §5. O conector **cliente** LDAP/AD (pacote 009) e o
      servidor RADIUS não são afetados. *(Antecipada ao Bloco 3 por ordem expressa; gate verde
      com o license-baseline — a GPL saiu da árvore, não figura no baseline.)*
- [x] **T-011** **Remover a senha-mestra do código** e a coluna correspondente via migration.
      Subtarefa obrigatória: **deletar `test/invariants/known_violations.txt`** — após T-011,
      a existência do arquivo é ela própria violação de INV-1 (design.md, nota transitória).
      *(Backdoor de auth removido em object/check.go; campo `MasterPassword` + hashing/masking/
      omit em organization.go; masking em application_util.go. `known_violations.txt` DELETADO
      no mesmo commit. **INV-1 verde SEM allowlist**. Campo do struct removido ⇒ coluna
      `master_password` não é criada em instalação nova (satisfaz o cenário INV-1 "coluna não
      existe no esquema"); DROP explícito p/ bancos existentes em T-013. Gate verde.)*
- [x] **T-012** Remover dialetos de banco não-PostgreSQL. *(ADR-0009: drivers mysql/mssql/sqlite
      removidos de ormer.go; backend + syncer externo postgres-only; guarda de init recusa
      driverName != postgres; sync/ e sync_v2/ (replicação MySQL, não usados) removidos.
      Esvazia baseline: go-sql-driver/mysql + modernc/mathutil → **baseline = 3** (só as MPL do
      ADR-0019). app.conf default = postgres. Gate verde.)* **Pendente:** cenário PG<15 recusa
      iniciar → validar no smoke test T-022 (PG real).
- [x] **T-013** Implantar migrations versionadas com travamento de execução concorrente.
      *(`internal/migrate/`: migrator próprio em pgx + `pg_advisory_lock`, `schema_migrations`,
      migrations SQL numeradas embutidas, idempotente; roda após o Sync2 legado; dep nova
      `jackc/pgx/v5` aprovada. Migration 0001 dropa as 3 colunas órfãs `cart`/`face_ids`/
      `master_password` — DROP explícito, nunca auto-sync. Testes de lógica pura verdes; gate
      verde.)* **Pendente:** aplicação real da migration validada no smoke test T-022 (PG real).
      Colunas `balance*` e de provedores sociais ficam (campos ainda no struct) para passo
      futuro que remova esses campos primeiro.
- [x] **T-014** Criar papéis de banco segregados (aplicação/migração/leitura). *(ADR-0009:
      `deploy/postgres/roles.sql` cria archguard_migrate (DDL), archguard_app (DML de domínio
      mas **sem UPDATE/DELETE nas tabelas de auditoria** = barreira física do INV-2) e
      archguard_readonly (SELECT). Segregação de conexão: `migrationDataSourceName` no app.conf
      faz o migrator conectar como archguard_migrate. Gate verde.)* **Pendente:** aplicação do
      roles.sql contra PG real validada no smoke test T-022. Conjunto de tabelas de auditoria
      cresce no pacote 003 (reafirmar o REVOKE a cada nova).
- [x] **T-015** Criar `internal/domain/**` e a regra de dependência no CI (ADR-0016). *(Criado
      `internal/domain/` com doc.go (contrato de fronteira) + `outcome.go` (primitivo fundamental
      livre de framework: distinção Allowed/Denied/Failed + fail-closed do INV-6, CLAUDE.md §6).
      Regra INV-3 agora ATIVA contra o diretório real — provado por injeção: import de beego no
      domínio quebra `make deps-check`, revertido volta a verde. Gate verde.)*
- [x] **T-016** Introduzir camada de persistência `pgx` para código novo. *(ADR-0016 §3:
      `internal/adapters/postgres/` com `NewPool` (pgxpool compartilhado) e `WithTx` (uma
      transação por operação de negócio, RFC-0002 §5; rollback em erro/panic, no-op após commit;
      interface `Beginner` estreita p/ testabilidade). doc.go fixa as regras (sem chamada remota
      em transação — outbox RFC-0004 §4; tenant no construtor). 4 testes com fake. pgx já entrara
      no T-013. Repositórios reais vêm nos pacotes 002+. Gate verde.)*
- [ ] **T-017** Rebranding: identificadores, cabeçalhos HTTP, assets, strings.
      *(PARCIAL — backend de marca visível FEITO: cookie de sessão `archguard_session_id`,
      appname `archguard`, issuer TOTP default `ArchGuard`, seed da app default (DisplayName/
      Logo/HomepageUrl). Nenhum header HTTP de marca existia. Corrigido teste órfão do FaceId
      (T-010). Estrutural mantido (provider Type "Casdoor" de federação, chaves de config, nomes
      built-in). Gate verde.)* **FALTA p/ fechar:** (a) **module path** `github.com/casdoor/
      casdoor` — DECISÃO do usuário (trocar p/ qual path, ou manter); toca todos os imports.
      (b) **assets/strings do frontend `web/`** — DEFERIDO ao pacote 008 (ADR-0004 substitui o
      console inteiro; rebrandizar UI descartada é desperdício).
- [x] **T-018** Implementar suíte de invariantes (4 testes do design).
- [x] **T-019a** Implementação local (não depende da forja): os três detectores de transição
      MPL do ADR-0019 §II.3 (hash vs proxy oficial; ausência de `replace` local; ausência de
      vendorização alterada), regra de licença dual (eleição explícita registrada; sem eleição
      = desconhecida = vermelho), SBOM CycloneDX + license gate como alvo local, módulo de
      ferramentas separado com versões fixadas, fail-closed em licença desconhecida.
- [ ] **T-019b** Imposição pela forja: o gate vira status check obrigatório. **Bloqueada**
      (forja provisionada + ADR-0018 ratificado).
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
