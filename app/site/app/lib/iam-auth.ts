/**
 * Minimal IAM auth module for the commerce portal.
 *
 * Handles OIDC/PKCE login via Casdoor (hanzo.id) and token storage.
 * Self-contained — no external auth package dependency.
 */

const KEY_PREFIX = 'hanzo_commerce_'
const KEY_ACCESS_TOKEN = `${KEY_PREFIX}access_token`
const KEY_REFRESH_TOKEN = `${KEY_PREFIX}refresh_token`
const KEY_ID_TOKEN = `${KEY_PREFIX}id_token`
const KEY_EXPIRES_AT = `${KEY_PREFIX}expires_at`
const KEY_CODE_VERIFIER = `${KEY_PREFIX}code_verifier`
const KEY_STATE = `${KEY_PREFIX}state`

export interface IamAuthConfig {
  iamServerUrl: string
  clientId: string
  redirectUri: string
}

// ── Defaults ──────────────────────────────────────────────────────────

const DEFAULT_IAM_SERVER = 'https://hanzo.id'
const DEFAULT_CLIENT_ID = 'app-hanzo'

export function getDefaultConfig(): IamAuthConfig {
  const origin = typeof window !== 'undefined' ? window.location.origin : 'https://commerce.hanzo.ai'
  return {
    iamServerUrl: DEFAULT_IAM_SERVER,
    clientId: DEFAULT_CLIENT_ID,
    redirectUri: `${origin}/auth/callback`,
  }
}

// ── PKCE helpers ──────────────────────────────────────────────────────

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

// ── Token helpers ─────────────────────────────────────────────────────

/** Check if user is logged in (has valid token). */
export function isLoggedIn(): boolean {
  if (typeof sessionStorage === 'undefined') return false
  const token = sessionStorage.getItem(KEY_ACCESS_TOKEN)
  const expiresAt = sessionStorage.getItem(KEY_EXPIRES_AT)
  if (!token) return false
  if (expiresAt && Date.now() > Number(expiresAt)) return false
  return true
}

/** Get the current access token. */
export function getAccessToken(): string | null {
  if (!isLoggedIn()) return null
  return sessionStorage.getItem(KEY_ACCESS_TOKEN)
}

// ── Login flow ────────────────────────────────────────────────────────

/**
 * Start OIDC/PKCE login flow — redirects to IAM login page.
 * @param postLoginRedirect URL to redirect to after successful login (default: admin dashboard)
 */
export async function startLogin(
  config?: IamAuthConfig,
  postLoginRedirect?: string,
): Promise<void> {
  const conf = config ?? getDefaultConfig()
  const state = generateRandom(32)
  const codeVerifier = generateRandom(64)
  const challengeBuffer = await sha256(codeVerifier)
  const codeChallenge = base64UrlEncode(challengeBuffer)

  sessionStorage.setItem(KEY_STATE, state)
  sessionStorage.setItem(KEY_CODE_VERIFIER, codeVerifier)
  if (postLoginRedirect) {
    sessionStorage.setItem(`${KEY_PREFIX}post_login_redirect`, postLoginRedirect)
  }

  const base = conf.iamServerUrl.replace(/\/+$/, '')
  const params = new URLSearchParams({
    client_id: conf.clientId,
    response_type: 'code',
    redirect_uri: conf.redirectUri,
    scope: 'openid profile email',
    state,
    code_challenge: codeChallenge,
    code_challenge_method: 'S256',
  })

  window.location.href = `${base}/oauth/authorize?${params}`
}

/**
 * Handle the OAuth callback — exchange code for tokens or accept implicit grant.
 * Returns the post-login redirect URL.
 */
export async function handleCallback(config?: IamAuthConfig): Promise<string> {
  const conf = config ?? getDefaultConfig()
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
    return getPostLoginRedirect()
  }

  if (!code) throw new Error('No authorization code in callback')

  const savedState = sessionStorage.getItem(KEY_STATE)
  if (!savedState || savedState !== state) throw new Error('State mismatch — possible CSRF')

  const codeVerifier = sessionStorage.getItem(KEY_CODE_VERIFIER)
  if (!codeVerifier) throw new Error('Missing code verifier')

  // Clean up state
  sessionStorage.removeItem(KEY_STATE)
  sessionStorage.removeItem(KEY_CODE_VERIFIER)

  // Exchange code for tokens
  const base = conf.iamServerUrl.replace(/\/+$/, '')
  const tokenRes = await fetch(`${base}/oauth/token`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: new URLSearchParams({
      grant_type: 'authorization_code',
      client_id: conf.clientId,
      code,
      redirect_uri: conf.redirectUri,
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
  const idToken = data.id_token
  const expiresIn = data.expires_in ?? 3600

  if (!accessToken) throw new Error('No access token received')

  // Store tokens
  sessionStorage.setItem(KEY_ACCESS_TOKEN, accessToken)
  if (refreshToken) sessionStorage.setItem(KEY_REFRESH_TOKEN, refreshToken)
  if (idToken) sessionStorage.setItem(KEY_ID_TOKEN, idToken)
  sessionStorage.setItem(KEY_EXPIRES_AT, String(Date.now() + expiresIn * 1000))

  return getPostLoginRedirect()
}

/** Clear all stored tokens. */
export function logout(): void {
  sessionStorage.removeItem(KEY_ACCESS_TOKEN)
  sessionStorage.removeItem(KEY_REFRESH_TOKEN)
  sessionStorage.removeItem(KEY_ID_TOKEN)
  sessionStorage.removeItem(KEY_EXPIRES_AT)
  sessionStorage.removeItem(KEY_STATE)
  sessionStorage.removeItem(KEY_CODE_VERIFIER)
  sessionStorage.removeItem(`${KEY_PREFIX}post_login_redirect`)
}

function getPostLoginRedirect(): string {
  const saved = sessionStorage.getItem(`${KEY_PREFIX}post_login_redirect`)
  sessionStorage.removeItem(`${KEY_PREFIX}post_login_redirect`)
  return saved || 'https://admin.commerce.hanzo.ai'
}
