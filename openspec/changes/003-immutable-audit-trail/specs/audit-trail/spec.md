# Spec — Capability: audit-trail

## ADDED Requirements

### Requirement: Registro obrigatório de eventos relevantes
O sistema SHALL registrar evento de auditoria para toda autenticação, decisão de autorização
privilegiada, mutação administrativa e acesso privilegiado.

#### Scenario: Autenticação bem-sucedida
- **WHEN** uma identidade autentica com sucesso
- **THEN** um evento com ator, resultado, contexto de origem e método de autenticação é
  persistido

#### Scenario: Autenticação negada
- **WHEN** uma tentativa de autenticação falha
- **THEN** um evento com resultado `denied` e motivo é persistido

### Requirement: Encadeamento criptográfico da trilha
O sistema SHALL encadear eventos por hash, por organização.

#### Scenario: Encadeamento na gravação
- **WHEN** um evento é persistido
- **THEN** seu `hash` é derivado do `prev_hash` e do conteúdo canônico do evento
- **AND** seu `seq` é o sucessor imediato do último `seq` daquela organização

#### Scenario: Gravações concorrentes
- **WHEN** dois eventos da mesma organização são gravados concorrentemente
- **THEN** ambos recebem `seq` distintos e consecutivos
- **AND** a cadeia permanece verificável

### Requirement: Detecção de adulteração
O sistema SHALL detectar alteração, remoção e reordenação de eventos.

#### Scenario: Evento alterado diretamente no banco
- **WHEN** o conteúdo de um evento é modificado fora da aplicação
- **AND** o verificador é executado
- **THEN** o relatório aponta divergência e identifica o primeiro `seq` afetado

#### Scenario: Evento removido diretamente no banco
- **WHEN** um evento é removido fora da aplicação
- **AND** o verificador é executado
- **THEN** o relatório aponta lacuna de sequência e quebra de cadeia

#### Scenario: Selo inválido
- **WHEN** a assinatura de um selo não confere com a chave pública correspondente ao `key_id`
- **THEN** o verificador reporta selo inválido
- **AND** um alerta de severidade máxima é emitido

### Requirement: Imutabilidade imposta pelo armazenamento
O sistema SHALL impedir mutação de eventos no nível do banco de dados.

#### Scenario: Tentativa de UPDATE
- **WHEN** o papel da aplicação tenta atualizar um evento de auditoria
- **THEN** a operação é rejeitada pelo banco

#### Scenario: Tentativa de DELETE
- **WHEN** o papel da aplicação tenta remover um evento de auditoria
- **THEN** a operação é rejeitada pelo banco

### Requirement: Escrita fail-closed em operações privilegiadas
O sistema SHALL negar operação privilegiada cujo evento de auditoria não possa ser persistido
de forma durável.

#### Scenario: Auditoria indisponível
- **WHEN** o subsistema de auditoria está indisponível
- **AND** um usuário solicita abertura de sessão privilegiada
- **THEN** a operação é negada
- **AND** o motivo retornado indica indisponibilidade de auditoria
- **AND** um alerta é emitido

### Requirement: Selagem periódica assinada
O sistema SHALL selar periodicamente o head da cadeia com assinatura custodiada em cofre.

#### Scenario: Selagem por intervalo ou volume
- **WHEN** o intervalo configurado decorre ou o volume de eventos é atingido
- **THEN** um selo com `seq_range`, `head_hash`, `key_id` e assinatura é persistido

#### Scenario: Chave privada não exposta
- **WHEN** a selagem é executada
- **THEN** a assinatura é produzida pelo cofre
- **AND** a aplicação NOT tem acesso ao material da chave privada

### Requirement: Verificação histórica após rotação de chave
O sistema SHALL permitir verificar selos antigos após rotação da chave de selagem.

#### Scenario: Selo anterior à rotação
- **WHEN** a chave de selagem é rotacionada
- **AND** o verificador processa um selo emitido antes da rotação
- **THEN** utiliza a chave pública correspondente ao `key_id` do selo
- **AND** a verificação é bem-sucedida

### Requirement: Retenção sem deleção seletiva
O sistema SHALL implementar retenção por arquivamento de partições seladas.

#### Scenario: Expiração de retenção
- **WHEN** o prazo de retenção de um período expira
- **THEN** a partição correspondente é arquivada com seus selos preservados
- **AND** nenhum evento individual é removido

### Requirement: Exportação verificável
O sistema SHALL permitir exportação assinada da trilha por organização.

#### Scenario: Exportação para auditor externo
- **WHEN** um administrador exporta a trilha de um período
- **THEN** o pacote contém eventos, selos, chaves públicas e procedimento de verificação
- **AND** a exportação é registrada como evento de auditoria
- **AND** a operação exige nível de garantia L3
