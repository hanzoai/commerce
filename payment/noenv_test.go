package payment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// No processor reads its configuration from the environment: it is the routing policy's
// account, applied at runtime. The one table of the names an old deployment used is
// payment/processor/envseed.go, and it reads nothing either.
func TestNoProcessorReadsTheEnvironment(t *testing.T) {
	var offenders []string
	_ = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, _ := os.ReadFile(path)
		for i, line := range strings.Split(string(b), "\n") {
			if t := strings.TrimSpace(line); strings.HasPrefix(t, "//") {
				continue
			}
			if strings.Contains(line, "os.Getenv(") || strings.Contains(line, "os.LookupEnv(") || strings.Contains(line, "os.Environ(") {
				offenders = append(offenders, path+":"+strings.TrimSpace(strings.Split(line, "\n")[0])+" (line "+itoa(i+1)+")")
			}
		}
		return nil
	})
	if len(offenders) > 0 {
		t.Fatalf("processors read the environment:\n  %s", strings.Join(offenders, "\n  "))
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}
