# ADR-0007 — Trilha de auditoria imutável e tamper-evident

- **Status:** Aceito
- **Data:** 2026-07-19
- **Invariantes tocados:** I-5.1 a I-5.4, I-3.4

## Contexto

O upstream registra auditoria em tabela relacional comum, acessada pelo mesmo ORM do restante
da aplicação. Isso significa que qualquer código com conexão ao banco — incluindo um operador
com acesso administrativo ao PostgreSQL — pode editar ou apagar registros sem deixar vestígio.

Para um IAM genérico isso é aceitável. Para um **PAM**, é falha de propósito: a trilha é o
produto. Se o registro de "quem acessou o servidor de produção às 3h da manhã" pode ser
editado por quem acessou, o sistema não prova nada.

Requisitos adicionais: LGPD (rastreabilidade de tratamento de dados pessoais), exigências de
auditoria de clientes corporativos, e o conflito entre imutabilidade e direito à eliminação.

## Decisão

**Implementar a trilha como log append-only, tamper-evident, com encadeamento por hash e
selagem periódica assinada, isolada do caminho de escrita transacional comum.**

### Estrutura do evento

Cada evento contém, no mínimo: identificador ordenável, `organization_id`, timestamp confiável,
ator (identidade + membership + sessão), ação, recurso alvo, resultado, contexto de origem
(IP, user-agent, dispositivo), correlação (`trace_id`), e — quando aplicável — a **justificativa
da decisão de autorização** (ADR-0005).

### Encadeamento

```
hash(n) = H( hash(n-1) || canonical_json(evento(n)) )
```

- Canonicalização determinística do payload (ordenação estável de chaves) — sem isso, a
  verificação é irreprodutível.
- Cadeia **por organização**, permitindo verificação e exportação independentes por tenant.
- **Selagem periódica**: a cada intervalo/volume, o head da cadeia é assinado (Ed25519) com
  chave custodiada no **OpenBao** (ADR-0012). O selo é o que torna a adulteração retroativa
  detectável mesmo por quem tem acesso ao banco.
- **Âncora externa opcional**: exportação do selo para destino WORM/externo controlado pelo
  cliente (S3 Object Lock, storage imutável on-prem), garantindo detecção mesmo em
  comprometimento total da instância.

### Garantias operacionais

- **Nenhum caminho de UPDATE/DELETE** na tabela de auditoria: privilégios do banco negam a
  operação para o usuário da aplicação (`INSERT`/`SELECT` apenas); *trigger* de bloqueio como
  defesa em profundidade.
- **Fail-closed (I-5.4)**: se o evento não pode ser persistido de forma durável, a operação
  privilegiada correspondente **é negada**. Auditoria não é *best effort*.
- **Verificador**: comando e endpoint que recomputam a cadeia e reportam o primeiro ponto de
  divergência, além de verificar assinaturas dos selos. Exposto no console (ADR-0004).
- **Exportação** para Loki/OTLP é **cópia para observabilidade**, jamais a fonte da verdade.

### LGPD (I-3.4)

Direito à eliminação **não apaga eventos**. Aplica-se **pseudonimização/crypto-shredding**:
identificadores diretos do titular são armazenados cifrados por chave *per-subject*;
a eliminação destrói a chave, tornando o dado irrecuperável e **preservando a cadeia intacta**.
Detalhamento no ADR-0014 e RFC-0003.

## Alternativas consideradas

| Opção | Por que não |
|---|---|
| Manter tabela de auditoria comum (upstream) | Não prova nada sob ataque com acesso ao banco — falha de propósito para PAM |
| Delegar exclusivamente a SIEM externo do cliente | Cria dependência externa dura (viola I-1.3); a integridade passa a depender de configuração de terceiro |
| Blockchain / ledger distribuído | Complexidade e custo desproporcionais; hash-chain + selo assinado + âncora WORM entrega a mesma propriedade de detecção |
| Apenas assinar cada evento individualmente | Detecta alteração, mas **não detecta remoção** de eventos — o encadeamento é o que fecha essa lacuna |

## Consequências

- Custo de escrita maior e caminho de persistência dedicado (hot path de login precisa de
  atenção a latência — a gravação é síncrona por I-5.4 nas operações privilegiadas).
- Rotação de chave de selagem exige procedimento formal com sobreposição de validade.
- Volume de armazenamento cresce monotonicamente: política de retenção **por selagem e
  arquivamento**, nunca por deleção seletiva.
