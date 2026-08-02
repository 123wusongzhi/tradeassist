package database

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func openAdminIdentityMigrationDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE admin_users (id text primary key, email text, phone text)`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func TestMigrateAdminLoginIdentityRejectsAmbiguousExistingEmail(t *testing.T) {
	db := openAdminIdentityMigrationDB(t, "admin_identity_duplicate")
	if err := db.Exec(`INSERT INTO admin_users (id, email, phone) VALUES ('a', 'User@Example.com', ''), ('b', ' user@example.com ', '')`).Error; err != nil {
		t.Fatal(err)
	}
	err := migrateAdminLoginIdentityIndexes(db)
	if err == nil || !strings.Contains(err.Error(), "duplicate normalized email") {
		t.Fatalf("expected duplicate email failure, got %v", err)
	}
}

func TestMigrateAdminLoginIdentityCreatesNormalizedUniqueIndexes(t *testing.T) {
	db := openAdminIdentityMigrationDB(t, "admin_identity_unique")
	if err := db.Exec(`INSERT INTO admin_users (id, email, phone) VALUES ('a', 'user@example.com', '+8613800000000'), ('empty-a', '', '')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateAdminLoginIdentityIndexes(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO admin_users (id, email, phone) VALUES ('empty-b', '', '')`).Error; err != nil {
		t.Fatalf("empty optional identities should remain reusable: %v", err)
	}
	if err := db.Exec(`INSERT INTO admin_users (id, email, phone) VALUES ('b', ' USER@EXAMPLE.COM ', '+8613900000000')`).Error; err == nil {
		t.Fatal("normalized duplicate email must be rejected")
	}
	if err := db.Exec(`INSERT INTO admin_users (id, email, phone) VALUES ('c', 'other@example.com', ' +8613800000000 ')`).Error; err == nil {
		t.Fatal("normalized duplicate phone must be rejected")
	}
}
