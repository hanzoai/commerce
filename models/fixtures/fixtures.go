package fixtures

import (
	"strings"

	"crowdstart.io/config"
	"crowdstart.io/util/task"
)

func init() {
	// Remove a few dangerous fixtures from production
	if config.IsProduction {
		task.Unregister("contributors")
		task.Unregister("skully-campaign")
		task.Unregister("users")
	}

	// Register all fixtures under a fixtures-all task name
	for name, tasks := range task.Registry {
		if strings.HasPrefix(name, "fixtures-") {
			task.Register("fixtures-all", tasks...)
		}
	}
}
