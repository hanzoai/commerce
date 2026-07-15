// Ambient declarations for runtime deps that ship no types and have no @types package.
// Keeps the tsup dts (declaration) build strict elsewhere without pulling `any` surprises.
declare module 'lodash.isequal' {
  const isEqual: (a: unknown, b: unknown) => boolean
  export default isEqual
}

declare module 'copy-to-clipboard' {
  interface CopyOptions {
    debug?: boolean
    message?: string
    format?: string
    onCopy?: (clipboardData: object) => void
  }
  const copy: (text: string, options?: CopyOptions) => boolean
  export default copy
}
