package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/objects"
)

func WithProjectID() gin.HandlerFunc {
	return func(c *gin.Context) {
		projectIDStr := c.GetHeader("X-Project-ID")
		if projectIDStr == "" {
			c.Next()
			return
		}

		projectID, parseErr := objects.ParseGUID(projectIDStr)
		if parseErr != nil || projectID.Type != ent.TypeProject {
			AbortWithError(c, http.StatusBadRequest, errors.New("Invalid project ID"))
			return
		}

		ctx := contexts.WithProjectID(c.Request.Context(), projectID.ID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
