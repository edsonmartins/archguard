# DEVELOPMENT.md — ArchGuard (desenvolvimento)

Guia mínimo para desenvolver no fork. A governança está no corpus
(`CLAUDE.md`, `CONSTITUTION.md`, `docs/adr/`, `docs/rfc/`, `openspec/`) — leia-a antes de
codar. Este arquivo cobre o **como rodar e verificar**.

## Pré-requisitos

- **Go 1.25+** (o toolchain é fixado em `go.mod`).
- **PostgreSQL 15+** — o único backend suportado (ADR-0009). Nenhum outro dialeto.
- `git` (o watcher de upstream e o gate de licença usam).

## Estrutura

```
object/, controllers/, routers/…   código herdado do upstream (Beego + XORM)
internal/domain/**                 domínio puro — SEM framework web nem ORM (INV-3)
internal/adapters/**               implementações das portas (pgx, keystore selado…)
internal/deploy/                   perfis de implantação (ADR-0017)
internal/migrate/                  migrations versionadas (pgx + advisory lock)
tools/                             módulo SEPARADO de ferramentas de CI (não no build)
test/invariants/**                 suíte que quebra o build (INV-1..8)
deploy/postgres/roles.sql          papéis de banco segregados
docs/upstream/                     FORK_POINT, DIVERGENCE, LAST_SYNC
```

## Gate de verificação (CLAUDE.md §5)

Uma tarefa só está pronta com **todos** verdes:

```bash
make lint          # gofmt + go vet (via lint-baseline, travas herdadas)
make test          # unitários + integração (precisa de PostgreSQL)
make invariants    # suíte INV-1..8 (quebra o build)
make deps-check    # regra de dependência de pacotes (INV-3)
make sbom          # SBOM CycloneDX + license gate (INV-4)
make build         # binário
```

- **`make invariants`/`make sbom`** rodam `go-licenses` (baixa a ferramenta fixada; requer
  rede na primeira vez). O license gate é **fail-closed**: licença não determinável é vermelho.
- **`license-baseline.txt`** quarentena o passivo de licença herdado; ver
  `openspec/changes/001-bootstrap-fork/design.md` (nota transitória). Só encolhe.
- **Disco/temp:** o build completo consome vários GB de temp; se faltar espaço, use
  `go clean -cache` e `go build -p 1 ./pacote/`.

## Rodando localmente (perfil dev)

O perfil de implantação é **obrigatório** (ADR-0017). Em `conf/app.conf`:

```ini
deploymentProfile = dev
dataSourceName = user=postgres password=... host=localhost port=5432 sslmode=disable
dbName = archguard
keystorePath = conf/keystore.sealed
```

O perfil `dev` usa um **keystore local selado** para a chave de assinatura (fora do banco,
I-4.3). Forneça o material de selagem no ambiente antes de subir:

```bash
export KEYSTORE_UNSEAL_KEY="uma-frase-secreta-forte"   # sem isto, o processo recusa iniciar
go run . 
```

O perfil `dev` é **não conforme** por construção: `/api/health` reporta
`compliance: non_conformant`, operações L3 são negadas e o boot recusa sob indício de exposição
pública. **Não use `dev` em produção** — produção é o perfil `production` com OpenBao.

## Upstream (ADR-0003)

```bash
make upstream-triage   # atualiza vendor/upstream e emite a fila de triagem classificada
```

Nunca faça merge de branch do upstream em `main`. Importação é só cherry-pick com o trailer
`Upstream-Commit: <sha>` (ver `docs/RUNBOOK.md` e o prompt de triagem em `PROMPT-CLAUDE-CODE.md`).
