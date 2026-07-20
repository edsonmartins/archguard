# Proposal — 003 · Trilha de auditoria imutável

## Por quê

No modelo herdado, a auditoria é uma tabela relacional comum, editável por qualquer código com
acesso ao banco. Para um IAM genérico isso passa; para um **PAM**, é falha de propósito: se o
registro de quem acessou produção pode ser alterado por quem acessou, o produto não prova nada
(ADR-0007).

## O que muda

- Evento de auditoria estruturado e versionado, com ator, alvo, resultado, justificativa da
  decisão e correlação.
- Encadeamento por hash **por organização**, com canonicalização determinística.
- Selagem periódica assinada (Ed25519) via cofre, com âncora externa WORM opcional.
- Restrições de banco (papel sem `UPDATE`/`DELETE`, triggers de bloqueio) e particionamento
  por tempo.
- Escrita **fail-closed** para operações privilegiadas.
- Verificador de integridade (comando + endpoint) e verificação automática diária.
- Exportação assinada por tenant.

## O que não muda

Telemetria (pacote 010) permanece separada e não vira fonte da verdade.

## Impacto

- **Depende de:** 001, 002 (cadeia por tenant).
- **Bloqueia:** 004 (break-glass exige auditoria confiável), 008 (telas de auditoria).
- **Risco:** latência no caminho quente; complexidade de rotação de chave de selagem.
