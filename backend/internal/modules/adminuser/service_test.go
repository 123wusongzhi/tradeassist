package adminuser

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateEnforcesSharedPasswordPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:adminuser_password_policy_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}); err != nil {
		t.Fatal(err)
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/v1/admin/users", nil)
	svc := &Service{DB: db, Cfg: &config.Config{
		AppEnv:                 "production",
		BootstrapAdminPassword: "ProductionBootstrap42!",
		Auth:                   config.AuthConfig{PasswordMinLength: 12},
	}}

	invalid := []struct {
		email    string
		password string
	}{
		{email: "short@example.com", password: "SafePass"},
		{email: "common@example.com", password: "Qwerty123"},
		{email: "bootstrap@example.com", password: "ProductionBootstrap42!"},
	}
	for _, tc := range invalid {
		if _, err := svc.Create(c, CreateBody{Email: tc.email, Password: tc.password}, nil); err == nil {
			t.Fatalf("Create accepted invalid password %q", tc.password)
		}
	}

	if _, err := svc.Create(c, CreateBody{Email: "valid@example.com", Password: "SafePassphrase42!", Role: "admin"}, nil); err != nil {
		t.Fatalf("Create rejected valid password: %v", err)
	}
}
