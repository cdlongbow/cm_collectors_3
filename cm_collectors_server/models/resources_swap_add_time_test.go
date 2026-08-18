package models

import (
	"cm_collectors_server/datatype"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newResourceSwapAddTimeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.AutoMigrate(&Resources{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	return db
}

func customTime(value time.Time) *datatype.CustomTime {
	result := datatype.CustomTime(value)
	return &result
}

func TestResourcesSwapAddTime(t *testing.T) {
	db := newResourceSwapAddTimeTestDB(t)
	firstTime := customTime(time.Date(2026, 8, 18, 10, 0, 0, 0, time.Local))
	secondTime := customTime(time.Date(2026, 8, 18, 11, 0, 0, 0, time.Local))
	resources := []Resources{
		{ID: "resource-1", FilesBasesID: "library-1", CreatedAt: firstTime, Status: true},
		{ID: "resource-2", FilesBasesID: "library-1", CreatedAt: secondTime, Status: true},
	}
	if err := db.Create(&resources).Error; err != nil {
		t.Fatalf("create resources: %v", err)
	}

	if err := (Resources{}).SwapAddTime(db, "resource-1", "resource-2"); err != nil {
		t.Fatalf("swap add time: %v", err)
	}

	var first, second Resources
	if err := db.First(&first, "id = ?", "resource-1").Error; err != nil {
		t.Fatalf("load first resource: %v", err)
	}
	if err := db.First(&second, "id = ?", "resource-2").Error; err != nil {
		t.Fatalf("load second resource: %v", err)
	}
	if first.CreatedAt == nil || time.Time(*first.CreatedAt).Format("2006-01-02 15:04:05") != time.Time(*secondTime).Format("2006-01-02 15:04:05") {
		t.Fatalf("first resource add time = %v, want %v", first.CreatedAt, secondTime)
	}
	if second.CreatedAt == nil || time.Time(*second.CreatedAt).Format("2006-01-02 15:04:05") != time.Time(*firstTime).Format("2006-01-02 15:04:05") {
		t.Fatalf("second resource add time = %v, want %v", second.CreatedAt, firstTime)
	}
}

func TestResourcesSwapAddTimeRejectsPinnedResource(t *testing.T) {
	db := newResourceSwapAddTimeTestDB(t)
	resources := []Resources{
		{ID: "resource-1", FilesBasesID: "library-1", PinToTop: 1, CreatedAt: customTime(time.Now()), Status: true},
		{ID: "resource-2", FilesBasesID: "library-1", CreatedAt: customTime(time.Now().Add(time.Hour)), Status: true},
	}
	if err := db.Create(&resources).Error; err != nil {
		t.Fatalf("create resources: %v", err)
	}
	if err := (Resources{}).SwapAddTime(db, "resource-1", "resource-2"); err == nil {
		t.Fatal("expected pinned resource swap to fail")
	}
}
