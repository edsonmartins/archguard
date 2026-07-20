# RFC-0004 — Modelo de autorização e integração com OpenFGA

- **Status:** Proposto
- **Data:** 2026-07-19
- **ADRs relacionados:** ADR-0005, ADR-0006, ADR-0010, ADR-0007

## 1. Objetivo

Definir a fronteira entre autorização coarse-grained (Casbin, herdado) e granular (OpenFGA), o
modelo de autorização de acesso privilegiado, o mecanismo de sincronização e o comportamento
sob falha.

## 2. Fronteira normativa entre os dois planos

| Pergunta | Responsável | Justificativa |
|---|---|---|
| "Este usuário pode acessar a aplicação X?" | **Casbin** | Decisão estática por papel |
| "Este usuário é admin do tenant?" | **Casbin** | Papel administrativo do próprio ArchGuard |
| "Este membership pode abrir sessão SSH no ativo `db-prod-03`?" | **OpenFGA** | Relacional, com herança de ativos |
| "Este acesso está dentro da janela aprovada, com break-glass vigente?" | **OpenFGA** + contexto | Condicional |
| "Quem tem acesso efetivo ao ativo Y?" (revisão de acesso) | **OpenFGA** | Consulta reversa no grafo |

**Regra de ouro:** uma decisão pertence a **exatamente um** plano. Duplicar regra nos dois é
defeito de arquitetura, não redundância defensiva. Toda regra nova declara seu plano no PR.

## 3. Modelo de autorização (esboço de tipos)

Tipos: `organization`, `membership`, `group`, `asset_group`, `asset`, `access_policy`.

Relações essenciais:

```
type asset
  relations
    define parent: [asset_group]
    define owner: [membership]
    define operator: [membership, group#member] or operator from parent
    define auditor: [membership, group#member] or auditor from parent
    define can_open_session: operator or owner
    define can_open_privileged_session: can_open_session and has_active_grant
    define has_active_grant: [membership with valid_window]
```

Notas:
- **Herança por hierarquia de ativos** (`operator from parent`) elimina a explosão
  combinatória que motivou o ADR-0005.
- `has_active_grant` materializa concessões temporárias e break-glass como relação com
  condição de janela temporal — a concessão expira no grafo, não apenas na aplicação.
- Grupos entram como `group#member`, refletindo grupos do tenant e do diretório sincronizado.
- **Todo objeto é qualificado por tenant** no identificador (`org:<id>/asset:<id>`),
  impedindo relação cruzando organizações (I-6.3).

## 4. Sincronização (fonte da verdade)

O PostgreSQL do ArchGuard é a **fonte da verdade**; o OpenFGA é **projeção derivada**.

```
mutação de domínio ──► transação ──► tabela + registro em outbox
                                            │
                                    publisher assíncrono
                                            ▼
                                   escrita de tuplas no OpenFGA
```

- **Outbox transacional**: a intenção de sincronizar é persistida na mesma transação da
  mudança. Nunca há chamada remota dentro da transação (RFC-0002 §5).
- **Idempotência**: escritas de tupla são idempotentes e reprocessáveis.
- **Reconciliação periódica**: varredura comparando o estado derivado esperado com as tuplas
  existentes; divergência gera métrica e alerta (ADR-0013) e é corrigida automaticamente
  quando segura.
- **Bootstrap e replay**: capacidade de reconstruir o store do zero a partir do banco —
  requisito de recuperação de desastre e de migração de PDP.

## 5. Caminho de decisão

1. Aplicação monta a consulta: `check(user=membership:<id>, relation=<rel>, object=<obj>, context={...})`.
2. Contexto inclui `acr` (ADR-0010), janela temporal, aprovações vigentes e origem.
3. Resposta é **cacheada por tempo muito curto** apenas para leituras de listagem; decisões de
   abertura de sessão privilegiada **nunca usam cache**.
4. A decisão (permitida/negada) e sua justificativa entram no evento de auditoria (RFC-0003).

## 6. Comportamento sob falha (fail-closed)

| Situação | Comportamento |
|---|---|
| PDP indisponível | Decisões granulares **negadas**; AuthN e emissão de token seguem; alerta |
| Timeout do PDP | Negação com evento `error` distinto de `denied` |
| Divergência detectada na reconciliação | Alerta; correção automática apenas para casos que **restringem** acesso; ampliação de acesso exige revisão |

**Não existe fail-open**, em nenhuma configuração. Não há flag para isso.

## 7. Portabilidade

Toda interação passa pela interface `PolicyDecisionPoint` (`check`, `listObjects`, `write`,
`read`). Nenhum tipo do SDK do OpenFGA vaza para o domínio — condição para trocar por SpiceDB
caso surja requisito de consistência forte (ADR-0005).

## 8. Testes

- **Testes de modelo**: cenários declarativos de autorização (esperado permitido/negado)
  executados contra o modelo em CI, incluindo casos de herança e expiração de janela.
- **Testes de travessia**: nenhuma relação concede acesso a objeto de outro tenant.
- **Teste de reconciliação**: divergência injetada é detectada e corrigida.

## 9. Questões em aberto

- Ativos são cadastrados no ArchGuard ou importados dos componentes (Warpgate/NetBird)? A
  proposta inicial é **importação com identificador canônico no ArchGuard**, a validar no M4.
- Granularidade de ativo: host, serviço ou conta-alvo? Impacta o volume de tuplas.
- Necessidade de `ListUsers` para campanhas de revisão em larga escala e seu custo.
