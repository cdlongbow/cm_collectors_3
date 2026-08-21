<template>
  <el-dialog
    v-model="visible"
    class="duplicate-compare-dialog"
    width="96vw"
    top="2vh"
    append-to-body
    destroy-on-close
    :close-on-click-modal="false"
    @close="handleClose"
    @closed="handleClosed"
  >
    <template #header>
      <div class="compare-header">
        <div>
          <strong>{{ title }}</strong>
          <span>同时对比 {{ entries.length }} 条视频</span>
        </div>
        <el-text v-if="truncatedCount > 0" type="warning">
          本组还有 {{ truncatedCount }} 条未加载；可在下方改选，单次最多对比 4 条。
        </el-text>
      </div>
      <div v-if="allEntries.length > maxPlayers" class="compare-picker">
        <el-select v-model="pendingCompareIds" multiple collapse-tags collapse-tags-tooltip
          :multiple-limit="maxPlayers" placeholder="选择 2～4 条参与同屏复核">
          <el-option v-for="entry in allEntries" :key="entry.drama_series_id"
            :label="entry.resource_title || entry.src" :value="entry.drama_series_id" />
        </el-select>
        <el-button size="small" type="primary" @click="applyCompareSelection">加载所选视频</el-button>
      </div>
    </template>

    <div class="compare-body" v-loading="loading">
      <div class="compare-grid" :class="`count-${entries.length}`">
        <article v-for="entry in entries" :key="entry.drama_series_id" class="compare-tile"
          :class="{ reference: audioSourceId === entry.drama_series_id }">
          <div class="tile-video">
            <videoPlay
              :ref="(instance) => setPlayerRef(entry.drama_series_id, instance)"
              :use-video-play-controls="false"
              :hide-controls="true"
              fit-container
            />
            <div v-if="errors[entry.drama_series_id]" class="tile-error">
              {{ errors[entry.drama_series_id] }}
            </div>
          </div>
          <div class="tile-info">
            <div class="tile-title" :title="entry.resource_title">{{ entry.resource_title }}</div>
            <div class="tile-path" :title="entry.src">{{ entry.src }}</div>
            <div class="tile-actions">
              <el-radio :model-value="audioSourceId" :value="entry.drama_series_id"
                :disabled="!canSelectReference(entry.drama_series_id)"
                @change="setAudioSource(entry.drama_series_id)">
                参考 / 声音
              </el-radio>
              <el-checkbox :model-value="markedDeleteIds.has(entry.drama_series_id)"
                @change="toggleDelete(entry)">
                标记删除
              </el-checkbox>
            </div>
          </div>
        </article>
      </div>

      <div class="compare-console">
        <div class="console-row console-modes">
          <el-radio-group v-model="syncMode" size="small" @change="handleSyncModeChange">
            <el-radio-button label="按时间同步" value="time" />
            <el-radio-button label="按比例同步" value="ratio" />
          </el-radio-group>
          <el-text type="info">播放中偏差超过 0.35 秒会自动校正；某一路缓冲时可手动暂停后重新同步。</el-text>
        </div>

        <div class="console-row timeline-row">
          <el-button size="small" @click="jump(-5)">-5 秒</el-button>
          <el-button size="small" @click="jump(-1)">-1 秒</el-button>
          <el-button type="primary" circle :loading="startingPlayback"
            :icon="playing ? VideoPause : VideoPlay" @click="togglePlayback" />
          <el-button size="small" @click="jump(1)">+1 秒</el-button>
          <el-button size="small" @click="jump(5)">+5 秒</el-button>
          <span class="time-label">{{ timelineText }}</span>
          <el-slider
            v-model="sliderValue"
            class="master-slider"
            :min="0"
            :max="sliderMax"
            :step="syncMode === 'ratio' ? 0.1 : 0.1"
            :show-tooltip="false"
            @mousedown="beginSeek"
            @touchstart="beginSeek"
            @input="handleSliderInput"
            @change="finishSeek"
          />
          <el-button size="small" @click="syncToSlider">重新同步</el-button>
        </div>

        <div class="console-row console-footer">
          <div class="volume-control">
            <span>参考音量</span>
            <el-slider v-model="volume" :min="0" :max="1" :step="0.05" @input="applyVolumes" />
          </div>
          <div class="footer-actions">
            <el-text v-if="markedDeleteIds.size" type="warning">已标记 {{ markedDeleteIds.size }} 条待删除</el-text>
            <el-button @click="visible = false">完成复核</el-button>
          </div>
        </div>
      </div>
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import videoPlay from '@/components/play/videoPlay.vue';
import { getPlayVideoURLAndType } from '@/common/play';
import type { I_DuplicateGroup, I_DuplicateItem } from '@/dataType/videoFingerprint.dataType';
import { VideoPause, VideoPlay } from '@element-plus/icons-vue';
import { ElMessage } from 'element-plus';
import type { ComponentPublicInstance } from 'vue';
import { computed, nextTick, reactive, ref } from 'vue';

