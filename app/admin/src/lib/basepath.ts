/**
 * Where this export is MOUNTED. `next.config.ts` feeds it to Next's `basePath`,
 * which rewrites every chunk URL and every route `<Link>`/`useRouter`/
 * `usePathname` resolves — so app code should keep using those and never spell
 * the prefix itself.
 *
 * This constant exists for the two things Next cannot rewrite: an absolute OIDC
 * redirect URI, and a hard navigation OUT of this export (the hanzoai/billing
 * bundle the Go binary mounts next door at `<base>/billing`). One definition, so
 * the built export and the server mount can never disagree about where it lives.
 */
export const BASE_PATH = '/admin'
