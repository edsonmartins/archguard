# ADR-0017 — Perfis de implantação e custódia de chaves no perfil mínimo

- **Status:** Aceito
- **Data:** 2026-07-20
- **Tipo:** **Emenda constitucional** (I-10.1) — altera I-1.3
- **Invariantes tocados:** I-1.3 (emendado), I-4.3 (preservado, esclarecido), I-1.2
- **Corrige:** contradição entre `CONSTITUTION.md` I-1.3, `RFC-0001` §4/§6 e `ADR-0012`
- **Impacta:** RFC-0001 §4/§6/§7, ADR-0012, `openspec/changes/001-bootstrap-fork/`

## Contexto — a contradição

Detectada na primeira leitura do corpus, antes de qualquer código:

| Fonte | Afirmação |
|---|---|
| `CONSTITUTION.md` I-1.3 | Deployment mínimo é **ArchGuard + PostgreSQL**, autenticando e emitindo tokens OIDC "sem qualquer serviço externo obrigatório"; OpenBao listado como opcional |
| `RFC-0001` §6 | Mínimo suportado (piloto) é **core + PostgreSQL + OpenBao** |
| `RFC-0001` §4 | `KeyCustodian` (OpenBao) é **obrigatório em produção** |
| `CONSTITUTION.md` I-4.3 | Chaves de assinatura **nunca** persistidas em claro; residem no OpenBao ou KMS equivalente |
| `openspec/001` T-021/T-022 | Smoke test exige emissão de token OIDC com JWKS válido em stack core + PostgreSQL |

A pergunta que o corpus não respondia: **na stack mínima sem OpenBao, onde vive a chave privada
que assina o token do smoke test?** As duas saídas óbvias eram inaceitáveis — persistir a chave
no banco viola I-4.3, e tornar o OpenBao obrigatório viola I-1.3.

A contradição existe porque I-1.3 confundiu dois conceitos distintos: **autossuficiência de
runtime** (o plano de autenticação não pode ter dependência dura que o derrube) e
**configuração suportada em produção** (o que a IntegrAllTech se compromete a sustentar em
cliente). São coisas diferentes e precisam de redação diferente.

## Decisão

### 1. Três perfis de implantação, nomeados e verificáveis

| Perfil | Composição | Custódia de chaves | Uso |
|---|---|---|---|
| **`dev`** | Core + PostgreSQL | Keystore local selado (§3) | Desenvolvimento, CI, smoke test, demonstração |
| **`pilot`** | Core + PostgreSQL + OpenBao | OpenBao | Piloto e homologação em cliente |
| **`production`** | Core + PostgreSQL + OpenBao (HA) + OpenFGA + coletor OTLP | OpenBao (HA) | **Única configuração suportada comercialmente** |

O perfil é **configuração explícita e obrigatória** na inicialização. Não há valor inferido:
subir sem declarar perfil é erro fatal de inicialização (I-4.4 — default seguro exige escolha
consciente, não conveniente).

### 2. Emenda a I-1.3

> **Texto anterior (revogado):** "O ArchGuard é autossuficiente em runtime: um deployment
> mínimo (ArchGuard + PostgreSQL) deve autenticar, emitir tokens OIDC e auditar sem qualquer
> serviço externo obrigatório. Componentes opcionais (OpenFGA, OpenBao, coletor OTLP) degradam
> funcionalidade, nunca disponibilidade da autenticação."

> **Texto vigente:** "O ArchGuard é **autossuficiente em continuidade de runtime**: a
> indisponibilidade transitória de qualquer serviço adjacente (OpenBao, OpenFGA, coletor OTLP)
> **não derruba** o plano de autenticação nem invalida sessões existentes. O perfil `dev`
> (ArchGuard + PostgreSQL) autentica, emite tokens OIDC e audita sem serviço externo, para fins
> de desenvolvimento, CI e demonstração. **A configuração suportada em produção é o perfil
> `production`**, no qual a custódia de chaves em OpenBao é obrigatória (I-4.3). Autossuficiência
> descreve o comportamento sob falha, não a configuração comercialmente suportada."

