# Spec — Capability: fork-baseline

## ADDED Requirements

### Requirement: Rastreabilidade do fork point
O sistema SHALL manter registro forense do commit-base do upstream a partir do qual o
ArchGuard foi derivado.

#### Scenario: Registro presente e verificável
- **WHEN** um auditor inspeciona o repositório do ArchGuard
- **THEN** encontra `docs/upstream/FORK_POINT.md` com SHA completo, tag, data e hash de
  verificação da árvore
- **AND** o `NOTICE` referencia a obra original, seus autores e a URL do repositório

### Requirement: Preservação das obrigações Apache 2.0
O sistema SHALL preservar os avisos de licença e copyright da obra original.

#### Scenario: Arquivo LICENSE intacto
- **WHEN** o `LICENSE` do ArchGuard é comparado com o do fork point
- **THEN** o conteúdo é idêntico

#### Scenario: Modificação declarada
- **WHEN** um arquivo herdado do upstream é modificado
- **THEN** o cabeçalho preserva o copyright original
- **AND** contém declaração de modificação pela IntegrAllTech

### Requirement: Ausência de credencial administrativa universal
O sistema SHALL NOT possuir qualquer mecanismo que autentique um usuário mediante credencial
que não seja dele.

#### Scenario: Senha-mestra ausente do código
- **WHEN** a suíte de invariantes é executada
- **THEN** nenhum caminho de autenticação aceita credencial de organização em lugar da
  credencial do usuário
- **AND** a coluna de senha-mestra não existe no esquema

#### Scenario: Reintrodução bloqueada
- **WHEN** um cherry-pick do upstream reintroduz autenticação por senha-mestra
- **THEN** a suíte de invariantes falha
- **AND** o build é rejeitado

### Requirement: Backend de persistência único
O sistema SHALL suportar exclusivamente PostgreSQL 15 ou superior.

#### Scenario: Dialeto não suportado
- **WHEN** a configuração aponta para banco diferente de PostgreSQL
- **THEN** a aplicação recusa iniciar com erro explícito

#### Scenario: Versão insuficiente
- **WHEN** o PostgreSQL conectado tem versão inferior a 15
- **THEN** a aplicação recusa iniciar com erro explícito

### Requirement: Isolamento do domínio em relação ao framework
O sistema SHALL manter pacotes de domínio livres de dependência de framework web e ORM.

#### Scenario: Importação proibida
- **WHEN** um pacote sob `internal/domain/**` importa o framework web ou o ORM
- **THEN** a regra de dependência do CI falha
- **AND** o build é rejeitado

### Requirement: Higiene de licenças de dependências
O sistema SHALL bloquear dependências fora da matriz de licenças aprovada.

#### Scenario: Dependência proibida introduzida
- **WHEN** uma dependência direta ou transitiva sob AGPL, GPL, SSPL ou BUSL é adicionada
- **THEN** o license gate falha e o build é rejeitado
- **AND** o SBOM gerado registra a ocorrência

### Requirement: Imutabilidade da trilha de auditoria no nível de código
O sistema SHALL NOT expor caminho de código que altere ou remova evento de auditoria.

#### Scenario: Mutação detectada
- **WHEN** a suíte de invariantes encontra `UPDATE` ou `DELETE` sobre tabela de auditoria
- **THEN** o build é rejeitado

### Requirement: Perfil de implantação explícito
O sistema SHALL exigir declaração explícita do perfil de implantação (`dev`, `pilot` ou
`production`) na inicialização.

#### Scenario: Perfil não declarado
- **WHEN** a aplicação é iniciada sem perfil declarado
- **THEN** a inicialização falha com erro explícito
- **AND** nenhum perfil é assumido por padrão

#### Scenario: Perfil reportado
- **WHEN** o endpoint de saúde é consultado
- **THEN** a resposta informa o perfil ativo e o custodiante de chaves em uso

### Requirement: Custódia de chaves sem persistência em claro em qualquer perfil
O sistema SHALL NOT persistir chave privada de assinatura em claro, inclusive no perfil `dev`.

#### Scenario: Keystore local do perfil dev
- **WHEN** a aplicação é iniciada no perfil `dev`
- **THEN** a chave de assinatura é mantida cifrada em keystore fora do banco de dados
- **AND** o material de selagem é fornecido no boot
- **AND** NOT é persistido junto ao keystore nem no banco

#### Scenario: Material de selagem ausente
- **WHEN** a aplicação é iniciada no perfil `dev` sem material de selagem
- **THEN** o processo NOT inicia
- **AND** nenhuma chave é gerada e persistida automaticamente

#### Scenario: Inspeção do banco
- **WHEN** o conteúdo do banco é inspecionado em qualquer perfil
- **THEN** nenhuma chave privada de assinatura está presente

### Requirement: Não conformidade do perfil dev é visível e restritiva
O sistema SHALL sinalizar e restringir instalações que operem no perfil `dev`.

#### Scenario: Sinalização
- **WHEN** a aplicação opera no perfil `dev`
- **THEN** emite aviso de inicialização
- **AND** o health check reporta a instalação como não conforme

#### Scenario: Operação privilegiada
- **WHEN** uma operação de nível L3 é solicitada no perfil `dev`
- **THEN** a operação é negada

#### Scenario: Indício de exposição pública
- **WHEN** o perfil `dev` é iniciado com indício de exposição pública
- **THEN** a inicialização é recusada

### Requirement: Implantação mínima funcional
O sistema SHALL iniciar, autenticar e emitir tokens no perfil `dev` com apenas core e
PostgreSQL disponíveis.

#### Scenario: Smoke test
- **WHEN** a stack do perfil `dev` é iniciada e um usuário válido autentica
- **THEN** um token OIDC é emitido com sucesso
- **AND** o endpoint de descoberta responde com JWKS válido

#### Scenario: JWKS estável entre reinícios
- **WHEN** a aplicação no perfil `dev` é reiniciada com o mesmo material de selagem
- **THEN** o JWKS publicado permanece o mesmo
- **AND** tokens emitidos antes do reinício continuam válidos até expirar
