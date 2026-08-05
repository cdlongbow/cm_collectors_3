package processors

import (
	"cm_collectors_server/models"
	processorsffmpeg "cm_collectors_server/processorsFFmpeg"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestWaitVideoTranscodeVerificationTimesOut(t *testing.T) {
	release := make(chan struct{})
	result := waitVideoTranscodeVerification(context.Background(), 5*time.Millisecond, func() videoTranscodeVerifyResult {
		<-release
		return videoTranscodeVerifyResult{}
	})
	close(release)
	if result.Err == nil || !strings.Contains(result.Err.Error(), "超过") {
		t.Fatalf("verification watchdog must time out: %v", result.Err)
	}
}

func TestWaitVideoTranscodeVerificationReturnsResult(t *testing.T) {
	expected := errors.New("verification failed")
	result := waitVideoTranscodeVerification(context.Background(), time.Second, func() videoTranscodeVerifyResult {
		return videoTranscodeVerifyResult{Err: expected}
	})
	if !errors.Is(result.Err, expected) {
		t.Fatalf("verification result must be returned: %v", result.Err)
	}
}

func TestCancelMediaReadsReleasesReplacementLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "playing.mp4")
	readContext, releaseRead := registerMediaRead(path, context.Background())
	unlockRead := lockMediaForRead(path)
	readerStopped := make(chan struct{})
	go func() {
		<-readContext.Done()
		unlockRead()
		releaseRead()
		close(readerStopped)
	}()

	cancelMediaReads(path)
	lockAcquired := make(chan struct{})
	go func() {
		lock := mediaPathLocks.get(path)
		lock.Lock()
		lock.Unlock()
		close(lockAcquired)
	}()

	select {
	case <-readerStopped:
	case <-time.After(time.Second):
		t.Fatal("active media reader was not cancelled")
	}
	select {
	case <-lockAcquired:
	case <-time.After(time.Second):
		t.Fatal("replacement lock remained blocked after cancelling media reads")
	}
}

func TestLockMediaForReplacementTimesOut(t *testing.T) {
	path := filepath.Join(t.TempDir(), "busy.mp4")
	unlockRead := lockMediaForRead(path)
	defer unlockRead()
	started := time.Now()
	if unlock, err := lockMediaForReplacement(context.Background(), path, 20*time.Millisecond); err == nil {
		unlock()
		t.Fatal("busy media lock must time out")
	}
	if time.Since(started) > time.Second {
		t.Fatal("replacement lock timeout took too long")
	}
}

func TestBlockMediaReadsRejectsReconnectUntilReplacementEnds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reconnecting.mp4")
	firstContext, firstRelease := registerMediaRead(path, context.Background())
	unblock := blockMediaReads(path)
	defer unblock()
	select {
	case <-firstContext.Done():
	case <-time.After(time.Second):
		t.Fatal("existing media read was not cancelled")
	}
	firstRelease()

	reconnectContext, reconnectRelease := registerMediaRead(path, context.Background())
	defer reconnectRelease()
	if reconnectContext.Err() == nil {
		t.Fatal("reconnecting media read must be rejected while replacement is blocked")
	}

	unblock()
	afterContext, afterRelease := registerMediaRead(path, context.Background())
	defer afterRelease()
	if afterContext.Err() != nil {
		t.Fatal("media reads must resume after replacement finishes")
	}
}

func TestCanRetryVideoTranscodeReplacementRequiresSafeTemporaryOutput(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "movie.mp4")
	temporary := filepath.Join(dir, ".movie.cmtranscode-task-1-run.mp4.part")
	if err := os.WriteFile(source, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(temporary, []byte("output"), 0o644); err != nil {
		t.Fatal(err)
	}
	task := models.VideoTranscodeTask{
		ID:            "task-1",
		Status:        models.VideoTranscodeStatusFailed,
		SourcePath:    source,
		TemporaryPath: temporary,
		OutputSize:    int64(len("output")),
		ErrorMessage:  "释放源视频读取句柄失败: timeout",
	}
	config := DefaultVideoTranscodeConfig()
	config.OutputMode = "replace"
	if !canRetryVideoTranscodeReplacement(task, config) {
		t.Fatal("validated temporary output should allow replacement retry")
	}
	task.OutputSize++
	if canRetryVideoTranscodeReplacement(task, config) {
		t.Fatal("changed temporary output must reject replacement retry")
	}
}

