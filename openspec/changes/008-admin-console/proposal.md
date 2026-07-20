# Proposal — 008 · ArchGuard Console (React 19 + Mantine v9 + Archbase)

## Por quê

O console herdado (CRA + antd) está fora do padrão da casa, expressa um modelo mental de IAM
genérico e não tem onde representar multi-tenancy B2B, break-glass, revisão de acesso e trilha
verificável. O fator que torna a substituição viável é que o console do upstream consome
**apenas a API pública** — não há API privada de UI (ADR-0004).

## O que muda

- Console novo, independente, em React 19 + TypeScript + Mantine v9 + Archbase.
- Cliente de API **gerado** a partir do OpenAPI; build falha se estiver defasado.
- Navegação orientada ao domínio de PAM, com seletor de tenant permanente.
- Step-up transparente com retomada da operação.
- Telas críticas: auditoria com verificação de integridade, break-glass, revisão de acesso.
- Console antigo removido da árvore de build.

## O que não muda

Nenhuma regra de autorização migra para o frontend. Esconder botão não é controle de acesso.

## Impacto

- **Depende de:** 002, 003, 004, 005, 007.
- **Risco:** lacunas da API pública viram trabalho de backend antes de virar tela; *drift*
  entre contrato e implementação.
