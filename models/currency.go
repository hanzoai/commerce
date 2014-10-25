package models

type Currency struct {
	value int64
}

func (c Currency) Add()    {}
func (c Currency) Sub()    {}
func (c Currency) Mul()    {}
func (c Currency) String() {}
