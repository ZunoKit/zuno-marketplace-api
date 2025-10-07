# Authentication Overview

## Overview

This document provides a high-level overview of the authentication system and references to detailed flow documentation.

## Authentication Flows

The authentication system consists of 6 main flows:

### 1. SIWE Sign-In Flow
**File**: [1-siwe-signin-flow.md](./1-siwe-signin-flow.md)

Complete Sign-In with Ethereum flow including:
- Nonce generation and validation
- Signature verification (EOA/EIP-1271)
- User and wallet linking
- Session creation with secure cookies

### 2. Session Refresh Flow
**File**: [2-session-refresh-flow.md](./2-session-refresh-flow.md)

Token refresh mechanism featuring:
- Refresh token rotation
- Reuse detection and security
- Session chain management
- Automatic token updates

### 3. Auto-Retry Flow
**File**: [3-auto-retry-flow.md](./3-auto-retry-flow.md)

Seamless token refresh on 401 errors:
- Request interception
- Automatic retry with new tokens
- Concurrent request handling
- Graceful error fallback

### 4. Silent Restore Flow
**File**: [4-silent-restore-flow.md](./4-silent-restore-flow.md)

Application startup authentication:
- Silent session restoration
- Cookie-based authentication
- Progressive enhancement
- Graceful degradation

### 5. Logout Flow
**File**: [5-logout-flow.md](./5-logout-flow.md)

Session termination and cleanup:
- Session revocation
- Cookie clearing
- Security event publishing
- Multiple logout scenarios

### 6. WebSocket Authentication Flow
**File**: [6-websocket-auth-flow.md](./6-websocket-auth-flow.md)

Real-time connection authentication:
- WebSocket token management
- Connection maintenance
- Subscription security
- Real-time token refresh

## Architecture Components

### Services
- **Auth Service**: SIWE authentication and session management
- **User Service**: User profiles and account management
- **Wallet Service**: Multi-wallet support and linking

### Databases
- **PostgreSQL**: Auth sessions, nonces, user data
- **Redis**: Session cache, nonce storage
- **RabbitMQ**: Auth events and notifications

### Security Features
- **SIWE Standard**: EIP-4361 compliant authentication
- **Token Rotation**: Refresh token security
- **HttpOnly Cookies**: XSS protection
- **CSRF Protection**: SameSite cookie policy
- **Session Security**: Compromise detection

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
  refreshSession: AuthPayload!
  logout: Boolean!
}

type Query {
  me: User
}
```

## gRPC Services

### Auth Service
```proto
service Auth {
  rpc GetNonce(GetNonceRequest) returns (GetNonceResponse);
  rpc VerifySiwe(VerifySiweRequest) returns (VerifySiweResponse);
  rpc RefreshSession(RefreshSessionRequest) returns (RefreshSessionResponse);
  rpc RevokeSession(RevokeSessionRequest) returns (RevokeSessionResponse);
}
```

### Wallet Service
```proto
service Wallets {
  rpc UpsertLink(UpsertLinkRequest) returns (UpsertLinkResponse);
  rpc GetUserWallets(GetUserWalletsRequest) returns (GetUserWalletsResponse);
}
```

## Implementation Guide

### Frontend Integration
1. **Connect Button**: Trigger SIWE flow
2. **Token Management**: Handle access/refresh tokens
3. **Auto-Retry**: Implement request interceptor
4. **Silent Auth**: Check authentication on app load
5. **WebSocket**: Maintain real-time connections

### Backend Implementation
1. **SIWE Verification**: Validate signatures and nonces
2. **Session Management**: Handle token lifecycle
3. **Security Events**: Publish auth events
4. **Database Design**: Optimize for auth queries
5. **Cache Strategy**: Redis for session performance

## Security Considerations

### Best Practices
- Use HTTPS for all authentication endpoints
- Implement rate limiting on auth endpoints
- Monitor for suspicious authentication patterns
- Regular security audits and penetration testing
- Keep dependencies updated

### Threat Mitigation
- **Token Theft**: Refresh token rotation
- **Replay Attacks**: One-time nonces
- **Session Hijacking**: Secure cookie flags
- **CSRF**: SameSite cookie policy
- **XSS**: HttpOnly cookies