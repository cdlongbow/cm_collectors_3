package processors

import (
	cmscraper "cm_collectors_server/api/cm_scraper"
	"cm_collectors_server/core"
	"cm_collectors_server/datatype"
	"cm_collectors_server/models"
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// 公共刮削配置被多个库复用，保存及启用前先验证依赖。不会启动浏览器或执行刮削。
func validateSharedScraper(raw string) error {
	var config datatype.Config_Scraper
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return err
	}
	if len(config.ScraperConfigs) == 0 {
		return fmt.Errorf("请先选择公共配置使用的刮削器")
	}
	for _, name := range config.ScraperConfigs {
		if name == "" || strings.ContainsAny(name, `/\:`) || name == "." || name == ".." {
			return fmt.Errorf("无效的刮削器名称")
		}
		if _, err := cmscraper.LoadConfig(Scraper{}.getConfigPath(name)); err != nil {
			return fmt.Errorf("刮削器 %s 不可用: %w", name, err)
		}
	}
	return nil
}

func SaveSharedLibraryConfig(module string, revision int, raw string) error {
	if module == "scraper" {
		if err := validateSharedScraper(raw); err != nil {
			return err
		}
	}
	return models.SaveSharedConfig(core.DBS(), module, revision, raw)
}

func FollowSharedLibraryConfig(tx *gorm.DB, id, module string, following bool, revision int, detachModes ...string) error {
	if following && module == "scraper" {
		state, err := models.SharedConfigStatus(tx, id, module)
		if err != nil {
			return err
		}
		raw, err := json.Marshal(state.Config)
		if err != nil {
			return err
		}
		if err := validateSharedScraper(string(raw)); err != nil {
			return err
		}
	}
	return models.SetLibraryConfigFollow(tx, id, module, following, revision, detachModes...)
}
