package router

import (
	"github.com/gin-gonic/gin"
)

type Router interface {
	Any(relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes
	GET(relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes
	PUT(relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes
	Use(middlewares ...gin.HandlerFunc) gin.IRoutes
	POST(relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes
	HEAD(relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes
	PATCH(relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes
	Group(relativePath string, handlers ...gin.HandlerFunc) *gin.RouterGroup
	Static(relativePath string, root string) gin.IRoutes
	DELETE(relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes
}
