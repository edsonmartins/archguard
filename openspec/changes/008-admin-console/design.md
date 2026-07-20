# Design — 008 · ArchGuard Console

Base normativa: RFC-0005.

## Contrato primeiro

OpenAPI é fonte da verdade. Cliente TypeScript gerado; chamada manual a `fetch` fora da camada
gerada é proibida por lint. Teste de contrato no CI impede *drift*.

## Estado

TanStack Query para estado de servidor. Sem store global de dados remotos. Estado local em
hooks/contexto.

## Seletor de tenant

Permanente no cabeçalho, com **indicação visual inequívoca** do tenant ativo — operar no tenant
errado é a classe de erro humano mais cara em PAM. Troca obtém novo token e pode disparar
step-up.

## Step-up transparente

Interceptor global captura o erro de garantia insuficiente, apresenta o desafio WebAuthn e
repete a operação **sem perder o estado do formulário**.

## Padrão de UX: agregados honestos com detalhe sob demanda

Toda superfície de resumo carrega sinal de severidade suficiente para indicar se o
*drill-down* é necessário. Um indicador verde no topo **não pode** coexistir com divergência de
cadeia de auditoria, falha de reconciliação do PDP ou break-glass pendente no detalhe. Se há
negativa no detalhe, o topo mostra.

## Telas críticas

**Auditoria**: linha do tempo com filtros; visão de correlação por `pcid` reunindo ArchGuard +
componentes; indicador de integridade sempre visível, com divergência em destaque máximo.

**Break-glass**: solicitação com justificativa; fila de aprovação; contagem regressiva da
concessão; revogação imediata.

**Revisão de acesso**: acesso efetivo do PDP com origem (direto/herdado/concessão); decisões em
lote, cada uma auditada.

## Segurança do cliente

Sessão por cookie `HttpOnly`/`Secure`/`SameSite` + CSRF; **sem token em `localStorage`**; CSP
restritiva sem `eval` e sem script inline; nenhum segredo no bundle; logout propaga
back-channel; inatividade encerra sessão conforme política do tenant.

## Acessibilidade e i18n

Navegação completa por teclado nos fluxos privilegiados. pt-BR primário, en-US secundário.
