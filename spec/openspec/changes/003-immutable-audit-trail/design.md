# Design — 003 · Trilha de auditoria imutável

Base normativa: RFC-0003.

## Escrita

Serviço `AuditSink` com dois modos:
- **síncrono durável** — obrigatório para operações privilegiadas (fail-closed, I-5.4);
- **assíncrono com fila durável** — permitido para eventos não privilegiados; perda é
  incidente de severidade alta.

Serialização por tenant (advisory lock por `organization_id` ou sequência dedicada) garante
ausência de lacunas em `seq` e ausência de corrida em `prev_hash`.

## Canonicalização

JSON canônico: chaves ordenadas, UTF-8 NFC, números e datas em forma normalizada, campos de
hash excluídos. **Testes de vetor fixo** garantem que a canonicalização é estável entre
versões — mudança silenciosa aqui invalida toda a verificação histórica.

## Selagem

`seal = { organization_id, seq_range, head_hash, sealed_at, key_id, signature }`.
Assinatura Ed25519 pelo *transit engine* do cofre — a chave privada nunca chega à aplicação.
`key_id` no selo viabiliza verificação após rotação.

Âncora externa opcional: exportação de selos para destino WORM controlado pelo cliente.

## Restrições de banco

Papel da aplicação com `INSERT`/`SELECT` apenas; triggers `BEFORE UPDATE/DELETE` que abortam;
particionamento declarativo por tempo; índices para consulta por ator, alvo, período e
correlação.

## Verificador

Recomputa a cadeia, valida assinaturas dos selos e detecta lacunas de `seq`. Reporta o primeiro
ponto de divergência e o tipo (alteração, remoção, reordenação, selo inválido). Execução
automática diária com alerta de severidade máxima.

## Retenção

Expiração leva a **arquivamento de partição selada**, nunca a deleção seletiva. Eliminação
LGPD é tratada por destruição de chave de titular (pacote 010).

## Desempenho

Alvo: gravação síncrona não deve dominar a latência do caminho de login. Medir desde o
primeiro dia; se ultrapassar o orçamento de latência do RFC-0001, otimizar a escrita — jamais
enfraquecer a garantia.
