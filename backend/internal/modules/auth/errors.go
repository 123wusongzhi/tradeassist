package auth

// Auth error codes for API responses.
const (
	ErrAccessTokenExpired       = "AUTH_ACCESS_TOKEN_EXPIRED"
	ErrRefreshTokenExpired      = "AUTH_REFRESH_TOKEN_EXPIRED"
	ErrRefreshTokenRevoked      = "AUTH_REFRESH_TOKEN_REVOKED"
	ErrRefreshTokenReused       = "AUTH_REFRESH_TOKEN_REUSED"
	ErrSessionRevoked           = "AUTH_SESSION_REVOKED"
	ErrUserDisabled             = "AUTH_USER_DISABLED"
	ErrReauthenticationRequired = "AUTH_REAUTHENTICATION_REQUIRED"
	ErrInvalidCredentials       = "AUTH_INVALID_CREDENTIALS"
	ErrAccountTemporarilyLocked = "AUTH_ACCOUNT_TEMPORARILY_LOCKED"
	ErrTooManyAttempts          = "AUTH_TOO_MANY_ATTEMPTS"
	ErrPasswordChangeRequired   = "AUTH_PASSWORD_CHANGE_REQUIRED"
	ErrAuthenticationRequired   = "AUTHENTICATION_REQUIRED"
	ErrPermissionDenied         = "PERMISSION_DENIED"
	ErrTenantAccessDenied       = "TENANT_ACCESS_DENIED"
	ErrShopAccessDenied         = "SHOP_ACCESS_DENIED"
	ErrSensitiveOperationDenied = "SENSITIVE_OPERATION_DENIED"
	ErrCSRFTokenMissing         = "CSRF_TOKEN_MISSING"
	ErrCSRFTokenInvalid         = "CSRF_TOKEN_INVALID"
	ErrOriginNotAllowed         = "ORIGIN_NOT_ALLOWED"
)
