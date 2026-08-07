package processors

import (
	"cm_collectors_server/datatype"
	"cm_collectors_server/models"
	"fmt"
	"testing"
)

func TestValidateVideoMetadataScope(t *testing.T) {
	if err := validateVideoMetadataScope(models.VideoMetadataScopeAll, nil); err != nil {
		t.Fatalf("all scope should not require ids: %v", err)
	}
	if err := validateVideoMetadataScope(models.VideoMetadataScopeSelected, nil); err == nil {
		t.Fatal("selected scope should require ids")
	}
	if err := validateVideoMetadataScope(models.VideoMetadataScopeSelected, []string{"a", "a"}); err != nil {
		t.Fatalf("selected scope should accept unique non-empty ids: %v", err)
	}
}

func TestValidVideoMetadataRunMode(t *testing.T) {
	validModes := []string{
		VideoMetadataRunMissing,
		VideoMetadataRunMissingStale,
		VideoMetadataRunFailed,
		VideoMetadataRunFailedForce,
		VideoMetadataRunAll,
	}
	for _, mode := range validModes {
		if !validVideoMetadataRunMode(mode) {
			t.Fatalf("expected run mode %q to be valid", mode)
		}
	}
	if validVideoMetadataRunMode("unknown") {
		t.Fatal("unknown run mode should be invalid")
	}
}

func TestValidateCronVideoMetadataScope(t *testing.T) {
	if err := validateCronJobScopeSelection(
		models.VideoMetadataScopeAll, nil, datatype.E_cronJobsType_VideoMetadata,
	); err != nil {
		t.Fatalf("video metadata cron should support all scope: %v", err)
	}
	if err := validateCronJobScopeSelection(
		models.VideoMetadataScopeSelected, []string{"a", "b"}, datatype.E_cronJobsType_VideoMetadata,
	); err != nil {
		t.Fatalf("video metadata cron should support multiple libraries: %v", err)
	}
	if err := validateCronJobScopeSelection(
		models.VideoMetadataScopeSelected, []string{"a", "b"}, datatype.E_cronJobsType_Import,
	); err == nil {
		t.Fatal("legacy cron job should still require exactly one library")
	}
}

func TestClassifyClearlyNonVideoMetadataScopeRunsDuringBackfill(t *testing.T) {
	db := newDramaSeriesSyncTestDB(t)
	series := []models.ResourcesDramaSeries{
		{ID: "image-old", ResourcesID: "resource-1", Src: "poster.jpg", DurationSeconds: 12},
		{ID: "video", ResourcesID: "resource-1", Src: "movie.mp4", DurationSeconds: 34},
		{ID: "image-manual", ResourcesID: "resource-1", Src: "manual.jpg", DurationSeconds: 56},
	}
	if err := db.Create(&series).Error; err != nil {
		t.Fatalf("create series: %v", err)
	}
	metadata := []models.ResourcesVideoMetadata{
		{DramaSeriesID: "image-old", ProbeStatus: models.VideoMetadataStatusSuccess, FileSize: 100},
		{DramaSeriesID: "video", ProbeStatus: models.VideoMetadataStatusSuccess, FileSize: 200},
		{DramaSeriesID: "image-manual", ProbeStatus: models.VideoMetadataStatusManual, MetadataSource: "manual", FileSize: 300},
	}
	if err := db.Create(&metadata).Error; err != nil {
		t.Fatalf("create metadata: %v", err)
	}

	corrected, err := classifyClearlyNonVideoMetadataScope(
		db, models.VideoMetadataScopeSelected, []string{"library-1"},
	)
	if err != nil {
		t.Fatalf("classify historical metadata: %v", err)
	}
	if corrected != 1 {
		t.Fatalf("corrected=%d, want 1", corrected)
	}

	var image models.ResourcesDramaSeries
	if err := db.First(&image, "id = ?", "image-old").Error; err != nil {
		t.Fatalf("image series should remain: %v", err)
	}
	if !image.VideoMetadataExcluded || image.DurationSeconds != 0 {
		t.Fatalf("historical image was not corrected: %#v", image)
	}
	var oldImageMetadata int64
	if err := db.Model(&models.ResourcesVideoMetadata{}).Where("drama_series_id = ?", "image-old").Count(&oldImageMetadata).Error; err != nil {
		t.Fatalf("count old image metadata: %v", err)
	}
	if oldImageMetadata != 0 {
		t.Fatalf("old image metadata should be removed, count=%d", oldImageMetadata)
	}

	var video models.ResourcesDramaSeries
	if err := db.First(&video, "id = ?", "video").Error; err != nil || video.VideoMetadataExcluded {
		t.Fatalf("video should remain unchanged: %#v, %v", video, err)
	}
	var manualImage models.ResourcesDramaSeries
	if err := db.First(&manualImage, "id = ?", "image-manual").Error; err != nil || manualImage.VideoMetadataExcluded {
		t.Fatalf("manual override should remain unchanged: %#v, %v", manualImage, err)
	}
}

func TestFindVideoMetadataFailuresIncludesLegacyFailuresAndPaginates(t *testing.T) {
	db := newDramaSeriesSyncTestDB(t)
	if err := db.AutoMigrate(&models.FilesBases{}); err != nil {
		t.Fatalf("migrate files bases: %v", err)
	}
	if err := db.Create(&models.FilesBases{ID: "library-1", Name: "Library"}).Error; err != nil {
		t.Fatalf("create files base: %v", err)
	}
	series := make([]models.ResourcesDramaSeries, 0, 27)
	metadata := make([]models.ResourcesVideoMetadata, 0, 25)
	for index := 0; index < 25; index++ {
		id := fmt.Sprintf("failed-%02d", index)
		series = append(series, models.ResourcesDramaSeries{ID: id, ResourcesID: "resource-1", Src: id + ".mp4"})
		metadata = append(metadata, models.ResourcesVideoMetadata{
			DramaSeriesID: id,
			ProbeStatus:   models.VideoMetadataStatusFailed,
			ErrorMessage:  "probe failed",
		})
	}
	series = append(series,
		models.ResourcesDramaSeries{
			ID: "legacy-failed", ResourcesID: "resource-1", Src: "legacy.mp4",
			DurationProbeStatus: models.DurationProbeStatusFailed,
		},
		models.ResourcesDramaSeries{
			ID: "excluded-failed", ResourcesID: "resource-1", Src: "poster.jpg",
			DurationProbeStatus: models.DurationProbeStatusFailed, VideoMetadataExcluded: true,
		},
	)
	if err := db.Create(&series).Error; err != nil {
		t.Fatalf("create failed series: %v", err)
	}
	if err := db.Create(&metadata).Error; err != nil {
		t.Fatalf("create failed metadata: %v", err)
	}

	firstPage, err := findVideoMetadataFailures(db, VideoMetadataFailureQuery{Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("query first page: %v", err)
	}
	if firstPage.Total != 26 || len(firstPage.DataList) != 10 {
		t.Fatalf("unexpected first page: total=%d, rows=%d", firstPage.Total, len(firstPage.DataList))
	}
	thirdPage, err := findVideoMetadataFailures(db, VideoMetadataFailureQuery{Page: 3, Limit: 10})
	if err != nil {
		t.Fatalf("query third page: %v", err)
	}
	if thirdPage.Total != 26 || len(thirdPage.DataList) != 6 {
		t.Fatalf("unexpected third page: total=%d, rows=%d", thirdPage.Total, len(thirdPage.DataList))
	}
}
