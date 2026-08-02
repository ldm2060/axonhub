package middleware

import (
	"context"
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

			// The header is client-supplied, so it must never be trusted on its own:
			// downstream privacy rules pin queries to this project id, and a system-level
			// scope would otherwise be enough to read/write another tenant's data.
			if !userCanAccessProject(c.Request.Context(), projectID.ID) {
				AbortWithError(c, http.StatusForbidden, errors.New("Project access denied"))
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

// userCanAccessProject reports whether the authenticated principal may act within projectID.
// System owners can reach any project; everyone else must be a member of it (or be targeting
// their own private project). The membership edges are already loaded by the auth middleware,
// so this check costs no extra query.
func userCanAccessProject(ctx context.Context, projectID int) bool {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		// Non-user principals (e.g. API keys) get their project from the credential itself,
		// which is resolved elsewhere; leave those flows untouched.
		return true
	}

	if user.IsOwner {
		return true
	}

	if user.PrivateProjectID != nil && *user.PrivateProjectID == projectID {
		return true
	}

	for _, up := range user.Edges.ProjectUsers {
		if up.ProjectID == projectID {
			return true
		}
	}

	return false
}