type PlayerInstance = InstanceType<typeof videoPlay>;
type SyncMode = 'time' | 'ratio';

const emit = defineEmits<{
  (event: 'selectionChange', payload: { items: I_DuplicateItem[]; selectedIds: string[] }): void;
}>();

const maxPlayers = 4;
const videoLoadTimeoutMs = 15000;
const playbackStartTimeoutMs = 8000;
const visible = ref(false);
const loading = ref(false);
const title = ref('重复视频同屏复核');
const allEntries = ref<I_DuplicateItem[]>([]);
const entries = ref<I_DuplicateItem[]>([]);
const pendingCompareIds = ref<string[]>([]);
const totalInGroup = ref(0);
const playerRefs = new Map<string, PlayerInstance>();
const durations = reactive<Record<string, number>>({});
const errors = reactive<Record<string, string>>({});
const readyIds = ref(new Set<string>());
const activePlayingIds = ref(new Set<string>());
const markedDeleteIds = ref(new Set<string>());
const audioSourceId = ref('');
const syncMode = ref<SyncMode>('time');
const sliderValue = ref(0);
const playing = ref(false);
const startingPlayback = ref(false);
const seeking = ref(false);
const resumeAfterSeek = ref(false);
const volume = ref(0.8);
let monitorTimer: number | undefined;
let seekFrame: number | undefined;
let openVersion = 0;
let playbackAttempt = 0;
const loadTimeouts = new Map<string, number>();

const truncatedCount = computed(() => Math.max(0, totalInGroup.value - entries.value.length));
const availableDurations = computed(() => entries.value
  .map((entry) => durations[entry.drama_series_id] || entry.duration || 0)
  .filter((duration) => Number.isFinite(duration) && duration > 0));
const maxDuration = computed(() => Math.max(0, ...availableDurations.value));
const sliderMax = computed(() => syncMode.value === 'ratio' ? 100 : Math.max(1, maxDuration.value));

