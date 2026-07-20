# Spec — Capability: observability-compliance

## ADDED Requirements

### Requirement: Separação entre telemetria e auditoria
O sistema SHALL manter a trilha de auditoria como fonte da verdade, independente da telemetria.

#### Scenario: Perda de telemetria
- **WHEN** o coletor de telemetria está indisponível
- **THEN** a autenticação e a gravação de auditoria permanecem funcionais
- **AND** nenhum evento de auditoria é perdido

#### Scenario: Cópia em ferramenta de log
- **WHEN** eventos de auditoria são exportados para a ferramenta de logs
- **THEN** são identificados como cópia
- **AND** a verificação de integridade utiliza exclusivamente a trilha própria

### Requirement: Higiene de dados sensíveis em telemetria
O sistema SHALL NOT emitir segredos, tokens ou dados pessoais em claro em sinais de telemetria.

#### Scenario: Token em log
- **WHEN** um caminho de código tenta registrar token ou credencial
- **THEN** o valor é redigido antes da emissão

#### Scenario: Verificação automatizada
- **WHEN** a suíte de verificação de telemetria detecta dado sensível
- **THEN** o build é rejeitado

#### Scenario: Identificação de usuário
- **WHEN** telemetria referencia um usuário
- **THEN** utiliza pseudônimo estável e NOT o endereço de e-mail

### Requirement: Ausência de telemetria externa
O sistema SHALL NOT transmitir dados para fora do perímetro do cliente sem consentimento
explícito.

#### Scenario: Instalação padrão
- **WHEN** uma instalação nova é iniciada
- **THEN** nenhum dado é enviado a destinos externos ao perímetro configurado

### Requirement: Alertas de integridade e disponibilidade
O sistema SHALL alertar sobre falhas que comprometam garantias de segurança.

#### Scenario: Falha de verificação de cadeia
- **WHEN** a verificação diária detecta divergência
- **THEN** um alerta de severidade máxima é emitido

#### Scenario: Indisponibilidade de subsistema crítico
- **WHEN** o cofre ou o PDP torna-se indisponível
- **THEN** um alerta é emitido
- **AND** o console reflete o estado degradado

#### Scenario: Solicitação de break-glass
- **WHEN** uma solicitação de break-glass é criada
- **THEN** um alerta informativo imediato é emitido

### Requirement: Custódia de material criptográfico
O sistema SHALL manter chaves e segredos fora do banco de dados.

#### Scenario: Inspeção do banco
- **WHEN** o conteúdo do banco é inspecionado
- **THEN** nenhuma chave privada ou segredo de client está presente em claro
- **AND** apenas referências ao cofre são encontradas

#### Scenario: Assinatura de selo
- **WHEN** um selo de auditoria é assinado
- **THEN** a operação ocorre no cofre
- **AND** a aplicação NOT obtém a chave privada

#### Scenario: Custódia local em produção
- **WHEN** uma instalação de produção opera com custódia local
- **THEN** o health check reporta instalação não conforme

### Requirement: Rotação de chaves sem indisponibilidade
O sistema SHALL rotacionar chaves preservando operações em curso.

#### Scenario: Rotação de JWKS
- **WHEN** a chave de assinatura é rotacionada
- **THEN** tokens emitidos anteriormente permanecem válidos até expirar

#### Scenario: Autorização da rotação
- **WHEN** um administrador aciona rotação de chave
- **THEN** a operação exige nível L3
- **AND** é registrada na auditoria

### Requirement: Classificação obrigatória de dados pessoais
O sistema SHALL exigir classificação de campos que contenham dados pessoais.

#### Scenario: Migration sem classificação
- **WHEN** uma migration adiciona campo pessoal sem categoria, finalidade, base legal e prazo
  de retenção
- **THEN** o CI rejeita a alteração

### Requirement: Eliminação por destruição de chave
O sistema SHALL atender à eliminação de dados pessoais sem comprometer a trilha de auditoria.

#### Scenario: Solicitação de eliminação
- **WHEN** a eliminação dos dados de um titular é executada
- **THEN** a chave do titular é destruída
- **AND** os dados pessoais tornam-se irrecuperáveis, inclusive em backups
- **AND** os eventos de auditoria permanecem com pseudônimo

#### Scenario: Integridade preservada
- **WHEN** o verificador é executado após uma eliminação
- **THEN** a cadeia permanece íntegra e verificável

#### Scenario: Confirmação da irreversibilidade
- **WHEN** um administrador aciona a eliminação
- **THEN** o sistema descreve a irreversibilidade
- **AND** exige nível L3
- **AND** registra o ato como evento de auditoria

### Requirement: Retenção por arquivamento
O sistema SHALL implementar retenção sem remoção seletiva de eventos.

#### Scenario: Período expirado
- **WHEN** o prazo de retenção configurado expira
- **THEN** a partição correspondente é arquivada com seus selos
- **AND** a restauração é possível e auditada

### Requirement: Atendimento a direitos do titular com isolamento
O sistema SHALL permitir exportação estruturada dos dados de um titular por organização.

#### Scenario: Requisição de acesso
- **WHEN** um titular solicita seus dados em uma organização
- **THEN** a exportação contém apenas dados daquela organização e da identidade global
- **AND** NOT inclui dados de outras organizações
- **AND** a requisição é auditada
