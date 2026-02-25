/**
 * IAM auth module for Hanzo Commerce.
 *
 * Handles OIDC/PKCE login via Casdoor (hanzo.id) and token storage.
 * Uses Casdoor-specific endpoints (proven compatible with hanzo.id).
 * Self-contained -- no external auth package dependency.
 *
 * This module uses browser APIs (sessionStorage, window, crypto.subtle).
 * It is intended for use in browser-based applications only.
 */

const KEY_PREFIX = 'hanzo_commerce_'
const KEY_ACCESS_TOKEN = `${KEY_PREFIX}access_token`
const KEY_REFRESH_TOKEN = `${KEY_PREFIX}refresh_token`
const KEY_EXPIRES_AT = `${KEY_PREFIX}expires_at`
const KEY_CODE_VERIFIER = `${KEY_PREFIX}code_verifier`
const KEY_STATE = `${KEY_PREFIX}state`

// ── Types ───────────────────────────────────────────────────────────────────

export interface IamUser {
  email: string
  displayName: string | null
  avatar: string | null
  sub: string
}

export interface IamAuthConfig {
  iamServerUrl: string
  clientId: string
  redirectUri: string
  orgName: string
  appName: string
}

// ── PKCE helpers ────────────────────────────────────────────────────────────

function generateRandom(length: number): string {
  const array = new Uint8Array(length)
  crypto.getRandomValues(array)
  return Array.from(array, (b) => b.toString(36).padStart(2, '0'))
    .join('')
    .slice(0, length)
}

async function sha256(message: string): Promise<ArrayBuffer> {
  const encoder = new TextEncoder()
  return crypto.subtle.digest('SHA-256', encoder.encode(message))
}

function base64UrlEncode(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer)
  let binary = ''
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

// ── Token storage ───────────────────────────────────────────────────────────

/** Check if user is logged in (has a non-expired token). */
export function isLoggedIn(): boolean {
  const token = sessionStorage.getItem(KEY_ACCESS_TOKEN)
  const expiresAt = sessionStorage.getItem(KEY_EXPIRES_AT)
  if (!token) return false
  if (expiresAt && Date.now() > Number(expiresAt)) return false
  return true
}

/** Get the current access token (null if expired or absent). */
export function getAccessToken(): string | null {
  if (!isLoggedIn()) return null
  return sessionStorage.getItem(KEY_ACCESS_TOKEN)
}

/** Parse the JWT payload to extract user info. */
export function getCurrentUser(): IamUser | null {
  const token = getAccessToken()
  if (!token) return null
  try {
    const payload = JSON.parse(atob(token.split('.')[1]))
    return {
      email: payload.email ?? payload.name ?? '',
      displayName: payload.displayName ?? payload.name ?? null,
      avatar: payload.avatar ?? null,
      sub: payload.sub ?? payload.name ?? '',
    }
  } catch {
    return null
  }
}

// ── Login flow ──────────────────────────────────────────────────────────────

/** Start OIDC/PKCE login flow -- redirects to Casdoor login page. */
export async function startLogin(config: IamAuthConfig): Promise<void> {
  const state = generateRandom(32)
  const codeVerifier = generateRandom(64)
  const challengeBuffer = await sha256(codeVerifier)
  const codeChallenge = base64UrlEncode(challengeBuffer)

  sessionStorage.setItem(KEY_STATE, state)
  sessionStorage.setItem(KEY_CODE_VERIFIER, codeVerifier)

  const base = config.iamServerUrl.replace(/\/+$/, '')
  const params = new URLSearchParams({
    client_id: config.clientId,
    response_type: 'code',
    redirect_uri: config.redirectUri,
    scope: 'openid profile email',
    state,
    code_challenge: codeChallenge,
    code_challenge_method: 'S256',
  })

  // Use Casdoor's login authorize endpoint (shows the login form)
  window.location.href = `${base}/login/oauth/authorize?${params}`
}

/** Handle the OAuth callback -- exchange code for tokens or accept implicit grant. */
export async function handleCallback(config: IamAuthConfig): Promise<IamUser | null> {
  const url = new URL(window.location.href)
  const code = url.searchParams.get('code')
  const state = url.searchParams.get('state')
  const error = url.searchParams.get('error')

  // Also check for implicit grant tokens (access_token in query or hash)
  const implicitToken =
    url.searchParams.get('access_token') ||
    new URLSearchParams(url.hash.replace(/^#/, '')).get('access_token')

  if (error) {
    const desc = url.searchParams.get('error_description') ?? error
    throw new Error(`OAuth error: ${desc}`)
  }

  // Implicit flow: IAM returned access_token directly
  if (implicitToken && !code) {
    const expiresIn = Number(url.searchParams.get('expires_in') ?? '3600')
    sessionStorage.setItem(KEY_ACCESS_TOKEN, implicitToken)
    sessionStorage.setItem(KEY_EXPIRES_AT, String(Date.now() + expiresIn * 1000))
    sessionStorage.removeItem(KEY_STATE)
    sessionStorage.removeItem(KEY_CODE_VERIFIER)
    return getCurrentUser()
  }

  if (!code) throw new Error('No authorization code in callback')

  const savedState = sessionStorage.getItem(KEY_STATE)
  if (!savedState || savedState !== state) throw new Error('State mismatch -- possible CSRF')

  const codeVerifier = sessionStorage.getItem(KEY_CODE_VERIFIER)
  if (!codeVerifier) throw new Error('Missing code verifier')

  // Clean up state
  sessionStorage.removeItem(KEY_STATE)
  sessionStorage.removeItem(KEY_CODE_VERIFIER)

  // Exchange code for tokens via Casdoor token endpoint
  const base = config.iamServerUrl.replace(/\/+$/, '')
  const tokenRes = await fetch(`${base}/api/login/oauth/access_token`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: new URLSearchParams({
      grant_type: 'authorization_code',
      client_id: config.clientId,
      code,
      redirect_uri: config.redirectUri,
      code_verifier: codeVerifier,
    }),
  })

  if (!tokenRes.ok) {
    const text = await tokenRes.text()
    throw new Error(`Token exchange failed: ${text}`)
  }

  const data = await tokenRes.json()
  const accessToken = data.access_token
  const refreshToken = data.refresh_token
  const expiresIn = data.expires_in ?? 3600

  if (!accessToken) throw new Error('No access token received')

  // Store tokens
  sessionStorage.setItem(KEY_ACCESS_TOKEN, accessToken)
  if (refreshToken) sessionStorage.setItem(KEY_REFRESH_TOKEN, refreshToken)
  sessionStorage.setItem(KEY_EXPIRES_AT, String(Date.now() + expiresIn * 1000))

  return getCurrentUser()
}

/** Clear all stored tokens. */
export function logout(): void {
  sessionStorage.removeItem(KEY_ACCESS_TOKEN)
  sessionStorage.removeItem(KEY_REFRESH_TOKEN)
  sessionStorage.removeItem(KEY_EXPIRES_AT)
  sessionStorage.removeItem(KEY_STATE)
  sessionStorage.removeItem(KEY_CODE_VERIFIER)
}
