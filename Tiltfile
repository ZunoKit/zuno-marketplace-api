allow_k8s_contexts('docker-desktop')
k8s_namespace('dev')

# Load the restart_process extension
load('ext://restart_process', 'docker_build_with_restart')

### K8s Config ###

# Load secrets for sensitive configuration
k8s_yaml('./infra/development/k8s/secrets.yaml')

# Load application configuration
k8s_yaml('./infra/development/k8s/app-config.yaml')

# Load infrastructure services
k8s_yaml('./infra/development/k8s/postgres.yaml')
k8s_yaml('./infra/development/k8s/redis.yaml')
k8s_yaml('./infra/development/k8s/rabbitmq.yaml')
k8s_yaml('./infra/development/k8s/mongo.yaml')

# Load individual service deployments
k8s_yaml('./infra/development/k8s/auth-service-deployment.yaml')
k8s_yaml('./infra/development/k8s/user-service-deployment.yaml') 
k8s_yaml('./infra/development/k8s/wallet-service-deployment.yaml')
k8s_yaml('./infra/development/k8s/graphql-gateway-deployment.yaml')
k8s_yaml('./infra/development/k8s/media-service-deployment.yaml')
k8s_yaml('./infra/development/k8s/chain-registry-service-deployment.yaml')
# Orchestrator, Catalog and Indexer are added below after build sections

### End of K8s Config ###

### PostgreSQL Database ###
local_resource(
  'postgres-build',
  cmd='infra\\development\\build\\postgres-build.bat',
  deps=['services/auth-service/db/up.sql', 'services/user-service/db/up.sql', 'services/wallet-service/db/up.sql', 'services/chain-registry-service/db/up.sql', 'infra/development/docker/postgres.dockerfile']
)

docker_build(
  'nft-postgres:latest',
  '.',
  dockerfile='infra/development/docker/postgres.dockerfile'
)

### GraphQL Gateway ###

gateway_compile_cmd = 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/graphql-gateway ./services/graphql-gateway'
if os.name == 'nt':
  gateway_compile_cmd = 'infra\\development\\build\\graphql-gateway-build.bat'

local_resource(
  'graphql-gateway-compile',
  gateway_compile_cmd,
  deps=['./services/graphql-gateway', './shared'], 
  labels="compiles")

docker_build_with_restart(
  'nft-marketplace/graphql-gateway',
  '.',
  entrypoint=['/app/build/graphql-gateway'],
  dockerfile='./infra/development/docker/graph-gateway.Dockerfile',
  only=[
    './build/graphql-gateway',
    './shared',
  ],
  live_update=[
    sync('./build', '/app/build'),
    sync('./shared', '/app/shared'),
  ],
)

k8s_resource('graphql-gateway', port_forwards=8081,
             resource_deps=['graphql-gateway-compile'], labels="services")

### End of GraphQL Gateway ###

### Auth Service ###

auth_compile_cmd = 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/auth-service ./services/auth-service/cmd'
if os.name == 'nt':
  auth_compile_cmd = 'infra\\development\\build\\auth-build.bat'

local_resource(
  'auth-service-compile',
  auth_compile_cmd,
  deps=['./services/auth-service', './shared'], 
  labels="compiles")

docker_build_with_restart(
  'nft-marketplace/auth-service',
  '.',
  entrypoint=['/app/build/auth-service'],
  dockerfile='./infra/development/docker/auth-service.Dockerfile',
  only=[
    './build/auth-service',
    './shared',
  ],
  live_update=[
    sync('./build', '/app/build'),
    sync('./shared', '/app/shared'),
  ],
)

k8s_resource('auth-service', port_forwards="50051:50051",
             resource_deps=['auth-service-compile', 'postgres', 'redis'], labels="services")

### End of Auth Service ###

### User Service ###

user_compile_cmd = 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/user-service ./services/user-service/cmd'
if os.name == 'nt':
  user_compile_cmd = 'infra\\development\\build\\user-build.bat'

local_resource(
  'user-service-compile',
  user_compile_cmd,
  deps=['./services/user-service', './shared'], 
  labels="compiles")

