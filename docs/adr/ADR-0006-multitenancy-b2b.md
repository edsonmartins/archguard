# ADR-0006 — Multi-tenancy B2B: usuário em múltiplas organizações

- **Status:** Aceito
- **Data:** 2026-07-19
- **Invariantes tocados:** I-6.1, I-6.2, I-6.3

## Contexto

No upstream, a organização é a fronteira de isolamento, mas **um usuário pertence a exatamente
uma organização** (limitação conhecida e discutida no rastreador do projeto). O modelo assume
identidade ligada permanentemente ao tenant.

Esse modelo quebra no caso de uso central do ArchGate. Cenários reais:

- Um engenheiro da IntegrAllTech opera ativos privilegiados de **Rio Quality** e **Grupo
  Marra** — dois tenants distintos. Com o modelo do upstream, ele precisaria de duas contas,
  duas credenciais, dois MFAs — e a trilha de auditoria perderia a identidade humana única,
  destruindo a rastreabilidade que é a razão de existir de um PAM.
- Um MSP parceiro atendendo múltiplos clientes.
- Um auditor externo com acesso de leitura à trilha de três tenants.

## Decisão

**Promover a relação usuário↔organização a entidade explícita (`membership`), com identidade
global única e contexto de tenant selecionável por sessão.**

### Modelo

```
User (identidade global, única por e-mail verificado / credencial)
  └── Membership (user_id, organization_id, status, papéis, política MFA efetiva)
        └── Organization (tenant — fronteira de isolamento)
```

- **Credencial é da identidade**, não do membership: uma senha, uma passkey, um MFA.
- **Autorização é do membership**: papéis, permissões e escopos são sempre por organização.
- **Sessão carrega tenant ativo.** O token emitido contém `org` (tenant ativo) e, quando
  aplicável, `orgs` (memberships elegíveis). Trocar de tenant é operação explícita e auditada,
  que **emite novo token** — nunca reaproveita token de outro tenant.
- **Isolamento por construção**: toda tabela de domínio carrega `organization_id`; o acesso a
  dados passa por repositório com predicado de tenant obrigatório, reforçado por **RLS
  (Row-Level Security) do PostgreSQL** como segunda barreira.
- **Convite e vinculação**: uma identidade existente é vinculada a nova organização por
  convite aceito, jamais por criação silenciosa de conta duplicada.
- **Política mais restritiva vence**: se a organização A exige WebAuthn e a B aceita TOTP, a
  sessão no tenant A exige WebAuthn. Requisitos de MFA são avaliados **por tenant ativo**.

### Escopo de isolamento de dados de identidade
Atributos de perfil compartilhados (nome, e-mail) pertencem à identidade global. Atributos
específicos do tenant (matrícula, centro de custo, atributos de diretório sincronizado)
pertencem ao membership — impedindo vazamento de dado corporativo de um cliente para outro.

## Alternativas consideradas

| Opção | Por que não |
|---|---|
| Manter 1 usuário : 1 organização (upstream) | Multiplica identidades da mesma pessoa; destrói rastreabilidade de auditoria; multiplica credenciais e superfície de ataque |
| Organização "federadora" com sub-organizações | Hierarquia não resolve o caso do operador externo que atende tenants sem relação entre si |
| Instância por tenant (silo físico) | Custo operacional proibitivo; impede visão consolidada de auditoria; não resolve a identidade única do operador |

## Consequências

### Positivas
- Rastreabilidade: uma pessoa, uma identidade, N contextos — requisito de auditoria de PAM.
- Base para revisão de acesso por tenant e para desligamento centralizado (revogar a
  identidade derruba todos os memberships).

### Negativas
- **Divergência estrutural profunda** do upstream no modelo de dados: é o maior gerador de
  conflito de cherry-pick (registrado em `DIVERGENCE.md`).
- Migração de dados exige cuidado: identidades duplicadas por e-mail precisam ser fundidas.
- Toda tela e todo endpoint precisam de noção de tenant ativo — não há caminho "sem contexto".

## Verificação
- Teste automatizado que **falha o build** diante de query a tabela de domínio sem predicado
  de tenant (análise estática + teste de integração com RLS ativo).
- Teste de travessia: token emitido no tenant A **não** autoriza recurso do tenant B.
