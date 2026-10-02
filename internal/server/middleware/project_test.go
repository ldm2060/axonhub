package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/ent"
)

// runProjectIDMiddleware drives WithProjectID with the given user and header, and
// reports the response status plus the project id that reached the handler.
func runProjectIDMiddleware(t *testing.T, u *ent.User, header string) (int, int, bool) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	var (
		gotProjectID int
		gotOK        bool
	)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		if u != nil {
			c.Request = c.Request.WithContext(contexts.WithUser(c.Request.Context(), u))
		}

		c.Next()
	})
	router.Use(WithProjectID())
	router.GET("/t", func(c *gin.Context) {
		gotProjectID, gotOK = contexts.GetProjectID(c.Request.Context())
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/t", nil)
	if header != "" {
		req.Header.Set("X-Project-Id", header)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return rec.Code, gotProjectID, gotOK
}

// X-Project-ID is client-supplied. Downstream privacy rules pin queries to it, so a
// non-member must not be able to point it at another tenant's project.
func TestWithProjectID_RejectsNonMemberProject(t *testing.T) {
	privateProject := 7
	attacker := &ent.User{
		ID:               42,
		PrivateProjectID: &privateProject,
		Scopes:           []string{"read_api_keys", "read_requests"},
	}
	attacker.Edges.ProjectUsers = []*ent.UserProject{{ProjectID: privateProject}}

	status, _, _ := runProjectIDMiddleware(t, attacker, "gid://axonhub/Project/1")
	require.Equal(t, http.StatusForbidden, status, "cross-tenant project header must be rejected")
}

func TestWithProjectID_AllowsMemberProject(t *testing.T) {
	privateProject := 7
	member := &ent.User{ID: 42, PrivateProjectID: &privateProject}
	member.Edges.ProjectUsers = []*ent.UserProject{
		{ProjectID: privateProject},
		{ProjectID: 3},
	}

	status, projectID, ok := runProjectIDMiddleware(t, member, "gid://axonhub/Project/3")
	require.Equal(t, http.StatusOK, status)
	require.True(t, ok)
	require.Equal(t, 3, projectID)
}

func TestWithProjectID_AllowsOwnPrivateProject(t *testing.T) {
	privateProject := 7
	u := &ent.User{ID: 42, PrivateProjectID: &privateProject}

	status, projectID, ok := runProjectIDMiddleware(t, u, "gid://axonhub/Project/7")
	require.Equal(t, http.StatusOK, status)
	require.True(t, ok)
	require.Equal(t, 7, projectID)
}

func TestWithProjectID_AllowsSystemOwnerAnyProject(t *testing.T) {
	owner := &ent.User{ID: 1, IsOwner: true}

	status, projectID, ok := runProjectIDMiddleware(t, owner, "gid://axonhub/Project/99")
	require.Equal(t, http.StatusOK, status)
	require.True(t, ok)
	require.Equal(t, 99, projectID)
}

func TestWithProjectID_FallsBackToPrivateProjectWithoutHeader(t *testing.T) {
	privateProject := 7
	u := &ent.User{ID: 42, PrivateProjectID: &privateProject}

	status, projectID, ok := runProjectIDMiddleware(t, u, "")
	require.Equal(t, http.StatusOK, status)
	require.True(t, ok)
	require.Equal(t, privateProject, projectID)
}

func TestWithProjectID_APIKeyIsPinnedToItsProject(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const keyProject = 3

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		// Reproduce what WithAdminGraphqlAuth injects for an API-key principal.
		ctx := contexts.WithAPIKey(c.Request.Context(), &ent.APIKey{ID: 1, ProjectID: keyProject})
		ctx = contexts.WithProjectID(ctx, keyProject)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	engine.Use(WithProjectID())
	engine.POST("/admin/graphql", func(c *gin.Context) {
		projectID, ok := contexts.GetProjectID(c.Request.Context())
		require.True(t, ok)
		c.JSON(http.StatusOK, projectID)
	})

	tests := []struct {
		name       string
		header     string
		wantStatus int
	}{
		{"no header keeps the key project", "", http.StatusOK},
		{"matching header accepted", "gid://axonhub/Project/3", http.StatusOK},
		{"other project rejected", "gid://axonhub/Project/4", http.StatusForbidden},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/admin/graphql", nil)
			if tc.header != "" {
				req.Header.Set("X-Project-Id", tc.header)
			}

			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, req)

			require.Equal(t, tc.wantStatus, recorder.Code)

			if tc.wantStatus == http.StatusOK {
				require.Equal(t, "3", recorder.Body.String())
			}
		})
	}
}

func TestWithProjectID_UserSelectsProject(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(WithProjectID())
	engine.POST("/admin/graphql", func(c *gin.Context) {
		projectID, ok := contexts.GetProjectID(c.Request.Context())
		require.True(t, ok)
		c.JSON(http.StatusOK, projectID)
	})

	req := httptest.NewRequest(http.MethodPost, "/admin/graphql", nil)
	req.Header.Set("X-Project-Id", "gid://axonhub/Project/9")

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "9", recorder.Body.String())
}
