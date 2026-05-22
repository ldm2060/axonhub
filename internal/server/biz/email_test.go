package biz

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmailServiceRenderAccountTemplates(t *testing.T) {
	svc := NewEmailService(EmailServiceParams{})

	tests := []struct {
		name     string
		expected []string
	}{
		{
			name:     "verify_email",
			expected: []string{"AxonHub", "Test User", "/admin/auth/verify-email?token=test-token"},
		},
		{
			name:     "reset_password",
			expected: []string{"AxonHub", "Test User", "/admin/auth/verify-email?token=test-token"},
		},
		{
			name:     "account_approved",
			expected: []string{"AxonHub", "Test User", "/admin/auth/verify-email?token=test-token"},
		},
		{
			name:     "account_rejected",
			expected: []string{"AxonHub", "Test User"},
		},
		{
			name:     "admin_notification",
			expected: []string{"AxonHub", "Test User", "/admin/auth/verify-email?token=test-token", "new-user@example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			htmlBody, textBody, err := svc.renderTemplate(tt.name, &emailTemplateData{
				BrandName:     "AxonHub",
				RecipientName: "Test User",
				ActionURL:     "/admin/auth/verify-email?token=test-token",
				Extra:         "new-user@example.com",
			})

			require.NoError(t, err)
			for _, expected := range tt.expected {
				require.Contains(t, htmlBody, expected)
				require.Contains(t, textBody, expected)
			}
		})
	}
}
