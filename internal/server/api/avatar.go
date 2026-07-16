package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/avatar"
	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/objects"
)

type AvatarHandlersParams struct {
	fx.In

	Avatar *avatar.Service
}

type AvatarHandlers struct {
	avatar *avatar.Service
}

func NewAvatarHandlers(params AvatarHandlersParams) *AvatarHandlers {
	return &AvatarHandlers{avatar: params.Avatar}
}

func (h *AvatarHandlers) Upload(c *gin.Context) {
	user, ok := contexts.GetUser(c.Request.Context())
	if !ok || user == nil {
		JSONError(c, http.StatusUnauthorized, errors.New("user not found in context"))
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.avatar.MaxBytes()+1<<20)
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("avatar file is required"))
		return
	}
	defer file.Close()

	if err := h.avatar.Save(user.ID, file); err != nil {
		if errors.Is(err, avatar.ErrInvalidImage) {
			JSONError(c, http.StatusBadRequest, err)
			return
		}
		JSONError(c, http.StatusInternalServerError, fmt.Errorf("save avatar: %w", err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"avatar": objects.UserAvatarURL(user.ID)})
}

func (h *AvatarHandlers) Serve(c *gin.Context) {
	userIDText := strings.TrimSuffix(c.Param("userID"), ".png")
	userID, err := strconv.Atoi(userIDText)
	if err != nil || userID <= 0 {
		c.Status(http.StatusNotFound)
		return
	}

	file, err := h.avatar.Open(userID)
	if err != nil {
		if legacyURL, ok := h.avatar.LegacyURL(userID); ok {
			c.Redirect(http.StatusTemporaryRedirect, legacyURL)
			return
		}
		c.Redirect(http.StatusTemporaryRedirect, objects.DefaultUserAvatarURL)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, objects.DefaultUserAvatarURL)
		return
	}

	c.Header("Cache-Control", "no-cache")
	http.ServeContent(c.Writer, c.Request, info.Name(), info.ModTime(), file)
}
