import { HttpTypes } from "@hanzo/commerce-types"
import { Client } from "../client"
import { ClientHeaders, Config } from "../types"

/**
 * Hanzo IAM (Identity and Access Management) configuration constants.
 * Authentication is handled via Hanzo ID (https://hanzo.id), which is a
 * Casdoor-based OIDC provider.
 */
const HANZO_IAM_ISSUER = "https://hanzo.id"
const HANZO_IAM_TOKEN_ENDPOINT = "https://hanzo.id/api/login/oauth/access_token"
const HANZO_IAM_DEFAULT_CLIENT_ID = "hanzo-app-client-id"

/**
 * Generate a cryptographically random string for PKCE code_verifier.
 */
const generateCodeVerifier = (): string => {
  if (typeof window !== "undefined" && window.crypto) {
    const array = new Uint8Array(32)
    window.crypto.getRandomValues(array)
    return Array.from(array, (byte) => byte.toString(16).padStart(2, "0")).join(
      ""
    )
  }
  // Node.js environment
  const crypto = require("crypto")
  return crypto.randomBytes(32).toString("hex")
}

/**
 * Generate a SHA-256 code_challenge from a code_verifier for PKCE.
 */
const generateCodeChallenge = async (verifier: string): Promise<string> => {
  if (typeof window !== "undefined" && window.crypto?.subtle) {
    const encoder = new TextEncoder()
    const data = encoder.encode(verifier)
    const digest = await window.crypto.subtle.digest("SHA-256", data)
    return btoa(String.fromCharCode(...new Uint8Array(digest)))
      .replace(/\+/g, "-")
      .replace(/\//g, "_")
      .replace(/=+$/, "")
  }
  // Node.js environment
  const crypto = require("crypto")
  return crypto
    .createHash("sha256")
    .update(verifier)
    .digest("base64url")
}

export class Auth {
  private client: Client
  private config: Config

  /**
   * The Hanzo IAM client ID used for OIDC/PKCE flows.
   */
  private clientId: string

  /**
   * Stored PKCE code_verifier for the current auth flow.
   * This is set during login() and consumed during callback().
   */
  private codeVerifier: string | null = null

  constructor(client: Client, config: Config) {
    this.client = client
    this.config = config
    this.clientId = HANZO_IAM_DEFAULT_CLIENT_ID
  }

  /**
   * This method is used to retrieve a registration JWT token for a user, customer, or custom actor type.
   * It sends a request to the backend's /auth/{actor}/{method}/register endpoint.
   *
   * Then, it stores the returned token and passes it in the header of subsequent requests. So, you can call the
   * store.customer.create method, for example, after calling this method.
   *
   * @param actor - The actor type. For example, `user` for admin user, or `customer` for customer.
   * @param method - The authentication provider to use. For example, `emailpass` or `hanzo`.
   * @param payload - The data to pass in the request's body for authentication. When using the `emailpass` provider,
   * you pass the email and password.
   * @returns The JWT token used for registration later.
   *
   * @tags auth
   *
   * @example
   * await sdk.auth.register(
   *   "customer",
   *   "emailpass",
   *   {
   *     email: "customer@gmail.com",
   *     password: "supersecret"
   *   }
   * )
   *
   * // all subsequent requests will use the token in the header
   * const { customer } = await sdk.store.customer.create({
   *   email: "customer@gmail.com",
   *   password: "supersecret"
   * })
   */
  register = async (
    actor: string,
    method: string,
    payload: HttpTypes.AdminSignUpWithEmailPassword | Record<string, unknown>
  ) => {
    const { token } = await this.client.fetch<{ token: string }>(
      `/auth/${actor}/${method}/register`,
      {
        method: "POST",
        body: payload,
      }
    )

    this.client.setToken(token)

    return token
  }

  /**
   * This method retrieves the JWT authenticated token for an admin user, customer, or custom
   * actor type. It sends a request to the backend's /auth/{actor}/{method} endpoint.
   *
   * ### Hanzo IAM (OIDC/PKCE)
   *
   * When the `method` is `hanzo`, this initiates an OIDC Authorization Code flow with PKCE.
   * A `location` URL will be returned pointing to https://hanzo.id for the user to authenticate.
   * After the user authenticates, they will be redirected back to your application with an
   * authorization code. Use the `callback()` method to exchange the code for a token.
   *
   * ### Email/Password Authentication
   *
   * When the `method` is `emailpass`, a direct token exchange is performed against the backend.
   *
   * ### Third-Party Authentication
   *
   * If the API route returns a `location` property, it means that the authentication requires additional steps,
   * typically in a third-party service. The `location` property is returned so that you
   * can redirect the user to the appropriate page.
   *
   * ### Session Authentication
   *
   * If the `auth.type` of the SDK is set to `session`, this method will also send a request to the
   * Set Authentication Session API route.
   *
   * ### Automatic Authentication
   *
   * If the authentication was successful, subsequent requests using the SDK will automatically have the necessary authentication headers / session
   * set, based on your SDK authentication configurations.
   *
   * @param actor - The actor type. For example, `user` for admin user, or `customer` for customer.
   * @param method - The authentication provider to use. For example, `emailpass`, `hanzo`, or `google`.
   * @param payload - The data to pass in the request's body for authentication. When using the `emailpass` provider,
   * you pass the email and password. When using `hanzo`, you can optionally pass `redirect_uri`.
   * @returns The authentication JWT token, or an object with a `location` property for OIDC redirect flows.
   *
   * @tags auth
   *
   * @example
   * // Email/password login
   * const result = await sdk.auth.login(
   *   "customer",
   *   "emailpass",
   *   {
   *     email: "customer@gmail.com",
   *     password: "supersecret"
   *   }
   * )
   *
   * @example
   * // Hanzo IAM OIDC login (redirect flow)
   * const result = await sdk.auth.login(
   *   "customer",
   *   "hanzo",
   *   { redirect_uri: "https://myapp.com/auth/callback" }
   * )
   *
   * if (typeof result !== "string") {
   *   window.location.href = result.location
   *   return
   * }
   */
  login = async (
    actor: string,
    method: string,
    payload: HttpTypes.AdminSignInWithEmailPassword | Record<string, unknown>
  ) => {
    // For Hanzo IAM OIDC/PKCE flow, build the authorization URL directly
    if (method === "hanzo") {
      const codeVerifier = generateCodeVerifier()
      this.codeVerifier = codeVerifier
      const codeChallenge = await generateCodeChallenge(codeVerifier)

      // Store the code verifier for later use in the callback
      if (typeof window !== "undefined" && "sessionStorage" in window) {
        window.sessionStorage.setItem(
          "hanzo_commerce_pkce_verifier",
          codeVerifier
        )
      }

      const redirectUri =
        (payload as Record<string, unknown>)?.redirect_uri as string ||
        (typeof window !== "undefined" ? window.location.origin + "/auth/callback" : "")

      const params = new URLSearchParams({
        response_type: "code",
        client_id: this.clientId,
        redirect_uri: redirectUri,
        scope: "openid profile email",
        code_challenge: codeChallenge,
        code_challenge_method: "S256",
        state: `${actor}:${method}`,
      })

      const location = `${HANZO_IAM_ISSUER}/login/oauth/authorize?${params.toString()}`
      return { location }
    }

    // Standard auth flow via the commerce backend
    const { token, location } = await this.client.fetch<{
      token?: string
      location?: string
    }>(`/auth/${actor}/${method}`, {
      method: "POST",
      body: payload,
    })

    // In the case of an oauth login, we return the redirect location to the caller.
    // They can decide if they do an immediate redirect or put it in an <a> tag.
    if (location) {
      return { location }
    }

    await this.setToken_(token as string)
    return token as string
  }

  /**
   * This method is used to validate an OAuth callback from Hanzo IAM or a third-party service.
   * For Hanzo IAM (OIDC/PKCE), it exchanges the authorization code for an access token
   * directly with the Hanzo ID token endpoint, then authenticates with the commerce backend.
   *
   * For other providers, it sends a request to the backend's /auth/{actor}/{method}/callback endpoint.
   *
   * The method stores the returned token and passes it in the header of subsequent requests.
   *
   * @param actor - The actor type. For example, `user` for admin user, or `customer` for customer.
   * @param method - The authentication provider to use. For example, `hanzo` or `google`.
   * @param query - The query parameters from the OAuth callback, which should be passed to the API route. This includes query parameters like
   * `code`, `state`, and `redirect_uri`.
   * @returns The authentication JWT token
   *
   * @tags auth
   *
   * @example
   * // Handle Hanzo IAM callback
   * const params = new URLSearchParams(window.location.search)
   * const token = await sdk.auth.callback(
   *   "customer",
   *   "hanzo",
   *   {
   *     code: params.get("code"),
   *     state: params.get("state"),
   *     redirect_uri: window.location.origin + "/auth/callback"
   *   }
   * )
   *
   * // all subsequent requests will use the token in the header
   * const { customer } = await sdk.store.customer.retrieve()
   *
   * @privateRemarks
   * The callback expects all query parameters from the OAuth callback to be passed to
   * the backend, and the provider is in charge of parsing and validating them.
   * For Hanzo IAM, the code is exchanged directly with the token endpoint.
   */
  callback = async (
    actor: string,
    method: string,
    query?: Record<string, unknown>
  ) => {
    if (method === "hanzo" && query?.code) {
      // Retrieve the stored PKCE code_verifier
      let codeVerifier = this.codeVerifier
      if (
        !codeVerifier &&
        typeof window !== "undefined" &&
        "sessionStorage" in window
      ) {
        codeVerifier = window.sessionStorage.getItem(
          "hanzo_commerce_pkce_verifier"
        )
        if (codeVerifier) {
          window.sessionStorage.removeItem("hanzo_commerce_pkce_verifier")
        }
      }

      const redirectUri =
        (query.redirect_uri as string) ||
        (typeof window !== "undefined"
          ? window.location.origin + "/auth/callback"
          : "")

      // Exchange the authorization code for an access token with Hanzo IAM
      const tokenResponse = await fetch(HANZO_IAM_TOKEN_ENDPOINT, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: new URLSearchParams({
          grant_type: "authorization_code",
          client_id: this.clientId,
          code: query.code as string,
          redirect_uri: redirectUri,
          ...(codeVerifier ? { code_verifier: codeVerifier } : {}),
        }).toString(),
      })

      if (!tokenResponse.ok) {
        const errorBody = await tokenResponse.text()
        throw new Error(
          `Hanzo Commerce Auth: Token exchange failed (${tokenResponse.status}): ${errorBody}`
        )
      }

      const tokenData = (await tokenResponse.json()) as {
        access_token: string
        token_type?: string
        refresh_token?: string
        id_token?: string
        expires_in?: number
      }

      // Now authenticate with the commerce backend using the IAM access token
      // The backend validates the IAM token and returns a commerce-scoped JWT
      const { token } = await this.client.fetch<{ token: string }>(
        `/auth/${actor}/${method}/callback`,
        {
          method: "GET",
          query: {
            ...query,
            access_token: tokenData.access_token,
          },
        }
      )

      this.codeVerifier = null
      await this.setToken_(token)
      return token
    }

    // Standard callback flow for non-Hanzo providers
    const { token } = await this.client.fetch<{ token: string }>(
      `/auth/${actor}/${method}/callback`,
      {
        method: "GET",
        query,
      }
    )

    await this.setToken_(token)
    return token
  }

  /**
   * This method refreshes a JWT authentication token, which is useful after validating the OAuth callback
   * with {@link callback}. It sends a request to the Refresh Authentication Token API route.
   *
   * The method stores the returned token and passes it in the header of subsequent requests. So, you can call other
   * methods that require authentication after calling this method.
   *
   * @param headers - Headers to pass in the request
   *
   * @returns The refreshed JWT authentication token.
   *
   * @tags auth
   *
   * @example
   * const token = await sdk.auth.refresh()
   *
   * // all subsequent requests will use the token in the header
   * const { customer } = await sdk.store.customer.retrieve()
   */
  refresh = async (headers?: ClientHeaders) => {
    const { token } = await this.client.fetch<{ token: string }>(
      "/auth/token/refresh",
      {
        method: "POST",
        headers,
      }
    )

    // Putting the token in session after refreshing is only useful when the new token has updated info (eg. actor_id).
    // Ideally we don't use the full JWT in session as key, but just store a pseudorandom key that keeps the rest of the auth context as value.
    await this.setToken_(token)
    return token
  }

  /**
   * This method logs out the currently authenticated user based on your SDK authentication configurations.
   *
   * If the `auth.type` of the SDK is set to `session`, this method will also send a request to the
   * Delete Authentication Session API route.
   *
   * The method also clears any stored tokens or sessions, based on your SDK authentication configurations.
   *
   * @tags auth
   *
   * @example
   * await sdk.auth.logout()
   *
   * // user is now logged out
   * // you can't send any requests that require authentication
   */
  logout = async () => {
    if (this.config?.auth?.type === "session") {
      await this.client.fetch("/auth/session", {
        method: "DELETE",
      })
    }

    // Clean up any stored PKCE state
    if (typeof window !== "undefined" && "sessionStorage" in window) {
      window.sessionStorage.removeItem("hanzo_commerce_pkce_verifier")
    }

    this.client.clearToken()
  }

  /**
   * This method requests a reset password token for an admin user, customer, or custom actor type.
   * It sends a request to the Generate Reset Password Token API route.
   *
   * To reset the password later using the token delivered to the user, use the {@link updateProvider} method.
   *
   * @param actor - The actor type. For example, `user` for admin user, or `customer` for customer.
   * @param provider - The authentication provider to use. For example, `emailpass`.
   * @param body - The data required to identify the user.
   *
   * @tags auth
   *
   * @example
   * sdk.auth.resetPassword(
   *   "customer",
   *   "emailpass",
   *   {
   *     identifier: "customer@gmail.com"
   *   }
   * )
   * .then(() => {
   *   // user receives token
   * })
   */
  resetPassword = async (
    actor: string,
    provider: string,
    body: {
      /**
       * The user's identifier. For example, when using the `emailpass` provider,
       * this would be the user's email.
       */
      identifier: string
      /**
       * Optional metadata to include in the reset password request.
       */
      metadata?: Record<string, unknown>
    }
  ) => {
    await this.client.fetch(`/auth/${actor}/${provider}/reset-password`, {
      method: "POST",
      body,
      headers: { accept: "text/plain" }, // 201 Created response
    })
  }

  /**
   * This method is used to update user-related authentication data.
   *
   * More specifically, use this method when updating the password of an admin user, customer, or
   * custom actor type after requesting to reset their password with {@link resetPassword}.
   *
   * @param actor - The actor type. For example, `user` for admin user, or `customer` for customer.
   * @param provider - The authentication provider to use. For example, `emailpass`.
   * @param body - The data necessary to update the user's authentication data. When resetting the user's password,
   * send the `password` property.
   *
   * @tags auth
   *
   * @example
   * sdk.auth.updateProvider(
   *   "customer",
   *   "emailpass",
   *   {
   *     password: "supersecret"
   *   },
   *   token
   * )
   * .then(() => {
   *   // password updated
   * })
   */
  updateProvider = async (
    actor: string,
    provider: string,
    body: HttpTypes.AdminUpdateProvider,
    token: string
  ) => {
    await this.client.fetch(`/auth/${actor}/${provider}/update`, {
      method: "POST",
      body,
      headers: { Authorization: `Bearer ${token}` },
    })
  }

  /**
   * @ignore
   */
  private setToken_ = async (token: string) => {
    // By default we just set the token in the configured storage, if configured to use sessions we convert it into session storage instead.
    if (this.config?.auth?.type === "session") {
      await this.client.fetch("/auth/session", {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
      })
    } else {
      this.client.setToken(token)
    }
  }
}
