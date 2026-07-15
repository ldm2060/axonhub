package static

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	ginstatic "github.com/gin-contrib/static"

	"github.com/ldm2060/axonhub/internal/objects"
)

func useTestStaticFS(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(tempDir+"/index.html", []byte("<html><body>test</body></html>"), 0o644))

	originalStaticFS := staticFS
	staticFS = ginstatic.LocalFile(tempDir, false)

	t.Cleanup(func() {
		staticFS = originalStaticFS
	})
}

func TestHandler_ReturnsJSON404ForUnknownAPIPaths(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	useTestStaticFS(t)

	router := gin.New()
	router.NoRoute(Handler())

	for _, path := range []string{"/v1/not-found", "/anthropic/not-found", "/admin/not-found"} {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, path, nil)

			router.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusNotFound, recorder.Code)
			require.Contains(t, recorder.Header().Get("Content-Type"), "application/json")

			var resp objects.ErrorResponse
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
			require.Equal(t, http.StatusText(http.StatusNotFound), resp.Error.Type)
			require.Equal(t, "path not found: "+path, resp.Error.Message)
		})
	}
}

func TestHandler_ServesSPAIndexForFrontendRoutes(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	useTestStaticFS(t)

	router := gin.New()
	router.NoRoute(Handler())

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/settings/profile", nil)

	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Header().Get("Content-Type"), "text/html")
	require.Equal(t, "no-cache, no-store, must-revalidate", recorder.Header().Get("Cache-Control"))
}

func TestHandler_DoesNotFallbackMissingStaticAssetToSPAIndex(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	useTestStaticFS(t)

	router := gin.New()
	router.NoRoute(Handler())

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/assets/definitely-missing.js", nil)

	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.NotContains(t, recorder.Header().Get("Content-Type"), "text/html")
}

func TestDefaultUserAvatarIsAvailableToFrontendBuild(t *testing.T) {
	t.Helper()

	avatar, err := os.ReadFile("../../../frontend/public/images/default-user-avatar.svg")
	require.NoError(t, err)
	require.Contains(t, string(avatar), "<svg")
	require.Contains(t, string(avatar), "A neutral user silhouette")
	require.True(t, isStaticAssetPath(objects.DefaultUserAvatarURL))
}

func TestFrontendBuildTargetsEmbeddedStaticDist(t *testing.T) {
	t.Helper()

	viteConfig, err := os.ReadFile("../../../frontend/vite.config.ts")
	require.NoError(t, err)
	packageJSON, err := os.ReadFile("../../../frontend/package.json")
	require.NoError(t, err)

	config := string(viteConfig)
	require.Contains(t, config, "outDir: '../internal/server/static/dist'")

	buildScript := string(packageJSON)
	require.Contains(t, buildScript, "vite build --emptyOutDir")
	require.Contains(t, buildScript, "../internal/server/static/dist/.gitkeep")
}

func TestRequestsTableFiltersHandleMissingFilterValues(t *testing.T) {
	t.Helper()

	columnsSource, err := os.ReadFile("../../../frontend/src/features/requests/components/requests-columns.tsx")
	require.NoError(t, err)

	source := string(columnsSource)
	require.Contains(t, source, "function getStringFilterValues")
	require.NotContains(t, source, "value.length === 0")
	require.NotContains(t, source, "value.includes(row.getValue")
}

func TestRequestsTableDefaultsFilterProps(t *testing.T) {
	t.Helper()

	tableSource, err := os.ReadFile("../../../frontend/src/features/requests/components/requests-table.tsx")
	require.NoError(t, err)

	source := string(tableSource)
	for _, expected := range []string{
		"statusFilter = []",
		"sourceFilter = []",
		"channelFilter = []",
		"apiKeyFilter = []",
		"modelIDFilter = ''",
		"userFilter = []",
	} {
		require.Contains(t, source, expected)
	}
}

func TestRequestsToolbarHandlesMissingFilterState(t *testing.T) {
	t.Helper()

	toolbarSource, err := os.ReadFile("../../../frontend/src/features/requests/components/data-table-toolbar.tsx")
	require.NoError(t, err)

	source := string(toolbarSource)
	require.Contains(t, source, "const columnFilters = table.getState().columnFilters ?? []")
	require.Contains(t, source, "Array.isArray(currentFilter) && currentFilter.length > 0")
	require.NotContains(t, source, "as string[] | undefined")
}

func TestFacetedFilterToleratesMissingFacetedValues(t *testing.T) {
	t.Helper()

	filterSource, err := os.ReadFile("../../../frontend/src/components/data-table-faceted-filter.tsx")
	require.NoError(t, err)

	source := string(filterSource)
	require.Contains(t, source, "try {")
	require.Contains(t, source, "column?.getFacetedUniqueValues() ?? new Map()")
	require.Contains(t, source, "return new Map()")
}

func TestHandler_ServesSPAForAdminFrontendRoutes(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	useTestStaticFS(t)

	router := gin.New()
	router.NoRoute(Handler())

	for _, path := range []string{"/admin/system", "/admin/system/", "/admin/requests", "/admin/requests/", "/admin/requests/123"} {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, path, nil)

			router.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusOK, recorder.Code)
			require.Contains(t, recorder.Header().Get("Content-Type"), "text/html")
		})
	}
}
