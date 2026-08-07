package models

import (
	"cm_collectors_server/datatype"
	"fmt"
	"slices"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestResourceDataListUsesStableRandomPages(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(
		&Resources{},
		&Performer{},
		&Tag{},
		&ResourcesDramaSeries{},
		&ResourcesVideoMetadata{},
	); err != nil {
		t.Fatal(err)
	}
	resources := make([]Resources, 120)
	for index := range resources {
		resources[index] = Resources{
			ID:           fmt.Sprintf("resource-%03d", index),
			FilesBasesID: "library-a",
			Title:        fmt.Sprintf("Resource %03d", index),
		}
	}
	if err = db.Create(&resources).Error; err != nil {
		t.Fatal(err)
	}

	request := datatype.ReqParam_ResourcesList{
		ParPaging:    datatype.ParPaging{Page: 1, Limit: 30, FetchCount: true},
		FilesBasesId: "library-a",
		RandomSeed:   "stable-seed",
		SearchData: datatype.ReqParam_SearchData{
			Sort: datatype.E_searchSort("random"),
		},
	}
	firstPage, total, err := (Resources{}).DataList(db, &request)
	if err != nil {
		t.Fatal(err)
	}
	if total != int64(len(resources)) {
		t.Fatalf("expected %d resources, got %d", len(resources), total)
	}
	repeatedFirstPage, _, err := (Resources{}).DataList(db, &request)
	if err != nil {
		t.Fatal(err)
	}
	request.Page = 2
	secondPage, _, err := (Resources{}).DataList(db, &request)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(resourceIDs(*firstPage), resourceIDs(*repeatedFirstPage)) {
		t.Fatal("reloading the same random page changed its order")
	}
	seen := make(map[string]struct{}, len(*firstPage))
	for _, id := range resourceIDs(*firstPage) {
		seen[id] = struct{}{}
	}
	for _, id := range resourceIDs(*secondPage) {
		if _, exists := seen[id]; exists {
			t.Fatalf("resource %q appeared on both database pages", id)
		}
	}
}

func resourceIDs(resources []Resources) []string {
	result := make([]string, len(resources))
	for index := range resources {
		result[index] = resources[index].ID
	}
	return result
}

func TestStableRandomResourcePageKeepsOrderAcrossRequests(t *testing.T) {
	ids := make([]string, 250)
	for index := range ids {
		ids[index] = fmt.Sprintf("resource-%03d", index)
	}

	firstPage := stableRandomResourcePage(ids, "seed-a", 0, 50)
	repeatedFirstPage := stableRandomResourcePage(ids, "seed-a", 0, 50)
	secondPage := stableRandomResourcePage(ids, "seed-a", 50, 50)

	if !slices.Equal(firstPage, repeatedFirstPage) {
		t.Fatal("same seed must keep the first page order stable")
	}
	seen := make(map[string]struct{}, len(firstPage))
	for _, id := range firstPage {
		seen[id] = struct{}{}
	}
	for _, id := range secondPage {
		if _, exists := seen[id]; exists {
			t.Fatalf("resource %q appeared on two pages", id)
		}
	}
}

func TestStableRandomResourcePageChangesWithSeed(t *testing.T) {
	ids := make([]string, 100)
	for index := range ids {
		ids[index] = fmt.Sprintf("resource-%03d", index)
	}

	firstOrder := stableRandomResourcePage(ids, "seed-a", 0, len(ids))
	secondOrder := stableRandomResourcePage(ids, "seed-b", 0, len(ids))

	if slices.Equal(firstOrder, secondOrder) {
		t.Fatal("different seeds should produce different resource orders")
	}
}

func TestStableRandomResourcePageHandlesLastAndInvalidPages(t *testing.T) {
	ids := []string{"a", "b", "c", "d", "e"}

	lastPage := stableRandomResourcePage(ids, "seed", 4, 3)
	if len(lastPage) != 1 {
		t.Fatalf("expected one resource on the last page, got %d", len(lastPage))
	}
	if page := stableRandomResourcePage(ids, "seed", len(ids), 3); len(page) != 0 {
		t.Fatalf("expected an empty page past the end, got %d resources", len(page))
	}
	if page := stableRandomResourcePage(ids, "seed", 0, 0); len(page) != 0 {
		t.Fatalf("expected an empty page for an invalid limit, got %d resources", len(page))
	}
}
