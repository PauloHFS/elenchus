-- name: GetTenantByID :one
SELECT * FROM tenants WHERE id = ? LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE tenant_id = ? AND email = ? LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ? LIMIT 1;

-- name: CreateUser :one
INSERT INTO users (tenant_id, email, password_hash, role_id) 
VALUES (?, ?, ?, ?) RETURNING *;

-- name: CreateJob :one
INSERT INTO jobs (tenant_id, type, payload, run_at) VALUES (?, ?, ?, ?) RETURNING *;

-- name: PickNextJob :one
UPDATE jobs 
SET status = 'processing', updated_at = CURRENT_TIMESTAMP
WHERE id = (
    SELECT id FROM jobs 
    WHERE status = 'pending' AND run_at <= CURRENT_TIMESTAMP 
    ORDER BY run_at ASC LIMIT 1
) RETURNING *;

-- name: CompleteJob :exec
UPDATE jobs 
SET status = 'completed', updated_at = CURRENT_TIMESTAMP 
WHERE id = ?;

-- name: FailJob :exec
UPDATE jobs SET status = 'failed', last_error = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: RescueZombies :exec
UPDATE jobs 
SET status = 'pending', attempt_count = attempt_count + 1 
WHERE status = 'processing' AND updated_at < datetime('now', '-5 minutes');

-- name: RecordJobProcessed :exec
INSERT INTO processed_jobs (job_id) VALUES (?)
ON CONFLICT(job_id) DO UPDATE SET processed_at = CURRENT_TIMESTAMP;

-- name: IsJobProcessed :one
SELECT EXISTS(SELECT 1 FROM processed_jobs WHERE job_id = ?);

-- name: CreateWebhook :one
INSERT INTO webhooks (source, external_id, payload, headers) 
VALUES (?, ?, ?, ?) RETURNING *;

-- name: UpsertPasswordReset :exec
INSERT INTO password_resets (email, token_hash, expires_at) 
VALUES (?, ?, ?)
ON CONFLICT(token_hash) DO UPDATE SET 
    email = excluded.email,
    expires_at = excluded.expires_at;

-- name: GetPasswordResetByToken :one
SELECT * FROM password_resets WHERE token_hash = ? LIMIT 1;

-- name: DeletePasswordReset :exec
DELETE FROM password_resets WHERE email = ?;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = ? WHERE email = ?;

-- name: UpdateUserAvatar :exec
UPDATE users SET avatar_url = ? WHERE id = ?;

-- name: ListUsersPaginated :many
SELECT * FROM users 
WHERE tenant_id = ? 
AND (email LIKE '%' || ? || '%' OR ? = '')
ORDER BY created_at DESC 
LIMIT ? OFFSET ?;

-- name: CountUsers :one
SELECT COUNT(*) FROM users WHERE tenant_id = ?;

-- name: UpsertEmailVerification :exec
INSERT INTO email_verifications (email, token, expires_at) 
VALUES (?, ?, ?)
ON CONFLICT(email) DO UPDATE SET 
    token = excluded.token,
    expires_at = excluded.expires_at;

-- name: GetEmailVerificationByToken :one
SELECT * FROM email_verifications WHERE token = ? LIMIT 1;

-- name: DeleteEmailVerification :exec
DELETE FROM email_verifications WHERE email = ?;

-- name: VerifyUser :exec
UPDATE users SET is_verified = TRUE WHERE email = ?;
