# Spec — Capability: authentication-assurance

## ADDED Requirements

### Requirement: MFA obrigatório para identidades privilegiadas
O sistema SHALL exigir ao menos um fator forte registrado para identidades com papel
privilegiado.

#### Scenario: Privilegiado sem fator
- **WHEN** uma identidade com papel privilegiado autentica sem fator forte registrado
- **THEN** a sessão entra em estado de enrolamento obrigatório
- **AND** somente operações de registro de fator são permitidas

#### Scenario: Concessão de papel privilegiado
- **WHEN** um papel privilegiado é atribuído a identidade sem fator forte
- **THEN** o próximo login exige enrolamento antes de qualquer outra operação

### Requirement: Níveis de garantia por operação
O sistema SHALL classificar cada operação em nível de garantia e recusar execução com garantia
insuficiente.

#### Scenario: Operação L3 com sessão antiga
- **WHEN** um usuário com sessão válida e antiga solicita abertura de sessão privilegiada
- **THEN** o sistema recusa a operação com erro de garantia insuficiente
- **AND** informa o nível `acr` exigido

#### Scenario: Step-up concluído
- **WHEN** o usuário conclui reautenticação WebAuthn após recusa por garantia
- **THEN** a operação original é executada sem perda de contexto
- **AND** o `acr` da sessão reflete o nível obtido

#### Scenario: Operação sem classificação
- **WHEN** uma operação da API não declara nível de garantia exigido
- **THEN** o build é rejeitado

### Requirement: Fator resistente a phishing para operações críticas
O sistema SHALL exigir fator resistente a phishing em operações de nível L3.

#### Scenario: TOTP em operação L3
- **WHEN** um usuário tenta satisfazer step-up de operação L3 com TOTP
- **THEN** o sistema recusa e exige fator WebAuthn

#### Scenario: SMS como fator
- **WHEN** um administrador tenta habilitar SMS como fator para acesso privilegiado
- **THEN** a configuração é rejeitada

### Requirement: Política de MFA por organização com precedência restritiva
O sistema SHALL aplicar a política de MFA do tenant ativo, prevalecendo a mais restritiva.

#### Scenario: Troca para tenant mais restritivo
- **WHEN** um usuário autenticado com TOTP troca para tenant que exige WebAuthn
- **THEN** o sistema exige step-up WebAuthn antes de concluir a troca

### Requirement: Recuperação sem credencial administrativa universal
O sistema SHALL NOT permitir reset de fator sem aprovação e auditoria.

#### Scenario: Perda de dispositivo
- **WHEN** um usuário perde o autenticador e não possui código de recuperação
- **THEN** a recuperação exige justificativa e aprovação de pares
- **AND** todo o processo é auditado e notificado

#### Scenario: Tentativa de reset silencioso
- **WHEN** um administrador tenta remover o fator forte de outro usuário
- **THEN** a operação exige nível L3
- **AND** a identidade afetada é notificada
- **AND** o evento é auditado

### Requirement: Proteção contra abuso de autenticação
O sistema SHALL limitar tentativas e detectar padrões de ataque.

#### Scenario: Tentativas repetidas
- **WHEN** o número de falhas de autenticação excede o limite configurado
- **THEN** o sistema aplica bloqueio progressivo
- **AND** registra evento de auditoria

#### Scenario: Padrão distribuído
- **WHEN** múltiplas identidades sofrem tentativas de uma mesma origem
- **THEN** um alerta de credential stuffing é emitido

### Requirement: Auditoria integral de eventos de fator
O sistema SHALL auditar registro, uso, falha e remoção de fatores.

#### Scenario: Remoção de fator
- **WHEN** um fator forte é removido de uma identidade
- **THEN** um evento de auditoria com ator, alvo e resultado é registrado
