# Fronteira Casbin × OpenFGA — plano de autorização do ArchGuard

Referência normativa: **ADR-0005** e **RFC-0004 §2**. Este documento é o guia
operacional dessa fronteira e o checklist que todo PR que toca autorização deve
cumprir (pacote 007, T-020). Em caso de conflito, a RFC-0004 prevalece.

## 1. Dois planos, uma decisão em cada

O ArchGuard tem **dois** motores de autorização, com escopos disjuntos:

| Plano | Motor | Decide |
|---|---|---|
| **AuthZ coarse** | **Casbin** (embutido, herdado do Casdoor) | Acesso a aplicações e papéis administrativos do próprio ArchGuard — decisões estáticas por papel |
| **AuthZ granular** | **PDP** (`domain.PolicyDecisionPoint`; avaliador em domínio, projetável para OpenFGA) | Acesso a **ativos privilegiados**: relação, herança por hierarquia de ativos, concessões com janela temporal, condições contextuais |

**Regra de ouro (RFC-0004 §2):** uma decisão pertence a **exatamente um** plano.
Duplicar a mesma regra nos dois é **defeito de arquitetura**, não redundância
defensiva. Todo PR que adiciona uma regra de autorização **declara o plano** a que
ela pertence.

### Como classificar uma decisão

- "Este usuário pode acessar a aplicação X?" → **Casbin**.
- "Este usuário é admin do tenant?" → **Casbin**.
- "Este membership pode abrir sessão (SSH/RDP/…) no ativo Y?" → **PDP**.
- "…dentro da janela aprovada, com concessão/break-glass vigente?" → **PDP** + contexto.
- "Quem tem acesso efetivo ao ativo Y?" (revisão de acesso) → **PDP** (consulta reversa).

Se a pergunta é **relacional** (depende de arestas entre objetos) ou **condicional**
(janela, acr, aprovação), é do PDP. Se é **estática por papel**, é do Casbin.

## 2. Propriedades inegociáveis do plano granular

Todo código do PDP obedece (verificado no gate `make invariants`):

- **Fonte da verdade é o PostgreSQL**; o PDP é **projeção derivada** (RFC-0004 §4).
  Tuplas nascem de projeção via **outbox transacional** — nunca escritas à mão,
  nunca por chamada remota dentro de transação de banco.
- **Isolamento de tenant no grafo (INV-5):** todo objeto/sujeito é qualificado por
  tenant (`org:<id>/<tipo>:<id>`). `ValidateTuple` recusa tupla cruzando
  organizações na escrita; `GuardSameTenant` nega consulta cruzada antes de resolver.
- **Fail-closed (INV-6):** PDP indisponível ⇒ **negação**. `denied` (decisão) é
  distinto de `error` (falha) na auditoria. Não existe flag de fail-open — o
  `domain.DecisionOutcome` é o único ponto de colapso e não é expressável de outra forma.
- **Sem cache para a decisão privilegiada** (RFC-0004 §5): cache curto **apenas**
  para listagens de revisão.
- **Concessão expira no grafo** (RFC-0004 §3): `has_active_grant` é tupla
  condicionada por `valid_window`; fora da janela nega ainda que a tupla persista.
- **Justificativa na auditoria** (RFC-0004 §5): a decisão e seu caminho de
  resolução entram no evento de auditoria (`BuildDecisionAuditInput`).
- **Reconciliação assimétrica** (RFC-0004 §6): divergência que **restringe** →
  correção automática; que **amplia** → alerta e revisão humana.

## 3. Checklist de PR (autorização)

Marque explicitamente no PR que toca autorização:

- [ ] **Plano declarado**: a regra é de Casbin (coarse) **ou** do PDP (granular),
      nunca das duas. Nenhuma decisão foi duplicada entre os planos.
- [ ] Nenhuma regra de negócio de PAM em controlador Beego (INV-3 / handlers finos).
- [ ] Toda tupla é **qualificada por tenant** e passa `ValidateTuple` (INV-5).
- [ ] Mutação de autorização reflete no grafo **via outbox transacional**, nunca
      por chamada remota dentro da transação.
- [ ] Caminho de decisão é **fail-closed**: erro ⇒ negação; `denied` distinto de
      `error` na auditoria (INV-6). Nenhuma flag/config de fail-open introduzida.
- [ ] Decisão de **abertura de sessão privilegiada não usa cache**.
- [ ] Concessão modelada como **tupla condicionada** (expira no grafo), não como
      filtro só na aplicação.
- [ ] Decisão anexa **justificativa** ao evento de auditoria.
- [ ] Cenários da spec (`fine-grained-authz`) têm teste correspondente **rodando**
      (não `t.Skip`, não asserção trivial).
- [ ] Nenhum tipo de SDK de PDP vaza para `internal/domain/**` (ADR-0005 §7 /
      INV-3): a troca de motor permanece contida atrás de `PolicyDecisionPoint`.
