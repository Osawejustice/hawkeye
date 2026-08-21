package respond

import (
	"errors"
	"net/http"

	"github.com/cohi-hq/cohi-api/internal/apierr"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type envelope struct {
	Success bool       `json:"success"`
	Data    any        `json:"data,omitempty"`
	Error   *errorBody `json:"error,omitempty"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func JSON(c *gin.Context, status int, data any) {
	c.JSON(status, envelope{Success: true, Data: data})
}

func Error(c *gin.Context, err error) {
	app := apierr.As(err)
	status := app.Status
	if status == 0 {
		status = http.StatusInternalServerError
	}

	message := app.Message
	if status >= 500 {
		message = "internal server error"
	}

	c.JSON(status, envelope{
		Success: false,
		Error: &errorBody{
			Code:    app.Code,
			Message: message,
			Details: app.Details,
		},
	})
}

func BindJSON(c *gin.Context, dest any) bool {
	if err := c.ShouldBindJSON(dest); err != nil {
		Error(c, apierr.ErrInvalidInput.WithMessage(validationMessage(err)).WithDetails(validationDetails(err)))
		return false
	}
	return true
}

func validationMessage(err error) string {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) && len(ve) > 0 {
		fe := ve[0]
		switch fe.Tag() {
		case "required":
			return fe.Field() + " is required"
		case "email":
			return "email is invalid"
		case "min":
			return fe.Field() + " is too short"
		case "max":
			return fe.Field() + " is too long"
		case "url":
			return fe.Field() + " is not a valid URL"
		default:
			return fe.Field() + " is invalid"
		}
	}
	if err.Error() == "EOF" {
		return "request body is required"
	}
	return "invalid request body"
}

func validationDetails(err error) any {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return nil
	}
	out := make([]map[string]string, 0, len(ve))
	for _, fe := range ve {
		out = append(out, map[string]string{
			"field": fe.Field(),
			"tag":   fe.Tag(),
		})
	}
	return out
}
