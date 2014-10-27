package models

import (
	"github.com/mholt/binding"
	"net/http"
)
type Currency struct {
	value int64
	FieldMapMixin
}

func (c Currency) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	return errs
}

func (c Currency) Add()    {}
func (c Currency) Sub()    {}
func (c Currency) Mul()    {}
func (c Currency) String() {}
