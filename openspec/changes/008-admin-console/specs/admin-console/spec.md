# Spec — Capability: admin-console

## ADDED Requirements

### Requirement: Console consome apenas a API pública
O sistema SHALL expor ao console exclusivamente endpoints públicos versionados.

#### Scenario: Endpoint exclusivo de UI
- **WHEN** um endpoint é criado para atender somente o console
- **THEN** a revisão o rejeita, exigindo publicação como API versionada e documentada

#### Scenario: Cliente defasado
- **WHEN** o contrato OpenAPI muda e o cliente gerado não é regenerado
- **THEN** o build do console falha

### Requirement: Contexto de tenant sempre visível
O sistema SHALL indicar de forma inequívoca o tenant ativo.

#### Scenario: Identidade multi-tenant
- **WHEN** um usuário com múltiplos memberships acessa o console
- **THEN** o tenant ativo é exibido permanentemente com distinção visual

#### Scenario: Troca de tenant
- **WHEN** o usuário troca o tenant ativo
- **THEN** o contexto é recarregado com novo token
- **AND** step-up é solicitado caso a política do destino seja mais restritiva

### Requirement: Step-up transparente
O sistema SHALL conduzir o step-up sem perda do trabalho em andamento.

#### Scenario: Operação L3 iniciada
- **WHEN** o usuário aciona operação que exige nível superior ao da sessão
- **THEN** o console apresenta o desafio WebAuthn
- **AND** ao concluir, executa a operação original com o estado preservado

#### Scenario: Step-up cancelado
- **WHEN** o usuário cancela o desafio
- **THEN** a operação NOT é executada
- **AND** o formulário permanece preenchido

### Requirement: Agregados honestos com detalhe sob demanda
O sistema SHALL apresentar, em toda superfície de resumo, sinal de severidade suficiente para
indicar necessidade de aprofundamento.

#### Scenario: Divergência de auditoria
- **WHEN** a verificação de integridade encontra divergência em qualquer período
- **THEN** a visão geral exibe estado de severidade máxima
- **AND** NOT apresenta indicador positivo agregado

#### Scenario: Pendências operacionais
- **WHEN** existem solicitações de break-glass pendentes ou divergências de reconciliação
- **THEN** a visão geral sinaliza a pendência com acesso direto ao detalhe

### Requirement: Visualização de auditoria correlacionada
O sistema SHALL permitir reconstruir a linha do tempo de um acesso privilegiado.

#### Scenario: Investigação de sessão
- **WHEN** um auditor abre um evento de sessão privilegiada
- **THEN** o console exibe os eventos correlacionados do ArchGuard e dos componentes
- **AND** identifica ator real e sujeito em caso de delegação

### Requirement: Operações destrutivas explicitadas
O sistema SHALL descrever a consequência de operações irreversíveis antes de executá-las.

#### Scenario: Eliminação de dados de titular
- **WHEN** um administrador aciona a eliminação de dados pessoais de um titular
- **THEN** o console descreve que a operação é irreversível e destrói a chave do titular
- **AND** exige confirmação explícita e nível L3

### Requirement: Autorização não reside no frontend
O sistema SHALL manter toda decisão de autorização no backend.

#### Scenario: Elemento oculto
- **WHEN** um controle é ocultado por falta de permissão
- **AND** a requisição correspondente é enviada diretamente à API
- **THEN** a API nega a operação

### Requirement: Segurança da sessão no navegador
O sistema SHALL proteger credenciais de sessão no cliente.

#### Scenario: Armazenamento de token
- **WHEN** a aplicação estabelece sessão autenticada
- **THEN** o token de sessão NOT é persistido em `localStorage` ou `sessionStorage`

#### Scenario: Encerramento por inatividade
- **WHEN** o período de inatividade definido pela política do tenant é atingido
- **THEN** a sessão é encerrada e o back-channel logout é propagado
