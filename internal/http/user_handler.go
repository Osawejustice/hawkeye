package httpx

import (
	"net/http"

	"github.com/cohi-hq/cohi-api/internal/http/middleware"
	"github.com/cohi-hq/cohi-api/internal/service"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	users *service.UserService
}

func NewUserHandler(users *service.UserService) *UserHandler {
	return &UserHandler{users: users}
}

type updateMeRequest struct {
	Name            *string `json:"name"`
	Password        *string `json:"password"`
	CurrentPassword *string `json:"current_password"`
}

func (h *UserHandler) Me(c *gin.Context) {
	actor := middleware.Actor(c)
	user, err := h.users.GetByID(c.Request.Context(), actor.UserID)
	if err != nil {
		Error(c, err)
		return
	}
	JSON(c, http.StatusOK, NewUserResponse(user))
}

func (h *UserHandler) UpdateMe(c *gin.Context) {
	var req updateMeRequest
	if !BindJSON(c, &req) {
		return
	}
	actor := middleware.Actor(c)
	user, err := h.users.UpdateProfile(c.Request.Context(), actor.UserID, service.UpdateProfileInput{
		Name:            req.Name,
		Password:        req.Password,
		CurrentPassword: req.CurrentPassword,
	})
	if err != nil {
		Error(c, err)
		return
	}
	JSON(c, http.StatusOK, NewUserResponse(user))
}
