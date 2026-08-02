# ArchGuard — Guia do Operador (Runbooks)

> Público: quem opera o ArchGuard em produção. Procedimentos task-oriented. Consolida e referencia
> `docs/RUNBOOK.md` (operação mínima) e `docs/DEVOPS-HANDOFF.md` (detalhe dev→devops) — a fonte da
> verdade de baixo nível é o código + esses documentos.
>
> **Princípio de segurança deste guia:** nenhum valor de segredo (senha, chave, token, share de
> unseal) aparece aqui — apenas os **procedimentos**. Segredos vivem na custódia (OpenBao) ou no
> gerenciador do operador (INV-7).
>
> Status: rascunho vivo (2026-08-02).

## 1. Topologia de produção

| Componente | Papel |
|---|---|
| **core** (`ghcr.io/edsonmartins/archguard`) | O plano de controle. Serve o console e `/api/v1`. |
| **PostgreSQL 15+** | Fonte da verdade. RLS por tenant. |
| **Redis** (dedicado) | Sessão persistente (sobrevive à recriação do core). |
| **OpenBao** | Custódia de chaves (perfil conforme). Acessado por HTTP, **nunca linkado**. |
| **Watchtower** (escopo `archguard`) | Auto-atualização do core a partir da imagem. |
| **Reverse proxy** (Caddy/Traefik) | TLS obrigatório na borda. |

**Perfis de implantação (ADR-0017):** `dev` (custódia local, nega L3), `pilot`/`production`
(conforme, OpenBao). O perfil ativo aparece em `/api/v1/health` (subsistema `deployment`).

## 2. Deploy e atualização

Fluxo padrão (pull-based, sem build na VPS):

1. Push para `main` → **CI** (GitHub Actions) roda o gate e publica `ghcr.io/edsonmartins/archguard:latest`.
2. **Watchtower** (poll ~5 min, escopo `archguard`) detecta a imagem nova e **recria o core**.
3. No boot, o core roda as **migrations** pendentes (§4) e sobe os schedulers (publisher, reconciler).

**Verificação pós-deploy:**
```
curl -s -o /dev/null -w '%{http_code}\n' https://<host>/api/health        # liveness
# no navegador autenticado, /api/v1/health deve trazer todos os subsistemas 'ok'
```

**O que esperar:** a recriação do core **encerra o processo**, mas a **sessão sobrevive** (Redis),
então usuários logados não são deslogados. Migrations aditivas e idempotentes rodam sozinhas.

> ⚠️ **Endurecimento recomendado:** o auto-update irrestrito de `:latest` é conveniente para o
> piloto, mas para produção madura prefira **releases controlados** (tag/pin de versão + promoção
> deliberada) — evita que um push a `main` recrie produção sem janela.

## 3. Custódia e unseal do OpenBao (perfil conforme)

O core **não sobe** se a custódia não estiver disponível (fail-closed — INV-6/INV-7). O OpenBao
precisa estar **unsealed**.

- **Operação normal:** um sidecar de **auto-unseal** mantém o OpenBao destravado; o core espera por
  ele no boot.
- **Se o core entra em crashloop** logo após subir o OpenBao: quase sempre é OpenBao **selado**.
  Procedimento:
  1. Verifique o status: `bao status` (dentro do container do OpenBao) — `Sealed: true` confirma.
  2. Destravar com as **shares de unseal** (as N chaves do operador, fora da árvore) até atingir o
     threshold. **As shares não ficam no servidor** — são do operador.
  3. Confirme `Sealed: false` e o core reinicia sozinho (ou reinicie o serviço do core).
- Detalhe e o fix definitivo (sidecar + wrapper de espera no core) em `docs/RUNBOOK.md` §"Material de
  selagem" e no histórico de incidentes.

## 4. Migrations (ADR-0009)

- **Automáticas no boot**, executadas como o papel `archguard_migrate` (que tem `CREATE`; o
  `archguard_app` não), **após** o `Sync2` do XORM legado.
- Descobertas por `//go:embed migrations/*.sql`, ordenadas por número (`NNNN_nome.sql`), aplicadas uma
  vez (registro de versão). **Idempotentes.**
- **Nunca** rode migrations manualmente em produção fora do boot. Uma migração aditiva nova entra
  sozinha no próximo deploy.
