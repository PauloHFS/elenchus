-- name: CreateCheckpoint :exec
INSERT INTO evaluation_checkpoints (
    evaluation_id,
    current_phase,
    messages,
    retry_count
) VALUES (?, ?, ?, 0)
ON CONFLICT(evaluation_id) DO UPDATE SET
    current_phase = excluded.current_phase,
    messages = excluded.messages,
    updated_at = CURRENT_TIMESTAMP;

-- name: GetCheckpoint :one
SELECT * FROM evaluation_checkpoints
WHERE evaluation_id = ?;

-- name: UpdateCheckpointPhase :exec
UPDATE evaluation_checkpoints
SET current_phase = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE evaluation_id = ?;

-- name: UpdateCheckpointMessages :exec
UPDATE evaluation_checkpoints
SET messages = ?
WHERE evaluation_id = ?;

-- name: UpdateCheckpointEmbeddings :exec
UPDATE evaluation_checkpoints
SET embedding_inicial = ?,
    embedding_confronto = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE evaluation_id = ?;

-- name: UpdateCheckpointDivergence :exec
UPDATE evaluation_checkpoints
SET divergencia_calculada = ?,
    diagnostico_final = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE evaluation_id = ?;

-- name: UpdateCheckpointRetry :exec
UPDATE evaluation_checkpoints
SET retry_count = retry_count + 1,
    last_retry_at = CURRENT_TIMESTAMP,
    next_retry_at = datetime('now', '+' || ? || ' seconds'),
    updated_at = CURRENT_TIMESTAMP
WHERE evaluation_id = ?;

-- name: ClearCheckpointRetry :exec
UPDATE evaluation_checkpoints
SET next_retry_at = NULL,
    retry_count = 0,
    updated_at = CURRENT_TIMESTAMP
WHERE evaluation_id = ?;

-- name: DeleteCheckpoint :exec
DELETE FROM evaluation_checkpoints WHERE evaluation_id = ?;

-- name: GetEvaluationsToRetry :many
SELECT e.* FROM evaluations e
INNER JOIN evaluation_checkpoints c ON e.id = c.evaluation_id
WHERE e.status = 'processing'
  AND c.next_retry_at IS NOT NULL
  AND c.next_retry_at <= CURRENT_TIMESTAMP;

-- name: GetStuckEvaluations :many
SELECT e.* FROM evaluations e
INNER JOIN evaluation_checkpoints c ON e.id = c.evaluation_id
WHERE e.status = 'processing'
  AND c.next_retry_at IS NULL
  AND e.updated_at < datetime('now', '-5 minutes');
