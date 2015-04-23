package mailchimp

import "github.com/mattbaird/gochimp"

const root = "http://mandrillapp.com/api/1.0"

func BatchSubscribe(apiKey string) error {
	chimpApi := gochimp.NewChimp(apiKey, true)
	chimpApi.BatchSubscribe(gochimp.BatchSubscribe{})
	return nil
}
