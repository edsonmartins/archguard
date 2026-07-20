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

### Requirement: Implantação mínima funcional
O sistema SHALL iniciar e autenticar com apenas core e PostgreSQL disponíveis, sob o perfil
`dev` (ADR-0017).

#### Scenario: Smoke test
- **WHEN** a stack mínima é iniciada **com o perfil `dev` declarado** e um usuário válido
  autentica
- **THEN** um token OIDC é emitido com sucesso
- **AND** o endpoint de descoberta responde com JWKS válido

### Requirement: Perfil de implantação explícito
O sistema SHALL exigir a declaração de um perfil de implantação (`dev`, `pilot` ou
`production`) na inicialização (ADR-0017, §1).

#### Scenario: Perfil não declarado
- **WHEN** a aplicação é iniciada sem perfil de implantação declarado
- **THEN** a inicialização falha com erro fatal explícito

#### Scenario: Perfil reportado
- **WHEN** os endpoints de health check (`/healthz`, `/readyz`) são consultados
- **THEN** a resposta informa o perfil ativo e o custodiante de chaves em uso

### Requirement: Custódia de chaves no perfil dev
O sistema SHALL, no perfil `dev`, manter a chave de assinatura em keystore local selado:
cifrada (AEAD), fora do banco de dados, com material de selagem fornecido no boot
(ADR-0017, §3).

#### Scenario: Material de selagem ausente
- **WHEN** a aplicação é iniciada no perfil `dev` sem o material de selagem
- **THEN** o processo recusa iniciar
- **AND** nenhuma chave persistida é gerada automaticamente

#### Scenario: Chave fora do banco
- **WHEN** a suíte de invariantes inspeciona o esquema e a escrita de chaves no perfil `dev`
- **THEN** nenhum material de chave privada ou de selagem está presente no banco de dados

### Requirement: Não conformidade ruidosa do perfil dev
O sistema SHALL marcar o perfil `dev` como não conforme e restringir suas capacidades
(ADR-0017, §4).

#### Scenario: Instalação marcada como não conforme
- **WHEN** a aplicação inicia no perfil `dev`
- **THEN** um aviso de inicialização é emitido
- **AND** o health check reporta `compliance: non_conformant`

#### Scenario: Operação L3 negada
- **WHEN** uma operação classificada L3 é requisitada em instalação no perfil `dev`
- **THEN** a operação é negada com evento de auditoria

#### Scenario: Exposição pública recusada
- **WHEN** a aplicação no perfil `dev` detecta indício de exposição pública (bind em
  interface externa, `issuer` com domínio público ou TLS terminado em ingress)
- **THEN** a inicialização é recusada com erro explícito
