# Design — 008 · ArchGuard Console (evolução do console herdado)

> Base normativa: **ADR-0020** (ratificado 2026-07-26) + RFC-0005 (diferido, referência da opção
> greenfield). Re-escopo do 008: evoluir o console herdado.

## Stack

Console herdado do upstream: **React (CRA) + antd**, já adaptado ao ArchGuard (tema verde,
ícones Tabler webfont via shim `TablerIcons.js`, pt-BR, rebrand, remoções de escopo). A evolução
acontece **neste** stack; o rewrite Mantine/Archbase fica diferido (ADR-0020).

## Contrato de API

O console consome **exclusivamente** o `/api/v1` público (plano de controle, pacote 011). A
camada de acesso vive num módulo dedicado (padrão dos `backend/*Backend.js` existentes,
estendido para o `/api/v1`); **nenhuma chamada crua de PAM fora dele**. Um **teste de contrato no
CI** compara as chamadas do console ao OpenAPI publicado e falha em caso de *drift* — substitui o
"cliente gerado que quebra o build" do ADR-0004 (não adotamos geração de cliente no herdado; a
garantia de não-*drift* vem do teste de contrato).

## Estado

Estado de servidor com o mecanismo já presente no console herdado (fetch + estado local por
componente/contexto). Não introduzir store global de dados remotos. (TanStack Query era diretriz
do greenfield; opcional aqui, não obrigatória.)

## Seletor de tenant

Permanente no cabeçalho, com **indicação visual inequívoca** do tenant ativo — operar no tenant
errado é a classe de erro humano mais cara em PAM. Troca obtém novo token e pode disparar step-up
se a política do destino for mais restritiva.

## Step-up transparente

Interceptor global captura o erro de garantia insuficiente (RFC 9470 / pacote 005), apresenta o
desafio WebAuthn e **repete a operação sem perder o estado do formulário**; cancelar mantém o form.

## Padrão de UX: agregados honestos com detalhe sob demanda

Toda superfície de resumo carrega sinal de severidade suficiente para indicar se o *drill-down* é
necessário. Um indicador verde no topo **não pode** coexistir com divergência de cadeia de
auditoria, falha de reconciliação do PDP ou break-glass pendente no detalhe. Se há negativa no
detalhe, o topo mostra.

## Telas críticas

**Auditoria**: linha do tempo com filtros; correlação por `pcid` reunindo ArchGuard + componentes;
indicador de integridade sempre visível, divergência em destaque máximo; verificação e exportação
assinada são L3.

**Break-glass**: solicitação com justificativa/incidente; fila de aprovação (separação de deveres,
sem autoaprovação); contagem regressiva da concessão; revogação imediata.

**Revisão de acesso**: acesso efetivo do PDP com origem (direto/herdado/concessão); decisões em
lote, cada uma auditada.

## Segurança do cliente

O console herdado do Casdoor já usa **sessão por cookie** (não guarda token em `localStorage`).
Endurecer: cookie `HttpOnly`/`Secure`/`SameSite` + CSRF; CSP restritiva; back-channel logout;
inatividade encerra sessão conforme política do tenant. Nenhum segredo no bundle.

## Acessibilidade e i18n

Navegação completa por teclado nos fluxos privilegiados. pt-BR primário (já base), en-US secundário.

## Fronteira com o backend

Onde o `/api/v1` não expõe a capacidade (break-glass, revisão de acesso, verificação de trilha,
saúde), **o endpoint público vem antes da tela** (I-7.6) — trabalho de backend do pacote 011,
registrado por tarefa quando surgir. As capacidades de domínio já existem (pacotes 003/004/005/
007); o que pode faltar é a **exposição HTTP** no `internal/http` + montagem no `/api/v1`.
