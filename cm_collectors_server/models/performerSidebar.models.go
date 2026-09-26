package models

import (
	"cm_collectors_server/core"
	"strings"

	"gorm.io/gorm"
)

type SidebarPerformerOptions struct {
	SortMode   string
	RecentDays int
}

// ListTopPreferredPerformers 保留手动顺序，统一过滤照片，再按当前库排名补齐。
func (t Performer) ListTopPreferredPerformers(db *gorm.DB, preferredIDs []string, mainBaseID string, shieldNoPhoto bool, limit int, filesBaseID string, options ...SidebarPerformerOptions) (*[]Performer, error) {
	result := []Performer{}
	if limit <= 0 {
		return &result, nil
	}
	opt := SidebarPerformerOptions{}
	if len(options) > 0 {
		opt = options[0]
	}
	if opt.RecentDays < 1 || opt.RecentDays > 365 {
		opt.RecentDays = 30
	}

	// 过滤只影响展示，不修改用户保存的优先演员列表。
	var preferred []Performer
	if len(preferredIDs) > 0 {
		q := db.Where("id IN ? AND status = 1", preferredIDs)
		if shieldNoPhoto {
			q = q.Where("TRIM(COALESCE(photo, '')) <> ''")
		}
		if err := q.Find(&preferred).Error; err != nil {
			return nil, err
		}
	}
	byID := make(map[string]Performer, len(preferred))
	for _, p := range preferred {
		byID[p.ID] = p
	}
	selected := []string{}
	for _, id := range preferredIDs {
		if p, ok := byID[id]; ok {
			result = append(result, p)
			selected = append(selected, id)
			delete(byID, id)
			if len(result) == limit {
				break
			}
		}
	}
	if len(result) < limit && strings.TrimSpace(mainBaseID) != "" {
		q := db.Model(&Performer{}).Where("performer.performerBases_id = ? AND performer.status = 1", mainBaseID)
		if shieldNoPhoto {
			q = q.Where("TRIM(COALESCE(performer.photo, '')) <> ''")
		}
		if len(selected) > 0 {
			q = q.Where("performer.id NOT IN ?", selected)
		}
		// 缺少文件库时兼容旧客户端，使用默认排序，避免将其它库热度混入。
		if filesBaseID != "" {
			switch opt.SortMode {
			case PerformerSortResourceCountDesc, "hotDesc", "recentDesc":
				links := db.Raw("SELECT performer_id, resources_id FROM resourcesPerformers UNION SELECT performer_id, resources_id FROM resourcesDirectors")
				rank := db.Table("(?) AS links", links).
					Joins("INNER JOIN resources r ON r.id = links.resources_id").
					Where("r.filesBases_id = ? AND r.status = 1", filesBaseID).
					Group("links.performer_id")
				metric := "COUNT(*)"
				if opt.SortMode == "hotDesc" {
					metric = "SUM(COALESCE(r.hot, 0))"
				}
				if opt.SortMode == "recentDesc" {
					now := core.TimeNow()
					daily := db.Model(&ResourcePlayDaily{}).
						Select("resources_id, SUM(play_count) AS recent_count").
						Where("play_date >= ? AND play_date <= ?", now.AddDate(0, 0, 1-opt.RecentDays).Format("2006-01-02"), now.Format("2006-01-02")).
						Group("resources_id")
					rank = rank.Joins("LEFT JOIN (?) AS daily ON daily.resources_id = r.id", daily)
					metric = "SUM(COALESCE(daily.recent_count, 0))"
				}
				rank = rank.Select("links.performer_id, " + metric + " AS rank_value")
				q = q.Select("performer.*").Joins("LEFT JOIN (?) AS ranks ON ranks.performer_id = performer.id", rank).
					Order("COALESCE(ranks.rank_value, 0) DESC")
			}
		}
		var rest []Performer
		if err := q.Order("performer.addTime DESC").Order("performer.id DESC").Limit(limit - len(result)).Find(&rest).Error; err != nil {
			return nil, err
		}
		result = append(result, rest...)
	}
	if err := t.fillResourceCounts(db, result, filesBaseID); err != nil {
		return nil, err
	}
	return &result, nil
}
