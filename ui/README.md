# commerce/ui

`dist/` is the embedded Commerce admin — **build output**, not tracked content.
Only `.gitkeep` is committed, so `//go:embed all:dist` resolves on a fresh clone.

## The one way to produce it

```bash
cd app && pnpm turbo run typecheck build --filter=@hanzo/commerce-dashboard
scripts/sync-admin-ui.sh          # app/admin/out -> ui/dist
go build ./cmd/commerced
```

`Dockerfile` runs exactly those three steps (admin-build stage → sync
→ `go build`), so the shipped binary's admin is always this commit's admin.

Served at `/admin/*`, and built for it: `app/admin/next.config.ts` sets
`basePath: '/admin'`. Move the mount and you must move `BASE_PATH`
(`app/admin/src/lib/basepath.ts`) — the export and the mount are one contract.
