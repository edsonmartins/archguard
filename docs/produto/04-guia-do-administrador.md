# ArchGuard — Guia do Administrador (Console)

> Público: administradores que operam identidade e acesso pelo **console** do ArchGuard. Orientado a
> tarefas. Os endpoints por trás de cada tela estão em `docs/produto/03-referencia-de-integracao.md`;
> a operação da plataforma em `docs/produto/02-guia-do-operador.md`. Status: rascunho vivo (2026-08-02).

## 1. Conceitos essenciais

- **Organização (tenant):** a fronteira de isolamento. Tudo o que você gerencia é **do tenant ativo**
  da sua sessão — o seletor no topo troca o tenant (e reemite a sessão).
- **Identidade × Membership:** a **identidade** é a pessoa (autentica uma vez); o **membership** é a
  presença dessa pessoa **num tenant** (com status e papéis). Papéis e acesso são por membership.
- **Nível de garantia (L1/L2/L3):** operações mais sensíveis exigem **step-up** (autenticação
  reforçada) antes de executar. Se o console pedir um step-up e não concluir, falta credencial
  reforçada cadastrada (§7).
- **Sem auto-registro (ADR-0021):** ninguém se cadastra sozinho no plano de controle. Usuários são
  **provisionados por admin** (ou SCIM). Esconder botão não é controle de acesso — o controle é a API.

## 2. Entrar

Login por **Authorization Code + PKCE** (o console é um cliente OIDC). Não há senha-mestra (INV-1). A
sessão persiste a atualizações do core (Redis), então um deploy não desloga você.

## 3. Tenants

- **Selecionar tenant:** use o seletor no topo. A sessão passa a operar naquele tenant.
- **Criar/gerir organizações:** pela administração de organizações (herdada). Cada nova org é uma
  fronteira RLS independente.

## 4. Usuários e memberships

- **Provisionar usuário:** criar o usuário no tenant (nome, e-mail opcional, grupos). **A senha é
  definida pelo admin** — o assistente/automação não define senhas.
- **Roster do tenant:** a tela de membros lista quem pertence ao tenant e o status.
- **Offboarding:** marcar `isForbidden` **bloqueia o próximo login**; revogar o membership **encerra
  as sessões** e **remove o acesso do grafo** (owner/operator/grupo), além de **cascade-revogar as
  concessões** — quem saiu não mantém acesso. Suspender é temporário e reversível (reativar restaura).

## 5. Ativos e acesso granular — tela **Gestão de acesso**

Quatro abas, cada uma sobre o modelo de autorização (o "quem pode operar o quê"):

- **Ativos:** registre os recursos protegidos (tipo + nome). O `id` canônico é do ArchGuard; um
  `external_ref` opcional referencia o recurso no componente de brokering (nunca um segredo).
- **Grupos:** dê **nome** a um grupo de acesso (catálogo nome↔id). Grupos organizam pessoas para
  conceder acesso em bloco.
- **Atribuições:** conceda **`operator`** ou **`auditor`** a um **membership** ou a um **grupo** sobre
  um ativo. Conceder a um grupo faz **todos os membros herdarem** o acesso.
- **Vínculos de grupo:** adicione membros a um grupo. Combinado com uma atribuição de grupo, forma a
  cadeia `membro → grupo → operator → ativo`.

**Herança:** operator/auditor num **grupo de ativos** desce para os ativos filhos (acesso *herdado*).

## 6. Revisão de acesso — tela **Revisão de acesso**

Escolha um ativo e veja **quem o alcança** e por **qual origem**: **direto** (owner/operator no
ativo), **herdado** (de um grupo de ativos ancestral), **via grupo** (o membro herda do grupo) ou
**concessão** (um grant privilegiado vigente). É a visão de certificação/campanha. Se o PDP não puder
responder, a tela sinaliza **"não pôde concluir"** — **nunca** "ninguém tem acesso" (fail-closed).

## 7. MFA e step-up

- **Cadastrar fator:** TOTP e **WebAuthn/passkey**. Para operações **L3** (ex.: verificar cadeia de
  auditoria, break-glass), é necessária uma credencial **reforçada** (WebAuthn, resistente a phishing)
  — TOTP não qualifica para break-glass.
- Sem o fator adequado, operações L3 respondem com um desafio de step-up e não executam.

## 8. Acesso privilegiado e break-glass

- **Concessões vigentes:** a tela lista os grants ativos (alvo, origem, janela de validade). Revogar
  encerra as sessões derivadas e limpa o acesso do grafo.
- **Break-glass (acesso emergencial):** solicite com **justificativa** e **referência de incidente**
  (L3, exige step-up reforçado). A solicitação vai para uma **fila de aprovação por pares**; aprovada,
  a concessão vale por uma **janela** e **expira no próprio grafo**. Cada passo é auditado
  (imutável). É a via correta para o excepcional — não existe atalho fora dela.

## 9. Auditoria

- **Linha do tempo:** os eventos do tenant, mais recentes primeiro — trilha **append-only** (INV-2).
- **Verificar cadeia (L3):** confirma a integridade da **cadeia de hash** da trilha (detecta
  adulteração). Exige step-up reforçado.

## 10. Saúde

A tela de **Saúde** mostra os subsistemas (database, custódia, deployment) com status honesto — não
exibe "tudo verde" se o perfil ativo não entrega. Útil antes e depois de mudanças.

---

## Apêndice — o que **não** é controle de acesso

- Esconder um botão no frontend. O controle é a **API versionada** com o nível de garantia imposto no
  servidor.
- Uma "senha-mestra" ou modo permissivo. **Não existem** (INV-1, INV-6). Acesso excepcional é
  break-glass auditado (§8).

*Ver também: `01-visao-e-modelo-de-seguranca.md` (as garantias por trás destas telas).*
