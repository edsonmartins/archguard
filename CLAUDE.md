# CLAUDE.md — ArchGuard

Este arquivo é lido em toda sessão. Ele define **como trabalhar neste repositório**. Em caso de
conflito entre este arquivo e qualquer instrução de sessão, **este arquivo prevalece** — exceto
quando o `CONSTITUTION.md` disser o contrário, que é a autoridade máxima.

---

## 1. O que é este projeto

ArchGuard é o **plano de controle de identidade** da plataforma ArchGate (PAM — Privileged
Access Management) da IntegrAllTech. É um **fork direto do Casdoor** (Go + Beego + XORM,
Apache License 2.0) com governança própria.

O ArchGuard responde por: identidade, credenciais, MFA, sessão, emissão/revogação de tokens,
federação e auditoria.
O ArchGuard **não** responde por: proxy de protocolo, gravação de sessão ou brokering de
credencial — isso é Warpgate, Apache Guacamole, NetBird e OpenBao.

**Isto é software de segurança.** Um bug aqui não é um bug de UX: é um acesso privilegiado
indevido a produção de um cliente, ou uma trilha de auditoria que não prova nada. Trabalhe com
esse peso.

---

## 2. Ordem de leitura obrigatória

Antes de escrever qualquer linha de código em uma sessão:

1. `CONSTITUTION.md` — invariantes. Leia sempre. Não é opcional.
2. O(s) ADR(s) citado(s) no pacote da sessão — `docs/adr/`.
3. O(s) RFC(s) citado(s) — `docs/rfc/`.
4. O pacote OpenSpec da sessão — `openspec/changes/<pacote>/`:
   `proposal.md` → `design.md` → `specs/<capability>/spec.md` → `tasks.md`.

**A `spec.md` é o contrato.** Os cenários WHEN/THEN são os critérios de aceite. Se o código
passa nos testes mas viola um cenário da spec, o código está errado — não a spec.

Se um ADR/RFC contradisser outro, **pare e reporte**. Não escolha um. Contradição em corpus de
governança é defeito a corrigir, não ambiguidade a resolver por conta própria.

---

## 3. Invariantes que quebram o build (memorize)

Estes não são "boas práticas". São condições de rejeição automática:

| # | Invariante | Onde |
|---|---|---|
| **INV-1** | Nenhum caminho autentica um usuário com credencial que não seja dele. Senha-mestra **não existe** | I-4.1 |
| **INV-2** | Nenhum `UPDATE`/`DELETE` em tabela de auditoria, em nenhuma camada | I-5.2 |
| **INV-3** | Pacote sob `internal/domain/**` **não importa** framework web nem ORM | I-7.2 / ADR-0016 |
| **INV-4** | Nenhuma dependência AGPL/GPL/SSPL/BUSL na árvore de build | I-2.2 / ADR-0002 |
| **INV-5** | Nenhuma query a tabela de domínio sem predicado de tenant | I-6.3 |
| **INV-6** | **Não existe fail-open.** Falha de PDP, cofre ou auditoria ⇒ negação | I-5.4 / ADR-0005 |
| **INV-7** | Segredos e chaves privadas nunca no banco nem em log/telemetria | I-4.3 / ADR-0013 |
| **INV-8** | Toda operação da API declara nível de garantia (L1/L2/L3) | ADR-0010 |

A suíte de invariantes (`test/invariants/`) existe para detectar violação. **Se um teste de
invariante falhar, corrija o código — nunca o teste.** Se você acredita que o teste está errado,
pare e reporte; alterar teste de invariante exige emenda de ADR.

---

## 4. Fluxo de trabalho por tarefa

Trabalhe **uma tarefa por vez**, na ordem de `tasks.md`.

```
1. LER      → spec.md (cenários da tarefa) + design.md (como) + código existente
2. PLANEJAR → apresentar o plano ANTES de codar; aguardar confirmação em tarefa
              que toque INV-1..8, esquema de banco, criptografia ou fluxo de auth
3. TESTAR   → escrever o teste derivado dos cenários WHEN/THEN primeiro
4. CODAR    → implementação mínima que satisfaz os cenários
5. VERIFICAR→ rodar o gate (§5). Só então marcar [x] em tasks.md
6. COMMITAR → um commit por tarefa, mensagem referenciando o ID
```

**Não avance para a próxima tarefa com o gate vermelho.** Não acumule tarefas em um commit
gigante. Não marque `[x]` em tarefa cujo gate não passou — o `tasks.md` é registro de estado
real, não lista de desejos.

### Quando parar e perguntar

Pare e reporte, em vez de decidir sozinho:
- a spec não cobre um caso que você encontrou no código;
- ADR/RFC se contradizem;
- a implementação correta exigiria violar um invariante;
- a tarefa exige dependência nova (ver §7);
- você identificou uma questão em aberto listada no RFC (elas estão marcadas — não invente a
  resposta);
- o esquema de banco precisa divergir do RFC-0002/0003.

Reportar cedo custa uma mensagem. Adivinhar errado custa uma migration irreversível.

---

## 5. Gate de verificação ("pronto")

Uma tarefa só está pronta quando **todos** passam:

