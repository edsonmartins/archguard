# Design — 007 · Autorização granular

Base normativa: RFC-0004.

## Modelo

Tipos: `organization`, `membership`, `group`, `asset_group`, `asset`, `access_policy`.
Herança por `parent` elimina a explosão combinatória. Concessões entram como relação com
condição de janela temporal — a concessão **expira no grafo**, não só na aplicação.

Todo objeto é qualificado por tenant no identificador (`org:<id>/asset:<id>`), impedindo
relação que atravesse organizações.

## Sincronização

Mutação de domínio grava tabela **e** registro de outbox na mesma transação. Publisher
assíncrono escreve tuplas de forma idempotente. Nunca há chamada remota dentro da transação.

Reconciliação periódica compara o estado derivado esperado com as tuplas existentes:
- divergência que **restringe** acesso ⇒ correção automática;
- divergência que **amplia** acesso ⇒ alerta e revisão humana (correção automática que amplia
  acesso é vetor de escalada silenciosa).

Bootstrap/replay reconstrói o store do zero a partir do banco — requisito de DR e de eventual
troca de PDP.

## Decisão

`check(user=membership:<id>, relation, object, context)` com contexto de `acr`, janela e
aprovações. **Sem cache** para abertura de sessão privilegiada; cache muito curto apenas para
listagens.

A resposta e sua justificativa entram no evento de auditoria (RFC-0003).

## Falha

Indisponibilidade ou timeout ⇒ **negação**. `error` é distinguido de `denied` na auditoria.
Não existe flag de fail-open.

## Portabilidade

Nenhum tipo do SDK vaza para o domínio. Interface `PolicyDecisionPoint` com `check`,
`listObjects`, `write`, `read`.