docker_build_with_restart(
  'nft-marketplace/user-service',
  '.',
  entrypoint=['/app/build/user-service'],
  dockerfile='./infra/development/docker/user-service.Dockerfile',
  only=[
    './build/user-service',
    './shared',
  ],
  live_update=[
    sync('./build', '/app/build'),
    sync('./shared', '/app/shared'),
  ],
)

k8s_resource('user-service', port_forwards="50052:50052",
             resource_deps=['user-service-compile', 'postgres', 'redis'], labels="services")

### End of User Service ###

### Wallet Service ###

wallet_compile_cmd = 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/wallet-service ./services/wallet-service/cmd'
if os.name == 'nt':
  wallet_compile_cmd = 'infra\\development\\build\\wallet-build.bat'

local_resource(
  'wallet-service-compile',
  wallet_compile_cmd,
  deps=['./services/wallet-service', './shared'], 
  labels="compiles")

docker_build_with_restart(
  'nft-marketplace/wallet-service',
  '.',
  entrypoint=['/app/build/wallet-service'],
  dockerfile='./infra/development/docker/wallet-service.Dockerfile',
  only=[
    './build/wallet-service',
    './shared',
  ],
  live_update=[
    sync('./build', '/app/build'),
    sync('./shared', '/app/shared'),
  ],
)

k8s_resource('wallet-service', port_forwards="50053:50053",
             resource_deps=['wallet-service-compile', 'postgres', 'redis'], labels="services")

### End of Wallet Service ###

### Media Service ###

media_compile_cmd = 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/media-service ./services/media-service/cmd'
if os.name == 'nt':
  media_compile_cmd = 'infra\\development\\build\\media-build.bat'

local_resource(
  'media-service-compile',
  media_compile_cmd,
  deps=['./services/media-service', './shared'], 
  labels="compiles")

docker_build_with_restart(
  'nft-marketplace/media-service',
  '.',
  entrypoint=['/app/build/media-service'],
  dockerfile='./infra/development/docker/media-service.Dockerfile',
  only=[
    './build/media-service',
    './shared',
  ],
  live_update=[
    sync('./build', '/app/build'),
    sync('./shared', '/app/shared'),
  ],
)

k8s_resource('media-service', port_forwards="50055:50055",
             resource_deps=['media-service-compile', 'postgres', 'redis'], labels="services")

### End of Media Service ###

### Orchestrator Service ###

orchestrator_compile_cmd = 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/orchestrator-service ./services/orchestrator-service/cmd'
if os.name == 'nt':
  orchestrator_compile_cmd = 'infra\\development\\build\\orchestrator-build.bat'

local_resource(
  'orchestrator-service-compile',
  orchestrator_compile_cmd,
  deps=['./services/orchestrator-service', './shared'], 
  labels="compiles")

docker_build_with_restart(
  'nft-marketplace/orchestrator-service',
  '.',
  entrypoint=['/app/build/orchestrator-service'],
  dockerfile='./infra/development/docker/orchestrator-service.Dockerfile',
  only=[
    './build/orchestrator-service',
    './shared',
  ],
  live_update=[
    sync('./build', '/app/build'),
    sync('./shared', '/app/shared'),
  ],
)

k8s_yaml('./infra/development/k8s/orchestrator-service-deployment.yaml')

k8s_resource('orchestrator-service', port_forwards="50054:50054",
             resource_deps=['orchestrator-service-compile', 'postgres', 'redis'], labels="services")

### End of Orchestrator Service ###

### Chain Registry Service ###

chain_registry_compile_cmd = 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/chain-registry-service ./services/chain-registry-service/cmd'
if os.name == 'nt':
  chain_registry_compile_cmd = 'infra\\development\\build\\chain-registry-build.bat'

local_resource(
  'chain-registry-service-compile',
  chain_registry_compile_cmd,
  deps=['./services/chain-registry-service', './shared'], 
  labels="compiles")

docker_build_with_restart(
  'nft-marketplace/chain-registry-service',
  '.',
  entrypoint=['/app/build/chain-registry-service'],
  dockerfile='./infra/development/docker/chain-registry-service.Dockerfile',
  only=[
    './build/chain-registry-service',
    './shared',
  ],
  live_update=[
    sync('./build', '/app/build'),
    sync('./shared', '/app/shared'),
  ],
)

