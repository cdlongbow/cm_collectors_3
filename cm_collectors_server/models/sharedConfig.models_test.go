package models

import (
	"cm_collectors_server/datatype"
	"encoding/json"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func sharedTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&FilesBases{}, &FilesBasesSetting{}, &SharedLibraryConfig{}, &LibraryConfigFollow{}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"A", "B", "C"} {
		if err := db.Create(&FilesBases{ID: id, Name: id, Status: true}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&FilesBasesSetting{FilesBasesID: id, ConfigJsonData: `{"pageLimit":12,"coverDisplayTag":["local-tag"],"sampleFolder":"local-folder","coverPosterData":[{"name":"local"}]}`}).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func displayTestConfig(t *testing.T, limit int) string {
	t.Helper()
	data, _ := json.Marshal(datatype.Config_FilesBases{})
	value, _ := parseConfig(string(data))
	value["casualViewModule"] = false
	value["casualViewNumber"] = 10
	value["pageLimit"] = limit
	// 空数组是有效设置，不能作为空值拒绝。
	for _, key := range SharedConfigFields("display") {
		if value[key] == nil {
			value[key] = []string{}
		}
	}
	value["coverDisplayTag"] = []string{"must-not-copy"}
	value["sampleFolder"] = "must-not-copy"
	data, _ = json.Marshal(value)
	return string(data)
}

func TestSharedDisplayIsolationDetachAndRevision(t *testing.T) {
	db := sharedTestDB(t)
	if err := SaveSharedConfig(db, "display", 0, displayTestConfig(t, 32)); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"A", "B"} {
		if err := db.Transaction(func(tx *gorm.DB) error { return SetLibraryConfigFollow(tx, id, "display", true, 1) }); err != nil {
			t.Fatal(err)
		}
	}
	read := func(id string) map[string]interface{} {
		var setting FilesBasesSetting
		if err := db.Where("filesBases_id = ?", id).Take(&setting).Error; err != nil {
			t.Fatal(err)
		}
		raw, err := EffectiveLibraryConfig(db, id, "display", setting.ConfigJsonData)
		if err != nil {
			t.Fatal(err)
		}
		value, err := parseConfig(raw)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	if read("A")["pageLimit"] != float64(32) || read("C")["pageLimit"] != float64(12) {
		t.Fatal("library isolation failed")
	}
	if read("A")["sampleFolder"] != "local-folder" || read("A")["coverDisplayTag"].([]interface{})[0] != "local-tag" {
		t.Fatal("local references overwritten")
	}
	if err := SaveSharedConfig(db, "display", 1, displayTestConfig(t, 64)); err != nil {
		t.Fatal(err)
	}
	if err := SaveSharedConfig(db, "display", 1, displayTestConfig(t, 16)); err == nil {
		t.Fatal("stale revision accepted")
	}
	if read("B")["pageLimit"] != float64(64) {
		t.Fatal("new public config not effective")
	}
	if err := GuardSharedConfigWrite(db, "A", "display", displayTestConfig(t, 99)); err == nil {
		t.Fatal("ordinary save bypassed following")
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return SetLibraryConfigFollow(tx, "B", "display", false, 2) }); err != nil {
		t.Fatal(err)
	}
	if err := SaveSharedConfig(db, "display", 2, displayTestConfig(t, 128)); err != nil {
		t.Fatal(err)
	}
	if read("B")["pageLimit"] != float64(64) || read("A")["pageLimit"] != float64(128) {
		t.Fatal("detach did not preserve effective values")
	}
	state, err := SharedConfigStatus(db, "B", "display")
	if err != nil {
		t.Fatal(err)
	}
	if state.Following || len(state.Libraries) != 1 || state.Libraries[0].ID != "A" {
		t.Fatalf("incorrect follower list: %#v", state)
	}
}

func TestSharedConfigLegacyAndFailedTransactions(t *testing.T) {
	db := sharedTestDB(t)
	raw, err := EffectiveLibraryConfig(db, "C", "display", "")
	if err != nil || raw != "" {
		t.Fatal("empty legacy config changed")
	}
	if err := SaveSharedConfig(db, "display", 0, `{"pageLimit":32}`); err == nil {
		t.Fatal("incomplete config accepted")
	}
	if err := SaveSharedConfig(db, "display", 0, displayTestConfig(t, 32)); err != nil {
		t.Fatal(err)
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := SetLibraryConfigFollow(tx, "A", "display", true, 1); err != nil {
			return err
		}
		return SetLibraryConfigFollow(tx, "A", "unknown", true, 1)
	})
	if err == nil {
		t.Fatal("invalid module accepted")
	}
	state, err := SharedConfigStatus(db, "A", "display")
	if err != nil || state.Following {
		t.Fatal("failed transaction left follow state")
	}
	if err := GuardSharedConfigWrite(db, "C", "display", `{"pageLimit":17}`); err != nil {
		t.Fatal("independent legacy save rejected", err)
	}
}
