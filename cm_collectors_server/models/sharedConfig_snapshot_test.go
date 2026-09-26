package models

import (
	"gorm.io/gorm"
	"testing"
)

func TestSharedSnapshotRestorePreservesCurrentLocalFields(t *testing.T) {
	for _, test := range []struct {
		module, public, before, during, key string
		original                            float64
	}{
		{"display", displayTestConfig(t, 80), `{"pageLimit":20,"sampleFolder":"before"}`, `{"pageLimit":80,"sampleFolder":"after","performerPreferred":["new"]}`, "pageLimit", 20},
		{"import", importTestConfig(), `{"similarNameToSeries":false,"scanDiskPaths":["before"]}`, `{"similarNameToSeries":true,"scanDiskPaths":["after"]}`, "", 0},
		{"scraper", scraperTestConfig(80), `{"timeout":20,"scanDiskPaths":["before"]}`, `{"timeout":80,"scanDiskPaths":["after"]}`, "timeout", 20},
	} {
		t.Run(test.module, func(t *testing.T) {
			db := sharedTestDB(t)
			col := sharedConfigColumn(test.module)
			if err := db.Model(&FilesBasesSetting{}).Where("filesBases_id = ?", "A").Update(col, test.before).Error; err != nil {
				t.Fatal(err)
			}
			if err := SaveSharedConfig(db, test.module, 0, test.public); err != nil {
				t.Fatal(err)
			}
			follow := func(on bool, mode string) error {
				return db.Transaction(func(tx *gorm.DB) error { return SetLibraryConfigFollow(tx, "A", test.module, on, 1, mode) })
			}
			if err := follow(true, "keep"); err != nil {
				t.Fatal(err)
			}
			state, err := SharedConfigStatus(db, "A", test.module)
			if err != nil || !state.CanRestore {
				t.Fatalf("missing snapshot: %v", err)
			}
			// 跟随期间保存当前通用参数及新的独立字段，不能破坏最初快照。
			if err := db.Model(&FilesBasesSetting{}).Where("filesBases_id = ?", "A").Update(col, test.during).Error; err != nil {
				t.Fatal(err)
			}
			if err := follow(true, "keep"); err != nil {
				t.Fatal(err)
			}
			if err := follow(false, "restore"); err != nil {
				t.Fatal(err)
			}
			var raw string
			db.Model(&FilesBasesSetting{}).Where("filesBases_id = ?", "A").Pluck(col, &raw)
			value, err := parseConfig(raw)
			if err != nil {
				t.Fatal(err)
			}
			if test.key != "" && value[test.key] != test.original {
				t.Fatalf("original value not restored: %v", value)
			}
			if test.module == "display" {
				if value["sampleFolder"] != "after" || value["performerPreferred"].([]interface{})[0] != "new" {
					t.Fatal("independent fields reverted")
				}
				if _, ok := value["country"]; ok {
					t.Fatal("absent original fields must remain absent")
				}
			} else if value["scanDiskPaths"].([]interface{})[0] != "after" {
				t.Fatal("paths reverted")
			}
			if test.module == "import" && value["similarNameToSeries"] != false {
				t.Fatal("false snapshot value lost")
			}
			state, err = SharedConfigStatus(db, "A", test.module)
			if err != nil || state.Following || state.CanRestore {
				t.Fatal("detach did not remove follow record")
			}
			// 下一次开启重新保存快照，选择 keep 后保持公共配置。
			if err := follow(true, "keep"); err != nil {
				t.Fatal(err)
			}
			if err := follow(false, "keep"); err != nil {
				t.Fatal(err)
			}
			db.Model(&FilesBasesSetting{}).Where("filesBases_id = ?", "A").Pluck(col, &raw)
			value, _ = parseConfig(raw)
			pub, _ := parseConfig(test.public)
			if test.key != "" && value[test.key] != pub[test.key] {
				t.Fatal("keep did not preserve public settings")
			}
		})
	}
}

func TestSharedLegacyFollowCannotRestore(t *testing.T) {
	db := sharedTestDB(t)
	if err := SaveSharedConfig(db, "display", 0, displayTestConfig(t, 80)); err != nil {
		t.Fatal(err)
	}
	db.Create(&LibraryConfigFollow{FilesBasesID: "A", Module: "display"})
	state, err := SharedConfigStatus(db, "A", "display")
	if err != nil || state.CanRestore {
		t.Fatal("legacy follow incorrectly offers restore")
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return SetLibraryConfigFollow(tx, "A", "display", false, 1, "restore") }); err == nil {
		t.Fatal("legacy restore should fail")
	}
	state, _ = SharedConfigStatus(db, "A", "display")
	if !state.Following {
		t.Fatal("failed restore removed follow record")
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return SetLibraryConfigFollow(tx, "A", "display", false, 1, "keep") }); err != nil {
		t.Fatal(err)
	}
}
