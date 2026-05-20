package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/ldm2060/axonhub/internal/server/biz"
)

type SignUpHandlersParams struct {
	fx.In

	SignUpService *biz.SignUpService
}

func NewSignUpHandlers(params SignUpHandlersParams) *SignUpHandlers {
	return &SignUpHandlers{
		SignUpService: params.SignUpService,
	}
}

type SignUpHandlers struct {
	SignUpService *biz.SignUpService
}

type SignUpRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type SignUpResponse struct {
	User  *objects.UserInfo `json:"user"`
	Token string            `json:"token,omitempty"`
}

func (h *SignUpHandlers) SignUp(c *gin.Context) {
	var req SignUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, token, err := h.SignUpService.SignUp(c.Request.Context(), biz.SignUpInput{
		Email:     req.Email,
		Password:  req.Password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
	})
	if err != nil {
		status := http.StatusBadRequest
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, SignUpResponse{
		User:  biz.ConvertUserToUserInfo(c.Request.Context(), user),
		Token: token,
	})
}

func (h *SignUpHandlers) AllowSignUp(c *gin.Context) {
	allowed := h.SignUpService.AllowSignUp(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"allowed": allowed})
}
