'use client'

const HANZO_IAM_URL = 'https://hanzo.id'
const CLIENT_ID = 'hanzo-app-client-id'
const REDIRECT_URI = typeof window !== 'undefined'
  ? `${window.location.origin}/callback`
  : 'https://admin.commerce.hanzo.ai/callback'

function buildAuthUrl(): string {
  const params = new URLSearchParams({
    client_id: CLIENT_ID,
    response_type: 'code',
    redirect_uri: REDIRECT_URI,
    scope: 'openid profile email',
    state: crypto.randomUUID(),
  })
  return `${HANZO_IAM_URL}/login/oauth/authorize?${params.toString()}`
}

export default function LoginPage() {
  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <div className="w-full max-w-sm">
        <div className="card">
          <div className="flex flex-col items-center text-center">
            <div className="mb-6 flex h-12 w-12 items-center justify-center rounded-lg bg-hanzo-red">
              <span className="text-lg font-bold text-white">H</span>
            </div>

            <h1 className="text-xl font-bold text-white">
              Hanzo Commerce
            </h1>
            <p className="mt-1 text-sm text-muted">
              Sign in to access the admin dashboard
            </p>

            <div className="mt-8 w-full space-y-3">
              <a
                href={buildAuthUrl()}
                className="btn-primary w-full"
              >
                Sign in with Hanzo ID
              </a>

              <p className="text-xs text-muted">
                You will be redirected to{' '}
                <span className="text-white">hanzo.id</span>{' '}
                to authenticate via OIDC
              </p>
            </div>

            <div className="mt-6 border-t border-border pt-6 w-full">
              <p className="text-xs text-muted">
                Don&apos;t have an account?{' '}
                <a
                  href={`${HANZO_IAM_URL}/signup`}
                  className="text-hanzo-red hover:text-hanzo-red-light transition-colors"
                >
                  Create one at hanzo.id
                </a>
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
