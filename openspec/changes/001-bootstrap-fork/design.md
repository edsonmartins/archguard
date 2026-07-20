# Design — 001 · Bootstrap do fork ArchGuard

## Fork point

Congelar commit-base e tag do upstream. Registrar em `NOTICE` e em `docs/upstream/FORK_POINT.md`
com: SHA completo, tag, data, e hash de verificação da árvore. O fork point é ativo de valor
(ADR-0002, §2): o grant Apache 2.0 sobre esse código é irrevogável.

**Antes de congelar**, verificar na aba *Insights/Releases* do repositório upstream a release
corrente e a base de mantenedores — há divergência conhecida entre fontes secundárias sobre a
última tag publicada. A fonte primária decide.

## Topologia de repositório

```
vendor/upstream   espelho somente-leitura (nunca merged)
main              linha do ArchGuard
feature/*         um branch por pacote OpenSpec
```

Commits importados carregam trailer `Upstream-Commit: <sha>`.

## Rebranding

Nome, domínios, identificadores de pacote/módulo, cabeçalhos de resposta HTTP, assets e
strings de UI. **Não** remove atribuição de autoria do `NOTICE` nem cabeçalhos de copyright
existentes (ADR-0002). Arquivos modificados ganham linha de modificação; arquivos novos
recebem cabeçalho IntegrAllTech.

## Remoção de escopo (ADR-0015)

Ordem: (1) mapear dependências internas de cada módulo alvo; (2) remover em commits pequenos e
independentes; (3) build + testes a cada remoção; (4) registrar em `DIVERGENCE.md`.

Alvos: pagamento/produto/assinatura; funcionalidades de agentes IA/MCP; provedores fora do
catálogo curado; **senha-mestra** (por invariante I-4.1 — removida aqui, redesenho completo em
004).

## PostgreSQL único (ADR-0009)

Remover dialetos, configuração e documentação. Estabelecer migrations versionadas com
travamento de execução concorrente. Criar papéis segregados: aplicação, migração, leitura.

## Fronteiras de framework (ADR-0016)

Criar `internal/domain/**` sem importação de framework web ou ORM. Regra de dependência
verificada no CI. Nova persistência via `pgx`; XORM permanece apenas no código herdado.

## Suíte de invariantes

Testes que **quebram o build** se:
- existir caminho de autenticação por credencial que não seja do próprio usuário
  (senha-mestra reintroduzida);
- existir endpoint ou código que faça `UPDATE`/`DELETE` em tabela de auditoria;
- um pacote de domínio importar framework web/ORM;
- uma dependência estiver fora da matriz de licenças (ADR-0002).

Esta suíte é a defesa real da estratégia de cherry-pick. Sem ela, o ADR-0003 é intenção, não
controle.

### Nota transitória — violações herdadas de INV-1 (decisão de 2026-07-20)

A suíte nasce no Bloco 2, antes das remoções do Bloco 3; a senha-mestra herdada só sai em
T-011. Para não suspender a disciplina de gate verde durante o intervalo, existe
`test/invariants/known_violations.txt` com **três travas estruturais**:

1. **Correspondência exata** — cada entrada identifica `arquivo:símbolo` específicos. Glob,
   regex ou padrão de diretório são **proibidos**: absorveriam violação nova em silêncio.
2. **Detecção de entrada obsoleta** — entrada listada que não corresponda mais a violação
   real no código faz a suíte **falhar**. O arquivo não pode apodrecer.
3. **Autodestruição** — escopo exclusivo de **INV-1**. T-011 inclui a subtarefa de deletar o
   arquivo; após T-011, a **existência** do arquivo é, ela própria, violação (a suíte falha
   se ele existir vazio ou com entradas obsoletas).

Nenhum dos outros sete invariantes recebe allowlist, em nenhuma circunstância.

### Nota transitória — baseline de lint herdado (decisão de 2026-07-20)

Achados de `go vet` **herdados do upstream e triados como estilo** vivem em
`lint-baseline.txt`, verificado por `tools/lintbaseline` dentro de `make lint`. Quatro travas
— análogas às do `known_violations.txt`, mas com regra de saída própria (não expira numa
tarefa):

