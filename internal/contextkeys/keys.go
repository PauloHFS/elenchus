package contextkeys

type contextKey string

const UserContextKey contextKey = "user"
const LocaleKey contextKey = "locale"
const CSRFTokenKey contextKey = "csrf_token"
