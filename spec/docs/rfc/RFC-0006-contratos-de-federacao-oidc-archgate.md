# RFC-0006 — Contratos de federação OIDC do ArchGate

- **Status:** Proposto
- **Data:** 2026-07-19
- **ADRs relacionados:** ADR-0011, ADR-0010, ADR-0006, ADR-0008, ADR-0012

## 1. Objetivo

Especificar o contrato de claims, o ciclo de vida de tokens, a propagação de nível de garantia,
o logout e a correlação de auditoria entre o ArchGuard e os componentes do ArchGate. Este
contrato é deliberadamente **agnóstico de fornecedor**: é o que mantém contido o custo de
eventual troca do IdP (ADR-0001, §Reversibilidade).

## 2. Perfis suportados

| Componente | Fluxo | Observação |
|---|---|---|
| Warpgate | Authorization Code + PKCE | SSO web e sessões de bastião |
| Apache Guacamole | Authorization Code (extensão OIDC) | Limitações da extensão exigem adaptação de borda |
| NetBird | Authorization Code + PKCE; **Device Authorization Grant** para clientes sem navegador | |
| OpenBao | *auth method* JWT/OIDC | Mapeamento de claims → políticas do cofre |
| Proxy Oracle JDBC (Java) | Validação de JWT (JWKS) | Sem fluxo interativo próprio |
| Produtos IntegrAllTech | Authorization Code + PKCE | |

**PKCE obrigatório** em todo fluxo interativo. *Implicit* e *Resource Owner Password
Credentials* **não são suportados**.

## 3. Contrato de claims (v1)

| Claim | Tipo | Descrição |
|---|---|---|
| `iss` | string | Emissor do ArchGuard |
| `sub` | string | Identificador **opaco e estável** da identidade global. Nunca e-mail |
| `org` | string | **Tenant ativo** da sessão (ADR-0006) |
| `mid` | string | Identificador do membership no tenant ativo |
| `acr` | string | Nível de garantia obtido (`L1`, `L2`, `L3` — ADR-0010) |
| `amr` | array | Métodos usados (`pwd`, `webauthn`, `otp`, `federated`) |
| `auth_time` | number | Momento da autenticação — base para frescor de step-up |
| `sid` | string | Sessão, usada em back-channel logout |
| `groups` | array | Grupos **normalizados** do tenant ativo |
| `roles` | array | Papéis do membership |
| `act` | object | Ator real em delegação/impersonation (RFC 8693) — ADR-0008 |
| `pcid` | string | **Correlação de sessão privilegiada** — une trilhas do ArchGuard e dos componentes |
| `grant_ref` | string | Referência da concessão temporária/break-glass, quando aplicável |

Regras:
- **Nenhum dado pessoal em claro** em claim (I-3.2). `email` só é liberado com escopo
  explícito, para componente que comprovadamente precise.
- Claims são **por tenant ativo**. Um token nunca carrega grupos ou papéis de outra
  organização.
- Extensões futuras usam *namespace* próprio; claims da v1 não mudam de semântica sem nova
  versão do contrato.

## 4. Nível de garantia e recusa pelo componente

- O ArchGuard emite `acr` refletindo o que foi efetivamente comprovado.
- **Cada componente declara o `acr` mínimo** para suas operações e **deve recusar** token com
  garantia inferior. Componente que ignora `acr` é não conforme e é registrado como risco
  aceito, com compensação (ex.: o ArchGuard só emite token para aquele componente após step-up).
- Frescor: operações L3 exigem `auth_time` recente; o componente valida a janela.

## 5. Ciclo de vida de tokens

| Token | TTL padrão | Regra |
|---|---|---|
| Código de autorização | ≤ 60 s | Uso único, PKCE obrigatório |
| Access token | 5–15 min | Escopo mínimo, audiência específica do componente |
| Refresh token | Horas, por política do tenant | **Rotação obrigatória** e detecção de reuso |
| ID token | Curto | Não usado como credencial de acesso |
| Token de sessão privilegiada | Duração da janela aprovada | Revogável; expira com a concessão |

**Detecção de reuso de refresh**: reuso de token rotacionado ⇒ revogação de toda a família de
tokens da sessão + evento de auditoria de severidade alta.

**Audiência específica**: cada componente recebe token com `aud` próprio. Token de um
componente não é aceito por outro (ADR-0011).

## 6. Revogação e logout

- **Back-channel logout OIDC** para todos os componentes que suportem: encerrar a sessão no
  ArchGuard encerra as derivadas.
- Componentes sem suporte recebem revogação por **introspecção com TTL curto** — e isso é
  documentado como limitação, com TTL reduzido como compensação.
- Revogar o membership, suspender a identidade ou expirar a concessão dispara revogação
  imediata das sessões correspondentes.
- **Revogação que não propaga é revogação fictícia** — teste de conformidade obrigatório
  (§8).

## 7. Chaves e descoberta

- `/.well-known/openid-configuration` e JWKS publicados; chaves custodiadas no OpenBao
  (ADR-0012).
- Rotação com **sobreposição** maior que o TTL máximo de token; múltiplas chaves publicadas
  simultaneamente; `kid` sempre presente no cabeçalho.
- Componentes **devem** honrar cache do JWKS com renovação em `kid` desconhecido.

## 8. Suíte de conformidade (gate de release)

Para **cada** componente, teste automatizado que valida: login completo; presença e semântica
dos claims; recusa quando `acr` insuficiente; comportamento na rotação de chave; back-channel
logout efetivo; e correlação de auditoria pelo `pcid` (evento no ArchGuard + evento no
componente). Falha em qualquer item bloqueia o release (I-9.4).

## 9. Riscos conhecidos

| Risco | Mitigação |
|---|---|
| Extensão OIDC do Guacamole com suporte limitado a claims/logout | Adaptação em camada de borda do próprio Guacamole; TTL curto; nunca degradar o contrato central |
| Device flow do NetBird em dispositivos sem navegador dificulta step-up | Política: operações L3 não são autorizadas por device flow |
| Mapeamento de claims → políticas do OpenBao divergir do modelo de papéis | Geração do mapeamento a partir da mesma fonte, com teste de conformidade |
| Componente ignorar `acr` | Compensação por emissão condicionada + registro de risco aceito |