1. **Correspondência exata** — `arquivo:linha:check`. Sem glob, sem supressão por pacote.
2. **Falha em entrada obsoleta** — entrada sem achado real correspondente quebra o build.
3. **Só encolhe** — nenhuma entrada nova após o fechamento do pacote 001. Código novo nasce
   limpo, sem exceção.
4. **Limpeza ao tocar** — arquivo modificado por qualquer tarefa futura sai do baseline
   naquele mesmo commit. *Boy scout ao tocar, nunca ao ver.*

Triagem registrada no cabeçalho do próprio arquivo. Defeito real herdado **não** é
baselinável: vira micro-tarefa.

### Distinção: gate local verde × gate imposto (decisão de 2026-07-20)

Dois conceitos que estavam colados e precisam ficar separados:

| Conceito | O que é | Estado | Papel |
|---|---|---|---|
| **Gate local verde** | `make lint/test/invariants/deps-check/sbom/build` verdes na máquina | **Existe hoje** | Disciplina de tarefa; autoriza marcar `[x]` |
| **Gate imposto** | Impossibilidade **mecânica** de merge com gate vermelho (status check obrigatório na forja) | Pende de forja + ADR-0018 (T-003 / T-019b) | Protege contra **esquecimento humano**, não contra ausência de teste |

Consequência: a antecipação do Bloco 2 já cumpriu seu propósito — a suíte existe e é verde, então a remoção da senha-mestra (T-011) nasce verificada por teste. O que falta (imposição pela forja) é item de fechamento do T-003/T-019b, **não pré-requisito das remoções do Bloco 3**.

### Divisão do T-019 (decisão de 2026-07-20)

- **T-019a — implementação** (não depende da forja; liberada): detectores de transição MPL do
  ADR-0019, regra de licença dual, geração de SBOM CycloneDX + license gate como alvo local,
  módulo de ferramentas separado com versões fixadas, fail-closed em licença desconhecida.
  Os detectores de transição MPL são implementados agora e ficam **dormentes quanto à
  semântica de permissão**: são necessários sob os dois regimes (vigente = MPL proibida;
  ADR-0019 = MPL não modificada permitida). Ratificado o ADR-0019, troca-se a **classificação**,
  não o detector.
- **T-019b — imposição pela forja**: gate vira status check obrigatório. Permanece bloqueada
  (forja provisionada + ADR-0018 ratificado).

### Condições do Bloco 3 (decisão de 2026-07-20)

a) `make invariants` e `make deps-check` verdes localmente **antes de cada `[x]`**, sem exceção.
b) T-011 executada com o detector INV-1 ativo e o `known_violations.txt` **deletado no mesmo
   commit** — a suíte passa a falhar se ele reaparecer (trava (c) da nota transitória).
c) Toda remoção que elimine achado de licença **reduz a contagem do INV-4 de forma
   verificável**: contagem antes/depois registrada em cada commit (prova de remoção real, não
   cosmética).
d) Nenhum `[x]` em T-003 ou T-019b até as ratificações (ADR-0018 e ADR-0019).

## CI

Etapas: lint → build → testes → **regra de dependência** → **suíte de invariantes** → SBOM
(CycloneDX) → **license gate** → imagem assinada. Nenhuma etapa pode ser pulada por flag.

O license gate (T-019) inclui, além da matriz de licenças:

- **Detectores de transição MPL** (ADR-0019 §II.3) — qualquer um ⇒ vermelho:
  (a) hash do módulo MPL difere do proxy oficial; (b) directive `replace` apontando para
  cópia local de módulo MPL; (c) vendorização alterada de arquivo coberto.
- **Licença dual** (decisão de 2026-07-20): dependência dual-licenciada exige **eleição
  explícita** da licença adotada, registrada no `NOTICE` e no SBOM. Dual sem eleição
  registrada = licença desconhecida ⇒ fail-closed. Caso `freetype`: eleger **FTL**
  (permissiva); se a FTL não for elegível, a dependência sai — GPLv2 não tem leitura
  permissiva. A eleição é feita na verificação manual do T-010, registrada, não apenas
  "verificada".
