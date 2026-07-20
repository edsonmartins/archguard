# Spec — Capability: directory-sync

## ADDED Requirements

### Requirement: Sincronização incremental com diretório corporativo
O sistema SHALL sincronizar usuários e grupos a partir de diretório LDAP/AD por organização.

#### Scenario: Novo usuário no diretório
- **WHEN** um usuário dentro do escopo configurado é criado no diretório
- **AND** a sincronização é executada
- **THEN** um membership correspondente é criado na organização
- **AND** o evento é auditado

#### Scenario: Escopo não definido
- **WHEN** um administrador configura conector sem filtro de escopo
- **THEN** a configuração é rejeitada

### Requirement: Desprovisionamento reflete o diretório
O sistema SHALL suspender acesso quando a identidade é desativada na origem.

#### Scenario: Usuário desativado no diretório
- **WHEN** um usuário sincronizado é desativado no diretório
- **AND** a sincronização é executada
- **THEN** o membership correspondente é suspenso
- **AND** as sessões daquele tenant são encerradas
- **AND** nenhum registro histórico é removido

### Requirement: Precedência de autoridade sobre atributos e papéis
O sistema SHALL tratar o diretório como autoritativo para atributos e grupos, e o ArchGuard
como autoritativo para papéis e concessões privilegiadas.

#### Scenario: Grupo de diretório sem mapeamento aprovado
- **WHEN** um usuário pertence a grupo do diretório sem mapeamento explícito aprovado
- **THEN** nenhum papel privilegiado é concedido automaticamente

### Requirement: Provisionamento SCIM de entrada
O sistema SHALL aceitar provisionamento SCIM 2.0 originado do IdP do cliente.

#### Scenario: Criação via SCIM
- **WHEN** o IdP envia requisição SCIM de criação de usuário
- **THEN** o sistema cria identidade e membership na organização correspondente
- **AND** responde conforme a especificação SCIM 2.0

#### Scenario: Usuário já existente
- **WHEN** o SCIM provisiona e-mail já associado a identidade existente
- **THEN** o sistema cria apenas o membership
- **AND** NOT cria identidade duplicada

#### Scenario: Desativação via SCIM
- **WHEN** o IdP marca o usuário como inativo via SCIM
- **THEN** o membership correspondente é suspenso

### Requirement: Federação de login sem duplicação de identidade
O sistema SHALL vincular login federado a identidade existente quando aplicável.

#### Scenario: JIT provisioning com e-mail conhecido
- **WHEN** um usuário autentica por IdP corporativo com e-mail já conhecido
- **THEN** o sistema cria ou ativa o membership da identidade existente
- **AND** NOT cria nova identidade

### Requirement: Garantia de nível não delegável
O sistema SHALL NOT aceitar nível de garantia informado por terceiro para operações L3.

#### Scenario: `acr` externo elevado
- **WHEN** um IdP externo declara nível de garantia elevado
- **AND** o usuário solicita operação de nível L3
- **THEN** o ArchGuard exige step-up com fator verificado por ele próprio

### Requirement: Canais legados restritos
O sistema SHALL restringir os servidores LDAP e RADIUS embutidos.

#### Scenario: Estado padrão
- **WHEN** uma instalação nova é iniciada
- **THEN** os servidores LDAP e RADIUS embutidos estão desabilitados

#### Scenario: Operação privilegiada por canal legado
- **WHEN** uma autenticação ocorre por LDAP ou RADIUS embutido
- **THEN** a sessão resultante NOT autoriza operações de nível L3
- **AND** o evento é auditado e sinalizado como canal legado

### Requirement: Importação sem senha
O sistema SHALL NOT importar credenciais de sistemas de origem.

#### Scenario: Lote importado
- **WHEN** identidades são importadas de exportação externa
- **THEN** entram em estado de enrolamento obrigatório
- **AND** nenhuma senha da origem é aceita para autenticação
