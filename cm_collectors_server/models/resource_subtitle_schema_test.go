package models

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMigrateResourceSubtitleSchemaPreservesExistingResource(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.Exec(`CREATE TABLE resources (
		id char(20) PRIMARY KEY,
		filesBases_id char(20),
		title varchar(200),
		issueNumber varchar(200),
		status tinyint DEFAULT 1
	)`).Error; err != nil {
		t.Fatalf("create legacy resources table: %v", err)
	}
	if err := db.Exec(
		"INSERT INTO resources (id, filesBases_id, title, issueNumber, status) VALUES (?, ?, ?, ?, ?)",
		"resource-1", "library-1", "Existing title", "NO-001", true,
	).Error; err != nil {
		t.Fatalf("create legacy resource: %v", err)
	}

	if err := migrateResourceSubtitleSchema(db); err != nil {
		t.Fatalf("migrate subtitle schema: %v", err)
	}
	if !db.Migrator().HasColumn(&Resources{}, "Subtitle") {
		t.Fatal("subtitle column was not created")
	}
	var resource Resources
	if err := db.First(&resource, "id = ?", "resource-1").Error; err != nil {
		t.Fatalf("load existing resource: %v", err)
	}
	if resource.Title != "Existing title" || resource.IssueNumber != "NO-001" || resource.Subtitle != "" {
		t.Fatalf("schema migration rewrote existing resource: %#v", resource)
	}
}
