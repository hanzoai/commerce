package util

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"io/ioutil"
)

func DecodeJson(c *gin.Context, v interface{}) error {
	content, err := ioutil.ReadAll(c.Request.Body)
	c.Request.Body.Close()
	if err != nil {
		return err
	}
	err = json.Unmarshal(content, v)
	if err != nil {
		return err
	}
	return nil
}
