# Integração de aplicações via OIDC — ArchGuard

Guia prático para autenticar uma aplicação **web ou mobile (Flutter)** contra o ArchGuard, sem
reimplementar login. Complementa o contrato de claims (`CLAIMS-v1.md`) e a borda de componentes
(`GUACAMOLE-EDGE.md`).

> **Autoridade:** RFC-0006 governa o contrato OIDC; ADR-0010 (MFA obrigatório/step-up) e o INV-1
> (senha nunca transita fora do dono) governam a escolha de fluxo. Este documento descreve o
> caminho suportado; onde ele e o corpus divergirem, o corpus prevalece.

---

## 1. Recomendação (leia antes)

**Use Authorization Code + PKCE com a página de login hospedada do ArchGuard.** Todo o resto do seu
app é 100% custom — só o momento da credencial fica na página do ArchGuard. É a **única** forma que:

- mantém a **senha fora do seu app** (INV-1; sem superfície de phishing/roubo);
- impõe corretamente **MFA e step-up L3** (ADR-0010) — a página hospedada conduz a cerimônia;
- suporta **federação** (login via IdP corporativo do cliente);
- funciona igual em web e mobile.

A página de signin é **personalizável por aplicação** (logo, cores, CSS), então ela pode ter a
cara do seu app sem abrir mão da segurança.

> ⛔ **Não** construa um formulário de login próprio que envia usuário/senha para `POST /api/login`
> em contexto de **acesso privilegiado**. Isso põe a senha no seu app e obriga você a reimplementar
> MFA/step-up. Ver §8.

---

## 2. Endpoints (OIDC discovery)

Descoberta automática em `https://<host>/.well-known/openid-configuration` (ou por aplicação:
`/.well-known/<application>/openid-configuration`). No piloto (`https://app.archguard.com.br`):

| Função | Endpoint |
|---|---|
| Issuer | `https://app.archguard.com.br` |
| Authorization | `https://app.archguard.com.br/login/oauth/authorize` |
| Token | `https://app.archguard.com.br/api/login/oauth/access_token` |
| UserInfo | `https://app.archguard.com.br/api/userinfo` |
| JWKS | `https://app.archguard.com.br/.well-known/jwks` |
| Logout (end session) | `https://app.archguard.com.br/api/logout` |
| Refresh | `https://app.archguard.com.br/api/login/oauth/refresh_token` |
| Introspect | `https://app.archguard.com.br/api/login/oauth/introspect` |

- **PKCE:** `code_challenge_methods_supported = ["S256"]` — **obrigatório** (código de autorização
  é uso único, TTL ≤ 60 s).
- **Grant types anunciados:** `authorization_code`, `refresh_token`, `client_credentials`,
  `device_code`, `token-exchange` (e os legados `password`/`implicit` — **não use** para apps).
- **Scopes:** `openid`, `email`, `profile`, `address`, `phone`, `offline_access`.

---

## 3. Configurar a aplicação-cliente no ArchGuard (uma vez, no console)

Em **Applications → sua app** (ou via API `add-application`/`update-application`):

1. **Redirect URLs** (`redirectUris`): as URLs de callback exatas.
   - Web: `https://seu-app.exemplo.com/callback`
   - Flutter: um esquema custom, ex. `com.suaempresa.seuapp:/oauthredirect`
2. **Grant types** (`grantTypes`): habilite `authorization_code` e `refresh_token`. **Não**
   habilite `password`/`implicit`.
3. **PKCE:** o `authorize` aceita `code_challenge`/`code_challenge_method=S256` — mande sempre.
   Para app **público** (SPA/mobile) não há `client_secret`: PKCE é a proteção.
4. **Scopes:** `openid profile` no mínimo; `offline_access` se quiser refresh token; `email`
   **só se justificado** (I-3.2 — e-mail é dado pessoal).
5. **Claims custom** (ver §6): `Token format = JWT-Custom` e liste os campos em **`Token fields`**.

Anote o **Client ID** (`clientId`) — e o `clientSecret` apenas se a app for **confidencial**
(backend server-side); SPA e mobile são **públicos** e não usam secret.

---

