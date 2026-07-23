# Contrato de claims OIDC do ArchGate — v1

**Versão:** `v1` (constante `domain.OIDCClaimsVersion`; claim `archguard_claims_version`).
**Base normativa:** RFC-0006 §3. **Autoridade:** este documento descreve; o RFC-0006 governa.
**Implementação de referência:** `internal/domain/oidc_claims.go` (`domain.OIDCClaims`).

O contrato é **agnóstico de fornecedor** (ADR-0001, §Reversibilidade): nenhuma particularidade do
fork vaza para os claims. Toda mudança de semântica de um claim v1 exige uma **nova versão** — a v1
nunca é redefinida silenciosamente. Extensões futuras usam *namespace* próprio.

## Claims

| Claim | JSON | Tipo | Obrigatório | Descrição |
|---|---|---|---|---|
| Emissor | `iss` | string | sim | Emissor do ArchGuard |
| Sujeito | `sub` | string | sim | Identificador **opaco e estável**. NUNCA e-mail |
| Audiência | `aud` | string | sim | Componente destinatário (um `aud` por token) |
| Expiração | `exp` | number | sim | Epoch (s) |
| Emissão | `iat` | number | sim | Epoch (s); `exp > iat` |
| Tenant ativo | `org` | string | sim | Organização ativa da sessão (ADR-0006) |
| Membership | `mid` | string | sim | Membership no tenant ativo |
| Garantia | `acr` | string | sim | `L1`\|`L2`\|`L3` (ADR-0010) |
| Métodos | `amr` | array | sim | `pwd`, `webauthn`, `otp`, `federated` (≥1) |
| Autenticação | `auth_time` | number | sim | Momento da autenticação (frescor de step-up) |
| Sessão | `sid` | string | sim | Sessão, usada em back-channel logout |
| Grupos | `groups` | array | não | Grupos normalizados do tenant ativo |
| Papéis | `roles` | array | não | Papéis do membership |
| Ator real | `act` | object | não | Delegação/impersonation (RFC 8693 / pacote 004): `{ "sub": ..., "act"?: {...} }` |
| Correlação | `pcid` | string | não | Correlação de sessão privilegiada — une as trilhas ArchGuard↔componente |
| Concessão | `grant_ref` | string | não | Referência da concessão temporária/break-glass, quando aplicável |
| E-mail | `email` | string | não | **Só sob escopo `email` explícito e justificado** (I-3.2) |
| Versão | `archguard_claims_version` | string | sim | `v1` |

## Regras invariantes

1. **Nenhum dado pessoal em claro** em claim (I-3.2). `sub` é opaco; `email` só sob escopo explícito.
2. Claims são **sempre do tenant ativo**. Um token nunca carrega `groups`/`roles` de outra organização.
3. Um token tem **um único `aud`** — o componente destinatário. Token de um componente não é aceito por
   outro (validação de audiência; RFC-0006 §5, ADR-0011).
4. `act` presente exatamente quando o token é de delegação (pacote 004); registra o ator real.
5. `pcid` presente quando o token abre/pertence a uma sessão privilegiada; é o que permite reconstruir
   a linha do tempo ponta a ponta.
6. Um claim set malformado (faltando obrigatório, `acr` inválido, versão errada) **não é assinado**
   (`domain.OIDCClaims.WellFormed` é o gate estrutural antes da emissão).

## Ciclo de vida (RFC-0006 §5, resumo)

| Token | TTL | Regra |
|---|---|---|
| Código de autorização | ≤ 60 s | Uso único, PKCE obrigatório |
| Access | 5–15 min | Escopo mínimo, `aud` específico |
| Refresh | Horas (política do tenant) | Rotação obrigatória + detecção de reuso |
| Sessão privilegiada | Duração da concessão | Revogável; expira com a concessão |
