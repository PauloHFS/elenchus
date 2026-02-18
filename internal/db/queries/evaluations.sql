-- name: ListEvaluationsByStatus :many
SELECT * FROM evaluations
WHERE tenant_id = ?
  AND user_id = ?
  AND status IN (?, ?)
ORDER BY created_at DESC;