## 4. Fluxo Web (Authorization Code + PKCE)

### 4.1 Passos

1. Gere `code_verifier` (aleatório) e `code_challenge = BASE64URL(SHA256(code_verifier))`.
2. Redirecione o usuário para o **authorization endpoint**:
   ```
   https://app.archguard.com.br/login/oauth/authorize
     ?client_id=<CLIENT_ID>
     &response_type=code
     &redirect_uri=https://seu-app.exemplo.com/callback
     &scope=openid%20profile
     &state=<aleatório-anti-CSRF>
     &code_challenge=<CHALLENGE>
     &code_challenge_method=S256
   ```
3. O usuário autentica na **página do ArchGuard** (senha + MFA/step-up conforme política). Volta
   para o `redirect_uri` com `?code=...&state=...`. **Valide o `state`.**
4. Troque o código por tokens no **token endpoint** (server-side de preferência):
   ```bash
   curl -s -X POST https://app.archguard.com.br/api/login/oauth/access_token \
     -H 'Content-Type: application/x-www-form-urlencoded' \
     -d grant_type=authorization_code \
     -d client_id=<CLIENT_ID> \
     -d redirect_uri=https://seu-app.exemplo.com/callback \
     -d code=<CODE> \
     -d code_verifier=<VERIFIER>
   # confidencial: acrescente -d client_secret=<SECRET>
   ```
   Resposta: `{ access_token (JWT), id_token (JWT), refresh_token?, expires_in, token_type }`.
5. **Valide o token** (§7) e leia os claims. Renove com o `refresh_token` (rotação obrigatória).

### 4.2 Gerar PKCE (browser, sem dependência)

```js
function b64url(bytes) {
  return btoa(String.fromCharCode(...bytes)).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}
const verifier = b64url(crypto.getRandomValues(new Uint8Array(32)));
const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(verifier));
const challenge = b64url(new Uint8Array(digest));
sessionStorage.setItem("pkce_verifier", verifier); // guarde para o passo 4
```

> Para SPA, prefira uma biblioteca madura (`oidc-client-ts`) que já cuida de PKCE, state, nonce,
> validação de JWKS e rotação de refresh — em vez de montar o fluxo à mão.

---

## 5. Fluxo Flutter / mobile

Use **`flutter_appauth`** (wrapper do AppAuth): o login abre a página hospedada no **navegador do
sistema** (Chrome Custom Tabs no Android / ASWebAuthenticationSession no iOS) — **nunca** em WebView
embutido (WebView expõe a credencial ao app e é rejeitado por muitos IdPs). PKCE é automático.

`pubspec.yaml`:
```yaml
dependencies:
  flutter_appauth: ^7.0.0
  flutter_secure_storage: ^9.0.0   # guarde o refresh_token no keychain/keystore
```

Login:
```dart
import 'package:flutter_appauth/flutter_appauth.dart';

const appId = 'com.suaempresa.seuapp';
final appAuth = const FlutterAppAuth();

final result = await appAuth.authorizeAndExchangeCode(
  AuthorizationTokenRequest(
    'SEU_CLIENT_ID',
    '$appId:/oauthredirect',            // = redirectUri registrado (esquema custom)
    issuer: 'https://app.archguard.com.br', // descobre os endpoints via discovery
    scopes: ['openid', 'profile', 'offline_access'],
    promptValues: ['login'],            // força re-autenticação quando necessário
    // PKCE é aplicado automaticamente pelo AppAuth
  ),
);

// result.accessToken (JWT), result.idToken, result.refreshToken, result.accessTokenExpirationDateTime
```

Registro do esquema de redirect:
- **Android** (`android/app/build.gradle`): `manifestPlaceholders += ['appAuthRedirectScheme': 'com.suaempresa.seuapp']`
- **iOS** (`Info.plist`): adicione `com.suaempresa.seuapp` em `CFBundleURLSchemes`.

Renovação (silenciosa):
```dart
final refreshed = await appAuth.token(TokenRequest(
  'SEU_CLIENT_ID', '$appId:/oauthredirect',
  issuer: 'https://app.archguard.com.br',
  refreshToken: storedRefreshToken,
  scopes: ['openid', 'profile', 'offline_access'],
));
```

