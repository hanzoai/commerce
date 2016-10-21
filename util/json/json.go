package json

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"

	"appengine"
)

func Encode(value interface{}) string {
	return string(EncodeBytes(value))
}

func EncodeBytes(value interface{}) []byte {
	var b []byte
	var err error

	if appengine.IsDevAppServer() {
		b, err = json.MarshalIndent(value, "", "  ")
	} else {
		b, err = json.Marshal(value)
	}

	if err != nil {
		fmt.Println("%v", err)
	}
	return b
}

func EncodeRaw(value interface{}) json.RawMessage {
	return json.RawMessage(EncodeBytes(value))
}

func EncodeBuffer(value interface{}) *bytes.Buffer {
	return bytes.NewBuffer(EncodeBytes(value))
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

func DecodeBytes(data []byte, v interface{}) error {
	err := json.Unmarshal(data, v)
	if err != nil {
		return err
	}
	return nil
}

func DecodeBuffer(buf *bytes.Buffer, v interface{}) error {
	err := json.Unmarshal(buf.Bytes(), v)
	if err != nil {
		return err
	}
	return nil
}

var Unmarshal = json.Unmarshal
