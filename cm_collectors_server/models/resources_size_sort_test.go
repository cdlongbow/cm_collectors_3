package models

import (
	"cm_collectors_server/datatype"
	"slices"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newResourceSizeSortTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.AutoMigrate(
		&Resources{},
		&ResourcesDramaSeries{},
		&ResourcesVideoMetadata{},
	); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	return db
}

func TestResourceSizeOrderUsesCompleteTotalsAndKeepsUnknownLast(t *testing.T) {
	db := newResourceSizeSortTestDB(t)
	resources := []Resources{
		{ID: "pinned-unknown", FilesBasesID: "library", Mode: datatype.E_resourceMode_Movies, PinToTop: 1},
		{ID: "large", FilesBasesID: "library", Mode: datatype.E_resourceMode_Movies},
		{ID: "small", FilesBasesID: "library", Mode: datatype.E_resourceMode_VideoLink},
		{ID: "excluded-missing", FilesBasesID: "library", Mode: datatype.E_resourceMode_Movies},
		{ID: "partial", FilesBasesID: "library", Mode: datatype.E_resourceMode_Movies},
		{ID: "stale", FilesBasesID: "library", Mode: datatype.E_resourceMode_Movies},
		{ID: "comic", FilesBasesID: "library", Mode: datatype.E_resourceMode_Comic},
	}
	if err := db.Create(&resources).Error; err != nil {
		t.Fatalf("create resources: %v", err)
	}
	series := []ResourcesDramaSeries{
		{ID: "large-success", ResourcesID: "large"},
		{ID: "large-failed", ResourcesID: "large"},
		{ID: "small-manual", ResourcesID: "small"},
		{ID: "excluded-known", ResourcesID: "excluded-missing"},
		{ID: "excluded-ignored", ResourcesID: "excluded-missing", VideoMetadataExcluded: true},
		{ID: "partial-known", ResourcesID: "partial"},
		{ID: "partial-missing", ResourcesID: "partial"},
		{ID: "stale-file", ResourcesID: "stale"},
		{ID: "comic-file", ResourcesID: "comic"},
	}
	if err := db.Create(&series).Error; err != nil {
		t.Fatalf("create drama series: %v", err)
	}
	metadata := []ResourcesVideoMetadata{
		{DramaSeriesID: "large-success", ProbeStatus: VideoMetadataStatusSuccess, MetadataVersion: CurrentVideoMetadataVersion, FileSize: 600},
		{DramaSeriesID: "large-failed", ProbeStatus: VideoMetadataStatusFailed, FileSize: 400},
		{DramaSeriesID: "small-manual", ProbeStatus: VideoMetadataStatusManual, FileSize: 200},
		{DramaSeriesID: "excluded-known", ProbeStatus: VideoMetadataStatusSuccess, MetadataVersion: CurrentVideoMetadataVersion, FileSize: 100},
		{DramaSeriesID: "partial-known", ProbeStatus: VideoMetadataStatusSuccess, MetadataVersion: CurrentVideoMetadataVersion, FileSize: 500},
		{DramaSeriesID: "stale-file", ProbeStatus: VideoMetadataStatusStale, MetadataVersion: CurrentVideoMetadataVersion, FileSize: 2000},
		{DramaSeriesID: "comic-file", ProbeStatus: VideoMetadataStatusSuccess, MetadataVersion: CurrentVideoMetadataVersion, FileSize: 3000},
	}
	if err := db.Create(&metadata).Error; err != nil {
		t.Fatalf("create metadata: %v", err)
	}

	assertResourceSizeOrder(t, db, datatype.E_searchSort_resourceSizeDesc, []string{
		"pinned-unknown", "large", "small", "excluded-missing", "comic", "partial", "stale",
	})
	assertResourceSizeOrder(t, db, datatype.E_searchSort_resourceSizeAsc, []string{
		"pinned-unknown", "excluded-missing", "small", "large", "comic", "partial", "stale",
	})
}

func assertResourceSizeOrder(t *testing.T, db *gorm.DB, order datatype.E_searchSort, expected []string) {
	t.Helper()
	var resources []Resources
	query := db.Model(&Resources{}).Where("filesBases_id = ?", "library")
	if err := (Resources{}).setDbSearchDataOrder(query, order, "library").Find(&resources).Error; err != nil {
		t.Fatalf("query resource size order %q: %v", order, err)
	}
	if actual := resourceIDs(resources); !slices.Equal(actual, expected) {
		t.Fatalf("unexpected resource size order %q: got %v, want %v", order, actual, expected)
	}
}