- Tabelas novas criadas pelo `archguard_migrate` herdam `SELECT/INSERT/UPDATE/DELETE` para o
  `archguard_app` via `ALTER DEFAULT PRIVILEGES`. Tabelas de **auditoria** têm `UPDATE/DELETE/TRUNCATE`
  **revogados** (INV-2) — ao adicionar uma trilha append-only, reaplique o `deploy/postgres/roles.sql`.

## 5. Rotação de segredos

Procedimento geral (sem expor valores):

| Segredo | Onde | Como rotacionar |
|---|---|---|
| **Senha root da VPS** | infra | Trocar no provedor; atualizar o acesso do operador. **Rotacione qualquer credencial que tenha aparecido em canal não-seguro.** |
| **Client secret de uma Application** | console → Applications → app → Client secret | Regenerar; atualizar o consumidor (ex.: `oidc_client_secret` no OpenBao, `ARCHGUARD_SA_TOKEN` no console ArchGate). |
| **Chave de custódia (OpenBao)** | OpenBao | Rotação/rekey pelo procedimento do OpenBao; nunca extrair a chave. |
| **Keystore selado (dev)** | perfil dev apenas | Não usar em produção — migrar para OpenBao. |

## 6. Backup e Recuperação de Desastre

**Estado atual (gap conhecido):** o piloto **não** tem PITR automatizado. Até o item de endurecimento
ser executado, faça backup manual e teste o restore.

- **Mínimo hoje:** `pg_dump`/`pg_basebackup` do PostgreSQL em cadência definida, armazenado fora da VPS.
- **A auditoria é append-only e encadeada por hash** — o restore deve preservar a cadeia; validar a
  integridade após o restore (verificar cadeia).
- **Recomendado (endurecimento):** WAL archiving + PITR, com **drill de restore** periódico. Backup
  sem restore testado não é backup.
- Ver `docs/RUNBOOK.md` §"Backup e DR".

## 7. Saúde e sinais

- **Liveness:** `GET /api/health` → 200.
- **Subsistemas:** `GET /api/v1/health` (admin) → `database`, `custody`, `deployment`, cada um com
  status. Um subsistema crítico indisponível é **honesto** — a tela não mostra "tudo verde" se o
  perfil não entrega.
- **Observabilidade rica (métricas/traces/logs — pacote 010): pendente.** Até lá, os logs do core são
  a principal fonte; o publisher e o reconciler logam avisos (`authz publisher:` / `authz reconciler:`).

## 8. Runbook de incidentes

| Sintoma | Causa provável | Ação |
|---|---|---|
| Core em **crashloop** | OpenBao selado; migração falhando; Postgres inacessível | Ver logs do core; `bao status`; §3; conferir conectividade do banco |
| `/api/v1/*` responde **401** após deploy, mesmo logado | Sessão de domínio não estabelecida | Um **login fresco** reestabelece a ponte; se persistir, ver logs (`ponte de sessão`) |
| Decisão privilegiada **negada** com PDP indisponível | **Fail-closed esperado** (INV-6), não é bug | Restaurar o PDP/custódia; a negação é a postura correta |
| Reconciler **removendo tuplas** | Divergência sendo curada (esperado) OU um caminho de mutação bypassando a projeção | Ver `authz reconciler:` nos logs; se remoção inesperada e recorrente, investigar a fonte da verdade |
| Usuário **bloqueado** deveria ter acesso | `isForbidden`/membership revogado/suspenso | Conferir status do usuário/membership; reativar limpa e restaura o grafo |
| Console pede **step-up** e não conclui | Fluxo L3 (WebAuthn) não configurado no ambiente | Ver §L3 do guia do administrador; garantir credencial reforçada cadastrada |

## 9. Acesso emergencial (break-glass)

O ArchGuard tem um fluxo de **break-glass** auditado (concessão privilegiada com justificativa e
referência de incidente, aprovação por pares e janela de validade). É a via correta para acesso
excepcional — **não** existe senha-mestra (INV-1). Procedimento de uso no Guia do Administrador; a
trilha registra cada passo (imutável, INV-2).

---

*Ver também: `docs/RUNBOOK.md` (operação mínima), `docs/DEVOPS-HANDOFF.md` (detalhe de composição no
boot), e o Guia do Administrador para os fluxos de console.*
