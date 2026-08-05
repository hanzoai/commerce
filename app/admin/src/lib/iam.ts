/**
 * The ONE IAM/OIDC client config for the admin — same shape the console's
 * `~/lib/auth/iam` exposes.
 *
 * `redirectUri` is a function, not a constant, because it reads `window`: the
 * export prerenders in Node. It points at this export's `/callback` route AS
 * MOUNTED (under `BASE_PATH`) — a root-absolute `/callback` would leave the
 * mount and the OIDC round-trip would land on a 404. It lives here once because
 * the provider, the login page and the callback page must present a BYTE-
 * IDENTICAL redirect_uri: OIDC requires the token exchange to match the
 * authorize request.
 */
import { BASE_PATH } from './basepath'

export const iamConfig = () => ({
  serverUrl: process.env.NEXT_PUBLIC_IAM_SERVER_URL || 'https://hanzo.id',
  clientId: process.env.NEXT_PUBLIC_IAM_CLIENT_ID || 'hanzo-commerce',
  redirectUri: typeof window === 'undefined' ? '' : `${window.location.origin}${BASE_PATH}/callback`,
})
