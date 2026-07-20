# RFC-0003 — Subsistema de auditoria imutável

- **Status:** Proposto
- **Data:** 2026-07-19
- **ADRs relacionados:** ADR-0007, ADR-0009, ADR-0012, ADR-0013, ADR-0014

## 1. Objetivo

Especificar o formato de evento, o encadeamento criptográfico, a selagem assinada, o
procedimento de verificação e a política de retenção da trilha de auditoria.

## 2. Modelo de evento

Campos normativos:

| Campo | Descrição |
|---|---|
| `seq` | Sequência monotônica **por organização** (sem lacunas) |
| `event_id` | UUIDv7 |
| `organization_id` | Tenant (cadeia é por tenant) |
| `occurred_at` | Timestamp com fuso, fonte de tempo confiável |
| `actor` | `{identity_subject, membership_id, session_id, act (delegação)}` |
| `action` | Verbo canônico (`auth.login`, `privileged.session.open`, `breakglass.request`…) |
| `target` | `{type, id, label}` do recurso afetado |
| `outcome` | `success` \| `denied` \| `error` |
| `reason` | Justificativa — inclui **decisão do PDP** quando aplicável (ADR-0005) |
| `context` | IP, user-agent, dispositivo, `acr`/`amr`, correlação de sessão privilegiada |
| `trace_id` | Correlação com telemetria (ADR-0013) |
| `prev_hash`, `hash` | Encadeamento |
| `schema_version` | Versionamento do formato |

**Dado pessoal** no evento aparece apenas como `identity_subject` (pseudônimo estável) e
campos cifrados por chave de titular — nunca em claro (ADR-0014).

## 3. Encadeamento

```
canonical(e)   = JSON canônico do evento sem os campos hash (chaves ordenadas,
                 números e datas em forma normalizada, UTF-8 NFC)
hash(0)        = H(organization_id || genesis_nonce)
hash(n)        = H( hash(n-1) || canonical(e_n) )        H = SHA-256
```

- Canonicalização determinística é **requisito de verificabilidade**: sem ela, a
  recomputação diverge e a prova é inútil.
- Cadeia por organização permite exportação e verificação independentes por tenant, e evita
  contenção global de escrita.
- Gravação com serialização por tenant (advisory lock ou sequência dedicada) garante ausência
  de lacunas e de corrida em `prev_hash`.

## 4. Selagem assinada

- A cada intervalo (padrão: 1 h) ou volume (padrão: 10.000 eventos), o head da cadeia é
  selado: `seal = { organization_id, seq_range, head_hash, sealed_at, key_id, signature }`.
- Assinatura **Ed25519 pelo transit engine do OpenBao** — a chave privada nunca é exposta
  à aplicação (ADR-0012).
- `key_id` registrado no selo permite verificação histórica após rotação.
- **Âncora externa opcional**: exportação dos selos para destino WORM do cliente
  (S3 Object Lock ou equivalente on-prem), tornando detectável a adulteração mesmo em
  comprometimento total da instância.

## 5. Garantias de imutabilidade

1. Papel de banco da aplicação possui apenas `INSERT`/`SELECT` na tabela de auditoria
   (ADR-0009).
2. Triggers `BEFORE UPDATE/DELETE` que abortam a operação — defesa em profundidade.
3. Nenhum endpoint da API expõe mutação de evento; o console é somente-leitura sobre a trilha.
4. Particionamento declarativo por tempo: arquivamento move partições seladas, **sem** deletar
   eventos individualmente.

## 6. Verificação

**Verificador** disponível como comando e endpoint (operação L3):
1. Lê eventos do intervalo em ordem de `seq`.
2. Recomputa a cadeia e compara com `hash` persistido.
3. Verifica assinaturas dos selos com as chaves públicas correspondentes a `key_id`.
4. Detecta lacunas de `seq`.
5. Emite relatório: íntegro, ou primeiro ponto de divergência (`seq`, tipo: alteração,
   remoção, reordenação, selo inválido).

**Execução automática diária** com alerta de severidade máxima em divergência (ADR-0013).

## 7. Escrita fail-closed (I-5.4)

Operações classificadas como privilegiadas gravam o evento **de forma síncrona e durável antes
de concluir**. Falha na gravação ⇒ a operação é negada e o erro retornado. Operações não
privilegiadas podem usar gravação assíncrona com fila durável, mas perda de evento é incidente
de severidade alta.

## 8. Retenção e LGPD

- Retenção mínima configurável por tenant (padrão sugerido: 5 anos para eventos de acesso
  privilegiado; validar com o jurídico e com o DPO do cliente).
- Expiração ⇒ **arquivamento** da partição selada em armazenamento frio, com o selo
  preservado. Restauração é possível e auditada.
- Eliminação LGPD ⇒ destruição da chave do titular. Os eventos permanecem, com o pseudônimo,
  e a cadeia permanece verificável (ADR-0014).

## 9. Exportação

- Cópia para Loki/OTLP para busca operacional (**não** é fonte da verdade).
- Exportação assinada por tenant (NDJSON + selos + chaves públicas + procedimento de
  verificação) para SIEM ou auditor externo. A exportação é evento auditado e operação L3.

## 10. Questões em aberto

- Fonte de tempo confiável: NTP autenticado é suficiente ou haverá exigência de carimbo de
  tempo (RFC 3161) em clientes regulados?
- Volume esperado por tenant para dimensionar partições e política de arquivamento.
- Formato de exportação padronizado (avaliar OCSF) para integração com SIEMs do mercado.
