# FORK_POINT.md — Registro forense do fork point

> **Estado: FORK POINT CONGELADO em 2026-07-20** (T-002). A evidência de fonte primária
> (T-001) que fundamentou o congelamento está preservada abaixo.

## Fork point congelado

| Campo | Valor |
|---|---|
| **Tag** | `v3.119.0` |
| **SHA completo** | `50e77ade0ee902a2e375fa83a57c86fc452c0a45` |
| **Data do commit** | 2026-07-18T00:58:16+08:00 |
| **Hash da árvore** | `291daef8c3a63747bee99e23b6439e44d0aa479c` |
| **Data do congelamento** | 2026-07-20 |
| **Aprovado por** | Edson Martins (Arquiteto de Software e Soluções), com aval registrado em sessão: convergência release-"latest"/tip-de-master elimina a ambiguidade tag-vs-commit; LICENSE Apache-2.0 íntegro e idêntico entre tag e master satisfaz o ADR-0002 |

**Natureza da tag (declaração do que a evidência prova):** `v3.119.0` é **tag leve e não
assinada** — aponta diretamente para o commit, sem objeto de tag anotado e sem assinatura
PGP. Ela prova apenas que o mantenedor nomeou este commit; **a prova forte deste registro é o
SHA do commit e o hash da árvore**, verificados localmente sobre os objetos git trazidos do
upstream, não a tag em si. A materialização em `main` referencia o SHA, não a tag.

**Risco de mantenedor único (consequência operacional):** a manutenção do upstream tem
*bus factor* 1 (`hsluoyz`: 1662 commits; segundo colocado: 311). Consequência aceita por
desenho: se o upstream estagnar, o ADR-0003 degrada de "triagem semanal" para **fork
soberano** sem alterar nenhuma decisão arquitetural — este é o comportamento pretendido, não
um risco a mitigar. O grant Apache 2.0 **irrevogável** sobre `50e77ade` (ADR-0002, §2) é o
que torna essa degradação segura.

> Nota de processo: a due diligence jurídica não bloqueou este congelamento — ela é
> pré-requisito de M1/GA (ADR-0001), e o risco jurídico nasce na distribuição, não no
> registro de um commit.

## Evidência de fonte primária (T-001)

- **Data da verificação:** 2026-07-20
- **Upstream:** `https://github.com/casdoor/casdoor`
- **Método:** (a) fetch completo do repositório upstream (objetos git, histórico e 1954 tags)
  para o espelho local `vendor/upstream`; (b) consulta à API do GitHub (aba Releases e base de
  contribuidores).

### Release corrente

Havia divergência conhecida entre fontes secundárias sobre a última tag publicada. A fonte
primária resolve: **as duas vias convergem em `v3.119.0`**.

| Fonte | Resultado |
|---|---|
| Aba Releases (API `releases/latest`) | `v3.119.0`, publicada em 2026-07-17T17:07:36Z |
| Tags do repositório (fetch direto) | Maior tag: `v3.119.0` (criada 2026-07-18 00:58 +08:00) |
| Tip de `master` no momento do fetch | `50e77ade0ee902a2e375fa83a57c86fc452c0a45` — **o mesmo commit apontado por `v3.119.0`** |

- **SHA completo do commit de `v3.119.0`:** `50e77ade0ee902a2e375fa83a57c86fc452c0a45`
- **Data do commit:** 2026-07-18T00:58:16+08:00
- **Assunto:** `feat: populate user last_signin_time and last_signin_ip on login`
- **Hash da árvore (`^{tree}`):** `291daef8c3a63747bee99e23b6439e44d0aa479c`

Cadência do upstream confirmada: 1954 tags no repositório; três releases entre 14 e 18 de
julho de 2026 (`v3.117.0`, `v3.118.0`, `v3.119.0`) — consistente com o pressuposto do
ADR-0003 (micro-releases por semantic-release).

### Licença vigente

- Arquivo `LICENSE` em `v3.119.0`: **Apache License, Version 2.0** — íntegro.
- **SHA-256 do `LICENSE`:** `c71d239df91726fc519c6eb72d318ec65820627232b2f796219e87dcf35d0ab4`
  (hash canônico do texto Apache-2.0, sem alterações).
- `LICENSE` idêntico entre `v3.119.0` e o tip de `master` (verificado por `git diff`).
- Licença declarada pelo GitHub (API): `Apache-2.0`. **Sem indício de relicenciamento.**

### Base de mantenedores

Top contribuidores (API `contributors`): `hsluoyz` (1662), `dacongda` (311), `nomeguy` (292),
`leo220yuyaodog` (178), demais abaixo de 60. Repositório ativo (último push 2026-07-17;
~14,0k estrelas; 105 issues abertas).

**Observação de risco:** a manutenção é fortemente concentrada em um mantenedor principal
(`hsluoyz`). Isso reforça a estratégia de governança própria do fork (I-1.2, ADR-0003): o
upstream é fonte de correções, não de direção — e a concentração aumenta o valor do fork
point como ativo (ADR-0002, §2).

## Recomendação para o congelamento (T-002)

Congelar o fork point em **`v3.119.0` = `50e77ade0ee902a2e375fa83a57c86fc452c0a45`**: é
simultaneamente a release corrente publicada e o tip de `master`, eliminando qualquer
ambiguidade entre "última release" e "último commit". *(Recomendação aprovada e executada em
2026-07-20 — ver seção "Fork point congelado" acima.)*
