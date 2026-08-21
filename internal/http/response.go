package httpx

import (
	"github.com/cohi-hq/cohi-api/internal/http/respond"
	"github.com/gin-gonic/gin"
)

func JSON(c *gin.Context, status int, data any) {
	respond.JSON(c, status, data)
}

func Error(c *gin.Context, err error) {
	respond.Error(c, err)
}

func BindJSON(c *gin.Context, dest any) bool {
	return respond.BindJSON(c, dest)
}
