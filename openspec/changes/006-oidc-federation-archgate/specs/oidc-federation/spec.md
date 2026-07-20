# Spec — Capability: oidc-federation

## ADDED Requirements

### Requirement: Contrato de claims versionado
O sistema SHALL emitir tokens conforme contrato de claims versionado e documentado.

#### Scenario: Emissão padrão
- **WHEN** um token é emitido para um componente registrado
- **THEN** contém `iss`, `sub` opaco, `org`, `mid`, `acr`, `amr`, `auth_time` e `sid`

#### Scenario: Dado pessoal restrito
- **WHEN** um cliente sem escopo de e-mail solicita token
- **THEN** o token NOT contém endereço de e-mail em claro

#### Scenario: Claims do tenant ativo
- **WHEN** uma identidade com múltiplos memberships obtém token
- **THEN** `groups` e `roles` referem-se exclusivamente ao tenant ativo

### Requirement: Fluxos de autorização suportados
O sistema SHALL suportar apenas fluxos considerados seguros.

#### Scenario: PKCE ausente
- **WHEN** um cliente inicia Authorization Code sem PKCE
- **THEN** a requisição é recusada

#### Scenario: Fluxo obsoleto
- **WHEN** um cliente solicita fluxo implicit ou ROPC
- **THEN** a requisição é recusada

#### Scenario: Device flow em operação crítica
- **WHEN** um token obtido por device flow é usado para operação de nível L3
- **THEN** a operação é negada

### Requirement: Audiência específica por componente
O sistema SHALL emitir tokens com audiência restrita ao componente destinatário.

#### Scenario: Reuso entre componentes
- **WHEN** um token emitido para o componente A é apresentado ao componente B
- **THEN** o componente B rejeita o token por audiência inválida

### Requirement: Rotação de refresh token com detecção de reuso
O sistema SHALL rotacionar refresh tokens e detectar reuso.

#### Scenario: Renovação normal
- **WHEN** um refresh token válido é utilizado
- **THEN** um novo refresh token é emitido e o anterior é invalidado

#### Scenario: Reuso detectado
- **WHEN** um refresh token já rotacionado é apresentado novamente
- **THEN** toda a família de tokens da sessão é revogada
- **AND** um evento de auditoria de severidade alta é registrado

### Requirement: Propagação de encerramento de sessão
O sistema SHALL propagar o encerramento de sessão aos componentes.

#### Scenario: Logout no ArchGuard
- **WHEN** um usuário encerra a sessão no ArchGuard
- **THEN** o back-channel logout é enviado aos componentes com sessão ativa
- **AND** as sessões derivadas são encerradas

#### Scenario: Revogação de membership
- **WHEN** o membership de um usuário é revogado
- **THEN** as sessões e tokens associados àquele tenant são revogados imediatamente

### Requirement: Propagação de nível de garantia
O sistema SHALL comunicar o nível de garantia obtido para que componentes possam recusar
acesso insuficiente.

#### Scenario: Garantia insuficiente no componente
- **WHEN** um componente exige L3 e recebe token com `acr` correspondente a L2
- **THEN** o componente recusa a operação
- **AND** o usuário é direcionado a step-up no ArchGuard

### Requirement: Correlação de auditoria entre planos
O sistema SHALL emitir identificador de correlação de sessão privilegiada.

#### Scenario: Linha do tempo unificada
- **WHEN** uma sessão privilegiada é aberta via componente do ArchGate
- **THEN** o token contém `pcid`
- **AND** os eventos de auditoria do ArchGuard e do componente compartilham esse identificador
- **AND** é possível reconstruir a sequência completa do acesso

### Requirement: Rotação de chaves de assinatura sem indisponibilidade
O sistema SHALL rotacionar chaves de assinatura mantendo a validação de tokens em circulação.

#### Scenario: Rotação com sobreposição
- **WHEN** a chave de assinatura é rotacionada
- **THEN** o JWKS publica simultaneamente a chave nova e a anterior
- **AND** tokens emitidos antes da rotação continuam válidos até expirar

#### Scenario: `kid` desconhecido
- **WHEN** um componente recebe token com `kid` ausente de seu cache
- **THEN** renova o JWKS antes de rejeitar o token
