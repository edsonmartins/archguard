# ADR-0023 — Endpoints OIDC v1: design da montagem, execução adiada

- **Status:** Aceito (2026-08-02)
- **Data:** 2026-08-02
- **Invariantes tocados:** INV-7 (segredos e chaves privadas nunca no banco/log), INV-8 (nível de
  garantia por operação), I-1.3 (core autentica sem serviços externos)
- **Relacionado:** RFC-0006 (contrato de claims v1, PKCE obrigatório, rotação de refresh com
  detecção de reuso, JWKS com `kid`), ADR-0005 (AuthN/AuthZ), ADR-0011 (uma audience por
  componente), ADR-0012/ADR-0014 (custódia de chaves), pacote 006

## Contexto

O pacote 006 implementa o contrato OIDC v1 no domínio + adapters: claims `org/mid/acr/amr/sid/pcid`,
PKCE obrigatório, rotação de refresh com detecção de reuso, JWKS com `kid`. Os handlers HTTP
existem e têm testes (`internal/http/oidc_{authorize,token,introspection,logout,discovery,server}.go`),
e o `OIDCServer.Handler()` já assembla tudo num único `http.Handler` montável sob um prefixo.

**Porém nada disso está montado.** O `internal/boot/mounts.go` registra 19 capacidades e nenhuma é
OIDC; quem serve OAuth é o **Casdoor legado** (`routers/router.go:297-299` → `GetOAuthToken`). O
próprio `tasks.md` do 006 registra o wiring como "deploy + pacote 008". Consequência: os consumidores
(hoje alcada/squadx; amanhã os componentes ArchGate) recebem **claims stock do Casdoor**
(`name`+`groups` via `tokenFormat: JWT`), **sem** `org/mid/acr` e **sem** rotação de refresh. O
contrato v1 é, hoje, documentação — não comportamento.

Uma auditoria do lado ArchGate (2026-08-01) levantou isso como "Bloqueador 1" e pediu: montar os
endpoints v1 no mux `/api/v1`, **ou registrar o adiamento num ADR**.

## Decisão

**Adiar a montagem dos endpoints OIDC v1**, executando-a depois como pacote próprio, e **registrar
aqui o design** para que a execução seja limpa. Até lá, os consumidores integram contra a superfície
**legada** do Casdoor (`tokenFormat: JWT` entrega `name`+`groups`, que é o que o ArchGate consome
hoje — ver a análise de formato de token abaixo). O contrato v1 rico permanece pendente e
documentado como tal.

### Por que adiar (e não montar agora)

A investigação (2026-08-02) mostrou que montar **não é fiação trivial** — é integração
multi-subsistema, com uma mudança de porta de custódia e risco sobre um IdP vivo:

1. **Keyset OIDC precisa persistir cifrado (INV-7).** `oidc/signer.GenerateSigningKey` gera uma RSA
   fresca; sem persistir, o `kid` muda a cada boot e tokens de vida ainda válida deixam de verificar.
   Persistir em tabela é a escolha (rotação com `kid`, estável entre boots), **mas a chave privada
   não pode ficar em claro no banco (INV-7)** — o `cert-archgate` guarda em claro só por ser legado
   Casdoor. Cifrar em repouso exige **estender `domain.KeyCustodian`** (que hoje só expõe `HashEmail`)
   com `Encrypt/Decrypt` genéricos, tocando os dois impls (keystore selado em dev, OpenBao em
   produção — ADR-0012/0014). É decisão de design de custódia, não wiring.

2. **`ClientRegistry` está vazio** e é distinto das `Application` do Casdoor. Precisa de um bridge
   `Application → OIDCClient` (mapeamento limpo: `clientId→ClientID`, `aud=clientId` — uma audience
   por componente, ADR-0011 —, `redirectUris`, `grantTypes→flows`, scopes).

3. **Coexistência num IdP vivo.** Montar o discovery v1 em `/.well-known/openid-configuration`
   substituiria o do legado ⇒ alcada/squadx que re-buscam o discovery migrariam sem querer. Exige
   estratégia de coexistência.

Os consumidores atuais só precisam de `name`+`groups`, que o legado entrega. Logo o custo/risco de
montar agora não se paga; o adiamento desbloqueia a ArchGate no legado sem tocar auth de IdP vivo.

## Design da execução futura (para o pacote de montagem)

1. **Keyset (`oidc_signing_key`).** Migração com a chave privada **cifrada** (`encrypted_private_key
   bytea`) via `KeyCustodian` estendido (`Encrypt/Decrypt`); colunas `kid`, `created_at`,
   `retired_at`. No boot: carrega a chave atual (decifra em memória para o `Signer`); se não houver,
   gera + cifra + persiste. A JWKS publica a(s) pública(s) por `kid`; rotação aposenta a antiga
   mantendo-a publicada enquanto houver token de vida válida.
2. **Bridge `Application → ClientRegistry`.** Adapter que lê as `Application` (org archgate e demais)
   e monta o `ClientRegistry` (uma `OIDCClient` por app; `Audience = clientId`). Fonte única da
   verdade: as applications do Casdoor.
3. **Assemblagem + mount.** Construir `BuildDiscoveryDocument(issuer, endpoints)` + os 6 handlers
   (authorize/token/introspect/logout/discovery/jwks) com seus adapters (authcode issuer/grant,
   refresh grant, introspection, endsession; `SessionResolver` = o `BridgingResolver` já existente)
   e montar `OIDCServer.Handler()` sob **`/api/v1/oidc/*`**, com discovery próprio em
   `/api/v1/oidc/.well-known/openid-configuration`. O **legado permanece intacto**
   (`/api/login/oauth/*`, `/.well-known` raiz); migração cliente a cliente, opt-in.
4. **Gate + validação.** `make conformance` verde + um fluxo Authorization Code + PKCE real por um
   cliente archgate, antes de apontar qualquer consumidor para o issuer v1.

## Consequências

- **Positivas:** ArchGate segue no legado (name+groups) sem espera; a montagem vira um pacote
  planejado, com o design fechado e o INV-7 endereçado desde a migração; nenhum risco sobre a
  superfície de auth do IdP vivo no curto prazo.
- **Negativas / dívida:** o contrato v1 rico (org/mid/acr/sid/pcid, rotação de refresh, detecção de
  reuso) **não está em produção** — todo consumidor que precise desses claims deve aguardar a
  montagem. Enquanto o legado servir, não há rotação de refresh nem detecção de reuso no caminho real.
- **Gatilho de revisão:** quando um consumidor exigir claims v1 (ex.: `org`/`mid` para decisão
  por-tenant, ou rotação de refresh com detecção de reuso), executar o pacote de montagem conforme o
  design acima.
