# Proposal — 007 · Autorização granular com OpenFGA

## Por quê

Decisões de PAM são relacionais e contextuais ("este operador pode abrir sessão SSH neste host
do tenant X, na janela aprovada, com concessão vigente, herdando acesso do grupo de ativos
pai?"). Modelar isso em listas RBAC produz explosão combinatória e regras impossíveis de
auditar (ADR-0005).

## O que muda

- Adoção do OpenFGA como PDP granular, atrás da interface `PolicyDecisionPoint`.
- Modelo de autorização com hierarquia de ativos, grupos e concessões com janela temporal.
- Sincronização por **outbox transacional** (PostgreSQL é a fonte da verdade; o PDP é
  projeção derivada) + reconciliação periódica.
- Justificativa da decisão anexada ao evento de auditoria.
- Comportamento **fail-closed** sem exceção configurável.

## O que não muda

Casbin permanece responsável pela autorização coarse-grained. A fronteira entre os planos é
normativa (RFC-0004, §2): uma decisão pertence a exatamente um plano.

## Impacto

- **Depende de:** 001, 002, 003, 004.
- **Bloqueia:** 008 (revisão de acesso e visualização de "por que este acesso").
- **Risco:** divergência entre banco e PDP; custo de reconciliação em larga escala.
