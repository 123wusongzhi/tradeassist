package customerchat

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func customerChatTenantContext(t *testing.T, tenantID int64) *gin.Context {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set(ctxkey.TenantID, tenantID)
	return c
}

func TestTenantScopedConversationAndSuggestionHideCrossTenantRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&CustomerConversation{}, &CustomerMessage{}, &CustomerReplySuggestion{}); err != nil {
		t.Fatal(err)
	}
	foreign := CustomerConversation{TenantID: 2, Platform: "manual", CustomerName: "other", CustomerLanguage: "en", Status: StatusOpen}
	if err := db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	message := CustomerMessage{ConversationID: foreign.ID, Role: RoleCustomer, Content: "private", Language: "en", MessageType: MessageTypeText, Source: SourceManual}
	if err := db.Create(&message).Error; err != nil {
		t.Fatal(err)
	}
	suggestion := CustomerReplySuggestion{ConversationID: foreign.ID, Status: SuggestionGenerated, SuggestedReply: "private"}
	if err := db.Create(&suggestion).Error; err != nil {
		t.Fatal(err)
	}

	svc := &Service{DB: db}
	c := customerChatTenantContext(t, 1)
	if _, err := svc.ListMessages(c, foreign.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("ListMessages cross-tenant error = %v, want not found", err)
	}
	if err := svc.UpdateSuggestion(c, suggestion.ID, UpdateSuggestionBody{EditedReply: "mutated"}, nil); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("UpdateSuggestion cross-tenant error = %v, want not found", err)
	}
	var got CustomerReplySuggestion
	if err := db.First(&got, "id = ?", suggestion.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.EditedReply != "" || got.Status != SuggestionGenerated {
		t.Fatalf("cross-tenant suggestion was mutated: %+v", got)
	}
}

func TestCreateConversationAssignsTrustedTenant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&CustomerConversation{}); err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: db}
	row, err := svc.CreateConversation(customerChatTenantContext(t, 7), CreateConversationBody{CustomerName: "test"}, func() *uuid.UUID { id := uuid.New(); return &id }())
	if err != nil {
		t.Fatal(err)
	}
	if row.TenantID != 7 {
		t.Fatalf("tenant = %d, want 7", row.TenantID)
	}
}
