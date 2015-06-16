package router

import (
	"github.com/gin-gonic/gin"
)

type Router interface {
	Group(relativePath string, handlers ...gin.HandlerFunc) *gin.RouterGroup
	Handle(httpMethod, relativePath string, handlers []gin.HandlerFunc)
	Static(relativePath, root string)
	Use(middlewares ...gin.HandlerFunc)
}
