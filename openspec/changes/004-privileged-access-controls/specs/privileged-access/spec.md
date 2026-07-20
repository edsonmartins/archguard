# Spec — Capability: privileged-access

## ADDED Requirements

### Requirement: Rastreabilidade do ator real em delegação
O sistema SHALL registrar o ator real e o sujeito impersonado em toda ação executada sob
delegação.

#### Scenario: Ação sob delegação
- **WHEN** um administrador opera sob delegação de outro usuário
- **THEN** o token contém o sujeito impersonado e o ator real no claim `act`
- **AND** cada evento de auditoria registra ambos

### Requirement: Escopo restrito da delegação
O sistema SHALL impedir que sessão de delegação execute operações administrativas, acesse
segredos ou aprove solicitações.

#### Scenario: Tentativa de escalada
- **WHEN** uma sessão de delegação tenta executar operação administrativa
- **THEN** a operação é negada
- **AND** o evento é auditado como tentativa de escalada

#### Scenario: Tentativa de aprovação
- **WHEN** uma sessão de delegação tenta aprovar solicitação de break-glass
- **THEN** a aprovação é recusada

### Requirement: Consentimento e visibilidade da delegação
O sistema SHALL exigir consentimento do usuário-alvo e informá-lo da delegação.

#### Scenario: Delegação padrão
- **WHEN** um administrador solicita delegação sobre um usuário
- **THEN** o sistema requer consentimento do usuário-alvo antes de iniciar a sessão
- **AND** notifica o usuário-alvo do início da sessão

#### Scenario: Revogação pelo alvo
- **WHEN** o usuário-alvo revoga a delegação em curso
- **THEN** a sessão delegada é encerrada imediatamente

### Requirement: Break-glass com justificativa e aprovação
O sistema SHALL conceder acesso emergencial somente mediante justificativa, autenticação
reforçada e aprovação de pares.

#### Scenario: Solicitação completa
- **WHEN** um operador solicita break-glass com justificativa vinculada a incidente
- **AND** conclui step-up com fator resistente a phishing
- **AND** obtém o número configurado de aprovações de pares distintos
- **THEN** a concessão torna-se ativa pela janela definida

#### Scenario: Fator insuficiente
- **WHEN** o solicitante tenta concluir o step-up apenas com TOTP
- **THEN** a solicitação é recusada

#### Scenario: Autoaprovação
- **WHEN** o solicitante tenta aprovar a própria solicitação
- **THEN** a aprovação é recusada

#### Scenario: Zero aprovadores em produção
- **WHEN** um administrador configura zero aprovadores em ambiente de produção
- **THEN** a configuração é rejeitada

### Requirement: Alerta imediato de break-glass
O sistema SHALL alertar os canais de segurança do tenant no momento da solicitação.

#### Scenario: Alerta na solicitação
- **WHEN** uma solicitação de break-glass é criada
- **THEN** um alerta é emitido imediatamente, antes de qualquer aprovação

#### Scenario: Canal indisponível
- **WHEN** nenhum canal de notificação está disponível
- **THEN** a solicitação de break-glass é negada

### Requirement: Expiração e revogação de concessões
O sistema SHALL encerrar automaticamente concessões privilegiadas ao fim da janela.

#### Scenario: Janela expirada
- **WHEN** a janela de uma concessão ativa expira
- **THEN** a concessão torna-se inativa
- **AND** as sessões derivadas são revogadas
- **AND** o evento de expiração é auditado

#### Scenario: Token emitido antes da expiração
- **WHEN** um token emitido sob a concessão é apresentado após a expiração
- **THEN** o acesso privilegiado é negado

### Requirement: Revisão pós-uso obrigatória
O sistema SHALL exigir registro de revisão após o uso de break-glass.

#### Scenario: Revisão pendente
- **WHEN** uma concessão de break-glass encerra sem revisão registrada
- **THEN** o sistema mantém a pendência visível e notifica os responsáveis

### Requirement: Contas de serviço não impersonáveis
O sistema SHALL impedir delegação sobre identidades do tipo conta de serviço.

#### Scenario: Tentativa de impersonar conta de serviço
- **WHEN** um administrador solicita delegação sobre conta de serviço
- **THEN** a solicitação é recusada

#### Scenario: Login interativo de conta de serviço
- **WHEN** uma conta de serviço tenta autenticar por fluxo interativo de navegador
- **THEN** a autenticação é recusada
