package ctxkey

// TraceID is the *gin.Context key for the request correlation id (see middleware.RequestID).
const TraceID = "trace_id"

// AdminID holds the authenticated admin UUID string (*gin.Context key).
const AdminID = "admin_id"

// AdminUsername holds the JWT username claim (*gin.Context key).
const AdminUsername = "admin_username"

// TenantID holds the authenticated tenant id (*gin.Context key).
const TenantID = "tenant_id"

// SessionID holds the authenticated session id (*gin.Context key).
const SessionID = "session_id"
