package json

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
)

func Encode(value interface{}) string {
	b, err := json.Marshal(value)
	if err != nil {
		fmt.Println("error:", err)
	}
	return string(b)
}

func Decode(body io.ReadCloser, v interface{}) error {
	content, err := ioutil.ReadAll(body)
	body.Close()
	if err != nil {
		return err
	}
	err = json.Unmarshal(content, v)
	if err != nil {
		return err
	}
	return nil
}
