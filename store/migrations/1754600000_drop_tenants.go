package migrations

import (
	"github.com/hanzoai/base/core"
)

// 1754600000 — the IAM org is the org; commerce keeps no registry of its own.
// These collections were that registry (hostname rows the public resolver
// stopped reading once host→brand→IAM-org became the one path); dropping them
// ends the split authority. Idempotent: a datadir that never had them passes
// through untouched, and there is no way back — the registry's schema lives
// only in git history.
func init() {
	core.AppMigrations.Register(func(app core.App) error {
		for _, name := range []string{"commerce_tenant_hostnames", "commerce_tenants"} {
			c, err := app.FindCollectionByNameOrId(name)
			if err != nil || c == nil {
				continue
			}
			if err := app.Delete(c); err != nil {
				return err
			}
		}
		return nil
	}, func(app core.App) error {
		return nil
	})
}
