package log

import (
	"fmt"
	"runtime/debug"
	"strings"
)

func errAndStack(err error) string {
	if !std.Verbose() {
		return fmt.Sprintf("%v", err)
	} else {
		return fmt.Sprintf("%v:\n%s", err, stack(6))
	}
}

// Grab stacktrace
func stack(offset int) string {
	stack := strings.Split(string(debug.Stack()), "\n")
	lines := []string{""}
	for i := offset; i < len(stack); i++ {
		if strings.Contains(stack[i], "github.com/onsi/ginkgo") {
			break
		}
		lines = append(lines, stack[i])
	}
	return strings.Join(lines, "\n")
}
