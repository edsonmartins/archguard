# ADR-0021 — Auto-registro (self-signup) proibido no plano de controle

- **Status:** Aceito (2026-07-27)
- **Data:** 2026-07-27
- **Invariantes tocados:** INV-1 / I-4.1 (fluxo de autenticação); anti-padrão CLAUDE.md §8 ("flag de configuração que permita bypass")
- **Motivador:** o piloto expunha "Não possui uma conta? Inscreva-se agora" na tela de login (default herdado do Casdoor)

## Contexto

A aplicação de login do console (`app-built-in`) era semeada com `EnableSignUp: true` — default
histórico do Casdoor. Isso exibe o link "Inscreva-se agora" e permite que **qualquer pessoa** crie
uma conta de login em `app.archguard.com.br`.

Para um plano de controle de identidade/PAM isso é uma brecha:

- **Superfície de ataque pública** (account farming, alvo de phishing) exatamente no plano que
  governa acesso privilegiado.
- **Contradiz o modelo de identidade do ArchGuard**: identidades são PROVISIONADAS (seed, API de
  admin, SCIM — pacote 009) e a ponte de login é **resolve-only** (nunca cria identidade no
  login). Um auto-cadastro seria uma conta órfã no IdP, sem identidade de domínio — inútil para o
  `/api/v1` e pura superfície.

## Decisão

**O plano de controle nunca permite auto-registro.** Três barreiras:

1. **Seed off** (`object/init.go`, `initBuiltInApplication`): a app `app-built-in` nasce com
   `EnableSignUp: false`.
2. **Enforcement de boot self-healing** (`object.EnforceNoSelfSignupOnControlPlane`, chamado no
   `main.go` em **todo perfil**): se o flag tiver derivado para ativo (instalação antiga como o
   piloto, ou um toggle acidental na UI), é forçado para `false` e um AVISO é logado. Escolheu-se
   self-heal em vez de fail-closed para não travar o boot por um flag de UI — mantendo "sem
   bypass" sem transformar um erro de config em indisponibilidade.
3. **Invariante estático** (`test/invariants/signup_guard_test.go`): o build quebra se o seed
   reabilitar `EnableSignUp: true`.

Contas de usuário são criadas por administrador (`/api/add-user`) e/ou provisionadas via SCIM
(camada PAM). Ver o runbook de provisionamento.

## Consequências

- O link "Inscreva-se agora" desaparece; ninguém se auto-cadastra no plano de controle.
- Instalações existentes (o piloto) são corrigidas automaticamente no próximo boot (com aviso).
- O onboarding de usuários passa a ser **explícito** (admin/SCIM) — coerente com PAM e com o
  modelo resolve-only da ponte de login.
- **Escopo:** a enforcement mira apenas `app-built-in` (a app do console). Não impede que uma
  aplicação de tenant fora do plano de controle tenha signup próprio — isso é decisão separada.