func TestValidateVideoTranscodeConfig(t *testing.T) {
	valid := DefaultVideoTranscodeConfig()
	if err := validateVideoTranscodeConfig(valid); err != nil {
		t.Fatalf("default config should be valid: %v", err)
	}

	copyWithResize := valid
	copyWithResize.VideoCodec = "copy"
	copyWithResize.ResolutionHeight = 1080
	if err := validateVideoTranscodeConfig(copyWithResize); err == nil {
		t.Fatal("stream copy with resize should be rejected")
	}

	invalidContainer := valid
	invalidContainer.Container = "avi"
	if err := validateVideoTranscodeConfig(invalidContainer); err == nil {
		t.Fatal("unsupported container should be rejected")
	}
}

func TestVideoTranscodePathsStayBesideSource(t *testing.T) {
	task := &models.VideoTranscodeTask{
		ID:         "task-1",
		SourcePath: filepath.Join("D:", "videos", "movie.mkv"),
	}
	config := DefaultVideoTranscodeConfig()
	config.OutputMode = "replace"
	temp, output, backup, err := videoTranscodePaths(task, config)
	if err != nil {
		t.Fatalf("build replacement paths: %v", err)
	}

	if filepath.Dir(temp) != filepath.Dir(task.SourcePath) ||
		filepath.Dir(output) != filepath.Dir(task.SourcePath) ||
		filepath.Dir(backup) != filepath.Dir(task.SourcePath) {
		t.Fatalf("all replacement files must stay beside source: %q, %q, %q", temp, output, backup)
	}
	if filepath.Ext(output) != ".mp4" || !strings.HasSuffix(temp, ".mp4.part") {
		t.Fatalf("unexpected output paths: %q, %q", output, temp)
	}
	nextTemp, _, nextBackup, err := videoTranscodePaths(task, config)
	if err != nil {
		t.Fatalf("build next replacement paths: %v", err)
	}
	if nextTemp == temp || nextBackup == backup {
		t.Fatal("each run must use unique temporary and backup paths")
	}
}

