-- 0019: fila durável de auditoria assíncrona (pacote 003, T-009). Eventos NÃO
-- privilegiados (L1/L2) não precisam travar a cadeia no caminho quente do login:
-- são enfileirados aqui (INSERT rápido, sem trava de cabeçalho) e um drainer de
-- fundo os sela na cadeia (audit_event) via AppendTx, preservando a ordem por
-- organização. Perda de evento é incidente de severidade alta (RFC-0003 §7), por
-- isso a fila é DURÁVEL (persistida) — o evento só sai daqui depois de selado na
-- cadeia, na mesma transação (at-least-once com dedução por id).
--
-- `id` é UUIDv7: ordenável no tempo, é a chave de ordem do drain por org. O
-- payload é o AuditEvent já construído no enfileiramento (event_id e occurred_at
-- capturados no momento em que o evento OCORREU, não no drain).
--
-- Operações PRIVILEGIADAS (L3) NÃO usam esta fila — vão pelo AuditSink síncrono
-- fail-closed (T-008): a durabilidade tem de preceder a conclusão da operação.
CREATE TABLE IF NOT EXISTS audit_event_queue (
    id              uuid        PRIMARY KEY,
    organization_id uuid        NOT NULL,
    payload         jsonb       NOT NULL,
    enqueued_at     timestamptz NOT NULL DEFAULT now()
);

-- Drain ordenado por organização (chave de ordem = id UUIDv7).
CREATE INDEX IF NOT EXISTS audit_event_queue_org_id_idx ON audit_event_queue (organization_id, id);
