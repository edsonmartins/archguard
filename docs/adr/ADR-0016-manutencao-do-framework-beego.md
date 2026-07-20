# ADR-0016 — Manter Beego no curto prazo, isolando-o atrás de fronteiras explícitas

- **Status:** Aceito
- **Data:** 2026-07-19
- **Invariantes tocados:** I-7.2, I-8.4, I-9.3

## Contexto

O upstream é construído sobre **Beego** (framework web Go) com **XORM** para persistência.
Ambos têm momentum modesto no ecossistema Go atual, cuja tendência dominante é
`net/http` + roteadores leves (chi, gin, echo) e `sqlc`/`pgx` para persistência. Isso tem
efeitos concretos: menor pool de contratação, menos material atualizado e risco de
manutenção reduzida das dependências no longo prazo.

A tentação é reescrever a camada web/persistência já no início do fork. Isso seria um erro
clássico: consumiria o orçamento do M1 numa mudança **sem valor perceptível para o cliente**,
maximizando o conflito com o upstream justamente quando ainda precisamos importar correções
com facilidade.

## Decisão

**Manter Beego e XORM na v1, isolando-os atrás de fronteiras explícitas para tornar a migração
futura uma decisão de negócio, não uma refatoração de emergência.**

### Fronteiras impostas a partir de agora

1. **Handlers finos.** Controladores Beego apenas traduzem HTTP↔domínio. Nenhuma regra de
   negócio nova em controlador.
2. **Domínio livre de framework.** Todo código novo (multi-tenancy, auditoria, break-glass,
   step-up, integração com PDP e cofre) vive em pacotes de domínio **sem importar Beego ou
   XORM**. Verificado por regra de dependência no CI: pacote de domínio que importe o
   framework **quebra o build**.
3. **Persistência nova via `pgx`/SQL explícito**, atrás de interfaces de repositório —
   especialmente a auditoria, cujas garantias (append-only, particionamento, papéis de banco)
   não são expressáveis confortavelmente no ORM.
4. **XORM permanece** para as tabelas herdadas, reduzindo conflito de cherry-pick no código
   preexistente.
5. **Coexistência de roteamento**: novas rotas podem ser servidas por `net/http`/chi montado ao
   lado, permitindo migração incremental por rota, sem *big bang*.

### Gatilhos de reavaliação (revisão formal)
- Vulnerabilidade relevante em Beego/XORM sem correção em prazo aceitável;
- Abandono efetivo de manutenção do framework;
- Divergência do upstream já tão alta que o benefício de compatibilidade desapareceu;
- Custo de contratação/produtividade medido como impeditivo.

Atendido qualquer gatilho, abre-se ADR de migração com pacote OpenSpec próprio.

## Alternativas consideradas

| Opção | Por que não |
|---|---|
| Migrar para chi/pgx já no M1 | Consome o orçamento inicial sem valor para o cliente; maximiza conflito de cherry-pick no momento de maior dependência do upstream |
| Reescrever o core em Java/Spring (alinhar stack) | Equivale a greenfield: perde-se a razão de forkar (protocolos prontos, criptografia validada). O alinhamento com Java se dá por **contrato OIDC/REST** (I-7.2), não por linguagem única |
| Aceitar Beego sem fronteiras | Espalha acoplamento por todo código novo, tornando a migração futura impossível na prática |

## Consequências

- Dívida técnica **conhecida, contida e monitorada** — não ignorada.
- Duas camadas de persistência convivendo (XORM legado + `pgx` novo): exige disciplina de
  transação, documentada no RFC-0002.
- Regra de dependência no CI é o que sustenta esta decisão; sem ela, o isolamento erode em
  semanas.
