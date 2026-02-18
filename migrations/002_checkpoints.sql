-- Evaluation Checkpoints: Permite retomar execuções de onde pararam
-- e economizar tokens ao evitar reprocessamento de fases já concluídas

CREATE TABLE IF NOT EXISTS evaluation_checkpoints (
    evaluation_id TEXT PRIMARY KEY REFERENCES evaluations(id) ON DELETE CASCADE,
    current_phase TEXT NOT NULL DEFAULT 'inicial',
    messages JSON NOT NULL DEFAULT '[]',
    embedding_inicial BLOB,
    embedding_confronto BLOB,
    divergencia_calculada REAL,
    diagnostico_final TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    last_retry_at DATETIME,
    next_retry_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_checkpoints_retry ON evaluation_checkpoints(next_retry_at);
CREATE INDEX IF NOT EXISTS idx_evaluations_idempotency ON evaluations(idempotency_key);

-- Nota: As colunas abaixo são adicionadas apenas se não existirem
-- Em SQLite, precisamos verificar manualmente a existência das colunas
-- Isso é feito via código Go, não via SQL puro
-- As colunas serão: idempotency_key, error_message, retry_count
