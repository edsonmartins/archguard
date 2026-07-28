# RUNBOOK.md — ArchGuard (operação mínima)

Runbook mínimo de operação do bootstrap (pacote 001). Cresce com os pacotes seguintes
(auditoria imutável, MFA/step-up, break-glass, observabilidade). Onde este runbook e o corpus
divergirem, o corpus (`CONSTITUTION.md`, ADRs, RFCs) prevalece.

## Perfis de implantação (ADR-0017)

| Perfil | Composição | Custódia de chaves | Uso |
|---|---|---|---|
| `dev` | Core + PostgreSQL | Keystore local selado | Desenvolvimento, CI, smoke test, demo. **Não suportado em produção** |
| `pilot` | Core + PostgreSQL + OpenBao | OpenBao | Piloto/homologação |
| `production` | Core + PostgreSQL + OpenBao (HA) + OpenFGA + OTLP | OpenBao (HA) | **Única configuração suportada comercialmente** |

O perfil é **obrigatório** (`deploymentProfile`). Subir sem perfil válido é erro fatal.

## Provisionamento do PostgreSQL

1. Criar o banco (PostgreSQL 15+; versões inferiores serão recusadas na inicialização).
2. Aplicar os **papéis segregados** (ADR-0009) como superusuário, **antes** de subir a app:
   ```bash
   psql -v db=archguard -f deploy/postgres/roles.sql
   ```
   Substitua as senhas placeholder. Resultado: `archguard_migrate` (DDL), `archguard_app`
   (runtime, **sem UPDATE/DELETE na auditoria** — barreira física do INV-2), `archguard_readonly`.
3. Conectar a aplicação como `archguard_app` (`dataSourceName`) e as migrations como
   `archguard_migrate` (`migrationDataSourceName`).

## Migrations (ADR-0009)

- Aplicadas automaticamente no boot (`object.RunMigrations`), após o `Sync2` legado, com
  `pg_advisory_lock` (seguro entre réplicas). Idempotentes e versionadas em
  `internal/migrate/migrations/`.
- Mudança destrutiva de esquema é **sempre** migration explícita — nunca auto-sync.

## Material de selagem do keystore (perfil dev)

- A chave de assinatura vive cifrada (AES-256-GCM) em `keystorePath`, **fora do banco** (I-4.3).
- O material de selagem (`KEYSTORE_UNSEAL_KEY`) é fornecido no boot, **nunca** persistido junto
  ao keystore nem no banco. Sem ele, o processo não inicia.
- **DR:** perder o material de selagem = perder a capacidade de assinar/verificar com aquela
  chave. Guarde-o com o mesmo rigor de um segredo de produção. (Em `pilot`/`production` a
  custódia é o OpenBao — ver ADR-0012.)

## Backup e DR

- **PostgreSQL:** backup + PITR são requisito, não opcional (ADR-0009). RPO ≤ 5 min / RTO ≤ 30
  min (metas do RFC-0001).
- **Keystore selado (dev):** inclua `keystorePath` e o material de selagem no plano de DR.
- **Repositório:** o histórico git é distribuído; `git bundle` periódico de `main` e
  `vendor/upstream` para storage independente (ADR-0018).

## Armazenamento de arquivos (avatar / upload do console)

Uploads do console (avatar de usuário, logos etc.) usam um **provider de Storage** herdado do
Casdoor. Não é dependência de segurança do plano de controle — é conveniência de UI — mas precisa
estar configurado, senão o upload falha com *"Nenhum provedor da categoria: Storage encontrado
para o aplicativo: app-built-in"*.

**Provider (uma vez, via console ou API):**

- Category `Storage`, Type **`Local File System`**. O tipo `Casdoor` **exige um cert** (assina as
  URLs) e falha com *"no cert for ..."* — não usar sem cert.
- Owner **`admin`** (global): `GetProviderByCategory` só enxerga providers de owner `admin` **ou**
  da organização da app.
- **Vincular à app** `app-built-in` (aba Providers → Add → **Save da aplicação**). O provider só
  resolve se estiver **na lista de providers da app** — criar sem vincular não basta.

**Persistência e permissão (infra):**

- O provider grava em **`/files/`** no container (ex.: `/files/avatar/<org>/...`).
- O processo roda como **UID 1000**; a imagem já cria `/files/avatar` com esse dono (Dockerfile).
  Sem isso: *"mkdir /files/avatar: permission denied"*.
- Em `pilot`/`production`, `/files` é um **volume nomeado** (`archguard-files`) para o upload
  **sobreviver a redeploy**. Volume nomeado pode nascer `root:root` — o deploy do
  `archguard-devops` (`50-deploy-archguard.sh`) faz `chown -R 1000:1000 /files` (mesma armadilha
  do volume do OpenBao).
- **DR:** inclua `archguard-files` no backup se os avatares importarem; são recriáveis (re-upload),
  logo prioridade menor que o banco.

## Triagem de upstream (ADR-0003)

- Cadência **semanal**: `make upstream-triage` atualiza o espelho e emite a fila classificada.
- **SEGURANÇA**: SLA de 72h (cherry-pick com trailer `Upstream-Commit: <sha>` + suíte de
  invariantes verde, ou mitigação própria documentada).
- **Mudança de LICENSE do upstream**: incidente de governança, triagem em 48h — não importe
  nada até a decisão (ADR-0002 §6). O watcher alerta.
- Atualize `docs/upstream/LAST_SYNC.md` ao concluir cada rodada.

## Acesso emergencial

Não existe backdoor administrativo nem senha-mestra (I-4.1 — removida no T-011; a suíte de
invariantes rejeita sua reintrodução). Acesso emergencial é **break-glass auditado**, desenhado
no pacote 004 (ADR-0008). Até lá, não há caminho de emergência — por design.
