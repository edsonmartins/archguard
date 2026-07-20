# Spec — Capability: fine-grained-authz

## ADDED Requirements

### Requirement: Decisão granular para acesso privilegiado
O sistema SHALL consultar o PDP para toda decisão de acesso a ativo privilegiado.

#### Scenario: Acesso direto concedido
- **WHEN** um membership possui relação de operador sobre um ativo
- **AND** solicita abertura de sessão privilegiada
- **THEN** o PDP retorna permitido
- **AND** a decisão e sua justificativa são registradas na auditoria

#### Scenario: Acesso herdado
- **WHEN** um membership é operador de um grupo de ativos
- **AND** solicita acesso a ativo filho desse grupo
- **THEN** o PDP retorna permitido por herança
- **AND** a justificativa registrada identifica a relação herdada

#### Scenario: Sem relação
- **WHEN** um membership sem qualquer relação solicita acesso ao ativo
- **THEN** o PDP retorna negado
- **AND** o evento é auditado com resultado `denied`

### Requirement: Concessões com janela temporal
O sistema SHALL respeitar a janela temporal das concessões na decisão de autorização.

#### Scenario: Concessão vigente
- **WHEN** existe concessão ativa dentro da janela
- **THEN** o acesso privilegiado correspondente é permitido

#### Scenario: Concessão expirada
- **WHEN** a janela da concessão expirou
- **THEN** o acesso é negado, ainda que a tupla de concessão persista no store

### Requirement: Isolamento de tenant no grafo de autorização
O sistema SHALL impedir relações que atravessem organizações.

#### Scenario: Tentativa de relação cruzada
- **WHEN** uma escrita de tupla relaciona sujeito de um tenant a objeto de outro
- **THEN** a escrita é rejeitada

#### Scenario: Consulta cruzada
- **WHEN** uma verificação é feita para membership do tenant A sobre ativo do tenant B
- **THEN** o resultado é negado

### Requirement: Fonte da verdade e sincronização
O sistema SHALL tratar o banco relacional como fonte da verdade e o PDP como projeção
derivada.

#### Scenario: Mutação de domínio
- **WHEN** um membership recebe papel que implica relação de autorização
- **THEN** a intenção de sincronizar é persistida na mesma transação da mudança
- **AND** a tupla correspondente é escrita de forma assíncrona e idempotente

#### Scenario: Reprocessamento
- **WHEN** o publisher reprocessa um registro de outbox já aplicado
- **THEN** o estado final do store permanece o mesmo

#### Scenario: Reconstrução completa
- **WHEN** o store do PDP é reconstruído a partir do banco
- **THEN** o estado resultante é equivalente ao anterior à reconstrução

### Requirement: Reconciliação com política assimétrica
O sistema SHALL reconciliar periodicamente e tratar divergências conforme o efeito sobre o
acesso.

#### Scenario: Divergência restritiva
- **WHEN** a reconciliação encontra tupla que concede acesso não previsto pelo banco
- **THEN** a tupla é removida automaticamente
- **AND** o evento é registrado

#### Scenario: Divergência ampliativa
- **WHEN** a reconciliação encontra ausência de tupla que ampliaria acesso
- **THEN** a correção NOT é aplicada automaticamente
- **AND** um alerta para revisão humana é emitido

### Requirement: Comportamento fail-closed
O sistema SHALL negar decisões granulares quando o PDP estiver indisponível.

#### Scenario: PDP fora do ar
- **WHEN** o PDP não responde
- **AND** um usuário solicita acesso privilegiado
- **THEN** o acesso é negado
- **AND** a autenticação e a emissão de token permanecem funcionais
- **AND** um alerta é emitido

#### Scenario: Ausência de configuração permissiva
- **WHEN** um operador busca configuração que permita liberar acesso na falha do PDP
- **THEN** nenhuma opção desse tipo existe no sistema

### Requirement: Consulta reversa para revisão de acesso
O sistema SHALL permitir listar o acesso efetivo a um ativo ou de um membership.

#### Scenario: Campanha de revisão
- **WHEN** um administrador abre campanha de revisão sobre um ativo
- **THEN** o sistema lista todos os memberships com acesso efetivo
- **AND** indica a origem de cada acesso (direto, herdado ou por concessão)
