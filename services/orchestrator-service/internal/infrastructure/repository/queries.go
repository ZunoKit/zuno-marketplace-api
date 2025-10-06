package repository

const (
	CreateIntentQuery = `
		INSERT INTO tx_intents (
			intent_id, kind, chain_id, preview_address, tx_hash, status, 
			created_by, req_payload_json, error, deadline_at, created_at, updated_at, auth_session_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	UpdateTxHashQuery = `
		UPDATE tx_intents 
		SET tx_hash = $1, preview_address = $2, updated_at = $3
		WHERE intent_id = $4
	`

	UpdateStatusQuery = `
		UPDATE tx_intents 
		SET status = $1, error = $2, updated_at = $3
		WHERE intent_id = $4
	`

	GetByIDQuery = `
		SELECT intent_id, kind, chain_id, preview_address, tx_hash, status,
			   created_by, req_payload_json, error, deadline_at, created_at, updated_at, auth_session_id
		FROM tx_intents 
		WHERE intent_id = $1
	`

	FindByChainTxQuery = `
		SELECT intent_id, kind, chain_id, preview_address, tx_hash, status,
			   created_by, req_payload_json, error, deadline_at, created_at, updated_at, auth_session_id
		FROM tx_intents 
		WHERE chain_id = $1 AND tx_hash = $2
	`

	InsertSessionIntentAuditQuery = `
		INSERT INTO session_intent_audit (session_id, intent_id, user_id, audit_data)
		VALUES ($1, $2, $3, $4)
	`
)
