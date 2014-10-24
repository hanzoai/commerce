package cookies

import (
	"github.com/gorilla/securecookie"
)

var cookieHandler = securecookie.New(securecookie.GenerateRandomKey(64),
									 securecookie.GenerateRandomKey(32))

type CookieParser struct {
	request *http.Request
}

func (c CookieParser) Read(name string) {
    if cookie, err := r.Cookie("cookie-name"); err == nil {
        value := make(map[string]string)
        if err = s2.Decode("cookie-name", cookie.Value, &value); err == nil {
            fmt.Fprintf(w, "The value of foo is %q", value["foo"])
        }
    }
}