k8s_resource('chain-registry-service', port_forwards="50056:50056",
             resource_deps=['chain-registry-service-compile', 'postgres', 'redis'], labels="services")

### End of Chain Registry Service ###

### Catalog Service ###

catalog_compile_cmd = 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/catalog-service ./services/catalog-service/cmd'
if os.name == 'nt':
  catalog_compile_cmd = 'infra\\development\\build\\catalog-build.bat'

local_resource(
  'catalog-service-compile',
  catalog_compile_cmd,
  deps=['./services/catalog-service', './shared'], 
  labels="compiles")

docker_build_with_restart(
  'nft-marketplace/catalog-service',
  '.',
  entrypoint=['/app/build/catalog-service'],
  dockerfile='./infra/development/docker/catalog-service.Dockerfile',
  only=[
    './build/catalog-service',
    './shared',
  ],
  live_update=[
    sync('./build', '/app/build'),
    sync('./shared', '/app/shared'),
  ],
)

k8s_yaml('./infra/development/k8s/catalog-service-deployment.yaml')

k8s_resource('catalog-service', port_forwards="50057:50057",
             resource_deps=['catalog-service-compile', 'postgres', 'redis'], labels="services")

### End of Catalog Service ###

### Indexer Service ###

indexer_compile_cmd = 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/indexer-service ./services/indexer-service/cmd'
if os.name == 'nt':
  indexer_compile_cmd = 'infra\\development\\build\\indexer-build.bat'

local_resource(
  'indexer-service-compile',
  indexer_compile_cmd,
  deps=['./services/indexer-service', './shared'], 
  labels="compiles")

docker_build_with_restart(
  'nft-marketplace/indexer-service',
  '.',
  entrypoint=['/app/build/indexer-service'],
  dockerfile='./infra/development/docker/indexer-service.Dockerfile',
  only=[
    './build/indexer-service',
    './shared',
  ],
  live_update=[
    sync('./build', '/app/build'),
    sync('./shared', '/app/shared'),
  ],
)

k8s_yaml('./infra/development/k8s/indexer-service-deployment.yaml')

k8s_resource('indexer-service', port_forwards="50058:50058",
             resource_deps=['indexer-service-compile', 'postgres', 'mongo', 'redis'], labels="services")

### End of Indexer Service ###

### Subscription Worker ###

subscription_compile_cmd = 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/subscription-worker ./services/subscription-worker/cmd'
if os.name == 'nt':
  subscription_compile_cmd = 'infra\\development\\build\\subscription-build.bat'

local_resource(
  'subscription-worker-compile',
  subscription_compile_cmd,
  deps=['./services/subscription-worker', './shared'], 
  labels="compiles")

docker_build_with_restart(
  'nft-marketplace/subscription-worker',
  '.',
  entrypoint=['/app/build/subscription-worker'],
  dockerfile='./infra/development/docker/subscription-worker.Dockerfile',
  only=[
    './build/subscription-worker',
    './shared',
  ],
  live_update=[
    sync('./build', '/app/build'),
    sync('./shared', '/app/shared'),
  ],
)

k8s_yaml('./infra/development/k8s/subscription-worker-deployment.yaml')
k8s_resource('subscription-worker', resource_deps=['subscription-worker-compile', 'postgres', 'redis', 'rabbitmq'], labels="workers")

### End of Subscription Worker ###

### Infrastructure Services ###

# PostgreSQL Database
k8s_resource('postgres', port_forwards="5432:5432", 
             resource_deps=['postgres-build'], labels="infrastructure")

# Redis Cache
k8s_resource('redis', port_forwards="6379:6379", labels="infrastructure")

# RabbitMQ Message Broker
k8s_resource('rabbitmq', port_forwards=["5672:5672", "15672:15672"], labels="infrastructure")

# MongoDB
k8s_resource('mongo', port_forwards="27017:27017", labels="infrastructure")

### End of Infrastructure Services ###

### Resource Groups ###

# ConfigMap and Secret resources are automatically handled by Tilt
# No need to explicitly reference them with k8s_resource()

### End of Resource Groups ###
### RabbitMQ (dev) — integration moved pending correct placement
