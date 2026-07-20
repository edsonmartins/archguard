# Design — 006 · Federação OIDC

Base normativa: RFC-0006.

## Princípio de contenção

O contrato é **agnóstico de fornecedor**. É ele que mantém contido o custo de eventual troca do
IdP (ADR-0001, §Reversibilidade). Nenhuma particularidade do fork vaza para os claims.

## Fluxos

Authorization Code + **PKCE obrigatório**. Device Authorization Grant apenas para clientes sem
navegador (NetBird). *Implicit* e ROPC não são suportados.

**Regra dura:** operações L3 **não** são autorizadas por device flow — o fluxo não sustenta
step-up confiável.

## Claims

Ver tabela normativa em RFC-0006, §3. Pontos de atenção:
- `sub` opaco e estável; e-mail apenas sob escopo explícito e justificado;
- claims sempre do **tenant ativo**;
- `pcid` (correlação de sessão privilegiada) é o que une a trilha do ArchGuard às trilhas dos
  componentes — sem ele, a auditoria do PAM fica fragmentada;
- `act` presente em delegação (pacote 004).

## Tokens

Access 5–15 min com `aud` específico; refresh com **rotação e detecção de reuso** (reuso ⇒
revogação de toda a família + evento de severidade alta); token de sessão privilegiada expira
com a concessão.

## Logout e revogação

Back-channel logout para quem suporta. Para quem não suporta: TTL curto + introspecção,
documentado como limitação com compensação explícita. Revogar membership, suspender identidade
ou expirar concessão dispara revogação imediata.

## Adaptação sem contaminação

Componente com suporte limitado recebe adaptação **na borda do próprio componente**. O contrato
central nunca é degradado para acomodar a implementação mais fraca.

## Suíte de conformidade (gate)

Por componente: login completo; semântica de claims; recusa por `acr` insuficiente;
comportamento na rotação de chave; back-channel logout efetivo; correlação `pcid` verificável
nas duas trilhas. Falha bloqueia release.
