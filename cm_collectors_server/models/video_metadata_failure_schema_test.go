package models

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMigrateVideoMetadataFailureManagementSchemaDoesNotRewriteHistory(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.Exec(`CREATE TABLE resourcesDramaSeries (
		id char(20) PRIMARY KEY,
		resources_id char(20),
		type varchar(50),
		src text,
		sort integer DEFAULT 0,
		durationSeconds integer DEFAULT 0,
		durationProbeStatus varchar(20),
		durationProbeTime datetime,
		m3u8BuilderTime datetime,
		m3u8BuilderStatus tinyint DEFAULT 0
	)`).Error; err != nil {
		t.Fatalf("create legacy drama series table: %v", err)
	}
	legacy := ResourcesDramaSeries{
		ID: "legacy-image", Src: "poster.jpg", DurationSeconds: 99,
	}
	if err := db.Exec(
		"INSERT INTO resourcesDramaSeries (id, src, durationSeconds) VALUES (?, ?, ?)",
		legacy.ID, legacy.Src, legacy.DurationSeconds,
	).Error; err != nil {
		t.Fatalf("create legacy row: %v", err)
	}

	if err := migrateVideoMetadataFailureManagementSchema(db); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}
	if !db.Migrator().HasColumn(&ResourcesDramaSeries{}, "VideoMetadataExcluded") {
		t.Fatal("video_metadata_excluded column was not created")
	}
	if !db.Migrator().HasColumn(&ResourcesVideoMetadata{}, "MetadataSource") {
		t.Fatal("metadata_source column was not created")
	}

	var after ResourcesDramaSeries
	if err := db.First(&after, "id = ?", legacy.ID).Error; err != nil {
		t.Fatalf("load legacy row: %v", err)
	}
	if after.VideoMetadataExcluded || after.DurationSeconds != 99 || after.Src != legacy.Src {
		t.Fatalf("schema migration rewrote historical data: %#v", after)
	}
}
