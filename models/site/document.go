package site

type Document struct {
	Id_    string
	Name   string
	Domain string
	Url    string
}

func (d Document) Id() string {
	return d.Id_
}
