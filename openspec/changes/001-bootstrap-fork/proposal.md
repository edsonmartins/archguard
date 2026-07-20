# Proposal — 001 · Bootstrap do fork ArchGuard

## Por quê

O ArchGuard começa como fork do Casdoor (ADR-0001). Antes de qualquer funcionalidade, é
preciso estabelecer a base: fork point congelado com rastreabilidade forense, obrigações de
licença cumpridas (ADR-0002), rebranding e remoção de escopo (ADR-0015), PostgreSQL como único
backend (ADR-0009), fronteiras de framework (ADR-0016) e a **suíte de invariantes** que
protege todas as decisões seguintes contra regressão por cherry-pick (ADR-0003).

Sem este pacote, todo trabalho posterior é construído sobre base não governada — e a
divergência do upstream vira dívida não rastreável em semanas.

## O que muda

- Fork criado, fork point congelado, `vendor/upstream` como espelho somente-leitura.
- `LICENSE` preservado; `NOTICE` acrescido de atribuição do ArchGuard.
- Rebranding completo; módulos fora de escopo removidos do build.
- Suporte a bancos não-PostgreSQL removido.
- Pipeline de CI com: build, testes, SBOM, license gate, regra de dependência de pacotes e
  suíte de invariantes.
- `DIVERGENCE.md` inicializado.
- Imagem de container e stack de implantação (Swarm + Traefik) mínimas e reprodutíveis.

## O que não muda

Comportamento funcional de autenticação herdado, exceto pelas remoções declaradas. Multi-tenancy,
auditoria imutável, MFA obrigatório e console novo são pacotes posteriores.

## Impacto

- **Bloqueia:** todos os demais pacotes.
- **Risco:** remoções amplas podem quebrar acoplamentos internos não óbvios do upstream.
- **Reversibilidade:** alta até o congelamento do fork point; média depois.