func TestVideoTranscodePathsUseNumberedNewFileWithoutBackup(t *testing.T) {
	dir := t.TempDir()
	task := &models.VideoTranscodeTask{ID: "task-1", SourcePath: filepath.Join(dir, "movie.mp4")}
	if err := os.WriteFile(filepath.Join(dir, "movie_转码.mp4"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	config := DefaultVideoTranscodeConfig()
	config.OutputMode = "new_file"
	_, output, backup, err := videoTranscodePaths(task, config)
	if err != nil {
		t.Fatalf("build new file paths: %v", err)
	}
	if filepath.Base(output) != "movie_转码_2.mp4" || backup != "" {
		t.Fatalf("unexpected new file paths: output=%q backup=%q", output, backup)
	}
}

func TestVideoTranscodePathsUseEditSuffixForEditedNewFile(t *testing.T) {
	dir := t.TempDir()
	task := &models.VideoTranscodeTask{
		ID: "task-1", SourcePath: filepath.Join(dir, "movie.mp4"), EditPlanJsonData: `{"version":1}`,
	}
	config := DefaultVideoTranscodeConfig()
	_, output, _, err := videoTranscodePaths(task, config)
	if err != nil {
		t.Fatalf("build edited output path: %v", err)
	}
	if filepath.Base(output) != "movie_剪辑.mp4" {
		t.Fatalf("unexpected edited output path: %q", output)
	}
}

func TestVideoTranscodePathsUseCustomNewFileName(t *testing.T) {
	dir := t.TempDir()
	task := &models.VideoTranscodeTask{ID: "task-1", SourcePath: filepath.Join(dir, "movie.mp4")}
	config := DefaultVideoTranscodeConfig()
	config.OutputMode = "new_file"
	config.OutputFileName = "我的成片"
	_, output, _, err := videoTranscodePaths(task, config)
	if err != nil {
		t.Fatalf("build custom output path: %v", err)
	}
	if filepath.Base(output) != "我的成片.mp4" {
		t.Fatalf("unexpected custom output path: %q", output)
	}
}

func TestValidateVideoTranscodeOutputFileNameRejectsUnsafeNames(t *testing.T) {
	for _, name := range []string{`..\outside`, "../outside", "CON", "trailing."} {
		if err := validateVideoTranscodeOutputFileName(name); err == nil {
			t.Fatalf("unsafe output file name must be rejected: %q", name)
		}
	}
	if err := validateVideoTranscodeOutputFileName("我的剪辑 01"); err != nil {
		t.Fatalf("normal output file name should pass: %v", err)
	}
}

func TestPublishVideoTranscodeOutputNeverOverwritesExistingFile(t *testing.T) {
	dir := t.TempDir()
	temporary := filepath.Join(dir, "temporary.part")
	output := filepath.Join(dir, "output.mp4")
	if err := os.WriteFile(temporary, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := publishVideoTranscodeOutput(temporary, output); err == nil {
		t.Fatal("publishing must reject an existing output file")
	}
	data, err := os.ReadFile(output)
	if err != nil || string(data) != "existing" {
		t.Fatalf("existing output was changed: %q, err=%v", data, err)
	}
}

func TestCountVideoTranscodeSubtitleStreams(t *testing.T) {
	info := processorsffmpeg.VideoFormatInfo{
		Streams: []processorsffmpeg.VideoStreamInfo{
			{CodecType: "video"},
			{CodecType: "subtitle"},
			{CodecType: "audio"},
			{CodecType: "subtitle"},
		},
	}
	if got := countVideoTranscodeStreams(info, "subtitle"); got != 2 {
		t.Fatalf("expected two subtitle streams, got %d", got)
	}
}

func TestHardwareVideoEncoderSelection(t *testing.T) {
	if got := hardwareVideoEncoder("h264", "nvenc", "libx264"); got != "h264_nvenc" {
		t.Fatalf("unexpected NVENC encoder: %s", got)
	}
	if got := hardwareVideoEncoder("h265", "qsv", "libx265"); got != "hevc_qsv" {
		t.Fatalf("unexpected QSV encoder: %s", got)
	}
	if got := hardwareVideoEncoder("h264", "", "libx264"); got != "libx264" {
		t.Fatalf("CPU fallback should be preserved: %s", got)
	}
}

func TestSelectVideoTranscodeGPUEncoder(t *testing.T) {
	capabilities := []processorsffmpeg.GPUEncoderCapability{
		{ID: "nvenc", VideoCodecs: []string{"h264", "h265"}},
		{ID: "qsv", VideoCodecs: []string{"h264"}},
	}
	config := DefaultVideoTranscodeConfig()
	if encoder, changed := selectVideoTranscodeGPUEncoder(config, capabilities); changed || encoder != "" {
		t.Fatalf("copy mode must not enable GPU: encoder=%q changed=%v", encoder, changed)
	}
	config.VideoCodec = "h264"
	if encoder, changed := selectVideoTranscodeGPUEncoder(config, capabilities); !changed || encoder != "nvenc" {
		t.Fatalf("first compatible GPU should be selected: encoder=%q changed=%v", encoder, changed)
	}
	config.GPUEncoder = "qsv"
	if encoder, changed := selectVideoTranscodeGPUEncoder(config, capabilities); changed || encoder != "qsv" {
		t.Fatalf("existing compatible GPU must be preserved: encoder=%q changed=%v", encoder, changed)
	}
	config.VideoCodec = "h265"
	if encoder, changed := selectVideoTranscodeGPUEncoder(config, capabilities); !changed || encoder != "nvenc" {
		t.Fatalf("incompatible existing GPU should be replaced: encoder=%q changed=%v", encoder, changed)
	}
}

func TestValidateVideoTranscodeRejectsGPUCopy(t *testing.T) {
	config := DefaultVideoTranscodeConfig()
	config.VideoCodec = "copy"
	config.GPUEncoder = "nvenc"
	if err := validateVideoTranscodeConfig(config); err == nil {
		t.Fatal("GPU encoder must be rejected when video stream is copied")
	}
}

func TestVideoTranscodeTemporaryPathValidation(t *testing.T) {
	if !isVideoTranscodeTemporaryPath(`D:\videos\.movie.cmtranscode-task-1-run.mp4.part`, "task-1") {
		t.Fatal("owned temporary path should be accepted")
	}
	if isVideoTranscodeTemporaryPath(`D:\videos\.movie.cmtranscode-task-2-run.mp4.part`, "task-1") {
		t.Fatal("another task's temporary path must not be accepted")
	}
	if isVideoTranscodeTemporaryPath(`D:\videos\movie.mp4.part`, "task-1") {
		t.Fatal("ordinary part file must not be treated as an owned temporary file")
	}
}

func TestCleanupVideoTranscodeTemporaryFileOnlyRemovesOwnedFile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "movie.mp4")
	owned := filepath.Join(dir, ".movie.cmtranscode-task-1-run.mp4.part")
	unrelated := filepath.Join(dir, "ordinary.mp4.part")
	if err := os.WriteFile(owned, []byte("owned"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unrelated, []byte("unrelated"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cleanupVideoTranscodeTemporaryFile(models.VideoTranscodeTask{
		ID: "task-1", SourcePath: source, TemporaryPath: owned,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(owned); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("owned temporary file must be removed: %v", err)
	}
	if err := cleanupVideoTranscodeTemporaryFile(models.VideoTranscodeTask{
		ID: "task-1", SourcePath: source, TemporaryPath: unrelated,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Fatalf("unrelated file must be preserved: %v", err)
	}
}

func TestDeleteDiscardableVideoTranscodeTasksByResourceIDs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.VideoTranscodeTask{}); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	statuses := map[string]string{
		"draft-task":   models.VideoTranscodeStatusDraft,
		"failed-task":  models.VideoTranscodeStatusFailed,
		"success-task": models.VideoTranscodeStatusSuccess,
		"queued-task":  models.VideoTranscodeStatusQueued,
	}
	paths := make(map[string]string, len(statuses))
	for id, status := range statuses {
		source := filepath.Join(dir, id+".mp4")
		temporary := filepath.Join(dir, "."+id+".cmtranscode-"+id+"-run.mp4.part")
		paths[id] = temporary
		if err := os.WriteFile(temporary, []byte(id), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&models.VideoTranscodeTask{
			ID: id, ResourceID: "resource-1", SourcePath: source,
			TemporaryPath: temporary, Status: status,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := deleteDiscardableVideoTranscodeTasksByResourceIDs(db, []string{"resource-1"}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"draft-task", "failed-task"} {
		var count int64
		if err := db.Model(&models.VideoTranscodeTask{}).Where("id = ?", id).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("discardable task %s must be deleted", id)
		}
		if _, err := os.Stat(paths[id]); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("discardable task temporary file %s must be removed: %v", id, err)
		}
	}
	for _, id := range []string{"success-task", "queued-task"} {
		var count int64
		if err := db.Model(&models.VideoTranscodeTask{}).Where("id = ?", id).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("task %s must be preserved", id)
		}
		if _, err := os.Stat(paths[id]); err != nil {
			t.Fatalf("preserved task temporary file %s must remain: %v", id, err)
		}
	}
}

func TestValidateVideoTranscodeSourceCompatibilityRejectsBitmapSubtitleInMP4(t *testing.T) {
	info := processorsffmpeg.VideoFormatInfo{
		Streams: []processorsffmpeg.VideoStreamInfo{
			{CodecType: "video", CodecName: "h264"},
			{CodecType: "subtitle", CodecName: "hdmv_pgs_subtitle"},
		},
	}
	config := DefaultVideoTranscodeConfig()
	if err := validateVideoTranscodeSourceCompatibility(info, config); err == nil {
		t.Fatal("MP4 output must reject bitmap subtitles before running FFmpeg")
	}
	config.Container = "mkv"
	if err := validateVideoTranscodeSourceCompatibility(info, config); err != nil {
		t.Fatalf("MKV should preserve bitmap subtitles: %v", err)
	}
}

func TestValidateVideoTranscodeOutputInfoChecksStreamsAndTargets(t *testing.T) {
	source := processorsffmpeg.VideoFormatInfo{
		Streams: []processorsffmpeg.VideoStreamInfo{
			{CodecType: "video", CodecName: "h264", Width: 1920, Height: 1080, AverageFrameRate: "30/1"},
			{CodecType: "audio", CodecName: "aac"},
			{CodecType: "subtitle", CodecName: "subrip"},
		},
	}
	source.Format.Duration = "60"
	output := processorsffmpeg.VideoFormatInfo{
		Streams: []processorsffmpeg.VideoStreamInfo{
			{CodecType: "video", CodecName: "h264", Width: 1280, Height: 720, AverageFrameRate: "24/1"},
			{CodecType: "audio", CodecName: "aac"},
			{CodecType: "subtitle", CodecName: "mov_text"},
		},
	}
	output.Format.Duration = "60"
	config := DefaultVideoTranscodeConfig()
	config.VideoCodec = "h264"
	config.AudioCodec = "aac"
	config.ResolutionHeight = 720
	config.FrameRate = 24
	if err := validateVideoTranscodeOutputInfo(source, output, 60, config, false); err != nil {
		t.Fatalf("matching output should pass validation: %v", err)
	}

	missingAudio := output
	missingAudio.Streams = []processorsffmpeg.VideoStreamInfo{
		output.Streams[0],
		output.Streams[2],
	}
	if err := validateVideoTranscodeOutputInfo(source, missingAudio, 60, config, false); err == nil {
		t.Fatal("missing audio stream must be rejected")
	}

	wrongFrameRate := output
	wrongFrameRate.Streams = append([]processorsffmpeg.VideoStreamInfo(nil), output.Streams...)
	wrongFrameRate.Streams[0].AverageFrameRate = "30/1"
	if err := validateVideoTranscodeOutputInfo(source, wrongFrameRate, 60, config, false); err == nil {
		t.Fatal("unexpected frame rate must be rejected")
	}
}

func TestNormalizeVideoEditPlanSupportsSplitDeleteAndReorder(t *testing.T) {
	plan := VideoEditPlan{Version: 1, Segments: []VideoEditSegment{
		{ID: "third", Start: 40, End: 50},
		{ID: "first", Start: 0, End: 10},
	}}
	normalized, duration, hasEdits, err := normalizeVideoEditPlan(plan, 60)
	if err != nil {
		t.Fatalf("normalize edit plan: %v", err)
	}
	if !hasEdits || duration != 20 || normalized.Segments[0].ID != "third" {
		t.Fatalf("unexpected normalized plan: %#v, duration=%v", normalized, duration)
	}
}

func TestNormalizeVideoEditPlanRejectsOverlap(t *testing.T) {
	plan := VideoEditPlan{Version: 1, Segments: []VideoEditSegment{
		{ID: "one", Start: 0, End: 10},
		{ID: "two", Start: 9, End: 20},
	}}
	if _, _, _, err := normalizeVideoEditPlan(plan, 60); err == nil {
		t.Fatal("overlapping source segments must be rejected")
	}
}

func TestVideoTranscodeConfigForEditAppliesSafeDefaults(t *testing.T) {
	config := DefaultVideoTranscodeConfig()
	adjustedConfig, adjusted := videoTranscodeConfigForEdit(config)
	if !adjusted || adjustedConfig.VideoCodec != "h264" || adjustedConfig.AudioCodec != "aac" {
		t.Fatalf("unexpected adjusted config: %#v, adjusted=%v", adjustedConfig, adjusted)
	}
}

func TestVideoTranscodeConfigForEditPreservesSelectedVideoEncoder(t *testing.T) {
	config := DefaultVideoTranscodeConfig()
	config.VideoCodec = "h265"
	config.AudioCodec = "aac"
	config.GPUEncoder = "nvenc"
	adjustedConfig, adjusted := videoTranscodeConfigForEdit(config)
	if adjusted || adjustedConfig.VideoCodec != "h265" || adjustedConfig.GPUEncoder != "nvenc" {
		t.Fatalf("existing edit-compatible config must be preserved: %#v, adjusted=%v", adjustedConfig, adjusted)
	}
}

func TestBuildVideoEditFilterGraphMapsVideoAndEveryAudioTrack(t *testing.T) {
	info := processorsffmpeg.VideoFormatInfo{Streams: []processorsffmpeg.VideoStreamInfo{
		{Index: 0, CodecType: "video"},
		{Index: 1, CodecType: "audio"},
		{Index: 2, CodecType: "audio"},
	}}
	plan := VideoEditPlan{Version: 1, Segments: []VideoEditSegment{
		{ID: "two", Start: 10, End: 20},
		{ID: "one", Start: 0, End: 5},
	}}
	config := DefaultVideoTranscodeConfig()
	config.VideoCodec = "h264"
	config.AudioCodec = "aac"
	graph, labels := buildVideoEditFilterGraph(0, info, plan, config)
	for _, expected := range []string{"trim=start=10.000:end=20.000", "concat=n=2:v=1:a=0", "[0:1]atrim", "[0:2]atrim"} {
		if !strings.Contains(graph, expected) {
			t.Fatalf("filter graph missing %q: %s", expected, graph)
		}
	}
	if len(labels) != 3 || labels[0] != "[vout]" {
		t.Fatalf("unexpected output labels: %#v", labels)
	}
}
