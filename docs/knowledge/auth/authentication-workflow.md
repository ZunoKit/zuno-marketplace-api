# Authentication Workflow - Complete SIWE Implementation

## Complete Authentication Flow

```mermaid
flowchart TD
    Start([User Visits App]) --> LoadApp[App Loads]
    LoadApp --> CheckCookie{Has refresh_token cookie?}

    %% Silent Restore Flow
    CheckCookie -->|Yes| SilentRestore[Silent Session Restore]
    SilentRestore --> ValidateRefresh{Refresh token valid?}
    ValidateRefresh -->|Yes| SetUser[Set User State]
    ValidateRefresh -->|No| ShowConnect[Show Connect Button]

    %% SIWE Sign-In Flow
    CheckCookie -->|No| ShowConnect
    ShowConnect --> UserClick[User Clicks Connect]
    UserClick --> GetNonce[Get SIWE Nonce]
    GetNonce --> SignMessage[Sign Message in Wallet]
    SignMessage --> VerifySignature[Verify Signature]
    VerifySignature --> CreateSession[Create Session + Tokens]
    CreateSession --> SetUser

    %% Authenticated State
    SetUser --> AuthenticatedApp[Authenticated App]
    AuthenticatedApp --> MakeRequest[Make API Request]

    %% Auto-Retry Flow
    MakeRequest --> CheckAccess{Access token valid?}
    CheckAccess -->|Yes| RequestSuccess[Request Success]
    CheckAccess -->|No| AutoRefresh[Auto Refresh Token]
    AutoRefresh --> RefreshSuccess{Refresh successful?}
    RefreshSuccess -->|Yes| RetryRequest[Retry Original Request]
    RefreshSuccess -->|No| ForceLogin[Force Re-login]
    RetryRequest --> RequestSuccess

    %% WebSocket Flow
    AuthenticatedApp --> WSConnect[WebSocket Connection]
    WSConnect --> WSAuth[Authenticate WebSocket]
    WSAuth --> WSActive[Active WebSocket]
    WSActive --> WSTokenExpire{Token expiring?}
    WSTokenExpire -->|Yes| WSRefresh[Refresh WS Token]
    WSTokenExpire -->|No| WSActive
    WSRefresh --> WSActive

    %% Logout Flow
    AuthenticatedApp --> UserLogout[User Clicks Logout]
    UserLogout --> RevokeSession[Revoke Session]
    RevokeSession --> ClearCookies[Clear Cookies]
    ClearCookies --> ClearState[Clear Client State]
    ClearState --> ShowConnect

    %% Error Handling
    ForceLogin --> ShowConnect

    %% Styling
    classDef authFlow fill:#e1f5fe
    classDef userAction fill:#f3e5f5
    classDef decision fill:#fff3e0
    classDef error fill:#ffebee

    class GetNonce,SignMessage,VerifySignature,CreateSession authFlow
    class UserClick,UserLogout userAction
    class CheckCookie,ValidateRefresh,CheckAccess,RefreshSuccess,WSTokenExpire decision
    class ForceLogin error
```

## Individual Authentication Sequences

```mermaid
sequenceDiagram
  autonumber
  actor U as User
  participant FE as FE (Next.js)
  participant WAL as Wallet
  participant GQL as GraphQL Gateway
  participant AUTH as Auth Svc (gRPC)
  participant USER as User Svc (gRPC)
  participant WALLET as Wallet Svc (gRPC)
  participant R as Redis (auth cache)
  participant PGA as Postgres (auth_db)
  participant DBU as Postgres (user_db)
  participant PGW as Postgres (wallet_db)
  participant MQ as RabbitMQ

  U->>FE: Click "Connect"
  FE->>WAL: request accounts + chainId
  FE->>GQL: signInSiwe(accountId, chainId, domain)
  GQL->>AUTH: GetNonce(...)

  Note over AUTH: Create one-time nonce (single-use, TTL)
  AUTH->>PGA: INSERT auth_nonces(...)
  AUTH->>R: SET siwe:nonce:{nonce} EX 300
  AUTH-->>GQL: {nonce}
  GQL-->>FE: {nonce}

  FE->>WAL: personal_sign(message)
  WAL-->>FE: signature
  FE->>GQL: verifySiwe(message, signature, accountId)
  GQL->>AUTH: VerifySiwe(...)

  Note over AUTH: Validate domain/chainId/nonce and recover (EOA/EIP-1271)
  AUTH->>R: GET siwe:nonce:{nonce}

  alt first time user
    AUTH->>USER: EnsureUser(accountId, address)
    USER->>DBU: UPSERT user & profile
    USER-->>AUTH: {user_id}
  else existing
    AUTH->>USER: EnsureUser(accountId, address)  # idempotent
    USER-->>AUTH: {user_id}
  end

  Note over AUTH: Create session & link wallet (idempotent)
  AUTH->>PGA: UPDATE auth_nonces SET used=true
  AUTH->>PGA: INSERT sessions(...)
  AUTH->>WALLET: UpsertLink(user_id, accountId, address, chainId, is_primary=true)
  WALLET->>PGW: INSERT ... ON CONFLICT DO NOTHING

  AUTH-->>GQL: {accessToken, refreshToken, userId, expiresAt}
  GQL-->>FE: session established

  AUTH->>MQ: publish auth.user_logged_in {...}
  WALLET->>MQ: publish wallet.linked {...}
```

## GraphQL Schema

```graphql
scalar Address
scalar ChainId
scalar Hex
scalar DateTime

type NoncePayload {
  nonce: String!
}

type AuthPayload {
  accessToken: String!
  refreshToken: String! # hoặc bỏ khỏi body và set cookie httpOnly
  expiresAt: DateTime!
  userId: ID!
}

input SignInSiweInput {
  accountId: String!
  chainId: ChainId!
  domain: String!
}

input VerifySiweInput {
  accountId: String!
  message: String!
  signature: Hex!
}

type Mutation {
  signInSiwe(input: SignInSiweInput!): NoncePayload!
  verifySiwe(input: VerifySiweInput!): AuthPayload!
}
```

## gRPC Services

### Auth Service

```proto
syntax = "proto3";
package auth.v1;
option go_package = "github.com/yourorg/packages/proto-gen-go/auth/v1;authv1";

message GetNonceRequest { string account_id = 1; string chain_id = 2; string domain = 3; }
message GetNonceResponse { string nonce = 1; }

message VerifySiweRequest { string account_id = 1; string message = 2; string signature = 3; }
message VerifySiweResponse {
  string access_token  = 1;
  string refresh_token = 2;
  string expires_at    = 3;
  string user_id       = 4;
  string address       = 5;
  string chain_id      = 6;
}

service Auth {
  rpc GetNonce(GetNonceRequest) returns (GetNonceResponse);
  rpc VerifySiwe(VerifySiweRequest) returns (VerifySiweResponse);
}
```

### Wallet Service

```proto
syntax = "proto3";
package wallets.v1;
option go_package = "github.com/yourorg/packages/proto-gen-go/wallets/v1;walletsv1";

message UpsertLinkRequest {
  string user_id   = 1;
  string account_id= 2;
  string address   = 3; // lowercase 0x…
  string chain_id  = 4; // CAIP-2
  bool   is_primary= 5;
}
message UpsertLinkResponse {}

service Wallets {
  rpc UpsertLink (UpsertLinkRequest) returns (UpsertLinkResponse);
}
```