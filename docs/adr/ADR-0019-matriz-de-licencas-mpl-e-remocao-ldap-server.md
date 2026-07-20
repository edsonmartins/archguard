# ADR-0019 — Correção da matriz de licenças: MPL-2.0 linkado e remoção do servidor LDAP embutido

- **Status:** **Proposto — aguarda ratificação dos sócios-fundadores (I-10.2)**
- **Data:** 2026-07-20
- **Tipo:** **Emenda constitucional a invariante pétreo** (I-2.2, seção 2)
- **Invariantes tocados:** I-2.2 (corrigido), I-2.3 (estendido), I-8.3 (aplicado)
- **Impacta:** ADR-0002, ADR-0015 §3, RFC-0007 §6, `openspec/changes/001-bootstrap-fork/`
- **Origem:** inventário de licenças do T-018/T-019, que encontrou três dependências MPL-2.0 e
  uma GPL herdadas transitivamente do upstream

---

## Parte I — Contexto: o erro de redação do I-2.2

### O que o I-2.2 diz hoje

> "Nenhuma dependência sob **AGPL, GPL, SSPL, BUSL ou licença 'community' com restrição por
> porte de empresa** entra na árvore de build do ArchGuard. Dependências MPL-2.0 são permitidas
> apenas como **serviço externo em processo separado**, nunca linkadas."

### Por que está errado

A regra trata a MPL-2.0 como se seu gatilho de copyleft fosse a **linkagem**, que é o modelo da
GPL/LGPL. Não é. A MPL-2.0 é **copyleft por arquivo**:

- **§3.2** obriga a disponibilizar a fonte do *Covered Software* — os arquivos originalmente
  MPL e suas modificações.
- **§3.3** autoriza expressamente distribuir a *Larger Work* (a obra combinada) sob termos
  diferentes, **inclusive proprietários**, desde que os arquivos MPL permaneçam sob MPL.

Ou seja: linkar uma biblioteca MPL **não modificada** a um binário proprietário é conduta
prevista e permitida pela própria licença. A obrigação resultante é de **aviso e
disponibilização** dos arquivos MPL — trivialmente satisfeita quando a dependência é consumida
sem modificação, já que a fonte está publicada no upstream.

O erro é meu, na redação original. A **intenção** do I-2.2 era correta e permanece válida:
impedir que copyleft contamine o produto proprietário. A **regra escrita** para implementar essa
intenção era tecnicamente incorreta e mais restritiva que o necessário.

### O achado concreto

O inventário do T-018/T-019 revelou que o próprio **Beego** — herdado do upstream, base de toda
a camada web — linka MPL transitivamente. Sob a redação vigente, o ArchGuard viola seu próprio
invariante pétreo desde o commit do fork point. Três dependências MPL sobreviventes e uma GPL
(`goldap`).

---

## Parte II — Decisão sobre MPL-2.0

### 1. Correção do I-2.2

> **Texto vigente (a ser substituído):** "Dependências MPL-2.0 são permitidas apenas como
> serviço externo em processo separado, nunca linkadas."

> **Texto proposto:** "Dependências sob copyleft por arquivo (MPL-2.0, EPL-2.0, CDDL) são
> permitidas **linkadas ao binário quando consumidas sem modificação**, sujeitas às obrigações
> de aviso, atribuição e disponibilização de fonte dos arquivos cobertos. Dependência sob
> copyleft por arquivo que seja **modificada, vendorizada com patch ou forkada** só é permitida
> como **serviço externo em processo separado** — caso contrário, os arquivos modificados
> tornam-se publicáveis sob a licença original, o que é incompatível com o produto proprietário.
> Nenhuma dependência sob AGPL, GPL, LGPL linkada, SSPL, BUSL ou licença com restrição por porte
> de empresa entra na árvore de build, em qualquer circunstância."

### 2. Classificação em três estados (substitui a matriz binária do ADR-0002)

| Estado | Condição | Permissão |
|---|---|---|
| **MPL não modificado** | Consumido do upstream, sem patch, sem vendorização alterada | **Linkagem permitida.** Obrigatório: `NOTICE`, SBOM, e referência à fonte upstream |
| **MPL modificado** | Qualquer patch, fork ou alteração de arquivo coberto | **Proibido linkado.** Só em processo separado (caso OpenBao) |
| **Copyleft forte** | AGPL, GPL, LGPL linkada, SSPL, BUSL | **Proibido**, sem exceção |

### 3. A armadilha que o gate precisa detectar

O risco real não é a presença de MPL — é a **transição silenciosa do estado 1 para o estado 2**.
Um patch aplicado a uma dependência MPL para corrigir um bug converte, sem aviso, uma linkagem
permitida em obrigação de publicação.

Portanto o gate do T-019 **não** verifica apenas "há MPL na árvore?". Ele verifica:

1. **Integridade da dependência MPL** — hash do módulo confere com o do proxy oficial;
2. **Ausência de patch/`replace`** apontando para cópia local de módulo MPL;
3. **Ausência de vendorização alterada** de arquivo coberto por MPL.

Qualquer um dos três ⇒ **vermelho**. Este é o controle que efetivamente protege o invariante;
proibir linkagem nunca foi.

### 4. Por que não a alternativa (b) — substituir as três MPL

Rejeitada por dois motivos:

- **É cara e recorrente.** Patch no Beego, troca da lib RADIUS e substituição do `gokrb5` são
  trabalho contínuo a cada atualização, para eliminar um risco que **não existe** sob a leitura
  correta da licença.
