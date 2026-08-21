package httpx

import (
	"log/slog"
	"time"

	"github.com/cohi-hq/cohi-api/internal/apierr"
	"github.com/cohi-hq/cohi-api/internal/config"
	"github.com/cohi-hq/cohi-api/internal/http/middleware"
	"github.com/cohi-hq/cohi-api/internal/service"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type Dependencies struct {
	Config  *config.Config
	Log     *slog.Logger
	Auth    *service.AuthService
	Users   *service.UserService
	Cameras *service.CameraService
	Health  *HealthHandler
}

func NewRouter(deps Dependencies) *gin.Engine {
	if deps.Config.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger(deps.Log))
	r.Use(middleware.CORS(deps.Config.CORSAllowedOrigins))
	r.Use(middleware.MaxBodyBytes(1 << 20))
	_ = r.SetTrustedProxies(deps.Config.TrustedProxies)

	r.MaxMultipartMemory = 8 << 20

	r.NoRoute(func(c *gin.Context) {
		Error(c, apierr.ErrNotFound.WithMessage("route not found"))
	})

	health := deps.Health
	r.GET("/health", health.Live)
	r.GET("/ready", health.Ready)

	authH := NewAuthHandler(deps.Auth)
	userH := NewUserHandler(deps.Users)
	camH := NewCameraHandler(deps.Cameras)

	v1 := r.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		auth.Use(middleware.RateLimit(rate.Every(6*time.Second), 20))
		{
			auth.POST("/register", authH.Register)
			auth.POST("/login", authH.Login)
			auth.POST("/refresh", authH.Refresh)
			auth.POST("/logout", authH.Logout)
		}

		protected := v1.Group("")
		protected.Use(middleware.Auth(deps.Auth))
		{
			protected.POST("/auth/logout-all", authH.LogoutAll)

			protected.GET("/users/me", userH.Me)
			protected.PATCH("/users/me", userH.UpdateMe)

			cameras := protected.Group("/cameras")
			{
				cameras.GET("", camH.List)
				cameras.POST("", camH.Create)
				cameras.GET("/:id", camH.Get)
				cameras.PATCH("/:id", camH.Update)
				cameras.DELETE("/:id", camH.Delete)
				cameras.POST("/:id/sync", camH.Sync)
				cameras.GET("/:id/status", camH.Status)
			}
		}
	}

	return r
}