```bash
make lint          # lint + vet + formatação
make test          # unitários + integração
make invariants    # suíte de invariantes (INV-1..8)
make deps-check    # regra de dependência de pacotes (INV-3)
make sbom          # CycloneDX + license gate (INV-4)
make build         # binário + imagem
```

Além disso: os cenários WHEN/THEN da tarefa têm teste correspondente **executando de verdade**
(não `t.Skip`, não asserção trivial), e `tasks.md` foi atualizado.

**"Compilou" não é pronto. "Passou nos testes que eu mesmo escolhi" não é pronto.** Pronto é o
gate completo verde com os cenários da spec cobertos.

---

## 6. Convenções de código

### Estrutura
```
internal/domain/**       domínio puro — SEM Beego, SEM XORM (INV-3)
internal/adapters/**     implementações das portas (pgx, OpenFGA, OpenBao, OTLP)
internal/http/**         handlers finos: traduzem HTTP ↔ domínio, nada mais
test/invariants/**       suíte que quebra o build
docs/upstream/**         FORK_POINT.md, DIVERGENCE.md
```

### Regras
- **Handlers finos.** Nenhuma regra de negócio nova em controlador Beego.
- **Persistência nova em `pgx`** com SQL explícito. XORM só no código herdado do upstream.
- **Uma transação por operação de negócio.** Nunca abra transações independentes em camadas
  diferentes para a mesma operação (RFC-0002 §5).
- **Nunca chamada remota dentro de transação de banco.** Use outbox transacional (RFC-0004 §4).
- **Erros com contexto**, sem engolir. Distinga `denied` (decisão) de `error` (falha) — a
  auditoria depende dessa distinção.
- **Repositórios exigem contexto de tenant no construtor.** Consulta cross-tenant usa tipo
  distinto e explícito, autorizado e auditado.
- Idioma: **código, identificadores e comentários em inglês**; documentação, ADR/RFC/OpenSpec e
  mensagens ao usuário em **pt-BR**.

### Commits
```
feat(002): adiciona entidade membership com unicidade por tenant

Implementa T-002 do pacote 002-identity-multitenancy.
Cenários cobertos: "Pessoa em dois tenants", "Vinculação a nova organização".
```
Commits importados do upstream levam o trailer `Upstream-Commit: <sha>`.

---

## 7. Regras sobre o upstream e dependências

- **NUNCA** faça merge de branch do upstream em `main`. Apenas cherry-pick seletivo com o
  trailer (ADR-0003). `vendor/upstream` é espelho somente-leitura.
- Toda divergência estrutural que você criar deve ser registrada em `docs/upstream/DIVERGENCE.md`
  — sem isso, a triagem futura de cherry-pick fica cega.
- **Dependência nova exige aprovação explícita.** Antes de adicionar: verifique a licença contra
  a matriz do ADR-0002 e **pergunte**. Não adicione biblioteca "porque facilita".
- Prefira a biblioteca padrão e o que já está na árvore. Este é um produto de segurança: cada
  dependência é superfície de ataque e um risco de relicenciamento futuro.

---

## 8. Anti-padrões (não faça)

- ❌ Criar endpoint "só para a tela". Se o console precisa, é **API pública versionada**.
- ❌ Autorização no frontend. Esconder botão **não é** controle de acesso.
- ❌ Flag de configuração que permita fail-open, "modo permissivo" ou bypass de MFA.
- ❌ Dado pessoal, token ou segredo em log, trace ou métrica.
- ❌ `t.Skip`, mock que sempre retorna sucesso, ou teste que não exercita o cenário real.
- ❌ Migration sem classificação LGPD de campo pessoal (ADR-0014).
- ❌ Reescrever código herdado "para ficar melhor" fora do escopo da tarefa — cada reescrita
  gratuita encarece todo cherry-pick futuro.
- ❌ Criar README, docs ou arquivos auxiliares não pedidos. O corpus de governança já existe.
- ❌ Implementar mais de uma tarefa por commit.
- ❌ Marcar `[x]` sem o gate verde.

---

## 9. Stack de referência

| Camada | Escolha |
|---|---|
| Core | Go (herdado), Beego + XORM legado, `pgx` para código novo |
| Banco | **PostgreSQL 15+ apenas**, com RLS |
| PDP | OpenFGA (interface `PolicyDecisionPoint`) |
| Cofre | OpenBao via HTTP (**nunca** linkado — MPL-2.0, ADR-0012) |
| Console | React 19 + TS + Mantine v9 + Archbase, cliente gerado do OpenAPI |
| Observabilidade | OpenTelemetry → VictoriaMetrics / Loki / Tempo |
| Deploy | Docker Swarm + Traefik, TLS obrigatório |

---

## 10. Ordem dos pacotes

```
001 bootstrap-fork  →  002 identity-multitenancy  →  003 immutable-audit-trail
                                                          ↓
        005 mfa-step-up  →  004 privileged-access  →  007 authz-openfga
                                     ↓                      ↓
                          006 oidc-federation      008 admin-console
                                     ↓
                          009 directory-sync  →  010 observability-compliance
```

Não comece um pacote cujas dependências não estejam com o gate verde.
