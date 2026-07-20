# ADR-0005 — Separação AuthN/AuthZ e adoção do OpenFGA como PDP granular

- **Status:** Aceito
- **Data:** 2026-07-19
- **Invariantes tocados:** I-7.4, I-1.3

## Contexto

O upstream embute **Casbin** e expõe um endpoint de *enforcement*. Casbin resolve bem
autorização **coarse-grained** ("usuário X pode acessar a aplicação Y", "papel Z tem
permissão W"). O ArchGuard, porém, precisa decidir questões de PAM que são inerentemente
**relacionais e contextuais**:

> *"O operador Carolina pode abrir sessão SSH privilegiada no host `db-prod-03` do tenant
> Rio Quality, na janela de manutenção aprovada, com aprovação vigente de dois pares, sendo
> membro do grupo `dba-oncall` que herda acesso do ativo pai `cluster-oracle-prod`?"*

Modelar isso em listas RBAC produz explosão combinatória de papéis e regras impossíveis de
auditar. É o problema clássico endereçado pelo modelo Zanzibar (ReBAC): autorização como grafo
de relações entre objetos.

Simultaneamente, um invariante impede a solução ingênua: **o core deve autenticar mesmo sem
serviços externos** (I-1.3). O PDP não pode ser dependência dura do caminho de login.

## Decisão

**Separar formalmente os planos:**

| Plano | Responsável | Escopo |
|---|---|---|
| **AuthN** | Core ArchGuard (fork) | Identidade, credenciais, MFA, sessão, emissão de token |
| **AuthZ coarse** | Casbin embutido (herdado) | Acesso a aplicações, papéis administrativos do próprio ArchGuard |
| **AuthZ granular** | **OpenFGA** (Apache 2.0, CNCF) como PDP externo | Decisões de acesso a ativos privilegiados, herança por hierarquia de ativos, condições contextuais |

**OpenFGA é escolhido sobre SpiceDB** por: DSL de modelagem mais acessível à equipe, ecossistema
e SDKs maduros, projeto CNCF com licença Apache 2.0 e menor custo operacional de deployment
inicial. **SpiceDB permanece como alternativa formal** caso surja requisito de consistência
forte com tokens de leitura (ZedTokens) em cenários multirregião — a integração é isolada
atrás de uma interface `PolicyDecisionPoint`, tornando a troca contida.

**Degradação controlada (I-1.3):** se o OpenFGA estiver indisponível, a autenticação e a
emissão de token OIDC continuam funcionando; **decisões de acesso privilegiado falham
fechado** (negação) e geram evento de auditoria de indisponibilidade de PDP. Nunca há
*fail-open*.

**Fonte da verdade:** as relações do OpenFGA são **projeção derivada** do modelo de identidade
do ArchGuard (usuários, memberships, grupos, ativos). A escrita de tuplas é feita por um
sincronizador transacional (outbox), nunca manualmente. Divergência entre banco e PDP é
detectada por reconciliação periódica.

## Alternativas consideradas

| Opção | Por que não |
|---|---|
| Só Casbin | Não modela relações nem herança de ativos sem explosão de regras; auditoria de "por que teve acesso" fica impraticável |
| Cedar (AWS) | Excelente linguagem de política, embeddable; porém ABAC-first, com menor aderência ao raciocínio relacional de ativos. Reavaliável para condições contextuais dentro das relações |
| OPA/Rego | Poderoso e genérico, mas Rego tem curva alta e o modelo de dados de relações precisaria ser construído do zero |
| Regras de autorização no core | Viola I-7.4; acopla domínio de PAM ao fork, aumentando custo de rebase |

## Consequências

### Positivas
- "Por que este acesso foi permitido?" torna-se resposta explicável e testável (trilha da
  decisão do PDP anexada ao evento de auditoria).
- Herança por hierarquia de ativos sem cadastro combinatório.
- Modelo de autorização versionável e testável isoladamente.

### Negativas
- Mais um componente no deployment (mitigado: opcional, com fail-closed).
- Necessidade de sincronizador confiável e reconciliação — subsistema com custo próprio.
- Duas superfícies de autorização (Casbin + OpenFGA) exigem fronteira nítida documentada,
  sob pena de regra duplicada em dois lugares. A fronteira está definida no RFC-0004.
