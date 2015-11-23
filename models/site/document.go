package site

type Document struct {
	Id_ string
}

func (d Document) Id() string {
	return string(d.Id_)
}
