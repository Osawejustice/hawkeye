package httpx

import (
	"net/http"

	"github.com/cohi-hq/cohi-api/internal/http/middleware"
	"github.com/cohi-hq/cohi-api/internal/service"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

type registerRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=72"`
	Name     string `json:"name" binding:"required,min=1,max=120"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if !BindJSON(c, &req) {
		return
	}
	res, err := h.auth.Register(c.Request.Context(), service.RegisterInput{
		Email:    req.Email,
		Password: req.Password,
		Name:     req.Name,
	}, sessionMeta(c))
	if err != nil {
		Error(c, err)
		return
	}
	JSON(c, http.StatusCreated, NewAuthResponse(res))
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if !BindJSON(c, &req) {
		return
	}
	res, err := h.auth.Login(c.Request.Context(), service.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	}, sessionMeta(c))
	if err != nil {
		Error(c, err)
		return
	}
	JSON(c, http.StatusOK, NewAuthResponse(res))
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if !BindJSON(c, &req) {
		return
	}
	res, err := h.auth.Refresh(c.Request.Context(), req.RefreshToken, sessionMeta(c))
	if err != nil {
		Error(c, err)
		return
	}
	JSON(c, http.StatusOK, NewAuthResponse(res))
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req logoutRequest
	if !BindJSON(c, &req) {
		return
	}
	if err := h.auth.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		Error(c, err)
		return
	}
	JSON(c, http.StatusOK, gin.H{"logged_out": true})
}

func (h *AuthHandler) LogoutAll(c *gin.Context) {
	actor := middleware.Actor(c)
	if err := h.auth.LogoutAll(c.Request.Context(), actor.UserID); err != nil {
		Error(c, err)
		return
	}
	JSON(c, http.StatusOK, gin.H{"logged_out": true, "all_sessions": true})
}

func sessionMeta(c *gin.Context) service.SessionMeta {
	return service.SessionMeta{
		UserAgent: c.Request.UserAgent(),
		IPAddress: c.ClientIP(),
	}
}
