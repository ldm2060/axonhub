package api

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/avatar"
	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/ent"
)

func TestAvatarHandlersUploadAndServe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := avatar.NewService(avatar.Config{Directory: t.TempDir()})
	handler := NewAvatarHandlers(AvatarHandlersParams{Avatar: service})

	var imageData bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 20, G: 40, B: 60, A: 255})
	require.NoError(t, png.Encode(&imageData, img))

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "avatar.png")
	require.NoError(t, err)
	_, err = part.Write(imageData.Bytes())
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	router := gin.New()
	router.POST("/admin/me/avatar", func(c *gin.Context) {
		c.Request = c.Request.WithContext(contexts.WithUser(c.Request.Context(), &ent.User{ID: 9}))
		handler.Upload(c)
	})
	router.GET("/avatars/:userID", handler.Serve)

	uploadRecorder := httptest.NewRecorder()
	uploadRequest := httptest.NewRequest(http.MethodPost, "/admin/me/avatar", &body)
	uploadRequest.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(uploadRecorder, uploadRequest)
	require.Equal(t, http.StatusOK, uploadRecorder.Code)
	require.JSONEq(t, `{"avatar":"/avatars/9.png"}`, uploadRecorder.Body.String())

	serveRecorder := httptest.NewRecorder()
	router.ServeHTTP(serveRecorder, httptest.NewRequest(http.MethodGet, "/avatars/9.png", nil))
	require.Equal(t, http.StatusOK, serveRecorder.Code)
	require.Equal(t, "image/png", serveRecorder.Header().Get("Content-Type"))
	require.NotEmpty(t, serveRecorder.Body.Bytes())
}

func TestAvatarHandlersServeFallsBackToDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewAvatarHandlers(AvatarHandlersParams{Avatar: avatar.NewService(avatar.Config{Directory: t.TempDir()})})

	router := gin.New()
	router.GET("/avatars/:userID", handler.Serve)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/avatars/12.png", nil))
	require.Equal(t, http.StatusTemporaryRedirect, recorder.Code)
	require.Equal(t, "/images/default-user-avatar.svg", recorder.Header().Get("Location"))
}
