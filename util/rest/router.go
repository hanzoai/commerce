package rest

import (
	"github.com/gin-gonic/gin"
)

type Router interface {
	GET(string, ...gin.HandlerFunc)
	POST(string, ...gin.HandlerFunc)
	PUT(string, ...gin.HandlerFunc)
	DELETE(string, ...gin.HandlerFunc)
	OPTIONS(string, ...gin.HandlerFunc)
	HEAD(string, ...gin.HandlerFunc)
	PATCH(string, ...gin.HandlerFunc)
	Use(...gin.HandlerFunc)
}
