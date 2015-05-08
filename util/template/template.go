package template

import (
	"os"

	"github.com/flosch/pongo2"
	"github.com/gin-gonic/gin"

	"crowdstart.com/config"
	"crowdstart.com/models/types/country"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/types/thankyou"
	"crowdstart.com/util/json"
	"crowdstart.com/util/log"
)

var cwd, _ = os.Getwd()

func TemplateSet() *pongo2.TemplateSet {
	set := pongo2.NewSet("default")

	set.Debug = config.IsDevelopment

	set.Globals["config"] = config.Get()

	set.Globals["isDevelopment"] = config.IsDevelopment
	set.Globals["isProduction"] = config.IsProduction
	set.Globals["isStaging"] = config.IsStaging

	set.Globals["siteTitle"] = config.SiteTitle

	set.Globals["urlFor"] = func(moduleName string, args ...string) string {
		return config.UrlFor(moduleName, args...)
	}

	// DEPRECATED: Remove as soon as all templates are updated to use `urlFor`.
	set.Globals["staticUrl"] = config.StaticUrl
	set.Globals["moduleUrl"] = func(moduleName string, args ...string) string {
		return config.UrlFor(moduleName, args...)
	}

	set.Globals["jsonify"] = json.Encode
	set.Globals["constants"] = struct {
		Countries     []country.Country
		CurrencyTypes []currency.Type
		ThankYouTypes []thankyou.Type
	}{
		Countries:     country.Countries,
		CurrencyTypes: currency.Types,
		ThankYouTypes: thankyou.Types,
	}
	return set
}

var templateSet = TemplateSet()

func Render(c *gin.Context, path string, pairs ...interface{}) (err error) {
	// All templates are expected to be in templates dir
	templatePath := cwd + "/templates/" + path

	// Get template from cache
	template, err := templateSet.FromCache(templatePath)
	if err != nil {
		log.Panic("Unable to render template: %v\n\n%v", path, err)
	}

	// Create context from pairs
	ctx := pongo2.Context{}

	// Make gin context available
	ctx["ctx"] = c.Keys

	for i := 0; i < len(pairs); i = i + 2 {
		ctx[pairs[i].(string)] = pairs[i+1]
	}

	// Render template
	if err := template.ExecuteWriter(ctx, c.Writer); err != nil {
		log.Panic("Unable to render template: %v\n\n%v", path, err)
	}

	// Set content type
	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")

	return
}

func RenderString(path string, pairs ...interface{}) string {
	// All templates are expected to be in templates dir
	templatePath := cwd + "/templates/" + path

	// Get template from cache
	template, err := templateSet.FromCache(templatePath)
	if err != nil {
		log.Panic("Unable to render template: %v\n\n%v", path, err)
	}

	// Create context from pairs
	ctx := pongo2.Context{}

	for i := 0; i < len(pairs); i = i + 2 {
		ctx[pairs[i].(string)] = pairs[i+1]
	}

	// Render template
	out, err := template.Execute(ctx)
	if err != nil {
		log.Panic("Unable to render template: %v\n\n%v", path, err)
	}

	return out
}

func RenderStringFromString(template string, pairs ...interface{}) string {
	// Create context from pairs
	ctx := pongo2.Context{}

	for i := 0; i < len(pairs); i = i + 2 {
		ctx[pairs[i].(string)] = pairs[i+1]
	}

	return pongo2.RenderTemplateString(template, ctx)
}
