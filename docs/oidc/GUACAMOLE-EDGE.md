# Adaptação de borda — Apache Guacamole (OIDC)

**Base normativa:** RFC-0006 §9 (riscos conhecidos) e o design 006 (§Adaptação sem contaminação).
**Princípio inegociável:** o contrato central de claims/tokens (RFC-0006 §3–§7) **nunca** é
degradado para acomodar a extensão OIDC do Guacamole. Toda compensação vive **na borda do próprio
Guacamole** — um shim/reverse-proxy à frente da instância — não no ArchGuard.

## Limitações da extensão OIDC do Guacamole

| Limitação | Compensação na borda |
|---|---|
| Sem **back-channel logout** confiável | Revogação por **introspecção de TTL curto** (`domain.RecommendedIntrospectionTTL`, 30s): a borda reintrospecta o token a cada requisição/curto intervalo; sessão revogada no ArchGuard → `active:false` → a borda encerra a sessão Guacamole. Documentado como limitação **com** compensação. |
| Suporte a claims restrito (mapeamento simples) | A borda lê os claims do contrato (`org`, `mid`, `acr`, `roles`, `groups`) e os apresenta ao Guacamole no formato que sua extensão entende — **traduzindo**, sem pedir ao ArchGuard para emitir claims fora do contrato. |
| Sem enforcement de `acr` próprio | A borda **recusa** a requisição quando `acr` do token é insuficiente para a operação e redireciona a step-up no ArchGuard (RFC-0006 §4). Componente que ignora `acr` é risco aceito registrado; a borda é a compensação. |
| Sem honra automática do cache do JWKS em `kid` desconhecido | A borda renova o JWKS no ArchGuard ao ver um `kid` ausente do cache antes de rejeitar (RFC-0006 §7). |

## O que a borda faz (resumo)

1. **Valida o JWT** (assinatura via JWKS do ArchGuard, `aud == guacamole`, `exp`, `iss`).
2. **Introspecta com TTL curto** para propagar revogação (sem back-channel logout).
3. **Enforce `acr`** por operação; redireciona a step-up quando insuficiente.
4. **Traduz claims** para o formato da extensão do Guacamole.
5. **Correlaciona** eventos pelo `pcid` (mesma linha do tempo do ArchGuard).

`domain.GuacamoleEdgeConfig` deriva os parâmetros da borda (TTL de introspecção, audiência,
claims a traduzir) do próprio contrato e do registro do cliente — nada é hard-coded fora do
contrato.
