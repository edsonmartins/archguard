# Design — 009 · Sincronismo e federação de entrada

Base normativa: RFC-0007.

## Conector LDAP/AD

Sincronização incremental por organização, com **filtro de escopo obrigatório** (sincronizar
"toda a árvore" é proibido). Mapeamento diretório→ArchGuard versionado: atributos, grupos,
escopo.

Precedência declarada: para identidades sincronizadas, o diretório é autoritativo para
atributos e pertencimento a grupos; **papéis e concessões privilegiadas são sempre do
ArchGuard** — jamais derivados automaticamente de grupo de diretório sem mapeamento explícito
e aprovado.

Desativação no diretório ⇒ **suspensão do membership**, nunca deleção (preserva a trilha).

Credenciais do conector no cofre.

## SCIM 2.0 de entrada

ArchGuard como alvo: criação, atualização e desativação de usuários e grupos por IdP do cliente.
Operações mapeiam para identidade + membership, respeitando deduplicação por `email_hash`.
Saída (ArchGuard provisionando terceiros) está fora do escopo v1.

## Federação de login

SAML 2.0 e OIDC contra o catálogo curado (ADR-0015). *JIT provisioning* cria membership para
identidade existente quando o e-mail já é conhecido — **nunca** identidade duplicada.

**Regra dura:** o `acr` de terceiro é informativo. Operações L3 exigem fator verificado pelo
próprio ArchGuard. Delegar isso anularia o controle sobre acesso privilegiado.

## Canais legados

LDAP e RADIUS embutidos: desabilitados por padrão, escopo mínimo, auditados e **sinalizados
como canal legado**. Não carregam `acr` nem correlação — portanto **nunca** autorizam L3.

## Importação

Nenhuma senha é importada. Identidade importada entra em `enrollment_required`. Deduplicação
com relatório de conflito; fusão automática silenciosa é proibida.
