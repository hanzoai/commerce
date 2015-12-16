package site

type Document struct {
	Id_ string
}

func (d Document) Id() string {
	return d.Id_
}
