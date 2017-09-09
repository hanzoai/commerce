package template

import (
	"os"

	"github.com/flosch/pongo2"
	"github.com/gin-gonic/gin"

	"hanzo.io/config"
	"hanzo.io/models/types/country"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/types/thankyou"
	"hanzo.io/util/json"
	"hanzo.io/util/log"
)

var cwd, _ = os.Getwd()

func TemplateSet() *pongo2.TemplateSet {
	loader := pongo2.MustNewLocalFileSystemLoader("")
	set := pongo2.NewSet("default", loader)

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
		CountriesByISOCode map[string]country.Country
		Countries          []country.Country
		CurrencyTypes      []currency.Type
		ThankYouTypes      []thankyou.Type
	}{
		CountriesByISOCode: country.ByISO3166_2,
		Countries:          country.Countries,
		CurrencyTypes:      currency.Types,
		ThankYouTypes:      thankyou.Types,
	}
	return set
}

var templateSet = TemplateSet()

func createContext(c *gin.Context, pairs ...interface{}) pongo2.Context {
	// Create context from pairs
	ctx := pongo2.Context{}

	if c != nil {
		// Make gin context available
		ctx["ctx"] = c.Keys
	}

	for i := 0; i < len(pairs); i = i + 2 {
		ctx[pairs[i].(string)] = pairs[i+1]
	}

	return ctx
}

func Render(c *gin.Context, path string, pairs ...interface{}) (err error) {
	// All templates are expected to be in templates dir
	templatePath := cwd + "/templates/" + path

	// Get template from cache
	template, err := templateSet.FromCache(templatePath)
	if err != nil {
		log.Panic("Unable to render template: %v\n\n%v", path, err)
	}

	// Create context
	ctx := createContext(c, pairs...)

	// Render template
	if err := template.ExecuteWriter(ctx, c.Writer); err != nil {
		log.Panic("Unable to render template: %v\n\n%v", path, err)
	}

	// Set content type
	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")

	return
}

func RenderString(c *gin.Context, path string, pairs ...interface{}) string {
	// All templates are expected to be in templates dir
	templatePath := cwd + "/templates/" + path

	// Get template from cache
	template, err := templateSet.FromCache(templatePath)
	if err != nil {
		log.Panic("Unable to render template: %v\n\n%v", path, err)
	}

	// Create context
	ctx := createContext(c, pairs...)

	// Render template
	out, err := template.Execute(ctx)
	if err != nil {
		log.Panic("Unable to render template: %v\n\n%v", path, err)
	}

	return out
}

func RenderStringFromString(template string, pairs ...interface{}) string {
	ctx := createContext(nil, pairs...)
	str, err := pongo2.RenderTemplateString(template, ctx)
	if err != nil {
		panic(err)
	}
	return str
}
