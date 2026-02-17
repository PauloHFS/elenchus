package web

import "fmt"

const (
	Home           = "/"
	Login          = "/login"
	Logout         = "/logout"
	Register       = "/register"
	ForgotPassword = "/forgot-password"
	ResetPassword  = "/reset-password"
	VerifyEmail    = "/verify-email"
	Dashboard      = "/dashboard"
	Health         = "/health"
	Metrics        = "/metrics"
)

// WebhookRoute generates the path for a webhook source
func WebhookRoute(source string) string {
	return fmt.Sprintf("/webhooks/%s", source)
}
