# 1. SIWE Sign-In Flow

## Overview

This document describes the complete Sign-In with Ethereum (SIWE) flow, including nonce generation, signature verification, and session creation.

## Sequence Diagram

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

  Note over GQL: Set-Cookie refresh_token with HttpOnly Secure SameSite=Strict Path=/ Max-Age=30d
  GQL-->>FE: {accessToken, refreshToken, userId, expiresAt}  # refreshToken cũng trả về trong body

  AUTH->>MQ: publish auth.user_logged_in {...}
  WALLET->>MQ: publish wallet.linked {...}
```

## Key Components

### SIWE Message Format
```
app.zuno.com wants you to sign in with your Ethereum account:
0x1234567890123456789012345678901234567890

I accept the Terms of Service: https://app.zuno.com/tos

URI: https://app.zuno.com
Version: 1
Chain ID: 1
Nonce: 32AlphaNumericChars
Issued At: 2024-01-01T12:00:00.000Z
```

### Nonce Generation
- **Format**: 32 character alphanumeric string
- **TTL**: 5 minutes (300 seconds)
- **Single Use**: Marked as used after verification
- **Storage**: Both PostgreSQL and Redis for reliability

### Signature Verification
- **EOA**: Standard ECDSA signature recovery
- **Smart Contracts**: EIP-1271 signature validation
- **Domain Binding**: Prevents signature replay attacks
- **Nonce Validation**: Ensures freshness and single use

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
  refreshToken: String!
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

## Security Features

### Nonce Protection
- Cryptographically secure random generation
- One-time use prevents replay attacks
- Short TTL limits attack window
- Database tracking for audit trails

### Session Security
- HttpOnly cookies prevent XSS access
- Secure flag for HTTPS-only transmission
- SameSite=Strict prevents CSRF
- 30-day refresh token rotation

### Wallet Linking
- Idempotent operations prevent duplicates
- Primary wallet designation
- Multi-chain address support
- Automatic profile creation