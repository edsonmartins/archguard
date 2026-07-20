# Spec — Capability: identity-multitenancy

## ADDED Requirements

### Requirement: Identidade global única
O sistema SHALL representar cada pessoa ou conta de serviço por exatamente uma identidade
global, independentemente do número de organizações a que pertença.

#### Scenario: Pessoa em dois tenants
- **WHEN** uma identidade possui memberships ativos nas organizações A e B
- **THEN** possui um único conjunto de credenciais e fatores MFA
- **AND** o claim `sub` emitido é o mesmo em ambos os contextos

#### Scenario: Vinculação a nova organização
- **WHEN** um administrador convida um e-mail já associado a identidade existente
- **THEN** o sistema cria um novo membership para a identidade existente
- **AND** NOT cria nova identidade

### Requirement: Autorização vinculada ao membership
O sistema SHALL vincular papéis, permissões e atributos de tenant ao membership.

#### Scenario: Papel não vaza entre tenants
- **WHEN** uma identidade tem papel administrativo na organização A e papel comum na B
- **AND** opera com a organização B como tenant ativo
- **THEN** as permissões efetivas são apenas as do membership em B

### Requirement: Contexto de tenant ativo na sessão
O sistema SHALL manter exatamente um tenant ativo por sessão.

#### Scenario: Múltiplos memberships no login
- **WHEN** uma identidade com mais de um membership ativo autentica
- **THEN** o sistema exige seleção explícita de tenant antes de emitir token de acesso

#### Scenario: Troca de tenant
- **WHEN** o usuário troca o tenant ativo
- **THEN** um novo token é emitido com o claim `org` correspondente
- **AND** o token anterior não é reaproveitado
- **AND** um evento de auditoria de troca de contexto é registrado

#### Scenario: Política mais restritiva no destino
- **WHEN** o tenant de destino exige fator mais forte que o comprovado na sessão
- **THEN** o sistema exige step-up antes de concluir a troca

### Requirement: Isolamento de dados por organização
O sistema SHALL impedir acesso a dados de uma organização a partir do contexto de outra.

#### Scenario: Barreira de aplicação
- **WHEN** uma consulta é executada com contexto do tenant A sobre recurso do tenant B
- **THEN** o resultado é vazio ou o acesso é negado

#### Scenario: Barreira de banco
- **WHEN** a barreira de aplicação é contornada em ambiente de teste
- **AND** a consulta é executada com o papel da aplicação
- **THEN** a política RLS impede o retorno de registros de outra organização

#### Scenario: Query sem predicado de tenant
- **WHEN** código novo consulta tabela de domínio sem predicado de tenant
- **THEN** o teste automatizado falha e o build é rejeitado

### Requirement: Ciclo de vida de membership
O sistema SHALL suportar suspensão e revogação de membership sem afetar a identidade global.

#### Scenario: Revogação de membership
- **WHEN** o membership de uma identidade na organização A é revogado
- **THEN** as sessões dessa identidade no tenant A são encerradas
- **AND** os memberships e sessões em outras organizações permanecem ativos

#### Scenario: Suspensão da identidade
- **WHEN** a identidade global é suspensa
- **THEN** todos os memberships e todas as sessões são encerrados

### Requirement: Segregação de atributos de tenant
O sistema SHALL manter atributos específicos de organização isolados no membership.

#### Scenario: Atributo corporativo não vaza
- **WHEN** a organização A registra matrícula e centro de custo para um membership
- **THEN** esses atributos NOT são visíveis para administradores da organização B
