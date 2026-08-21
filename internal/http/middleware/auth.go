package middleware

import (
	"strings"

	"github.com/cohi-hq/cohi-api/internal/apierr"
	"github.com/cohi-hq/cohi-api/internal/auth"
	"github.com/cohi-hq/cohi-api/internal/http/respond"
	"github.com/cohi-hq/cohi-api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	ContextUserID = "user_id"
	ContextOrgID  = "org_id"
	ContextEmail  = "email"
	ContextRole   = "role"
	ContextClaims = "claims"
)

type tokenParser interface {
	ParseAccess(token string) (*auth.AccessClaims, error)
}

func Auth(parser tokenParser) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			respond.Error(c, apierr.ErrUnauthorized)
			c.Abort()
			return
		}
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			respond.Error(c, apierr.ErrUnauthorized.WithMessage("invalid authorization header"))
			c.Abort()
			return
		}

		claims, err := parser.ParseAccess(strings.TrimSpace(parts[1]))
		if err != nil {
			respond.Error(c, err)
			c.Abort()
			return
		}

		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			respond.Error(c, apierr.ErrTokenInvalid)
			c.Abort()
			return
		}
		orgID, err := uuid.Parse(claims.OrganizationID)
		if err != nil {
			respond.Error(c, apierr.ErrTokenInvalid)
			c.Abort()
			return
		}

		c.Set(ContextUserID, userID)
		c.Set(ContextOrgID, orgID)
		c.Set(ContextEmail, claims.Email)
		c.Set(ContextRole, claims.Role)
		c.Set(ContextClaims, claims)
		c.Next()
	}
}

func Actor(c *gin.Context) service.Actor {
	userID, _ := c.Get(ContextUserID)
	orgID, _ := c.Get(ContextOrgID)
	role, _ := c.Get(ContextRole)

	actor := service.Actor{}
	if id, ok := userID.(uuid.UUID); ok {
		actor.UserID = id
	}
	if id, ok := orgID.(uuid.UUID); ok {
		actor.OrgID = id
	}
	if r, ok := role.(string); ok {
		actor.Role = r
	}
	return actor
}
