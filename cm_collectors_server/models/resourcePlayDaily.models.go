package models

import (
	"cm_collectors_server/datatype"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ResourcePlayDaily 按服务器本地日期累计播放次数，不从旧的累计热度推测历史。
type ResourcePlayDaily struct {
	ResourcesID string `gorm:"column:resources_id;type:char(20);primaryKey"`
	PlayDate    string `gorm:"column:play_date;type:char(10);primaryKey;index:idx_resource_play_daily_date"`
	PlayCount   int64  `gorm:"column:play_count;not null;default:0"`
}

func (ResourcePlayDaily) TableName() string { return "resource_play_daily" }

// RecordResourcePlay 将累计热度、最后播放信息和每日计数作为一个事务写入。
func RecordResourcePlay(db *gorm.DB, resourceID, lastPlayFile string, now time.Time) error {
	return db.Transaction(func(tx *gorm.DB) error {
		lastPlayTime := datatype.CustomTime(now)
		result := tx.Model(&Resources{}).Where("id = ?", resourceID).Updates(map[string]interface{}{
			"hot":          gorm.Expr("COALESCE(hot, 0) + 1"),
			"lastPlayTime": &lastPlayTime,
			"lastPlayFile": lastPlayFile,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "resources_id"}, {Name: "play_date"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"play_count": gorm.Expr("play_count + 1")}),
		}).Create(&ResourcePlayDaily{ResourcesID: resourceID, PlayDate: now.Format("2006-01-02"), PlayCount: 1}).Error
	})
}
