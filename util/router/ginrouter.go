package router

import (
	"github.com/gin-gonic/gin"
)

type Router interface {
	DELETE(relativePath string, handlers ...gin.HandlerFunc)
	GET(relativePath string, handlers ...gin.HandlerFunc)
	HEAD(relativePath string, handlers ...gin.HandlerFunc)
	OPTIONS(relativePath string, handlers ...gin.HandlerFunc)
	PATCH(relativePath string, handlers ...gin.HandlerFunc)
	POST(relativePath string, handlers ...gin.HandlerFunc)
	PUT(relativePath string, handlers ...gin.HandlerFunc)
	Group(relativePath string, handlers ...gin.HandlerFunc) *gin.RouterGroup
	Handle(httpMethod, relativePath string, handlers []gin.HandlerFunc)
	Static(relativePath, root string)
	Use(middlewares ...gin.HandlerFunc)
}
