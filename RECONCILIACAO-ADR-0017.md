# Reconciliação — texto canônico das seções alteradas

> Compare com sua transcrição. Onde divergir, **este texto prevalece**: ele foi escrito
> junto com o ADR-0017, não derivado dele depois. Divergência de redação em `spec.md` é
> divergência de contrato, não questão de estilo.

---

## 1. `CONSTITUTION.md` — cabeçalho e I-1.3

```markdown
> **Versão:** 1.1.0 *(emenda de I-1.3 por ADR-0017 — ver Anexo B)*

**I-1.3** *(emendado por ADR-0017 em 2026-07-20)* O ArchGuard é **autossuficiente em
continuidade de runtime**: a indisponibilidade transitória de qualquer serviço adjacente
(OpenBao, OpenFGA, coletor OTLP) **não derruba** o plano de autenticação nem invalida sessões
existentes. O perfil `dev` (ArchGuard + PostgreSQL) autentica, emite tokens OIDC e audita sem
serviço externo, para fins de desenvolvimento, CI e demonstração. **A configuração suportada em
produção é o perfil `production`**, no qual a custódia de chaves em OpenBao é obrigatória
(I-4.3). Autossuficiência descreve o comportamento sob falha, não a configuração comercialmente
suportada. Perfis normativos em ADR-0017.
```

Adotei seu bump 1.0.0 → 1.1.0 no corpus de origem. Boa chamada: eu não havia versionado a emenda.

## 2. `CONSTITUTION.md` — Anexo B

```markdown
## Anexo B — Registro de emendas

| Data | Invariante | ADR | Motivo |
|---|---|---|---|
| 2026-07-20 | I-1.3 | ADR-0017 | Contradição com I-4.3 e RFC-0001: autossuficiência de runtime confundida com configuração suportada em produção. Invariante pétreo I-4.3 preservado sem alteração. |
```

## 3. `RFC-0001` §4 — linha da tabela de portas

```markdown
| `KeyCustodian` | OpenBao (HTTP) | **Sim** em `pilot`/`production`; keystore local selado em `dev` (ADR-0017) | Cache curto; expirado ⇒ emissão degrada e L3 falha fechado |
```

## 4. `RFC-0001` §6 — perfis (substitui o parágrafo "Mínimo suportado (piloto)")

```markdown
**Perfis de implantação (normativo: ADR-0017).** O perfil é configuração explícita e
obrigatória; ausência de declaração é erro fatal de inicialização.

| Perfil | Composição | Custódia | Uso |
|---|---|---|---|
| `dev` | Core + PostgreSQL | Keystore local selado | Desenvolvimento, CI, smoke test, demonstração. **Não suportado em produção**; operações L3 negadas; marcado como não conforme no health check |
| `pilot` | Core + PostgreSQL + OpenBao | OpenBao | Piloto e homologação. Sem OpenFGA, decisões privilegiadas granulares ficam indisponíveis (negadas), não permissivas |
| `production` | Core + PostgreSQL + OpenBao (HA) + OpenFGA + coletor OTLP | OpenBao (HA) | **Única configuração suportada comercialmente** |
```

## 5. `RFC-0001` §7 — linha acrescida na tabela de degradação

```markdown
| Perfil `dev` em uso | Operações L3 **negadas**; instalação marcada como não conforme; recusa de inicialização sob indício de exposição pública (ADR-0017) |
```

## 6. `ADR-0012` — item do modo degradado

```markdown
- **Perfil `dev`**: keystore local selado (chave cifrada fora do banco; material de selagem
  fornecido no boot, nunca persistido junto nem no banco) — **explicitamente não suportado em
  produção**, com operações L3 negadas e *health check* sinalizando não conformidade.
  Especificação normativa em **ADR-0017**.
- Indisponibilidade prolongada do cofre: emissão de novos tokens degrada primeiro; operações
  L3 falham fechado.
```

## 7. `spec.md` do pacote 001 — três requirements novos + reescrita do smoke test

Inseridos ao final do arquivo, substituindo o antigo `### Requirement: Implantação mínima funcional`.

```markdown
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
```

## 8. `tasks.md` do pacote 001 — ordem normativa e Bloco 0

```markdown
## Ordem de execução (normativa)

Executar por blocos, **nesta ordem** — que difere da numeração sequencial:

| Bloco | Tarefas | Motivo |
|---|---|---|
| 1 | T-000 a T-006 | Ambiente, fork point, licença, atribuição |
| **2** | **T-018, T-019** | **Suíte de invariantes e CI ANTES das remoções**, para que a remoção da senha-mestra (T-011) nasça verificada por teste no momento em que acontece |
| 3 | T-007 a T-014 | Remoções de escopo e PostgreSQL único |
| 4 | T-015 a T-017 | Fronteiras de framework e rebranding |
| 5 | T-020 a T-025 | Perfis, imagem, stack, smoke test, watcher, docs |

## Bloco 0 — Preparação do ambiente

- [ ] **T-000a** Inicializar repositório git; clonar o Casdoor como base do fork.
- [ ] **T-000b** Mover o corpus de governança de `spec/` para a raiz, conforme os caminhos
      referenciados no `CLAUDE.md` (`CONSTITUTION.md`, `docs/adr/`, `docs/rfc/`,
      `openspec/changes/`).
- [ ] **T-000c** Criar `Makefile` com os alvos do gate (`lint`, `test`, `invariants`,
      `deps-check`, `sbom`, `build`). Os alvos `invariants`, `deps-check` e `sbom` começam como
      *stub* que falha com mensagem explícita, e são implementados de fato em T-018/T-019 —
      gate parcial nas tarefas do Bloco 1 é esperado e aceito.
```

### Tarefas acrescidas e alteradas nos Blocos 5

```markdown
- [ ] **T-020a** Implementar perfis de implantação `dev`/`pilot`/`production` com declaração
      obrigatória e reporte no health check (ADR-0017).
- [ ] **T-020b** Implementar keystore local selado do perfil `dev` (chave cifrada fora do banco;
      material de selagem no boot; recusa de inicialização sem material).
- [ ] **T-020c** Implementar travas do perfil `dev`: aviso, marca de não conformidade, negação
      de L3 e recusa sob indício de exposição pública.
- [ ] **T-022** Smoke test ponta a ponta no perfil `dev`: subir, autenticar, emitir token OIDC,
      validar JWKS e verificar estabilidade do JWKS entre reinícios.
- [ ] **T-025** Registrar em `DIVERGENCE.md` as remoções e a introdução de perfis.
```

### Gate de verificação atualizado

```markdown
## Gate de verificação
CI verde ponta a ponta; suíte de invariantes falhando corretamente quando um comportamento
proibido é injetado (teste do teste); smoke test aprovado no perfil `dev` com JWKS estável entre
reinícios; nenhuma chave privada no banco em qualquer perfil; `NOTICE` revisado; due diligence
de licença iniciada com o jurídico.
```