# 2. Session Refresh Flow

## Overview

This document describes the session refresh flow for maintaining authentication state using refresh token rotation.

## Sequence Diagram

```mermaid
sequenceDiagram
  autonumber
  participant FE as FE (Next.js)
  participant GQL as GraphQL Gateway
  participant AUTH as Auth Svc (gRPC)
  participant PGA as Postgres (auth_db)
  participant R as Redis (auth cache)
  participant MQ as RabbitMQ

  FE->>GQL: refreshSession()  # không gửi body, dùng cookie HttpOnly
  Note over GQL: Đọc cookie refresh_token
  GQL->>AUTH: RefreshSession(refresh_token, ua, ip)

  alt refresh token hợp lệ & chưa dùng lại
    AUTH->>PGA: UPDATE sessions SET rotated_at=now(), prev_refresh_revoked=true
    AUTH->>R: SET session:{sid} (TTL extend)
    AUTH-->>GQL: {accessTokenNew, refreshTokenNew, expiresAt}
    Note over GQL: Set-Cookie refresh_token new with HttpOnly Secure SameSite=Strict
    GQL-->>FE: {accessTokenNew, refreshTokenNew, expiresAt}
    AUTH->>MQ: publish auth.session_refreshed {...}
  else reuse-detected / revoked / expired
    AUTH->>PGA: REVOKE session chain (all devices)
    AUTH-->>GQL: error UNAUTHENTICATED
    GQL-->>FE: 401 UNAUTHENTICATED
  end
```

## Key Components

### Refresh Token Rotation
- **New Token**: Generated on each refresh
- **Old Token**: Immediately invalidated
- **Reuse Detection**: Revokes entire session chain
- **Secure Storage**: HttpOnly cookie only

### Session Management
- **TTL Extension**: Redis cache extended on refresh
- **Chain Revocation**: All devices logged out on compromise
- **Audit Trail**: Track refresh attempts and sources
- **Rate Limiting**: Prevent refresh token abuse

### Security Features
- **Automatic Rotation**: Prevents token theft
- **Immediate Invalidation**: Old tokens unusable
- **Compromise Detection**: Reuse triggers security response
- **Device Tracking**: User agent and IP logging

## Error Scenarios

### Token Reuse Detection
```javascript
// If refresh token is used more than once
if (refreshToken.used) {
  // Revoke all sessions for this user
  await revokeAllUserSessions(userId)
  throw new Error('SECURITY_VIOLATION: Token reuse detected')
}
```

### Expired Tokens
- Natural expiration after 30 days
- Graceful degradation to login
- Clear security cookie
- Return to unauthenticated state

### Invalid Tokens
- Malformed or tampered tokens
- Unknown session references
- Database inconsistencies
- Immediate session termination

## Frontend Integration

### Automatic Refresh
```javascript
// Intercept 401 responses
if (response.status === 401) {
  const refreshResult = await refreshSession()
  if (refreshResult.success) {
    // Retry original request with new token
    return retryWithNewToken(originalRequest)
  } else {
    // Redirect to login
    redirectToLogin()
  }
}
```

### Token Storage
- Access tokens in memory only
- Refresh tokens in HttpOnly cookies
- No localStorage/sessionStorage usage
- Automatic cleanup on logout