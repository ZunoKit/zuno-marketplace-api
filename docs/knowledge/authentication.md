
## 1. Sign-in (SIWE) hoàn chỉnh + Set-Cookie tại GQL

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

## 2. Refresh Session

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

## 3. Auto-retry khi access hết hạn (401 → refresh → replay)

```mermaid

sequenceDiagram
  autonumber
  participant FE as FE (Next.js)
  participant GQL as GraphQL Gateway
  participant AUTH as Auth Svc (gRPC)

  FE->>GQL: anyQuery/mutation (Authorization: Bearer access)
  alt access hết hạn/không hợp lệ
    GQL-->>FE: 401 UNAUTHENTICATED
    FE->>GQL: refreshSession()  # dùng cookie HttpOnly
    GQL->>AUTH: RefreshSession(...)
    AUTH-->>GQL: {accessNew, refreshNew}
    GQL-->>FE: {accessNew, refreshNew}
    FE->>GQL: replay original request (Authorization: Bearer accessNew)
    GQL-->>FE: 200 OK (data)
  else hợp lệ
    GQL-->>FE: 200 OK (data)
  end

```

## 4. Silent session restore khi app load

```mermaid
sequenceDiagram
  autonumber
  participant FE as FE (Next.js)
  participant GQL as GraphQL Gateway
  participant AUTH as Auth Svc (gRPC)

  FE->>GQL: me()  # không kèm Authorization
  Note over GQL: Nếu có cookie refresh_token thì thử refresh
  GQL->>AUTH: RefreshSession(refresh_token)
  alt thành công
    AUTH-->>GQL: {accessNew, refreshNew, user}
    Note over GQL: Set-Cookie refresh_token mới
    GQL-->>FE: {user, accessNew}
  else thất bại
    GQL-->>FE: {user:null}  # FE hiển thị nút Connect
  end





```

## 5. Logout

```mermaid
sequenceDiagram
  autonumber
  participant FE as FE (Next.js)
  participant GQL as GraphQL Gateway
  participant AUTH as Auth Svc (gRPC)
  participant PGA as Postgres (auth_db)
  participant MQ as RabbitMQ

  FE->>GQL: logout()
  GQL->>AUTH: RevokeSession(current_refresh_or_sid)
  AUTH->>PGA: UPDATE sessions SET revoked=true, revoked_at=now()
  AUTH-->>GQL: {ok:true}
      Note over GQL: Clear refresh_token cookie with Max-Age=0 HttpOnly Secure
  GQL-->>FE: {ok:true}
  AUTH->>MQ: publish auth.session_revoked {...}



```

## 6. (WS) re-auth khi access hết hạn

```mermaid
sequenceDiagram
  autonumber
  participant FE as FE (Next.js)
  participant GQL as GraphQL WS

  FE->>GQL: connection_init { accessToken }
  GQL-->>FE: connection_ack

  Note over FE,GQL: ... streaming subscription data ...

  alt accessToken sắp hết hạn
    FE->>GQL: connection_ping  # tùy lib
    FE->>GQL: refreshSession() (HTTP, cookie)  # lấy access mới
    FE->>GQL: connection_update { accessTokenNew } # hoặc reconnect
    GQL-->>FE: connection_ack
  else token hết hạn mà không update
    GQL-->>FE: connection_error 4401
    FE->>GQL: refreshSession() → reconnect với access mới
  end

```

