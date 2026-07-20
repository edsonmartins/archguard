# DIVERGENCE.md — Registro de divergência estrutural em relação ao upstream

> Exigido pelo ADR-0003. Todo desvio estrutural entra aqui com subsistema e motivo; a triagem
> de cherry-pick cruza cada commit do upstream com esta tabela. Commit que toque subsistema
> divergente exige revisão manual.

## Topologia do fork (nota permanente)

`main` nasceu do corpus de governança e recebeu a árvore do fork point (`v3.119.0` =
`50e77ade`) por **um único merge de bootstrap** no T-002, autorizado em 2026-07-20. Esse merge
existe para dar a `main` ancestralidade com o upstream — é o que fornece base de merge aos
cherry-picks do ADR-0003. Ele é o ato de criação do fork, não sincronismo: **a proibição de
merge do ADR-0003 vale integralmente daqui em diante** — importação futura é exclusivamente
cherry-pick com trailer `Upstream-Commit: <sha>`.

## Divergências

| Data | Subsistema / caminho | Divergência | Motivo | Referência |
|---|---|---|---|---|
| 2026-07-20 | `README.md` | README do upstream movido intacto (sem remoção de avisos) para `docs/upstream/README-upstream.md`; raiz passa a ter o índice do corpus de governança | O README raiz é o índice normativo do ArchGuard; atribuição preservada conforme ADR-0002. Cherry-pick futuro que toque `README.md` exige revisão manual | T-002, ADR-0002 |
| 2026-07-20 | `Makefile` | Makefile do upstream movido intacto para `docs/upstream/Makefile-upstream`; raiz passa a ter o Makefile do gate (CLAUDE.md §5) | O gate de verificação é contrato do método (I-9.4); alvos úteis do upstream serão absorvidos em T-019 | T-002, T-000c |
| 2026-07-20 | `.gitignore` | União do `.gitignore` do upstream com entradas do ArchGuard | Preservar padrões de build do upstream reduzindo conflito de cherry-pick | T-002 |
| 2026-07-20 | raiz do repositório | Arquivos de governança adicionados: `CLAUDE.md`, `CONSTITUTION.md`, `docs/adr/`, `docs/rfc/`, `openspec/`, `docs/upstream/` | Método SDD do ArchGuard (I-9.1); inexistentes no upstream, sem sobreposição de caminho | pacote 001 |
