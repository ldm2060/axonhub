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
		// If X-Project-ID header is provided, use it
		projectIDStr := c.GetHeader("X-Project-ID")
		if projectIDStr != "" {
			projectID, parseErr := objects.ParseGUID(projectIDStr)
			if parseErr != nil || projectID.Type != ent.TypeProject {
				AbortWithError(c, http.StatusBadRequest, errors.New("Invalid project ID"))
				return
			}

			ctx := contexts.WithProjectID(c.Request.Context(), projectID.ID)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}

		// No header provided — auto-resolve from user's private_project_id
		if _, ok := contexts.GetProjectID(c.Request.Context()); !ok {
			if user, ok := contexts.GetUser(c.Request.Context()); ok && user != nil && user.PrivateProjectID != nil && *user.PrivateProjectID != 0 {
				ctx := contexts.WithProjectID(c.Request.Context(), *user.PrivateProjectID)
				c.Request = c.Request.WithContext(ctx)
			}
		}

		c.Next()
	}
}