- **É autodestrutiva.** Aplicar patch no Beego para remover MPL exigiria forkar o Beego —
  investindo esforço permanente exatamente no componente que o **ADR-0016 classifica como dívida
  técnica transitória, a ser abandonada**. Gastar orçamento de M1 endurecendo um componente que
  se pretende descartar é o pior uso possível do recurso.

---

## Parte III — Decisão sobre a GPL (`goldap`)

### O problema

`goldap` é GPL. Copyleft forte, gatilho na linkagem, sem leitura permissiva possível. Não há
emenda que o acomode: a proibição de GPL linkada permanece absoluta na proposta do §II.1.

Ele sustenta o **servidor LDAP embutido** herdado do upstream.

### Decisão: remover o servidor LDAP embutido da v1 — não substituir a biblioteca

A remoção **não é exceção ao ADR-0015: é aplicação literal dele**. O ADR-0015 §5 estabelece três
critérios de remoção. O servidor LDAP embutido satisfaz os três:

| Critério (ADR-0015 §5) | Situação do servidor LDAP embutido |
|---|---|
| (a) Sem caso de uso em PAM no roadmap de 12 meses | **Sim.** RFC-0007 §6 já o define como canal legado, desabilitado por padrão e **proibido para operações L3** |
| (b) Amplia superfície de ataque | **Sim.** Servidor de protocolo exposto, sem `acr` e sem correlação de auditoria |
| (c) Traz dependência fora da matriz de licenças | **Sim.** É exatamente o caso `goldap`/GPL |

Substituir a biblioteca custaria orçamento de M1 — em um ecossistema Go pobre em servidores LDAP
permissivos — para preservar uma funcionalidade que a própria governança já classificou como
periférica, desligada por padrão e fora do caminho crítico.

### O que **não** é afetado

Isto precisa ficar explícito, porque a confusão entre os dois papéis é fácil e cara:

- **Cliente/conector LDAP/AD (pacote 009) permanece intacto.** É ele que sincroniza com o Active
  Directory dos clientes — o caminho de integração que realmente importa. Usa biblioteca
  distinta, sob licença permissiva.
- **Servidor RADIUS permanece.** Sua dependência é MPL não modificada, permitida sob o §II.1.

O que se perde é a capacidade de o ArchGuard **atuar como servidor LDAP** para aplicações
legadas que autenticam contra LDAP. Perda real, conscientemente aceita.

### Reabertura futura

Se um cliente exigir contratualmente o servidor LDAP, abre-se pacote próprio com: avaliação de
bibliotecas LDAP server permissivas, ou implementação em processo separado, ou implementação
própria do subconjunto necessário do protocolo. Não antes, e não em M1.

### Alterações documentais decorrentes

- **ADR-0015 §3**: retirar o servidor LDAP embutido do catálogo mantido.
- **RFC-0007 §6**: passa a tratar **apenas RADIUS** como canal legado de borda; registrar a
  remoção do servidor LDAP e a preservação integral do conector cliente.
- **Pacote 009**: nenhuma alteração — o requisito de canal legado restringe-se a RADIUS.
- **`DIVERGENCE.md`**: registrar a remoção.

---

## Parte IV — Consequências

### Positivas
- Elimina violação de invariante pétreo existente desde o fork point.
- Evita trabalho recorrente e autodestrutivo sobre um componente transitório (Beego).
- Reduz superfície de ataque coerentemente com a filosofia do ADR-0015.
- Substitui uma proibição inexequível por um **controle verificável** (detecção de patch em MPL),
  que é o que de fato protege o produto.

### Negativas
- Perda do servidor LDAP embutido como funcionalidade de v1.
- Gate de licença fica mais complexo: precisa verificar integridade de módulo, não só licença.
- A distinção MPL-modificado × não-modificado exige disciplina de PR: aplicar um patch
  "temporário" em dependência MPL passa a ser evento de conformidade, não conveniência técnica.

### Risco residual
Classificação incorreta por ferramenta automatizada. Mitigado pela regra já decidida de
**fail-closed em licença desconhecida** e pela due diligence jurídica de M1 — que ganha aqui seu
primeiro caso concreto para validar, em vez de análise abstrata.

---

## Parte V — Ratificação (I-10.2)

O I-2.2 pertence à **seção 2 (licenciamento)**, declarada pétrea na vigência da v1. Este ADR é
uma **correção de erro de redação** — preserva integralmente a intenção original (impedir
contaminação copyleft do produto proprietário) e corrige a regra técnica que a implementava de
forma incorreta. Ainda assim, por tocar seção pétrea, exige ratificação dos sócios-fundadores.

**Este ADR não entra em vigor, e o T-018 não fecha, sem as duas assinaturas abaixo.**

| Sócio | Papel | Data | Ratificação |
|---|---|---|---|
| Edson Martins | Arquiteto de Software e Soluções | ______ | ☐ |
| Neimar Chagas | Sócio-fundador | ______ | ☐ |

**Parecer jurídico:** este documento é **panorama técnico-jurídico, não parecer legal**. A
interpretação da MPL-2.0 §3.3 aqui adotada deve ser confirmada pela due diligence jurídica de M1
(ADR-0001, ADR-0002). Recomenda-se que a ratificação seja condicionada a essa confirmação, ou
que registre expressamente que a precede.
