# ADR-0004 — Console administrativo próprio em React 19 + Mantine v9 + Archbase

- **Status:** Superado por [ADR-0020](ADR-0020-evolucao-do-console-herdado.md) (2026-07-26)
- **Data:** 2026-07-19
- **Nota:** a decisão de reescrever o console em Mantine/Archbase foi **diferida** pelo ADR-0020
  (evoluir o console herdado). O escopo funcional de PAM e os invariantes deste ADR permanecem
  válidos; muda o *stack* de implementação. RFC-0005 vira referência da opção greenfield adiada.
- **Invariantes tocados:** I-7.3, I-7.6
- **Precedente:** DeskLenz (core Go mantido, frontend reescrito)

## Contexto

O console do upstream é uma aplicação React (CRA) com Ant Design, voltada a administração
genérica de IAM. Ele é inadequado ao ArchGuard por quatro motivos:

1. **Identidade visual e de produto**: o ArchGuard é módulo do ArchGate e deve compartilhar
   linguagem visual e padrões de interação com os demais produtos IntegrAllTech.
2. **Modelo mental errado**: a UI do upstream expõe conceitos de IAM genérico; o operador de
   PAM raciocina em *quem pode acessar qual ativo privilegiado, quando, sob qual aprovação*.
3. **Divergência funcional**: multi-tenancy B2B, break-glass, revisão de acesso e trilha
   imutável não existem no upstream e não têm onde ser expressos na UI atual.
4. **Stack**: CRA + antd está fora do padrão da casa (React 19 + TS + Mantine v9 + Archbase),
   duplicando custo de manutenção e impedindo reuso de componentes.

Fator decisivo de viabilidade: **o console do upstream consome apenas a API REST pública** —
não existe API privada exclusiva de UI. A substituição é, portanto, um recorte limpo.

## Decisão

**Construir o ArchGuard Console como aplicação independente em React 19 + TypeScript +
Mantine v9 + Archbase, consumindo exclusivamente a API pública versionada. O console do
upstream é removido da árvore de build.**

Diretrizes:
- **Cliente de API gerado** a partir do contrato OpenAPI do backend (fonte da verdade única).
  Chamadas manuais a `fetch` são proibidas fora da camada gerada.
- **TanStack Query** para estado de servidor; sem store global de dados remotos.
- **Sem acesso direto ao banco** e sem endpoint criado "só para a UI": se o console precisa,
  é API pública, versionada e documentada (I-7.6).
- **Design tokens** compartilhados com o ArchGate.
- **i18n pt-BR como idioma primário**, en-US secundário.
- **Acessibilidade**: navegação completa por teclado nos fluxos privilegiados.

Escopo funcional do console (v1): organizações e memberships, usuários e grupos, aplicações e
clientes OIDC/SAML, provedores de identidade e sincronismos, políticas de MFA, papéis e
permissões, **visualizador de trilha de auditoria com verificação de cadeia**, **fluxo de
break-glass**, e **campanhas de revisão de acesso**.

## Alternativas consideradas

| Opção | Por que não |
|---|---|
| Manter o console antd e apenas tematizar | Não resolve modelo mental de PAM; mantém stack fora do padrão; toda feature nova exige trabalho em duas stacks |
| Estender o console antd com telas novas | Acopla o ArchGuard ao ciclo de release do frontend do upstream; conflitos permanentes de rebase |
| Micro-frontend embutindo telas do upstream | Complexidade sem benefício: a API já é pública; não há lógica exclusiva a preservar |

## Consequências

### Positivas
- Console alinhado ao domínio de PAM e ao portfólio IntegrAllTech.
- Reuso de Archbase acelera CRUD e reduz superfície de código próprio.
- Frontend e backend evoluem em cadências independentes; a API vira contrato real.

### Negativas
- Esforço frontend relevante (estimativa: 2–3 meses de squad dedicada para o escopo v1).
- Toda lacuna da API pública do upstream vira trabalho de backend antes de virar tela.
- Risco de *drift* entre contrato OpenAPI e implementação — mitigado por geração de cliente
  e teste de contrato no CI.

## Verificação
- Build do console **falha** se o cliente gerado estiver desatualizado em relação ao OpenAPI.
- Teste de fumaça cobrindo os fluxos privilegiados (break-glass, revisão de acesso).
