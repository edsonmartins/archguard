# ADR-0022 — GlobalAuthorizer real: acesso cross-tenant self-confinado vs amplo

- **Status:** Aceito (2026-07-30)
- **Data:** 2026-07-30
- **Invariantes tocados:** I-1.3 (core autentica sem serviços externos), INV-6 (fail-closed),
  I-5.4 (acesso não auditável não acontece), INV-1 (só lê o que é do próprio dono)
- **Relacionado:** ADR-0005 (separação AuthN/AuthZ; OpenFGA como PDP **granular**), RFC-0002 §4
  (acesso cross-tenant carrega principal + motivo)

## Contexto

Toda leitura que **atravessa tenants** (ex.: "todos os meus memberships") passa pela porta
`domain.GlobalAuthorizer` antes de o `GlobalRepository` ativar a flag de leitura global (RLS).
Hoje a única implementação é o `ProfileAuthorizer` **provisional**: permite no perfil `dev` e
**nega em qualquer perfil conforme** (pilot/production), como placeholder até uma política real.

Isso quebra o login em perfil conforme. O `EstablishSession` (ponte de login) lê os memberships
**da própria identidade** via `WithGlobalTx`; o provisional nega ⇒ a sessão do `/api/v1` nunca é
estabelecida ⇒ todas as telas do plano de controle ficam indisponíveis. Foi o observado no
piloto (perfil `production`, sem authorizer real): `T-004b: ponte de sessão de domínio falhou
… PDP de acesso cross-tenant não configurado`.

Dois pontos do corpus enquadram a solução:

- **I-1.3 / ADR-0005:** o core deve **autenticar mesmo sem serviços externos**; o PDP não é
  dependência dura do login. Logo, o gate cross-tenant do login **não pode** depender do OpenFGA
  (que é o PDP **granular de recursos**, e explicitamente **opcional** — ADR-0005/RFC-0004 §7).
  O provisional, ao negar em conforme, na prática **viola I-1.3**.
- Os **únicos dois** usos reais de `WithGlobalTx` são **self-confinados**: o próprio usuário
  lendo seus próprios memberships (login e seletor de tenant do console). Não há, hoje, leitura
  cross-tenant **ampla** em produção.

## Decisão

**1. O `GlobalAccess` declara o ESCOPO do acesso.** Além de `Principal` e `Reason`, passa a
carregar um escopo:

- **`self`** — a leitura é **confinada à identidade do próprio principal** (login/console lendo
  os próprios memberships). A garantia INV-1 já vive no call-site, que consulta exclusivamente a
  identidade autenticada (resolvida da sessão, nunca do request).
- **`cross-tenant`** — leitura **ampla** entre tenants (relatórios globais, admin lendo dados de
  terceiros).

O **default é `cross-tenant`** (o mais restrito): esquecer de declarar cai no caminho conservador,
nunca no permissivo (fail-safe).

**2. `GlobalAuthorizer` real, em Go embutido (sem serviço externo — honra I-1.3):**

- **`self`:** permitido em **qualquer perfil**. É intrínseco ao login/console e não é acesso
  cross-tenant arbitrário.
- **`cross-tenant`:** **fail-closed** em perfil conforme até existir política real (papel de
  operador global / delegação ao PDP) — nega, **nunca** fail-open (INV-6). Em `dev`, permitido.

**3. Todo acesso é auditado de forma DURÁVEL** (`AccessAuditor` sobre a trilha imutável do
pacote 003) **antes** de rodar (I-5.4). O `MemoryAuditor` (não durável) fica restrito a dev/teste.

**4. O boot escolhe por perfil:** real + auditor durável em conforme; provisional + memory em
dev. A escolha é centralizada (hoje está inline nos call-sites do `GlobalRepository`).

## Consequências

- **Login/console voltam a funcionar em perfil conforme, sem serviço externo** — I-1.3 honrado.
- **Cross-tenant amplo permanece fail-closed em produção** até haver política real — sem
  regressão de segurança; o caminho perigoso continua negado por padrão.
- **OpenFGA (PDP granular de recursos) fica fora do caminho de login** e permanece trabalho
  separado e opcional (ADR-0005). Este ADR **não** o introduz nem o exige.
- O `GlobalAccess` ganha um campo; call-sites de leitura própria declaram `self` explicitamente,
  tornando a intenção **auditável** (o motivo já era exigido; o escopo passa a ser também).

## Alternativas descartadas

- **Implantar OpenFGA para o gate de login** — viola I-1.3 (dependência externa no login) e
  confunde as portas: OpenFGA é o PDP **granular de recursos**, não o gate **cross-tenant**.
- **Rebaixar o piloto para o perfil `dev`** — o provisional permitiria, mas `dev` nega L3 e usa
  custódia local (não OpenBao); é retrocesso de segurança e "não suportado em produção" (ADR-0017).
- **Permitir todo cross-tenant desde que auditado** — fraco demais: um acesso amplo em produção
  não deve passar sem política, apenas por estar auditado.