Justificativa da emenda: a redação anterior transformava uma propriedade de resiliência em
promessa de topologia, colidindo com o invariante pétreo I-4.3. A emenda preserva integralmente
a intenção original — nenhum cliente fica sem autenticar porque um serviço adjacente caiu — e
elimina a colisão. **I-4.3 não é alterado**; seção 4 é pétrea.

### 3. Keystore local selado (perfil `dev`)

A chave de assinatura do perfil `dev` **não** é persistida em claro, o que preservaria I-4.3
mesmo se o perfil fosse usado indevidamente:

- Chave privada cifrada (AEAD) em arquivo fora do banco de dados;
- Chave de selagem (*unseal*) fornecida por variável de ambiente ou arquivo montado no
  boot — **nunca** derivada de valor padrão, **nunca** persistida junto ao keystore, **nunca**
  no banco;
- Ausência do material de selagem ⇒ o processo **não inicia**. Não há geração automática
  silenciosa de chave persistida;
- Rotação não é suportada no perfil `dev` — é precisamente por isso que ele não é de produção.

### 4. Conformidade verificável e ruidosa

- `GET /healthz` e `/readyz` reportam o perfil ativo e o custodiante em uso.
- Perfil `dev` emite **aviso de inicialização** e marca a instalação como **não conforme**
  (`compliance: non_conformant`) no health check e nas métricas.
- Perfil `dev` com indicativo de exposição pública (bind em interface externa, `issuer` com
  domínio público, TLS terminado em ingress) ⇒ **recusa de inicialização**.
- Operações de nível L3 são **negadas** no perfil `dev`. Um ambiente sem custódia real não abre
  sessão privilegiada, não aprova break-glass e não rotaciona chave.
- Selagem de auditoria no perfil `dev` usa o keystore local; o selo carrega marcação de
  procedência `dev` e **não é aceito** como prova em verificação de instalação `production`.

### 5. Alinhamento documental (executado junto com este ADR)

- `RFC-0001` §4: `KeyCustodian` passa a "**Sim** (perfis `pilot`/`production`)".
- `RFC-0001` §6: substitui "mínimo suportado (piloto)" pela tabela de três perfis.
- `RFC-0001` §7: acrescenta linha de degradação por perfil.
- `ADR-0012`: o "modo de instalação mínima (`dev`/`edge`)" passa a referenciar este ADR como
  fonte normativa e adota o nome `dev`.
- `openspec/changes/001-bootstrap-fork`: smoke test declara perfil `dev` explicitamente;
  novo requisito de conformidade de perfil na `spec.md`.

## Alternativas consideradas

| Opção | Por que não |
|---|---|
| Tornar OpenBao obrigatório sempre | Inviabiliza CI, desenvolvimento local e demonstração; empurraria a equipe a contornos informais — o pior desfecho |
| Persistir chave no banco no modo mínimo | Viola I-4.3 (pétreo). Dump do banco entregaria a capacidade de assinar qualquer token |
| Manter I-1.3 e não emitir token no smoke test | Esvazia o smoke test: "sobe e conecta no banco" não prova que o produto funciona |
| Gerar chave efêmera em memória a cada boot | Invalida todos os tokens a cada restart e impede o próprio smoke test de verificar JWKS estável entre reinícios |

## Consequências

### Positivas
- Contradição eliminada com preservação integral dos invariantes pétreos.
- Perfil vira conceito de primeira classe: operável, observável e testável.
- Instalação fora de conformidade passa a ser **detectável**, não presumida.

### Negativas
- Mais um eixo de configuração e uma matriz de teste por perfil.
- Implementação do keystore local é código criptográfico adicional que existe **apenas** para
  não-produção — custo aceito para não comprometer I-4.3 nem a ergonomia de desenvolvimento.
- Risco de uso indevido do perfil `dev` em produção, mitigado pelas quatro travas do §4
  (aviso, não conformidade, recusa sob exposição pública, negação de L3).

## Nota de processo

Esta contradição foi detectada pela leitura obrigatória do corpus antes da primeira linha de
código, exatamente como o método prevê (I-9.1, CLAUDE.md §2). O custo de corrigi-la aqui foi um
ADR. O custo de descobri-la no M6, com clientes em piloto e chaves já emitidas, teria sido
migração de material criptográfico em produção.