Guarde o `refresh_token` em **armazenamento seguro** (keychain/keystore via
`flutter_secure_storage`), **nunca** em `SharedPreferences`.

---

## 6. Claims `org_id` / `pessoa_id` (e o contrato normativo)

Há duas camadas — saiba qual você está usando:

### 6.1 O que funciona hoje no endpoint OIDC padrão (JWT-Custom)

O token emitido em `/api/login/oauth/access_token` inclui **claims custom** quando a aplicação tem:

- `Token format = JWT-Custom`;
- `Token fields = ["org_id", "pessoa_id", ...]`.

O valor de cada campo vem das **`properties` do usuário**. Ao provisionar (via `add-user`/
`update-user` ou SCIM), grave:

```json
{ "name": "alice", "owner": "acme", "type": "normal-user",
  "properties": { "org_id": "acme", "pessoa_id": "b1f2..." } }
```

Resultado: o access/id token carrega `org_id` e `pessoa_id` como claims de topo. É o mecanismo que
o **alcada** usa em produção.

### 6.2 O contrato normativo (alvo) — `CLAIMS-v1.md`

O contrato **v1** (RFC-0006, pacote 006) padroniza claims **agnósticos de fornecedor**: `sub`
(opaco, nunca e-mail), `org` (tenant ativo), `mid` (membership), `acr` (`L1|L2|L3`), `amr`, `act`
(delegação), `pcid` (correlação de sessão privilegiada), `archguard_claims_version`. Prefira
consumir os claims **padrão** onde possível e trate `org_id`/`pessoa_id` como extensão da sua app.
Ver `docs/oidc/CLAIMS-v1.md` para o contrato completo e as regras invariantes (ex.: `sub` nunca é
e-mail; um token tem um único `aud`).

---

## 7. Validar o token (obrigatório)

Nunca confie no token sem validar. No seu backend/app:

1. Busque as chaves públicas em **`/.well-known/jwks`** (cacheie por `kid`).
2. Verifique a **assinatura** (a chave privada vive no cofre — OpenBao/transit; ADR-0012/0013).
3. Cheque `iss` == issuer esperado, `aud` == seu `client_id`/componente, `exp`/`iat` (com folga
   de clock), e `nonce` se você enviou.
4. Para operações sensíveis, cheque `acr` (`L2`/`L3`) e `auth_time` (frescor do step-up).

Use uma lib de JOSE/JWKS madura (não valide assinatura à mão).

---

## 8. Logout e segurança

- **Logout:** `end_session_endpoint` = `/api/logout` (encerra a sessão; back-channel logout usa o
  `sid` do token — ver CLAIMS-v1 §Sessão).
- **Por que não `POST /api/login` (formulário próprio) para acesso privilegiado:**
  1. **MFA é imposto** nesse caminho (o servidor responde `RequiredMfa`) — seu app teria que
     reimplementar senha → TOTP → step-up L3, e qualquer lacuna vira um caminho de auth mais fraco.
  2. Seu app passa a **manipular a senha crua** (phishing, roubo) — o oposto do modelo PAM (INV-1).
  3. **Sem federação** (login via IdP do cliente).
  `POST /api/login` só é aceitável para apps **não-privilegiadas** que aceitam essas limitações —
  **nunca** para o plano de controle nem para fluxos que abram sessão privilegiada.

---

## 9. Checklist de integração

- [ ] Aplicação criada no ArchGuard com `redirectUris` exatos e `authorization_code` + `refresh_token`.
- [ ] `Token format = JWT-Custom` + `tokenFields` se precisar de `org_id`/`pessoa_id`.
- [ ] Usuários provisionados com `properties.org_id`/`properties.pessoa_id` (se aplicável).
- [ ] Fluxo com **PKCE (S256)** e `state` validado; sem `client_secret` em app público (SPA/mobile).
- [ ] Mobile via `flutter_appauth` (navegador do sistema, não WebView); refresh token em storage seguro.
- [ ] Token **validado** (assinatura JWKS, `iss`, `aud`, `exp`, `acr` quando aplicável).
- [ ] Logout via `end_session_endpoint`.
