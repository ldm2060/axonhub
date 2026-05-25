package biz

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmailServiceRenderAccountTemplates(t *testing.T) {
	svc := NewEmailService(EmailServiceParams{})

	tests := []struct {
		name       string
		actionURL  string
		expected   []string
		unexpected []string
	}{
		{
			name:       "verify_email",
			actionURL:  "123456",
			expected:   []string{"AxonHub", "Test User", "123456", "5 minutes"},
			unexpected: []string{"verify-email?token="},
		},
		{
			name:      "reset_password",
			actionURL: "/admin/auth/verify-email?token=test-token",
			expected:  []string{"AxonHub", "Test User", "/admin/auth/verify-email?token=test-token"},
		},
		{
			name:      "account_approved",
			actionURL: "/admin/auth/verify-email?token=test-token",
			expected:  []string{"AxonHub", "Test User", "/admin/auth/verify-email?token=test-token"},
		},
		{
			name:      "account_rejected",
			actionURL: "/admin/auth/verify-email?token=test-token",
			expected:  []string{"AxonHub", "Test User"},
		},
		{
			name:      "admin_notification",
			actionURL: "/admin/auth/verify-email?token=test-token",
			expected:  []string{"AxonHub", "Test User", "/admin/auth/verify-email?token=test-token", "new-user@example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			htmlBody, textBody, err := svc.renderTemplate(tt.name, &emailTemplateData{
				BrandName:     "AxonHub",
				RecipientName: "Test User",
				ActionURL:     tt.actionURL,
				Extra:         "new-user@example.com",
			})

			require.NoError(t, err)
			for _, expected := range tt.expected {
				require.Contains(t, htmlBody, expected)
				require.Contains(t, textBody, expected)
			}
			for _, unexpected := range tt.unexpected {
				require.NotContains(t, htmlBody, unexpected)
				require.NotContains(t, textBody, unexpected)
			}
		})
	}
}