const formatTime = (value: number) => {
  const safeValue = Number.isFinite(value) ? Math.max(0, value) : 0;
  const totalSeconds = Math.floor(safeValue);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  const tenths = Math.floor((safeValue - totalSeconds) * 10);
  const base = hours > 0
    ? `${hours}:${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
    : `${minutes}:${String(seconds).padStart(2, '0')}`;
  return `${base}.${tenths}`;
};

const timelineText = computed(() => syncMode.value === 'ratio'
  ? `${sliderValue.value.toFixed(1)}%`
  : `${formatTime(sliderValue.value)} / ${formatTime(maxDuration.value)}`);

const setPlayerRef = (id: string, instance: Element | ComponentPublicInstance | null) => {
  if (instance) {
    playerRefs.set(id, instance as PlayerInstance);
  } else {
    playerRefs.delete(id);
  }
};

const durationFor = (entry: I_DuplicateItem) => durations[entry.drama_series_id] || entry.duration || 0;

const targetTimeFor = (entry: I_DuplicateItem, value = sliderValue.value) => {
  const duration = durationFor(entry);
  if (syncMode.value === 'ratio') {
    return duration > 0 ? Math.min(duration, Math.max(0, value / 100 * duration)) : 0;
  }
  return duration > 0 ? Math.min(duration, Math.max(0, value)) : Math.max(0, value);
};

const pauseAll = () => {
  playbackAttempt++;
  startingPlayback.value = false;
  playerRefs.forEach((player) => player.pause());
  activePlayingIds.value = new Set();
  playing.value = false;
};

const syncPlayers = (force = false) => {
  for (const entry of entries.value) {
    const player = playerRefs.get(entry.drama_series_id);
    if (!player || errors[entry.drama_series_id]) continue;
    if (playing.value && !activePlayingIds.value.has(entry.drama_series_id)) continue;
    const target = targetTimeFor(entry);
    const current = player.getCurrentTime();
    if (force || Math.abs(current - target) > 0.35) {
      player.setCurrentTime(target);
    }
  }
};

const syncToSlider = () => syncPlayers(true);

const handleSyncModeChange = () => {
  const referenceEntry = entries.value.find((entry) => entry.drama_series_id === audioSourceId.value)
    || entries.value[0];
  const referencePlayer = referenceEntry ? playerRefs.get(referenceEntry.drama_series_id) : undefined;
  const current = referencePlayer?.getCurrentTime() || 0;
  const duration = referenceEntry
    ? durationFor(referenceEntry) || referencePlayer?.getDuration() || 0
    : 0;
  sliderValue.value = syncMode.value === 'ratio' && duration > 0 ? current / duration * 100 : current;
  syncPlayers(true);
};

const applyVolumes = () => {
  playerRefs.forEach((player, id) => player.setVolume(id === audioSourceId.value ? volume.value : 0, false));
};

const availableReferenceIds = () => playing.value ? activePlayingIds.value : readyIds.value;

const canSelectReference = (id: string) => availableReferenceIds().has(id);

const ensureReferenceAvailable = () => {
  const candidateIds = availableReferenceIds();
  if (candidateIds.has(audioSourceId.value)) return true;
  const fallback = entries.value.find((entry) => candidateIds.has(entry.drama_series_id));
  audioSourceId.value = fallback?.drama_series_id || '';
  applyVolumes();
  if (!fallback && playing.value) {
    pauseAll();
    ElMessage.warning('所有视频均已失效，已停止同步播放');
  }
  return Boolean(fallback);
};

const setAudioSource = (id: string) => {
  if (!canSelectReference(id)) return;
  audioSourceId.value = id;
  applyVolumes();
};

const toggleDelete = (entry: I_DuplicateItem) => {
  const next = new Set(markedDeleteIds.value);
  if (next.has(entry.drama_series_id)) {
    next.delete(entry.drama_series_id);
  } else {
    if (next.size >= allEntries.value.length - 1) {
      ElMessage.warning('同一重复组内至少保留一个分集');
      return;
    }
    next.add(entry.drama_series_id);
  }
  markedDeleteIds.value = next;
  emit('selectionChange', { items: entries.value, selectedIds: [...next] });
};

const startPlayback = async () => {
  if (startingPlayback.value) return;
  const pendingCount = entries.value.filter((entry) =>
    !readyIds.value.has(entry.drama_series_id) && !errors[entry.drama_series_id]
  ).length;
  if (pendingCount > 0) {
    ElMessage.warning(`还有 ${pendingCount} 路视频正在加载，请稍后再试`);
    return;
  }
  if (!ensureReferenceAvailable()) {
    ElMessage.warning('当前没有加载成功的视频');
    return;
  }
  const playableEntries = entries.value.filter((entry) =>
    readyIds.value.has(entry.drama_series_id) && !errors[entry.drama_series_id]
  );
  if (playableEntries.length === 0) {
    ElMessage.warning('当前没有可播放的视频');
    return;
  }
  const attempt = ++playbackAttempt;
  startingPlayback.value = true;
  activePlayingIds.value = new Set();
  syncPlayers(true);
  const playWithTimeout = async (entry: I_DuplicateItem) => {
    const player = playerRefs.get(entry.drama_series_id);
    if (!player) return false;
    let timeoutId: number | undefined;
    const timeoutResult = new Promise<boolean>((resolve) => {
      timeoutId = window.setTimeout(() => resolve(false), playbackStartTimeoutMs);
    });
    const started = await Promise.race([player.play(), timeoutResult]);
    if (timeoutId !== undefined) window.clearTimeout(timeoutId);
    if (!started) player.pause();
    return started;
  };
  const results = await Promise.all(playableEntries.map(async (entry) => {
    const started = await playWithTimeout(entry);
    const player = playerRefs.get(entry.drama_series_id);
    if (attempt !== playbackAttempt) {
      if (started) player?.pause();
      return { id: entry.drama_series_id, started: false };
    }
    if (started) {
      if (playing.value) player?.setCurrentTime(targetTimeFor(entry));
      const nextActiveIds = new Set(activePlayingIds.value);
      nextActiveIds.add(entry.drama_series_id);
      activePlayingIds.value = nextActiveIds;
      if (!playing.value) {
        playing.value = true;
        startingPlayback.value = false;
      }
      ensureReferenceAvailable();
    }
    return { id: entry.drama_series_id, started };
  }));
  if (attempt !== playbackAttempt) return;
  startingPlayback.value = false;
  const startedIds = results
    .filter((result) => result.started && activePlayingIds.value.has(result.id))
    .map((result) => result.id);
  if (startedIds.length === 0) {
    playing.value = false;
    ElMessage.warning('视频播放启动失败，请检查视频加载状态');
    return;
  }
  if (startedIds.length < playableEntries.length) {
    ElMessage.warning(`${playableEntries.length - startedIds.length} 路视频未能启动，已继续播放其余视频`);
  }
  if (!startedIds.includes(audioSourceId.value)) ensureReferenceAvailable();
};

const togglePlayback = async () => {
  if (playing.value) {
    pauseAll();
    return;
  }
  await startPlayback();
};

const beginSeek = () => {
  if (seeking.value) return;
  seeking.value = true;
  resumeAfterSeek.value = playing.value;
  pauseAll();
};

const scheduleSeek = () => {
  if (seekFrame !== undefined) window.cancelAnimationFrame(seekFrame);
  seekFrame = window.requestAnimationFrame(() => {
    seekFrame = undefined;
    syncPlayers(true);
  });
};

const handleSliderInput = () => {
  if (!seeking.value) beginSeek();
  scheduleSeek();
};

const finishSeek = () => {
  syncPlayers(true);
  seeking.value = false;
  if (resumeAfterSeek.value) {
    void startPlayback();
  }
  resumeAfterSeek.value = false;
};

const jump = (seconds: number) => {
  if (syncMode.value === 'ratio') {
    const referenceEntry = entries.value.find((entry) => entry.drama_series_id === audioSourceId.value)
      || entries.value[0];
    const referencePlayer = referenceEntry ? playerRefs.get(referenceEntry.drama_series_id) : undefined;
    const duration = referenceEntry
      ? durationFor(referenceEntry) || referencePlayer?.getDuration() || 1
      : 1;
    sliderValue.value = Math.min(100, Math.max(0, sliderValue.value + seconds / duration * 100));
  } else {
    sliderValue.value = Math.min(sliderMax.value, Math.max(0, sliderValue.value + seconds));
  }
  syncPlayers(true);
};

const updateFromReference = () => {
  if (!playing.value || seeking.value) return;
  const referenceEntry = entries.value.find((entry) => entry.drama_series_id === audioSourceId.value)
    || entries.value[0];
  if (!referenceEntry) return;
  const referencePlayer = playerRefs.get(referenceEntry.drama_series_id);
  if (!referencePlayer || errors[referenceEntry.drama_series_id]
    || !activePlayingIds.value.has(referenceEntry.drama_series_id)) {
    ensureReferenceAvailable();
    return;
  }
  const current = referencePlayer.getCurrentTime();
  const duration = durationFor(referenceEntry) || referencePlayer.getDuration();
  sliderValue.value = syncMode.value === 'ratio' && duration > 0 ? current / duration * 100 : current;
  syncPlayers(false);
  if (duration > 0 && current >= duration - 0.05) pauseAll();
};

const loadEntry = async (entry: I_DuplicateItem, version: number) => {
  const player = playerRefs.get(entry.drama_series_id);
  if (!player) return;
  let settled = false;
  const clearLoadTimeout = () => {
    const timeoutId = loadTimeouts.get(entry.drama_series_id);
    if (timeoutId !== undefined) window.clearTimeout(timeoutId);
    loadTimeouts.delete(entry.drama_series_id);
  };
  const markLoadError = (message: string) => {
    if (settled || version !== openVersion || !visible.value) return;
    settled = true;
    clearLoadTimeout();
    errors[entry.drama_series_id] = message;
    const nextReadyIds = new Set(readyIds.value);
    nextReadyIds.delete(entry.drama_series_id);
    readyIds.value = nextReadyIds;
    const nextActiveIds = new Set(activePlayingIds.value);
    nextActiveIds.delete(entry.drama_series_id);
    activePlayingIds.value = nextActiveIds;
    player.releaseSource();
    if (playing.value) ensureReferenceAvailable();
  };
  loadTimeouts.set(entry.drama_series_id, window.setTimeout(() => {
    markLoadError('视频加载超时');
  }, videoLoadTimeoutMs));
  try {
    const source = await getPlayVideoURLAndType(entry.drama_series_id);
    if (settled || version !== openVersion || !visible.value) return;
    if (!source.playUrl) throw new Error('无法获取播放地址');
    player.setVideoSource(source.playUrl, source.playType, () => {
      if (settled || version !== openVersion || !visible.value) return;
      settled = true;
      clearLoadTimeout();
      durations[entry.drama_series_id] = player.getDuration() || entry.duration || 0;
      const nextReadyIds = new Set(readyIds.value);
      nextReadyIds.add(entry.drama_series_id);
      readyIds.value = nextReadyIds;
      if (!audioSourceId.value || errors[audioSourceId.value]) ensureReferenceAvailable();
      applyVolumes();
      player.setCurrentTime(targetTimeFor(entry));
    }, entry.src || entry.resource_title, 0, () => {
      markLoadError('视频加载失败');
    });
  } catch (error) {
    markLoadError(error instanceof Error ? error.message : '视频加载失败');
  }
};

const releasePlayers = () => {
  pauseAll();
  loadTimeouts.forEach((timeoutId) => window.clearTimeout(timeoutId));
  loadTimeouts.clear();
  playerRefs.forEach((player) => player.releaseSource());
  playerRefs.clear();
};

const handleClose = () => {
  openVersion++;
  if (monitorTimer !== undefined) window.clearInterval(monitorTimer);
  monitorTimer = undefined;
  if (seekFrame !== undefined) window.cancelAnimationFrame(seekFrame);
  seekFrame = undefined;
  releasePlayers();
};

const handleClosed = () => {
  loading.value = false;
  allEntries.value = [];
  entries.value = [];
};

const loadCurrentEntries = async (version: number) => {
  await nextTick();
  if (version !== openVersion || !visible.value) return;
  await Promise.all(entries.value.map((entry) => loadEntry(entry, version)));
  if (version !== openVersion || !visible.value) return;
  loading.value = false;
  if (monitorTimer !== undefined) window.clearInterval(monitorTimer);
  monitorTimer = window.setInterval(updateFromReference, 300);
};

const applyCompareSelection = async () => {
  if (pendingCompareIds.value.length < 2) {
    ElMessage.warning('请至少选择两条视频进行同屏复核');
    return;
  }
  const version = ++openVersion;
  if (monitorTimer !== undefined) window.clearInterval(monitorTimer);
  monitorTimer = undefined;
  releasePlayers();
  Object.keys(durations).forEach((key) => delete durations[key]);
  Object.keys(errors).forEach((key) => delete errors[key]);
  readyIds.value = new Set();
  activePlayingIds.value = new Set();
  const selectedIds = new Set(pendingCompareIds.value);
  entries.value = allEntries.value.filter((entry) => selectedIds.has(entry.drama_series_id));
  if (!selectedIds.has(audioSourceId.value)) audioSourceId.value = entries.value[0]?.drama_series_id || '';
  sliderValue.value = 0;
  playing.value = false;
  loading.value = true;
  await loadCurrentEntries(version);
};

const open = async (group: I_DuplicateGroup, groupNumber: number, selectedIds: string[] = []) => {
  const version = ++openVersion;
  releasePlayers();
  Object.keys(durations).forEach((key) => delete durations[key]);
  Object.keys(errors).forEach((key) => delete errors[key]);
  readyIds.value = new Set();
  activePlayingIds.value = new Set();
  totalInGroup.value = group.items.length;
  allEntries.value = [...group.items];
  entries.value = allEntries.value.slice(0, maxPlayers);
  pendingCompareIds.value = entries.value.map((entry) => entry.drama_series_id);
  title.value = `重复组 #${groupNumber} · 同屏复核`;
  markedDeleteIds.value = new Set(selectedIds.filter((id) => allEntries.value.some((entry) => entry.drama_series_id === id)));
  audioSourceId.value = entries.value[0]?.drama_series_id || '';
  syncMode.value = 'time';
  sliderValue.value = 0;
  playing.value = false;
  loading.value = true;
  visible.value = true;
  await loadCurrentEntries(version);
};

