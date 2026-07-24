// Small script-loader for the Square Web Payments SDK. Injects the CDN script
// once (idempotent — a second call resolves against the in-flight load) and
// resolves with the global `Square` handle. Sandbox vs production is the only
// axis that changes the CDN host.
export interface SquareSdk {
  payments(applicationId: string, locationId: string): SquarePayments
}

export interface SquarePayments {
  card(): Promise<SquareCardInstance>
}

export interface SquareCardInstance {
  attach(selector: string | HTMLElement): Promise<void>
  destroy(): Promise<void>
  tokenize(): Promise<SquareTokenResult>
}

export interface SquareTokenResult {
  status: string
  token?: string
  errors?: { message?: string }[]
}

const PROD = 'https://web.squarecdn.com/v1/square.js'
const SANDBOX = 'https://sandbox.web.squarecdn.com/v1/square.js'

let pending: Promise<SquareSdk> | null = null

export function loadSquare(environment: 'sandbox' | 'production'): Promise<SquareSdk> {
  const existing = (globalThis as { Square?: SquareSdk }).Square
  if (existing) return Promise.resolve(existing)
  if (pending) return pending

  pending = new Promise<SquareSdk>((resolve, reject) => {
    const script = document.createElement('script')
    script.src = environment === 'sandbox' ? SANDBOX : PROD
    script.async = true
    script.onload = () => {
      const sdk = (globalThis as { Square?: SquareSdk }).Square
      if (sdk) resolve(sdk)
      else reject(new Error('Square SDK loaded but unavailable.'))
    }
    script.onerror = () => {
      pending = null
      reject(new Error('Could not load Square card payments.'))
    }
    document.head.appendChild(script)
  })
  return pending
}
