FROM postgres:15-alpine

# Copy all migration scripts to init directory
# PostgreSQL will execute these in alphabetical order
COPY services/auth-service/db/up.sql /docker-entrypoint-initdb.d/01-auth.sql
COPY services/user-service/db/up.sql /docker-entrypoint-initdb.d/02-user.sql  
COPY services/wallet-service/db/up.sql /docker-entrypoint-initdb.d/03-wallet.sql
COPY services/chain-registry-service/db/up.sql /docker-entrypoint-initdb.d/04-chain-registry.sql
COPY services/orchestrator-service/db/up.sql /docker-entrypoint-initdb.d/05-orchestrator.sql
COPY services/catalog-service/db/up.sql /docker-entrypoint-initdb.d/06-catalog.sql
COPY services/indexer-service/db/up.sql /docker-entrypoint-initdb.d/07-indexer.sql
EXPOSE 5432
