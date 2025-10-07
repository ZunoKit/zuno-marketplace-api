# Collection Orchestrator Service

Implements Prepare and TrackTx for Create Collection flow.

gRPC API:

- Prepare(PrepareCreateCollectionRequest) → PrepareCreateCollectionResponse
- TrackTx(TrackCollectionTxRequest) → TrackCollectionTxResponse

Data:

- Postgres `tx_intents` stores intents and tx_hash mapping
- Redis key `intent:status:{intentId}` caches status with 6h TTL

Run locally (example):

- Postgres at localhost:5432 (db: orchestrator_db)
- Redis at localhost:6379
- Service listens on :50054

Migrations: see `db/up.sql` and `db/down.sql`.

Notes:

- Factory address and calldata generation are stubbed; replace with chain-registry lookup and ABI encoding.
- Idempotency: `UpdateIntentTx` safely updates same intent repeatedly.

