package middleware

import (
	"crypto/subtle"
	"strings"

	"github.com/cohi-hq/cohi-api/internal/apierr"
	"github.com/cohi-hq/cohi-api/internal/http/respond"
	"github.com/gin-gonic/gin"
)

const ContextService = "service_auth"

func ServiceToken(expected string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if expected == "" {
			respond.Error(c, apierr.ErrUnauthorized.WithMessage("internal api disabled"))
			c.Abort()
			return
		}

		got := strings.TrimSpace(c.GetHeader("X-Service-Token"))
		if got == "" {
			header := c.GetHeader("Authorization")
			parts := strings.SplitN(header, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				got = strings.TrimSpace(parts[1])
			}
		}
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
			respond.Error(c, apierr.ErrUnauthorized.WithMessage("invalid service token"))
			c.Abort()
			return
		}
		c.Set(ContextService, true)
		c.Next()
	}
}
