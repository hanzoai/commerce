package http

type Error struct {
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

func (e Error) Error() string {
	return e.Message
}
