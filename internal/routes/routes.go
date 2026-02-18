package routes

const (
	Home             = "/"
	Login            = "/login"
	Logout           = "/logout"
	Register         = "/register"
	ForgotPassword   = "/forgot-password"
	ResetPassword    = "/reset-password"
	VerifyEmail      = "/verify-email"
	Dashboard        = "/dashboard"
	Health           = "/health"
	Metrics          = "/metrics"
	EvaluationsPage  = "/evaluations"
	EvaluationStart  = "/htmx/evaluations"
	EvaluationStatus = "/htmx/evaluations/{id}/events"  // SSE endpoint
	EvaluationResult = "/htmx/evaluations/{id}/result"
	EvaluationsList  = "/htmx/evaluations/list"
)
