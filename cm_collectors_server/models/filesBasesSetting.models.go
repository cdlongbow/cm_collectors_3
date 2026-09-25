package models

import "gorm.io/gorm"

type FilesBasesSetting struct {
	FilesBasesID             string `json:"filesBases_id" gorm:"column:filesBases_id;primaryKey;type:char(20);"`
	ConfigJsonData           string `json:"config_json_data" gorm:"type:text;"`
	NfoJsonData              string `json:"-" gorm:"type:text;"`
	SimpleJsonData           string `json:"-" gorm:"type:text;"`
	ScanDiskJsonData         string `json:"-" gorm:"type:text;"`
	ScraperJsonData          string `json:"-" gorm:"type:text;"`
	ScraperPerformerJsonData string `json:"-" gorm:"type:text;"`
}

func (FilesBasesSetting) TableName() string {
	return "filesBasesSetting"
}

func (FilesBasesSetting) InfoByFilesBasesID(db *gorm.DB, filesBasesID string) (*FilesBasesSetting, error) {
	var info FilesBasesSetting
	err := db.Model(&info).Where("filesBases_id = ?", filesBasesID).First(&info).Error
	return &info, err
}

func (FilesBasesSetting) Update(db *gorm.DB, filesBasesID string, filesBasesSetting *FilesBasesSetting, fields []string) error {
	for _, field := range fields {
		if field == "scan_disk_json_data" {
			if err := GuardSharedConfigWrite(db, filesBasesID, "import", filesBasesSetting.ScanDiskJsonData); err != nil {
				return err
			}
		}
		if field == "scraper_json_data" {
			if err := GuardSharedConfigWrite(db, filesBasesID, "scraper", filesBasesSetting.ScraperJsonData); err != nil {
				return err
			}
		}
		if field == "config_json_data" {
			if err := GuardSharedConfigWrite(db, filesBasesID, "display", filesBasesSetting.ConfigJsonData); err != nil {
				return err
			}
		}
	}
	result := db.Model(&filesBasesSetting).Where("filesBases_id = ?", filesBasesID).Select(fields).Updates(filesBasesSetting)
	if result.RowsAffected == 0 {
		return nil
	}
	return result.Error
}

func (FilesBasesSetting) CreateNull(db *gorm.DB, filesBasesID string) error {
	fbm := &FilesBasesSetting{
		FilesBasesID: filesBasesID,
	}
	return db.Create(&fbm).Error
}

// DeleteByFilesBasesID 删除指定文件库的配置记录。
// 文件库真实删除时，配置 JSON 已不再可达，需要跟随文件库一起物理清理。
func (FilesBasesSetting) DeleteByFilesBasesID(db *gorm.DB, filesBasesID string) error {
	if err := db.Where("filesBases_id = ?", filesBasesID).Delete(&LibraryConfigFollow{}).Error; err != nil {
		return err
	}
	return db.Unscoped().Where("filesBases_id = ?", filesBasesID).Delete(&FilesBasesSetting{}).Error
}
