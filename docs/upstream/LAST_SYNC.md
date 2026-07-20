# LAST_SYNC.md — último ponto sincronizado com o upstream

> Registro do último commit do upstream (`vendor/upstream`) já triado/incorporado.
> O watcher (`make upstream-triage`, ADR-0003) lista os commits **novos desde este ponto** e
> os classifica para a fila de triagem. Atualize este arquivo após concluir uma rodada de
> triagem (ver o entregável do prompt de triagem em `PROMPT-CLAUDE-CODE.md`).

| Campo | Valor |
|---|---|
| **Último SHA sincronizado** | `50e77ade0ee902a2e375fa83a57c86fc452c0a45` |
| **Tag** | `v3.119.0` |
| **Data da sincronização** | 2026-07-20 |
| **Observação** | Igual ao fork point (FORK_POINT.md). Nenhum cherry-pick do upstream aplicado ainda; a linha `main` diverge por construção própria, não por importação. |