defineExpose({ open });
</script>

<style lang="scss">
.duplicate-compare-dialog {
  height: 96vh;
  margin: 0 auto;
  display: flex;
  flex-direction: column;

  .el-dialog__header {
    padding-bottom: 8px;
  }

  .el-dialog__body {
    flex: 1;
    min-height: 0;
    padding-top: 0;
  }

  .compare-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 16px;

    strong {
      font-size: 16px;
    }

    span {
      margin-left: 12px;
      color: var(--el-text-color-secondary);
    }
  }

  .compare-picker {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 8px;

    .el-select {
      width: min(680px, 75vw);
    }
  }

  .compare-body {
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .compare-grid {
    flex: 1;
    min-height: 0;
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;

    &.count-1 {
      grid-template-columns: minmax(0, 1fr);
    }
  }

  .compare-tile {
    min-height: 0;
    border: 1px solid var(--el-border-color);
    border-radius: 6px;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    background: #080808;

    &.reference {
      border-color: var(--el-color-primary);
      box-shadow: inset 0 0 0 1px var(--el-color-primary);
    }
  }

  .tile-video {
    flex: 1;
    min-height: 120px;
    position: relative;
    overflow: hidden;

    .video-player-container,
    .video-player-windows,
    .video-js {
      width: 100%;
      height: 100%;
    }

    .vjs-control-bar,
    .center-play-button {
      display: none !important;
    }
  }

  .tile-error {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    color: var(--el-color-danger);
    background: rgba(0, 0, 0, 0.75);
  }

  .tile-info {
    padding: 6px 8px;
    background: var(--el-bg-color);
  }

  .tile-title,
  .tile-path {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .tile-title {
    font-weight: 600;
  }

  .tile-path {
    margin-top: 2px;
    color: var(--el-text-color-secondary);
    font-size: 12px;
  }

  .tile-actions {
    display: flex;
    justify-content: space-between;
    margin-top: 4px;
  }

  .compare-console {
    flex-shrink: 0;
    border: 1px solid var(--el-border-color);
    border-radius: 6px;
    padding: 8px 12px;
    background: var(--el-bg-color);
  }

  .console-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .console-modes {
    margin-bottom: 6px;
    justify-content: space-between;
  }

  .timeline-row {
    .time-label {
      min-width: 118px;
      text-align: center;
      font-variant-numeric: tabular-nums;
    }

    .master-slider {
      flex: 1;
      min-width: 180px;
    }
  }

  .console-footer {
    justify-content: space-between;
    margin-top: 4px;
  }

  .volume-control {
    width: 260px;
    display: grid;
    grid-template-columns: 72px 1fr;
    align-items: center;
    gap: 8px;
  }

  .footer-actions {
    display: flex;
    align-items: center;
    gap: 12px;
  }
}

@media (max-width: 900px) {
  .duplicate-compare-dialog {
    .compare-grid {
      grid-template-columns: minmax(0, 1fr);
      overflow: auto;
    }

    .compare-tile {
      min-height: 300px;
    }

    .timeline-row {
      flex-wrap: wrap;

      .master-slider {
        flex-basis: 100%;
        order: 2;
      }
    }
  }
}
</style>
