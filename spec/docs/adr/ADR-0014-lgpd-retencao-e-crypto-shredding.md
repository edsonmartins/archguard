# ADR-0014 — Conformidade LGPD: retenção, eliminação e crypto-shredding

- **Status:** Aceito
- **Data:** 2026-07-19
- **Invariantes tocados:** I-3.1 a I-3.4, I-5.2, I-5.3

## Contexto

O ArchGuard trata dados pessoais de titulares (colaboradores e operadores dos clientes) e é
operado por clientes brasileiros sujeitos à LGPD (Lei 13.709/2018). Surge um conflito real e
frequentemente mal resolvido:

- **LGPD, Art. 18, VI**: o titular tem direito à eliminação dos dados pessoais.
- **ADR-0007 / I-5.2**: a trilha de auditoria é imutável — apagar um evento destrói a cadeia e
  a prova de integridade que sustenta o produto.

Apagar registros de auditoria para atender à eliminação inviabilizaria o PAM. Ignorar a
eliminação inviabilizaria a venda.

## Decisão

### 1. Papéis
Nas instalações on-premises, o **cliente é o controlador** e a IntegrAllTech, quando presta
operação, é **operadora**. A documentação do produto declara isso explicitamente e fornece
material de apoio ao RIPD do cliente.

### 2. Classificação obrigatória no modelo de dados
Todo campo que contenha dado pessoal é classificado em metadados de migração com: categoria,
finalidade, base legal e prazo de retenção. **Migration com campo pessoal não classificado é
rejeitada no CI.** Sem isso, conformidade vira exercício de documentação desatualizada.

### 3. Bases legais aplicáveis (referência)
Dados de identidade e trilha de acesso sustentam-se tipicamente em **cumprimento de obrigação
legal/regulatória** e **legítimo interesse** em segurança da informação — o que é justamente o
fundamento para **não** eliminar a trilha de acesso privilegiado. O produto expõe isso de
forma auditável, mas a determinação final da base legal é do controlador.

### 4. Eliminação por crypto-shredding
Dados pessoais diretos (nome, e-mail, identificadores externos) são cifrados com **chave por
titular** custodiada no OpenBao (ADR-0012):

- **Eliminação = destruição da chave.** O dado torna-se irrecuperável em todas as cópias,
  inclusive backups, **sem alterar um único byte da cadeia de auditoria**.
- A trilha preserva o **pseudônimo estável** (`sub` opaco), mantendo íntegras a cadeia e a
  utilidade forense: continua sendo possível provar que *uma identidade específica* acessou
  determinado ativo, sem revelar quem era.
- O ato de eliminação é, ele próprio, evento de auditoria.

### 5. Retenção e arquivamento
- Retenção configurável por tenant, com **mínimo** definido por política (auditoria de acesso
  privilegiado tem valor probatório de longo prazo).
- Expiração leva ao **arquivamento** da partição selada (ADR-0009), não à deleção seletiva de
  eventos.
- Retenção de telemetria (ADR-0013) é curta e independente — e não substitui a trilha.

### 6. Direitos do titular
Acesso, correção e portabilidade são atendidos por exportação estruturada por identidade,
com escopo por tenant (ADR-0006) e sem vazar dados de outra organização. Toda requisição de
titular é auditada.

### 7. Incidentes
Runbook de notificação (ANPD e titulares) com prazos, e evidências extraídas da trilha
verificável.

## Alternativas consideradas

| Opção | Por que não |
|---|---|
| Apagar eventos de auditoria do titular | Quebra a cadeia (I-5.2/I-5.3) e destrói o valor probatório do PAM |
| Anonimizar por sobrescrita no lugar | É `UPDATE` na tabela de auditoria — proibido por ADR-0009; e não alcança backups |
| Recusar a eliminação alegando obrigação legal | Defensável para a trilha, mas insuficiente para dados de perfil; crypto-shredding atende ambos |

## Consequências

- Custo criptográfico em leitura de dados pessoais e complexidade de gestão de chaves por
  titular.
- **Perder a chave de um titular é eliminação irreversível** — precisa estar no runbook e na
  UI com confirmação explícita e aprovação (operação L3).
- Este ADR é panorama técnico, **não parecer jurídico**: validação com advogado e com o DPO do
  cliente é pré-requisito de GA.
