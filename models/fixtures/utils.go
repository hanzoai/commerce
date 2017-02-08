package fixtures

// Inline all styles before use
import (
	"github.com/vanng822/go-premailer/premailer"

	"hanzo.io/config"
	"hanzo.io/util/fs"
)

func readEmailTemplate(path string) string {
	template := string(fs.ReadFile(config.WorkingDir + path))
	return template

	prem := premailer.NewPremailerFromString(template, premailer.NewOptions())
	html, err := prem.Transform()
	if err != nil {
		panic(err)
	}
	return html
}
