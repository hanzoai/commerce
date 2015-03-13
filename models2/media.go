package models

import (
	"fmt"
	"net/http"

	"github.com/mholt/binding"
)

type MediaType string

const (
	MediaTypeVideo      MediaType = "video"
	MediaTypeImage                = "image"
	MediaTypeLiveStream           = "livestream"
	MediaTypeWebGL                = "webgl"
	MediaTypeAudio                = "audio"
	MediaTypeEmbed                = "embed"
)

type Media struct {
	Type MediaType
	Alt  string
	Url  string
	X    int
	Y    int
}

func (i Media) Dimensions() string {
	return fmt.Sprintf("%sx%s", i.X, i.Y)
}

func (i Media) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	if i.Url == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Url"},
			Classification: "InputError",
			Message:        "Image does not have a URL",
		})
	}
	return errs
}
