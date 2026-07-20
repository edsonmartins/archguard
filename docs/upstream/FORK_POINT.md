# FORK_POINT.md — Registro forense do fork point

> **Estado: FORK POINT AINDA NÃO CONGELADO.** Este arquivo contém, por ora, a evidência de
> fonte primária exigida por T-001. O congelamento (T-002) só ocorre após aprovação da
> recomendação ao final.

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
ambiguidade entre "última release" e "último commit".

<!-- Seção a preencher em T-002, após aprovação:
## Fork point congelado
- Tag: | SHA: | Data: | Hash da árvore: | Data do congelamento: | Aprovado por:
-->
