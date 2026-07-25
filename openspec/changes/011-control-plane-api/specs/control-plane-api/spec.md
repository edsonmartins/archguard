# Spec — Capability: control-plane-api

## ADDED Requirements

### Requirement: API pública versionada, sem endpoint exclusivo de UI
O sistema SHALL expor as capacidades ArchGuard como endpoints públicos versionados sob um
prefixo estável, documentados no contrato OpenAPI.

#### Scenario: Capacidade montada é alcançável
- **WHEN** um handler de `internal/http` é integrado ao composition root
- **THEN** existe uma rota do servidor que o invoca (o handler NOT permanece órfão)
- **AND** o endpoint responde sob o prefixo versionado `/api/v1`

#### Scenario: Endpoint exclusivo de UI é rejeitado
- **WHEN** um endpoint é proposto para atender somente o console
- **THEN** a revisão o rejeita, exigindo publicação como API versionada e documentada (I-7.6)

### Requirement: Todo endpoint de domínio opera sob contexto de tenant
O sistema SHALL resolver e aplicar o contexto de tenant em toda operação sobre tabela de
domínio exposta pela API.

#### Scenario: Requisição sem tenant resolvido
- **WHEN** uma requisição a um endpoint de domínio chega sem contexto de tenant resolúvel
- **THEN** a operação é negada
- **AND** nenhuma consulta cross-tenant é emitida (INV-5)

#### Scenario: Store exige tenant na construção
- **WHEN** o composition root constrói um store de tabela de domínio
- **THEN** a construção exige o contexto de tenant (consulta cross-tenant usa tipo distinto,
  explícito e auditado)

### Requirement: Fail-closed no composition root
O sistema SHALL negar serviço quando uma dependência crítica não puder ser satisfeita — nunca
subir em modo permissivo.

#### Scenario: Dependência crítica indisponível
- **WHEN** PDP, cofre ou subsistema de auditoria está indisponível no momento de servir
- **THEN** a operação afetada é negada (INV-6), distinguindo `denied` de `error` na auditoria

#### Scenario: Adapter de desenvolvimento em perfil conforme
- **WHEN** o perfil ativo é conforme e um adapter provisional/dev seria selecionado para uma
  capacidade de custódia
- **THEN** o composition root recusa servir aquela capacidade (não expõe custódia dev em
  produção — INV-7, ADR-0017)

### Requirement: Nível de garantia declarado e sinalizado por operação
O sistema SHALL declarar o nível de garantia (L1/L2/L3) de cada operação e sinalizar garantia
insuficiente de forma processável pelo cliente.

#### Scenario: Operação exige nível superior ao da sessão
- **WHEN** uma operação classificada acima do nível da sessão corrente é acionada
- **THEN** a API responde erro específico de garantia insuficiente com o `acr` exigido (INV-8)
- **AND** o cliente pode conduzir step-up e repetir a operação (base do 008 T-007)

### Requirement: Contrato OpenAPI é fonte da verdade
O sistema SHALL manter um contrato OpenAPI da API nova, verificado no CI, do qual o cliente do
console é gerado.

#### Scenario: Handler montado sem entrada no contrato
- **WHEN** um endpoint é montado sem descrição correspondente no OpenAPI
- **THEN** o gate de contrato falha

#### Scenario: Cliente defasado
- **WHEN** o contrato OpenAPI muda e o cliente gerado do console não é regenerado
- **THEN** o build do console falha (liga-se ao 008 T-002)

### Requirement: Composition root não contamina o domínio
O sistema SHALL manter a regra de dependência: a camada de boot/infra importa framework e
driver; o domínio, não.

#### Scenario: Pureza do domínio preservada
- **WHEN** o composition root importa Beego e pgx para montar e injetar
- **THEN** `internal/domain/**` permanece livre de framework/ORM e `make deps-check` (INV-3)
  continua verde

### Requirement: Reuso sem reescrita
O sistema SHALL integrar a camada `internal/http` existente, não reimplementá-la.

#### Scenario: Capacidade já implementada é montada, não reescrita
- **WHEN** uma capacidade já tem handler testado em `internal/http`
- **THEN** o composition root a monta e injeta suas dependências
- **AND** NOT cria um controller Beego paralelo que duplique a camada HTTP
