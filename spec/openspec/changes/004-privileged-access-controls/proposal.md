# Proposal — 004 · Controles de acesso privilegiado: impersonation e break-glass

## Por quê

O upstream oferece dois mecanismos incompatíveis com PAM: senha-mestra (removida no pacote 001)
e impersonation com registro insuficiente. Ambos destroem o não-repúdio. Ao mesmo tempo,
suprimir todo acesso emergencial é irreal: sem procedimento formal, a equipe cria backdoors
informais — que é o pior desfecho possível (ADR-0008).

## O que muda

- Impersonation redesenhada como **delegação explícita**: identidade dupla no token (`act`),
  consentimento por padrão, escopo reduzido, tempo-limitada, com notificação ao alvo.
- **Break-glass** como procedimento formal: justificativa, step-up, aprovação de N pares,
  janela curta, alerta em tempo real, revisão pós-uso e fail-closed.
- **Contas de serviço** separadas de identidades humanas, sem login interativo e nunca
  impersonáveis.
- Concessões privilegiadas (`privileged_grant`) com expiração automática e revogação em
  cascata.

## O que não muda

O mecanismo de step-up em si é entregue no pacote 005; aqui ele é consumido como dependência.

## Impacto

- **Depende de:** 001, 002, 003, 005 (step-up).
- **Bloqueia:** 007 (concessões como relação no PDP), 008 (fila de aprovação).
- **Risco:** perda deliberada de conveniência operacional para o suporte.
