<template>
  <el-dialog
    :model-value="modelValue"
    fullscreen
    destroy-on-close
    class="video-edit-dialog"
    :before-close="beforeClose"
    @opened="initializeEditor"
    @closed="handleClosed"
  >
    <template #header>
      <div class="editor-header">
        <div>
          <strong>视频剪辑</strong>
          <span>{{ task.resourceTitle }} · {{ fileName(task.sourcePath) }}</span>
        </div>
        <div class="duration-summary">
          原始 {{ formatTime(sourceDuration) }} → 保留 {{ formatTime(editedDuration) }}
        </div>
      </div>
    </template>

    <div class="editor-layout" v-loading="initializing">
      <section class="preview-panel">
        <div class="preview-stage">
          <videoPlay v-show="!transitionPreviewing" ref="videoPlayRef" class="editor-player" fit-container hide-controls />
          <video
            v-show="transitionPreviewing"
            ref="transitionVideoRef"
            class="transition-player"
            :src="transitionPreviewUrl"
            playsinline
            @play="isPlaying = true"
            @pause="isPlaying = false"
            @timeupdate="handleTransitionPreviewTimeUpdate"
            @ended="finishTransitionPreview"
          />
        </div>
        <div class="player-controls">
          <el-button
            circle
            :icon="isPlaying ? VideoPause : VideoPlay"
            :title="isPlaying ? '暂停' : '播放'"
            :loading="transitionPreviewLoading"
            :disabled="initializing || !segments.length"
            @click="togglePlayback"
          />
          <el-button
            circle
            :icon="muted || volume === 0 ? Mute : Microphone"
            :title="muted || volume === 0 ? '取消静音' : '静音'"
            @click="toggleMute"
          />
          <el-slider
            class="volume-slider"
            :model-value="muted ? 0 : volume"
            :min="0"
            :max="1"
            :step="0.01"
            :show-tooltip="false"
            @input="setVolume"
          />
          <span class="time-label">{{ formatTime(currentOutputTime) }} / {{ formatTime(editedDuration) }}</span>
          <el-slider
            class="output-slider"
            :model-value="currentOutputTime"
            :min="0"
            :max="Math.max(editedDuration, 0.01)"
            :step="0.01"
            :show-tooltip="false"
            :disabled="initializing || !segments.length"
            @input="seekOutputTime"
          />
          <div class="preview-status">
          <span>片段 {{ activeSegmentIndex + 1 }}/{{ segments.length }}</span>
          <span class="preview-tip">
            {{ transitionPreviewing ? '正在预览片段切点转场' : '当前预览为保留片段按现有顺序拼接后的效果' }}
          </span>
          </div>
        </div>
      </section>

      <section class="timeline-panel">
        <div class="timeline-toolbar">
          <el-button type="primary" :disabled="!canSplit" @click="splitCurrentSegment">
            在播放头处分割
          </el-button>
          <el-button type="danger" plain :disabled="segments.length <= 1" @click="removeCurrentSegment">
            删除片段
          </el-button>
          <el-button :disabled="!undoStack.length" @click="undo">撤销</el-button>
          <el-button :disabled="!redoStack.length" @click="redo">重做</el-button>
          <el-button @click="resetPlan">恢复完整视频</el-button>
          <el-button-group class="timeline-zoom-controls">
            <el-button :icon="ZoomOut" title="缩小时间轴" @click="zoomTimeline(1 / 1.25)" />
            <el-button title="让全部片段适配当前宽度" @click="fitTimeline">适配</el-button>
            <el-button :icon="ZoomIn" title="放大时间轴" @click="zoomTimeline(1.25)" />
          </el-button-group>
          <small class="zoom-value">{{ timelinePixelsPerSecond.toFixed(1) }} px/秒</small>
          <span>拖动片段可以调整成片顺序，点击片段可以定位播放。</span>
        </div>

        <div class="timeline-ruler">
          <span>{{ formatTime(timelineVisibleStart) }}</span>
          <span>{{ formatTime(timelineVisibleMiddle) }}</span>
          <span>{{ formatTime(timelineVisibleEnd) }}</span>
        </div>
        <div class="timeline-viewport-shell">
          <div
            v-if="segments.length"
            ref="timelineViewportRef"
            class="timeline-viewport"
            @wheel="onTimelineWheel"
            @scroll="updateTimelineRuler"
          >
            <div ref="timelineTrackRef" class="timeline-track">
              <template v-for="(segment, index) in segments" :key="segment.id">
                <div
                  class="timeline-segment"
                  :class="{ active: segment.id === activeSegmentId, dragging: dragIndex === index }"
                  :style="segmentStyle(segment)"
                  :data-segment-id="segment.id"
                  draggable="true"
                  @click="seekWithinSegment($event, segment)"
                  @dragstart="startDrag($event, index)"
                  @dragover.prevent
                  @drop.prevent="dropSegment(index)"
                  @dragend="dragIndex = -1"
                >
                  <div class="segment-thumbnails">
                    <img
                      v-for="frame in framesForSegment(segment)"
                      :key="frame.time"
                      :src="frame.url"
                      draggable="false"
                    />
                    <div v-if="!framesForSegment(segment).length" class="thumbnail-placeholder">加载缩略图</div>
                  </div>
                  <div class="segment-label">
                    <strong>{{ index + 1 }}</strong>
                    <span>{{ formatTime(segment.start) }} - {{ formatTime(segment.end) }}</span>
                    <span>{{ formatTime(segment.end - segment.start) }}</span>
                  </div>
                </div>

                <el-popover
                  v-if="index < segments.length - 1"
                  placement="top"
                  :width="300"
                  trigger="click"
                >
                  <div class="transition-editor" @click.stop>
                    <strong>片段 {{ index + 1 }} → {{ index + 2 }} 的转场</strong>
                    <el-select
                      :model-value="segment.transition?.type || ''"
                      placeholder="无转场"
                      clearable
                      :teleported="false"
                      @change="setTransitionType(index, String($event || ''))"
                    >
                      <el-option
                        v-for="effect in transitionEffects"
                        :key="effect.value"
                        :label="effect.label"
                        :value="effect.value"
                      />
                    </el-select>
                    <div v-if="segment.transition" class="transition-duration-row">
                      <span>时长</span>
                      <el-input-number
                        :model-value="segment.transition.duration"
                        :min="0.1"
                        :max="transitionMaxDuration(index)"
                        :step="0.1"
                        :precision="1"
                        controls-position="right"
                        @change="setTransitionDuration(index, Number($event))"
                      />
                      <span>秒</span>
                    </div>
                    <div v-if="segment.transition" class="transition-audio-row">
                      <span>音频淡出淡入</span>
                      <el-switch
                        :model-value="!!segment.transition.audioFade"
                        @change="setTransitionAudioFade(index, Boolean($event))"
                      />
                      <small>默认保持原音量，开启后在切点前后各淡化一半时长。</small>
                    </div>
                    <el-button
                      v-if="segment.transition"
                      type="primary"
                      plain
                      :loading="transitionPreviewLoading && transitionPreviewIndex === index"
                      @click="playTransitionPreview(index, false)"
                    >
                      预览转场
                    </el-button>
                  </div>
                  <template #reference>
                    <button
                      class="transition-node"
                      :class="{ active: !!segment.transition }"
                      type="button"
                      @click.stop
                    >
                      <span>{{ transitionShortLabel(segment) }}</span>
                      <small v-if="segment.transition">{{ segment.transition.duration.toFixed(1) }}s</small>
                    </button>
                  </template>
                </el-popover>
              </template>
              <div
                class="timeline-playhead"
                :class="{ dragging: draggingPlayhead }"
                :style="{ left: `${playheadLeft}px` }"
                title="拖动调整播放位置"
                @pointerdown.stop.prevent="startPlayheadDrag"
              >
                <div class="playhead-handle" />
              </div>
            </div>
          </div>
          <div v-if="thumbnailLoading" class="timeline-thumbnail-loading">
            <el-icon class="is-loading"><Loading /></el-icon>
            <span>正在生成时间轴缩略图（{{ thumbnailLoadingCount }} 张）…</span>
          </div>
        </div>
        <el-alert
          v-if="hasEdits && (task.config.videoCodec === 'copy' || task.config.audioCodec === 'copy')"
          title="剪辑需要重新编码；保存剪辑方案时将自动把视频设为 H.264、音频设为 AAC，之后仍可在转码参数中改为 H.265。"
          type="info"
          :closable="false"
        />
      </section>
    </div>

    <template #footer>
      <div class="editor-footer">
        <span>共 {{ segments.length }} 个片段<span v-if="dirty"> · 有未保存修改</span></span>
        <div class="editor-actions">
          <span class="output-mode-label">输出方式</span>
          <el-select v-model="outputMode" class="output-mode-select">
            <el-option label="保存为新文件" value="new_file" />
            <el-option label="替换源文件" value="replace" />
          </el-select>
          <el-input
            v-if="outputMode === 'new_file'"
            v-model="outputFileName"
            class="output-file-name"
            maxlength="60"
            placeholder="新文件名"
          >
            <template #append>.{{ task.config.container }}</template>
          </el-input>
          <el-button @click="requestClose">取消</el-button>
          <el-button type="primary" :loading="saving" :disabled="!segments.length" @click="savePlan">
            保存剪辑方案
          </el-button>
        </div>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, h, nextTick, onBeforeUnmount, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Loading, Microphone, Mute, VideoPause, VideoPlay, ZoomIn, ZoomOut } from '@element-plus/icons-vue';
import videoPlay from '@/components/play/videoPlay.vue';
import { getPlayVideoURLAndType } from '@/common/play';
import type {
  I_videoEditPlan,
  I_videoEditSegment,
  I_videoTranscodeEditPlanResult,
  I_videoTranscodeTask,
} from '@/dataType/videoTranscode.dataType';
import { videoTranscodeServer } from '@/server/videoTranscode.server';

interface ThumbnailFrame {
  time: number;
  url: string;
}

const props = defineProps<{
  modelValue: boolean;
  task: I_videoTranscodeTask;
}>();
const emit = defineEmits<{
  'update:modelValue': [value: boolean];
  saved: [result: I_videoTranscodeEditPlanResult];
  closed: [];
}>();

const videoPlayRef = ref<InstanceType<typeof videoPlay>>();
const transitionVideoRef = ref<HTMLVideoElement>();
const timelineViewportRef = ref<HTMLElement>();
const timelineTrackRef = ref<HTMLElement>();
const segments = ref<I_videoEditSegment[]>([]);
const undoStack = ref<I_videoEditSegment[][]>([]);
const redoStack = ref<I_videoEditSegment[][]>([]);
const thumbnailFrames = ref<ThumbnailFrame[]>([]);
const thumbnailLoading = ref(false);
const thumbnailLoadingCount = ref(0);
const timelinePixelsPerSecond = ref(12);
const timelineVisibleStart = ref(0);
const timelineVisibleMiddle = ref(0);
const timelineVisibleEnd = ref(0);
const sourceDuration = ref(0);
const activeSegmentId = ref('');
const currentSourceTime = ref(0);
const initialSnapshot = ref('');
const initializing = ref(false);
const saving = ref(false);
const dragIndex = ref(-1);
const isPlaying = ref(false);
const volumePreferenceKey = 'cm-video-transcode-editor-volume-v1';
const loadVolumePreference = () => {
  try {
    const stored = JSON.parse(localStorage.getItem(volumePreferenceKey) || '{}');
    const storedVolume = Number(stored.volume);
    return {
      volume: Number.isFinite(storedVolume) ? Math.min(1, Math.max(0, storedVolume)) : 1,
      muted: stored.muted === true,
    };
  } catch {
    return { volume: 1, muted: false };
  }
};
const initialVolumePreference = loadVolumePreference();
const volume = ref(initialVolumePreference.volume);
const muted = ref(initialVolumePreference.muted);
const lastAudibleVolume = ref(initialVolumePreference.volume > 0 ? initialVolumePreference.volume : 1);
const draggingPlayhead = ref(false);
const playheadLeft = ref(2);
const outputMode = ref<'replace' | 'new_file'>('new_file');
const outputFileName = ref('');
const transitionPreviewUrl = ref('');
const transitionPreviewing = ref(false);
const transitionPreviewLoading = ref(false);
const transitionPreviewAuto = ref(false);
const transitionPreviewIndex = ref(-1);
let pollTimer: number | undefined;
let thumbnailLoadVersion = 0;
let thumbnailRefreshTimer: number | undefined;
let thumbnailAbortController: AbortController | undefined;
let initializedDurationFromPlayer = false;
let hadStoredEditPlan = false;
let closingAfterSave = false;
let transitionPreviewRequestID = 0;
let transitionPreviewStopAt = Number.POSITIVE_INFINITY;
let transitionPreviewAbortController: AbortController | undefined;
const thumbnailCache = new Map<string, ThumbnailFrame[]>();
const transitionPreviewCache = new Map<string, string>();

const timelineMinPixelsPerSecond = 0.5;
const timelineMaxPixelsPerSecond = 100;
const transitionEffects = [
  { value: 'fade', label: '淡入淡出至白色', short: '淡白' },
  { value: 'fadeblack', label: '淡入淡出至黑色', short: '淡黑' },
  { value: 'dissolve', label: '溶解', short: '溶解' },
  { value: 'wipeleft', label: '向左擦除', short: '左擦' },
  { value: 'wiperight', label: '向右擦除', short: '右擦' },
  { value: 'slideleft', label: '向左滑动', short: '左滑' },
  { value: 'slideright', label: '向右滑动', short: '右滑' },
  { value: 'hlslice', label: '水平百叶窗', short: '横百叶' },
  { value: 'vuslice', label: '垂直百叶窗', short: '竖百叶' },
  { value: 'circleopen', label: '圆形展开', short: '圆形' },
  { value: 'pixelize', label: '像素化', short: '像素' },
] as const;

const cloneSegments = (items: I_videoEditSegment[]) => items.map(item => ({
  ...item,
  transition: item.transition ? { ...item.transition } : undefined,
}));
const makeSegmentID = () => `segment-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
const snapshot = () => JSON.stringify({
  segments: cloneSegments(segments.value),
  outputMode: outputMode.value,
  outputFileName: outputFileName.value,
});
const editedDuration = computed(() => segments.value.reduce((sum, item) => sum + item.end - item.start, 0));
const activeSegmentIndex = computed(() => Math.max(0, segments.value.findIndex(item => item.id === activeSegmentId.value)));
const activeSegment = computed(() => segments.value[activeSegmentIndex.value]);
const dirty = computed(() => snapshot() !== initialSnapshot.value);
const hasEdits = computed(() => segments.value.length !== 1 ||
  Boolean(segments.value[0] && (segments.value[0].start > 0.01 ||
    Math.abs(segments.value[0].end - sourceDuration.value) > 0.05)));
const canSplit = computed(() => {
  const segment = activeSegment.value;
  return Boolean(segment && currentSourceTime.value - segment.start >= 0.1 &&
    segment.end - currentSourceTime.value >= 0.1);
});
const currentOutputTime = computed(() => {
  const index = activeSegmentIndex.value;
  const before = segments.value.slice(0, index).reduce((sum, item) => sum + item.end - item.start, 0);
  const segment = activeSegment.value;
  return segment ? before + Math.max(0, Math.min(currentSourceTime.value, segment.end) - segment.start) : 0;
});
const applyVolumeToPlayers = () => {
  const actualVolume = muted.value ? 0 : volume.value;
  videoPlayRef.value?.setVolume(actualVolume);
  if (transitionVideoRef.value) {
    transitionVideoRef.value.volume = actualVolume;
    transitionVideoRef.value.muted = false;
  }
};
const saveVolumePreference = () => {
  try {
    localStorage.setItem(volumePreferenceKey, JSON.stringify({
      volume: volume.value,
      muted: muted.value,
    }));
  } catch {
    // 本地存储不可用时仍保留当前会话中的音量设置。
  }
};
const toggleMute = () => {
  if (muted.value && volume.value <= 0) volume.value = lastAudibleVolume.value;
  muted.value = !muted.value;
  applyVolumeToPlayers();
  saveVolumePreference();
};
const setVolume = (value: number | number[]) => {
  const nextVolume = Array.isArray(value) ? value[0] : value;
  volume.value = Math.min(1, Math.max(0, Number(nextVolume) || 0));
  muted.value = volume.value === 0;
  if (volume.value > 0) lastAudibleVolume.value = volume.value;
  applyVolumeToPlayers();
  saveVolumePreference();
};
const sourcePositionFromOutput = (value: number) => {
  const clamped = Math.max(0, Math.min(value, editedDuration.value));
  let elapsed = 0;
  for (let index = 0; index < segments.value.length; index++) {
    const segment = segments.value[index];
    const duration = segment.end - segment.start;
    if (clamped < elapsed + duration || index === segments.value.length - 1) {
      return {
        segment,
        sourceTime: segment.start + Math.max(0, Math.min(duration, clamped - elapsed)),
      };
    }
    elapsed += duration;
  }
  return undefined;
};
const seekOutputTime = (value: number | number[]) => {
  const nextTime = Array.isArray(value) ? value[0] : value;
  const target = sourcePositionFromOutput(Number(nextTime) || 0);
  if (!target) return;
  const resumePlayback = isPlaying.value;
  if (transitionPreviewing.value || transitionPreviewLoading.value) stopTransitionPreview();
  seekToSegment(target.segment, target.sourceTime);
  if (resumePlayback) {
    void videoPlayRef.value?.play();
    isPlaying.value = true;
  }
};
const initializeEditor = async () => {
  initializing.value = true;
  closingAfterSave = false;
  thumbnailCache.clear();
  thumbnailFrames.value = [];
  thumbnailLoading.value = false;
  stopTransitionPreview(false);
  sourceDuration.value = props.task.sourceDuration || 0;
  const savedSegments = props.task.editPlan?.segments || [];
  hadStoredEditPlan = savedSegments.length > 0;
  outputMode.value = hadStoredEditPlan ? (props.task.config.outputMode || 'new_file') : 'new_file';
  outputFileName.value = hadStoredEditPlan && props.task.config.outputFileName
    ? props.task.config.outputFileName
    : `${fileStem(props.task.sourcePath)}_剪辑`;
  segments.value = savedSegments.length
    ? cloneSegments(savedSegments)
    : sourceDuration.value > 0
      ? [{ id: makeSegmentID(), start: 0, end: sourceDuration.value }]
      : [];
  activeSegmentId.value = segments.value[0]?.id || '';
  currentSourceTime.value = segments.value[0]?.start || 0;
  undoStack.value = [];
  redoStack.value = [];
  initialSnapshot.value = snapshot();
  initializedDurationFromPlayer = false;
  await nextTick();
  fitTimeline();
  window.addEventListener('resize', handleEditorResize);
  const { playUrl, playType } = await getPlayVideoURLAndType(props.task.dramaSeriesId);
  if (playUrl) {
    videoPlayRef.value?.setVideoSource(playUrl, playType, () => {
      applyVolumeToPlayers();
      seekToSegment(segments.value[0]);
    }, fileName(props.task.sourcePath));
  }
  startPolling();
  initializing.value = false;
  if (sourceDuration.value > 0) scheduleThumbnailRefresh(true);
};

const startPolling = () => {
  window.clearInterval(pollTimer);
  pollTimer = window.setInterval(() => {
    const player = videoPlayRef.value;
    if (!player) return;
    const playerDuration = player.getDuration();
    if (!initializedDurationFromPlayer && Number.isFinite(playerDuration) && playerDuration > 0) {
      initializedDurationFromPlayer = true;
      const durationChanged = Math.abs(playerDuration - sourceDuration.value) > 0.05;
      sourceDuration.value = playerDuration;
      if (!hadStoredEditPlan) {
        segments.value = [{ id: makeSegmentID(), start: 0, end: playerDuration }];
        activeSegmentId.value = segments.value[0].id;
        initialSnapshot.value = snapshot();
      }
      if (durationChanged || !thumbnailFrames.value.length) {
        void nextTick().then(() => {
          fitTimeline();
          scheduleThumbnailRefresh(true);
        });
      }
    }
    if (transitionPreviewing.value) {
      isPlaying.value = !(transitionVideoRef.value?.paused ?? true);
      return;
    }
    if (transitionPreviewLoading.value) return;
    const time = player.getCurrentTime();
    if (!Number.isFinite(time)) return;
    currentSourceTime.value = time;
    isPlaying.value = player.isPlaying();
    updatePlayheadPosition();
    const segment = activeSegment.value;
    if (!segment || !player.isPlaying() || transitionPreviewing.value || transitionPreviewLoading.value) return;
    if (time < segment.start - 0.05) {
      player.setCurrentTime(segment.start);
      return;
    }
    const transitionHalf = segment.transition ? segment.transition.duration / 2 : 0;
    if (segment.transition && time >= segment.end - transitionHalf - 0.04) {
      void playTransitionPreview(activeSegmentIndex.value, true);
      return;
    }
    if (time >= segment.end - 0.04) {
      const next = segments.value[activeSegmentIndex.value + 1];
      if (!next) {
        player.pause();
        isPlaying.value = false;
        player.setCurrentTime(segment.end);
        return;
      }
      activeSegmentId.value = next.id;
      player.setCurrentTime(next.start);
      void player.play();
      isPlaying.value = true;
    }
  }, 100);
};

const desiredThumbnailTimes = () => {
  if (!segments.value.length) return [];
  if (segments.value.length > 160) {
    const active = activeSegment.value;
    const ordered = active
      ? [active, ...segments.value.filter(item => item.id !== active.id)]
      : segments.value;
    return ordered.slice(0, 160).map(segment => (segment.start + segment.end) / 2);
  }
  const counts = segments.value.map(segment => Math.max(
    1,
    Math.ceil((segment.end - segment.start) * timelinePixelsPerSecond.value / 140),
  ));
  let total = counts.reduce((sum, count) => sum + count, 0);
  while (total > 160) {
    let largestIndex = -1;
    for (let index = 0; index < counts.length; index++) {
      if (counts[index] > 1 && (largestIndex < 0 || counts[index] > counts[largestIndex])) {
        largestIndex = index;
      }
    }
    if (largestIndex < 0) break;
    counts[largestIndex]--;
    total--;
  }
  return segments.value.flatMap((segment, segmentIndex) => {
    const duration = segment.end - segment.start;
    const count = counts[segmentIndex];
    return Array.from({ length: count }, (_, index) =>
      segment.start + duration * (index + 0.5) / count);
  });
};

const thumbnailCacheKey = (times: number[]) => times.map(time => time.toFixed(3)).join(',');
const refreshTimelineThumbnails = async () => {
  const times = desiredThumbnailTimes();
  if (!times.length) return;
  const loadVersion = ++thumbnailLoadVersion;
  thumbnailAbortController?.abort();
  thumbnailAbortController = undefined;
  const key = thumbnailCacheKey(times);
  const cached = thumbnailCache.get(key);
  if (cached) {
    thumbnailFrames.value = cached;
    thumbnailLoading.value = false;
    thumbnailLoadingCount.value = 0;
    return;
  }
  const controller = new AbortController();
  thumbnailAbortController = controller;
  thumbnailLoading.value = true;
  thumbnailLoadingCount.value = times.length;
  try {
    const result = await videoTranscodeServer.thumbnails(props.task.id, times, controller.signal);
    if (loadVersion !== thumbnailLoadVersion || controller.signal.aborted) return;
    if (!result.status) {
      ElMessage.error(result.msg || '生成时间轴缩略图失败');
      return;
    }
    const frames = result.data.filter(item => item.url).sort((left, right) => left.time - right.time);
    thumbnailFrames.value = frames;
    thumbnailCache.set(key, frames);
    while (thumbnailCache.size > 4) {
      const oldest = thumbnailCache.keys().next().value as string | undefined;
      if (!oldest) break;
      thumbnailCache.delete(oldest);
    }
  } finally {
    if (loadVersion === thumbnailLoadVersion) {
      thumbnailLoading.value = false;
      thumbnailLoadingCount.value = 0;
    }
  }
};

const ensureTimelineThumbnails = async (reset = false) => {
  if (reset) {
    thumbnailCache.clear();
    thumbnailFrames.value = [];
  }
  await refreshTimelineThumbnails();
};
const scheduleThumbnailRefresh = (immediate = false) => {
  window.clearTimeout(thumbnailRefreshTimer);
  if (immediate) {
    void ensureTimelineThumbnails();
    return;
  }
  thumbnailRefreshTimer = window.setTimeout(() => void ensureTimelineThumbnails(), 320);
};
const revokeThumbnails = () => {
  window.clearTimeout(thumbnailRefreshTimer);
  thumbnailAbortController?.abort();
  thumbnailAbortController = undefined;
  thumbnailLoadVersion++;
  thumbnailLoading.value = false;
  thumbnailFrames.value.forEach(item => {
    if (item.url.startsWith('blob:')) URL.revokeObjectURL(item.url);
  });
  thumbnailFrames.value = [];
  thumbnailCache.clear();
};
const framesForSegment = (segment: I_videoEditSegment) => {
  return thumbnailFrames.value.filter(frame => frame.time >= segment.start && frame.time <= segment.end);
};
const segmentStyle = (segment: I_videoEditSegment) => ({
  width: `${Math.max(8, (segment.end - segment.start) * timelinePixelsPerSecond.value)}px`,
  flex: '0 0 auto',
});

const clampTimelineZoom = (value: number) => Math.min(
  timelineMaxPixelsPerSecond,
  Math.max(timelineMinPixelsPerSecond, value),
);
const timelineAnchorAt = (clientX: number) => {
  const track = timelineTrackRef.value;
  const viewport = timelineViewportRef.value;
  if (!track || !viewport) return undefined;
  const elements = Array.from(track.querySelectorAll<HTMLElement>('.timeline-segment'));
  let bestIndex = 0;
  let bestDistance = Number.POSITIVE_INFINITY;
  let ratio = 0;
  elements.forEach((element, index) => {
    const rect = element.getBoundingClientRect();
    const clampedX = Math.min(rect.right, Math.max(rect.left, clientX));
    const distance = Math.abs(clientX - clampedX);
    if (distance < bestDistance) {
      bestIndex = index;
      bestDistance = distance;
      ratio = Math.min(1, Math.max(0, (clientX - rect.left) / Math.max(rect.width, 1)));
    }
  });
  return {
    index: bestIndex,
    ratio,
    viewportOffset: clientX - viewport.getBoundingClientRect().left,
  };
};
const setTimelineZoom = (value: number, anchorClientX?: number) => {
  const viewport = timelineViewportRef.value;
  if (!viewport) return;
  const clientX = anchorClientX ?? viewport.getBoundingClientRect().left + viewport.clientWidth / 2;
  const anchor = timelineAnchorAt(clientX);
  const nextValue = clampTimelineZoom(value);
  if (Math.abs(nextValue - timelinePixelsPerSecond.value) < 0.001) return;
  timelinePixelsPerSecond.value = nextValue;
  scheduleThumbnailRefresh();
  void nextTick(() => {
    if (anchor) {
      const element = timelineTrackRef.value
        ?.querySelectorAll<HTMLElement>('.timeline-segment')[anchor.index];
      if (element) {
        viewport.scrollLeft = element.offsetLeft + anchor.ratio * element.offsetWidth - anchor.viewportOffset;
      }
    }
    updatePlayheadPosition();
    updateTimelineRuler();
  });
};
const currentPlayheadClientX = () => {
  const element = timelineTrackRef.value
    ?.querySelectorAll<HTMLElement>('.timeline-segment')[activeSegmentIndex.value];
  const segment = activeSegment.value;
  if (!element || !segment) return undefined;
  const rect = element.getBoundingClientRect();
  const ratio = Math.min(1, Math.max(0,
    (currentSourceTime.value - segment.start) / Math.max(segment.end - segment.start, 0.001)));
  return rect.left + ratio * rect.width;
};
const zoomTimeline = (factor: number) => setTimelineZoom(
  timelinePixelsPerSecond.value * factor,
  currentPlayheadClientX(),
);
const onTimelineWheel = (event: WheelEvent) => {
  if (!event.deltaY) return;
  event.preventDefault();
  setTimelineZoom(
    timelinePixelsPerSecond.value * Math.pow(1.2, -event.deltaY / 100),
    event.clientX,
  );
};
const fitTimeline = () => {
  const viewport = timelineViewportRef.value;
  if (!viewport || editedDuration.value <= 0) return;
  const transitionCount = Math.max(0, segments.value.length - 1);
  const childCount = segments.value.length + transitionCount;
  const fixedWidth = 28 + transitionCount * 58 + Math.max(0, childCount - 1) * 4;
  timelinePixelsPerSecond.value = clampTimelineZoom(
    Math.max(1, viewport.clientWidth - fixedWidth) / editedDuration.value,
  );
  viewport.scrollLeft = 0;
  scheduleThumbnailRefresh();
  void nextTick(() => {
    updatePlayheadPosition();
    updateTimelineRuler();
  });
};
const seekToSegment = (segment?: I_videoEditSegment, at?: number) => {
  if (!segment) return;
  activeSegmentId.value = segment.id;
  const time = at ?? segment.start;
  currentSourceTime.value = time;
  videoPlayRef.value?.setCurrentTime(time);
  void nextTick().then(updatePlayheadPosition);
};
const seekWithinSegment = (event: MouseEvent, segment: I_videoEditSegment) => {
  const target = event.currentTarget as HTMLElement;
  const rect = target.getBoundingClientRect();
  const ratio = Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width));
  seekToSegment(segment, segment.start + ratio * (segment.end - segment.start));
};
const updatePlayheadPosition = () => {
  const track = timelineTrackRef.value;
  const segment = activeSegment.value;
  if (!track || !segment) return;
  const elements = Array.from(track.querySelectorAll<HTMLElement>('.timeline-segment'));
  const element = elements[activeSegmentIndex.value];
  if (!element) return;
  const ratio = Math.max(0, Math.min(1,
    (currentSourceTime.value - segment.start) / Math.max(segment.end - segment.start, 0.001)));
  playheadLeft.value = element.offsetLeft + ratio * element.offsetWidth;
};
const outputTimeAtClientX = (clientX: number) => {
  const elements = Array.from(
    timelineTrackRef.value?.querySelectorAll<HTMLElement>('.timeline-segment') || [],
  );
  let elapsed = 0;
  for (let index = 0; index < elements.length; index++) {
    const element = elements[index];
    const segment = segments.value[index];
    if (!segment) break;
    const duration = segment.end - segment.start;
    const rect = element.getBoundingClientRect();
    if (clientX <= rect.left) return elapsed;
    if (clientX < rect.right) {
      const ratio = Math.max(0, Math.min(1, (clientX - rect.left) / Math.max(rect.width, 1)));
      return elapsed + duration * ratio;
    }
    elapsed += duration;
  }
  return editedDuration.value;
};
const updateTimelineRuler = () => {
  const viewport = timelineViewportRef.value;
  if (!viewport || !segments.value.length) {
    timelineVisibleStart.value = 0;
    timelineVisibleMiddle.value = 0;
    timelineVisibleEnd.value = 0;
    return;
  }
  const rect = viewport.getBoundingClientRect();
  timelineVisibleStart.value = outputTimeAtClientX(rect.left);
  timelineVisibleMiddle.value = outputTimeAtClientX(rect.left + rect.width / 2);
  timelineVisibleEnd.value = outputTimeAtClientX(rect.right);
};
const handleEditorResize = () => {
  updatePlayheadPosition();
  updateTimelineRuler();
};
const seekFromPointer = (clientX: number) => {
  const track = timelineTrackRef.value;
  if (!track) return;
  const elements = Array.from(track.querySelectorAll<HTMLElement>('.timeline-segment'));
  let bestIndex = -1;
  let bestDistance = Number.POSITIVE_INFINITY;
  let bestRatio = 0;
  elements.forEach((element, index) => {
    const rect = element.getBoundingClientRect();
    const clampedX = Math.max(rect.left, Math.min(clientX, rect.right));
    const distance = Math.abs(clientX - clampedX);
    if (distance < bestDistance) {
      bestDistance = distance;
      bestIndex = index;
      bestRatio = Math.max(0, Math.min(1, (clampedX - rect.left) / Math.max(rect.width, 1)));
    }
  });
  const segment = segments.value[bestIndex];
  if (!segment) return;
  seekToSegment(segment, segment.start + bestRatio * (segment.end - segment.start));
};
const movePlayhead = (event: PointerEvent) => {
  if (!draggingPlayhead.value) return;
  seekFromPointer(event.clientX);
};
const stopPlayheadDrag = () => {
  draggingPlayhead.value = false;
  window.removeEventListener('pointermove', movePlayhead);
  window.removeEventListener('pointerup', stopPlayheadDrag);
};
const startPlayheadDrag = (event: PointerEvent) => {
  videoPlayRef.value?.pause();
  isPlaying.value = false;
  draggingPlayhead.value = true;
  seekFromPointer(event.clientX);
  window.addEventListener('pointermove', movePlayhead);
  window.addEventListener('pointerup', stopPlayheadDrag, { once: true });
};
const togglePlayback = () => {
  if (transitionPreviewing.value) {
    const preview = transitionVideoRef.value;
    if (!preview) return;
    if (preview.paused) {
      void preview.play();
      isPlaying.value = true;
    } else {
      preview.pause();
      isPlaying.value = false;
    }
    return;
  }
  const player = videoPlayRef.value;
  if (!player) return;
  if (player.isPlaying()) {
    player.pause();
    isPlaying.value = false;
    return;
  }
  if (currentOutputTime.value >= editedDuration.value - 0.02) {
    seekToSegment(segments.value[0]);
  }
  const segment = activeSegment.value;
  if (!segment) return;
  if (currentSourceTime.value < segment.start || currentSourceTime.value >= segment.end - 0.04) {
    seekToSegment(segment);
  }
  void player.play();
  isPlaying.value = true;
};

const transitionShortLabel = (segment: I_videoEditSegment) => {
  if (!segment.transition) return '转场';
  return transitionEffects.find(effect => effect.value === segment.transition?.type)?.short || '转场';
};
const transitionMaxDuration = (index: number) => {
  const current = segments.value[index];
  const next = segments.value[index + 1];
  if (!current || !next) return 0;
  const incomingHalf = index > 0 ? (segments.value[index - 1].transition?.duration || 0) / 2 : 0;
  const nextOutgoingHalf = (next.transition?.duration || 0) / 2;
  const currentAvailable = current.end - current.start - incomingHalf - 0.05;
  const nextAvailable = next.end - next.start - nextOutgoingHalf - 0.05;
  return Math.max(0, Math.min(3, currentAvailable * 2, nextAvailable * 2));
};
const setTransitionType = (index: number, type: string) => {
  const next = cloneSegments(segments.value);
  const segment = next[index];
  if (!segment || index >= next.length - 1) return;
  if (!type) {
    segment.transition = undefined;
    commitChange(next);
    return;
  }
  const maximum = transitionMaxDuration(index);
  if (maximum < 0.1) return ElMessage.warning('相邻片段太短，无法添加转场');
  segment.transition = {
    type,
    duration: Math.min(segment.transition?.duration || 1, maximum),
    audioFade: segment.transition?.audioFade || false,
  };
  commitChange(next);
};
const setTransitionDuration = (index: number, value: number) => {
  if (!Number.isFinite(value)) return;
  const next = cloneSegments(segments.value);
  const transition = next[index]?.transition;
  if (!transition) return;
  transition.duration = Math.max(0.1, Math.min(value, transitionMaxDuration(index)));
  commitChange(next);
};
const setTransitionAudioFade = (index: number, value: boolean) => {
  const next = cloneSegments(segments.value);
  const transition = next[index]?.transition;
  if (!transition) return;
  transition.audioFade = value;
  commitChange(next);
};
const transitionContext = (index: number) => {
  const left = segments.value[index];
  const right = segments.value[index + 1];
  const transition = left?.transition;
  if (!left || !right || !transition) return undefined;
  const half = transition.duration / 2;
  const lead = Math.min(0.75, Math.max(0, left.end - left.start - half));
  const tail = Math.min(0.75, Math.max(0, right.end - right.start - half));
  return { left, right, transition, half, lead, tail };
};
const transitionPreviewKey = (index: number) => {
  const context = transitionContext(index);
  return context ? JSON.stringify({
    left: context.left,
    right: context.right,
  }) : '';
};
const waitForPreviewMetadata = async (video: HTMLVideoElement) => {
  if (video.readyState >= 1) return;
  await new Promise<void>((resolve, reject) => {
    const loaded = () => { cleanup(); resolve(); };
    const failed = () => { cleanup(); reject(new Error('浏览器无法加载转场预览')); };
    const cleanup = () => {
      video.removeEventListener('loadedmetadata', loaded);
      video.removeEventListener('error', failed);
    };
    video.addEventListener('loadedmetadata', loaded, { once: true });
    video.addEventListener('error', failed, { once: true });
  });
};
const playTransitionPreview = async (index: number, automatic: boolean) => {
  const context = transitionContext(index);
  if (!context || transitionPreviewLoading.value || transitionPreviewing.value) return;
  const requestID = ++transitionPreviewRequestID;
  transitionPreviewAbortController?.abort();
  const controller = new AbortController();
  transitionPreviewAbortController = controller;
  transitionPreviewLoading.value = true;
  transitionPreviewIndex.value = index;
  transitionPreviewAuto.value = automatic;
  videoPlayRef.value?.pause();
  isPlaying.value = false;
  try {
    const key = transitionPreviewKey(index);
    let url = transitionPreviewCache.get(key);
    if (!url) {
      const result = await videoTranscodeServer.transitionPreview(
        props.task.id,
        cloneSegments([context.left])[0],
        cloneSegments([context.right])[0],
        controller.signal,
      );
      if (!result.status) throw new Error(result.msg || '生成转场预览失败');
      url = URL.createObjectURL(result.data);
      transitionPreviewCache.set(key, url);
    }
    if (requestID !== transitionPreviewRequestID || controller.signal.aborted) return;
    transitionPreviewUrl.value = url;
    transitionPreviewing.value = true;
    transitionPreviewStopAt = automatic
      ? context.lead + context.transition.duration
      : Number.POSITIVE_INFINITY;
    await nextTick();
    const preview = transitionVideoRef.value;
    if (!preview) throw new Error('转场预览播放器未就绪');
    preview.load();
    await waitForPreviewMetadata(preview);
    applyVolumeToPlayers();
    preview.currentTime = automatic ? context.lead : 0;
    await preview.play();
    isPlaying.value = true;
  } catch (error) {
    if (controller.signal.aborted) return;
    const message = error instanceof Error ? error.message : '生成转场预览失败';
    ElMessage.error(message);
    if (automatic) {
      seekToSegment(context.right, context.right.start);
      void videoPlayRef.value?.play();
      isPlaying.value = true;
    }
  } finally {
    if (requestID === transitionPreviewRequestID) transitionPreviewLoading.value = false;
  }
};
const handleTransitionPreviewTimeUpdate = () => {
  const preview = transitionVideoRef.value;
  const context = transitionContext(transitionPreviewIndex.value);
  if (!preview || !context) return;
  const previewTime = preview.currentTime;
  const midpoint = context.lead + context.half;
  if (previewTime < midpoint) {
    activeSegmentId.value = context.left.id;
    currentSourceTime.value = context.left.end - context.half - context.lead + previewTime;
  } else {
    activeSegmentId.value = context.right.id;
    currentSourceTime.value = context.right.start + previewTime - midpoint;
  }
  updatePlayheadPosition();
  if (transitionPreviewAuto.value && previewTime >= transitionPreviewStopAt - 0.025) {
    finishTransitionPreview();
  }
};
const finishTransitionPreview = () => {
  const context = transitionContext(transitionPreviewIndex.value);
  const automatic = transitionPreviewAuto.value;
  if (!context) return stopTransitionPreview(false);
  const resumeAt = context.right.start + context.half + (automatic ? 0 : context.tail);
  stopTransitionPreview(false);
  seekToSegment(context.right, Math.min(context.right.end, resumeAt));
  if (automatic) {
    void videoPlayRef.value?.play();
    isPlaying.value = true;
  }
};
const stopTransitionPreview = (cancelRequest = true) => {
  if (cancelRequest) {
    transitionPreviewRequestID++;
    transitionPreviewAbortController?.abort();
  }
  transitionVideoRef.value?.pause();
  transitionPreviewing.value = false;
  transitionPreviewLoading.value = false;
  transitionPreviewAuto.value = false;
  transitionPreviewIndex.value = -1;
  transitionPreviewStopAt = Number.POSITIVE_INFINITY;
  isPlaying.value = false;
};
const revokeTransitionPreviews = () => {
  stopTransitionPreview();
  transitionPreviewCache.forEach(url => URL.revokeObjectURL(url));
  transitionPreviewCache.clear();
  transitionPreviewUrl.value = '';
};

const commitChange = (next: I_videoEditSegment[]) => {
  if (transitionPreviewing.value || transitionPreviewLoading.value) stopTransitionPreview();
  undoStack.value.push(cloneSegments(segments.value));
  redoStack.value = [];
  segments.value = next;
  if (!segments.value.some(item => item.id === activeSegmentId.value)) {
    activeSegmentId.value = segments.value[0]?.id || '';
  }
  void nextTick().then(() => {
    updatePlayheadPosition();
    updateTimelineRuler();
    scheduleThumbnailRefresh();
  });
};
const splitCurrentSegment = () => {
  if (!canSplit.value) return;
  const index = activeSegmentIndex.value;
  const segment = activeSegment.value;
  const splitAt = Math.round(currentSourceTime.value * 1000) / 1000;
  const left = { ...segment, id: makeSegmentID(), end: splitAt, transition: undefined };
  const right = {
    ...segment,
    id: makeSegmentID(),
    start: splitAt,
    transition: segment.transition ? { ...segment.transition } : undefined,
  };
  const next = cloneSegments(segments.value);
  next.splice(index, 1, left, right);
  commitChange(next);
  seekToSegment(right);
};
const removeCurrentSegment = () => {
  if (segments.value.length <= 1) return ElMessage.warning('至少需要保留一个片段');
  const index = activeSegmentIndex.value;
  const next = cloneSegments(segments.value);
  next.splice(index, 1);
  if (index > 0) next[index - 1].transition = undefined;
  commitChange(next);
  seekToSegment(next[Math.min(index, next.length - 1)]);
};
const resetPlan = () => {
  if (sourceDuration.value <= 0) return;
  const next = [{ id: makeSegmentID(), start: 0, end: sourceDuration.value }];
  commitChange(next);
  seekToSegment(next[0]);
};
const undo = () => {
  const previous = undoStack.value.pop();
  if (!previous) return;
  stopTransitionPreview();
  redoStack.value.push(cloneSegments(segments.value));
  segments.value = previous;
  seekToSegment(segments.value.find(item => item.id === activeSegmentId.value) || segments.value[0]);
  scheduleThumbnailRefresh();
};
const redo = () => {
  const next = redoStack.value.pop();
  if (!next) return;
  stopTransitionPreview();
  undoStack.value.push(cloneSegments(segments.value));
  segments.value = next;
  seekToSegment(segments.value.find(item => item.id === activeSegmentId.value) || segments.value[0]);
  scheduleThumbnailRefresh();
};
const startDrag = (event: DragEvent, index: number) => {
  dragIndex.value = index;
  event.dataTransfer?.setData('text/plain', String(index));
  if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move';
};
const dropSegment = (targetIndex: number) => {
  const sourceIndex = dragIndex.value;
  dragIndex.value = -1;
  if (sourceIndex < 0 || sourceIndex === targetIndex) return;
  const next = cloneSegments(segments.value);
  const hadTransitions = next.some(item => item.transition);
  const [moved] = next.splice(sourceIndex, 1);
  next.splice(targetIndex, 0, moved);
  next.forEach(item => { item.transition = undefined; });
  commitChange(next);
  if (hadTransitions) ElMessage.info('片段顺序已变化，原转场已清除，请重新设置');
};

const savePlan = async () => {
  const plan: I_videoEditPlan = { version: 1, segments: cloneSegments(segments.value) };
  if (outputMode.value === 'new_file' && !outputFileName.value.trim()) {
    return ElMessage.warning('请输入新文件名');
  }
  const savesAsNewFile = outputMode.value === 'new_file';
  const confirmation = savesAsNewFile
    ? h('div', [
      h('p', { style: 'margin: 0 0 8px' }, '输出方式：保存为新文件'),
      h('p', { style: 'margin: 0; color: var(--el-text-color-secondary)' },
        `预计文件：${outputFileName.value.trim()}.${props.task.config.container}（重名会自动编号，不会替换源文件）`),
    ])
    : h('div', [
      h('p', { style: 'margin: 0 0 8px' }, '输出方式：替换源文件'),
      h('p', { style: 'margin: 0; color: var(--el-color-danger)' },
        '转码校验成功后将替换当前源视频，替换完成后无法在任务中撤销。'),
    ]);
  try {
    await ElMessageBox.confirm(confirmation, '确认剪辑输出方式', {
      type: savesAsNewFile ? 'info' : 'warning',
      confirmButtonText: savesAsNewFile ? '确认另存并保存方案' : '确认替换并保存方案',
      cancelButtonText: '返回检查',
    });
  } catch {
    return;
  }
  saving.value = true;
  const result = await videoTranscodeServer.saveEditPlan(
    props.task.id,
    plan,
    outputMode.value,
    outputFileName.value.trim(),
  );
  saving.value = false;
  if (!result.status) return ElMessage.error(result.msg || '保存剪辑方案失败');
  segments.value = cloneSegments(result.data.plan.segments);
  initialSnapshot.value = snapshot();
  emit('saved', result.data);
  const outputTip = result.data.config.outputMode === 'new_file' ? '，转码后将保存为新文件' : '';
  if (result.data.hasEdits && result.data.configAdjusted) {
    const videoCodec = result.data.config.videoCodec === 'h265' ? 'H.265' : 'H.264';
    ElMessage.success(`剪辑方案已保存，转码参数已自动设为 ${videoCodec} / AAC${outputTip}`);
  } else {
    ElMessage.success((result.data.hasEdits ? '剪辑方案已保存' : '已恢复为完整视频') + outputTip);
  }
  closingAfterSave = true;
  emit('update:modelValue', false);
};

const beforeClose = async (done: () => void) => {
  if (closingAfterSave || !dirty.value) return done();
  try {
    await ElMessageBox.confirm('当前剪辑方案尚未保存，确定关闭吗？', '放弃剪辑修改', { type: 'warning' });
    done();
  } catch {
    // 继续编辑。
  }
};
const requestClose = async () => {
  if (dirty.value) {
    try {
      await ElMessageBox.confirm('当前剪辑方案尚未保存，确定关闭吗？', '放弃剪辑修改', { type: 'warning' });
    } catch {
      return;
    }
  }
  emit('update:modelValue', false);
};
const handleClosed = () => {
  window.clearInterval(pollTimer);
  window.clearTimeout(thumbnailRefreshTimer);
  videoPlayRef.value?.releaseSource();
  revokeTransitionPreviews();
  stopPlayheadDrag();
  window.removeEventListener('resize', handleEditorResize);
  revokeThumbnails();
  emit('closed');
};
const fileName = (path: string) => path.replace(/\\/g, '/').split('/').pop() || path;
const fileStem = (path: string) => {
  const name = fileName(path);
  const dotIndex = name.lastIndexOf('.');
  return dotIndex > 0 ? name.slice(0, dotIndex) : name;
};
const formatTime = (seconds: number) => {
  const safe = Math.max(0, Number.isFinite(seconds) ? seconds : 0);
  const hours = Math.floor(safe / 3600);
  const minutes = Math.floor((safe % 3600) / 60);
  const remaining = Math.floor(safe % 60);
  const milliseconds = Math.floor((safe % 1) * 10);
  return `${hours > 0 ? `${String(hours).padStart(2, '0')}:` : ''}${String(minutes).padStart(2, '0')}:${String(remaining).padStart(2, '0')}.${milliseconds}`;
};

onBeforeUnmount(() => {
  window.clearInterval(pollTimer);
  window.clearTimeout(thumbnailRefreshTimer);
  videoPlayRef.value?.releaseSource();
  revokeTransitionPreviews();
  stopPlayheadDrag();
  window.removeEventListener('resize', handleEditorResize);
  revokeThumbnails();
});
</script>

<style scoped lang="scss">
.editor-header,
.editor-footer,
.player-controls,
.preview-status,
.timeline-toolbar,
.timeline-ruler {
  display: flex;
  align-items: center;
}
.editor-header {
  justify-content: space-between;
  padding-right: 30px;
  color: var(--el-text-color-primary);
  strong { font-size: 18px; margin-right: 12px; color: var(--el-text-color-primary); }
  span { color: var(--el-text-color-secondary); }
}
.duration-summary { font-size: 14px; }
.editor-layout {
  height: calc(100vh - 122px);
  min-height: 580px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.preview-panel,
.timeline-panel {
  min-height: 0;
  border: 1px solid var(--el-border-color);
  border-radius: 6px;
  background: var(--el-bg-color);
}
.preview-panel {
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
  background: #121212;
}
.preview-stage {
  position: relative;
  flex: 1 1 auto;
  display: flex;
  min-height: 0;
  overflow: hidden;
  background: #000;
}
.editor-player {
  flex: 1;
  width: 100%;
  height: 100%;
  min-height: 0;
  overflow: hidden;
}
.transition-player {
  width: 100%;
  height: 100%;
  min-height: 0;
  flex: 1;
  object-fit: contain;
  background: #000;
}
.player-controls {
  min-height: 48px;
  flex: 0 0 auto;
  gap: 10px;
  padding: 0 14px;
  color: #d6d6d6;
  background: #1f1f1f;
}
.player-controls :deep(.el-button.is-circle) {
  width: 30px;
  height: 30px;
  flex: 0 0 30px;
  border-color: #4b4b4b;
  color: #e8e8e8;
  background: #2a2a2a;
}
.player-controls :deep(.el-button.is-circle:hover) {
  border-color: var(--el-color-primary);
  color: var(--el-color-primary-light-3);
}
.volume-slider { flex: 0 0 100px; width: 100px; }
.output-slider { min-width: 120px; flex: 1 1 auto; }
.time-label {
  min-width: 116px;
  color: #d6d6d6;
  font-variant-numeric: tabular-nums;
  text-align: center;
  white-space: nowrap;
}
.player-controls :deep(.el-slider__runway) { background-color: #555; }
.preview-status {
  flex: 0 0 auto;
  gap: 10px;
  color: #d6d6d6;
  font-size: 12px;
}
.preview-tip { color: #909399; white-space: nowrap; }
.timeline-panel {
  flex: 0 0 auto;
  display: flex;
  flex-direction: column;
  padding: 10px 12px 4px;
  overflow: hidden;
}
.timeline-toolbar { gap: 4px; flex-shrink: 0; }
.timeline-toolbar > span { margin-left: auto; color: var(--el-text-color-secondary); font-size: 12px; }
.timeline-zoom-controls { margin-left: 8px; }
.zoom-value { align-self: center; min-width: 72px; color: var(--el-text-color-secondary); text-align: center; }
.timeline-ruler {
  justify-content: space-between;
  margin-top: 8px;
  padding: 0 4px 5px;
  color: var(--el-text-color-secondary);
  font-size: 11px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.timeline-viewport-shell { position: relative; }
.timeline-viewport {
  width: 100%;
  min-height: 100px;
  overflow-x: scroll;
  overflow-y: hidden;
  scrollbar-width: thin;
  scrollbar-gutter: stable;
}
.timeline-track {
  position: relative;
  min-height: 100px;
  width: max-content;
  min-width: 100%;
  box-sizing: border-box;
  display: flex;
  align-items: stretch;
  gap: 4px;
  padding: 4px 14px 12px;
}
.timeline-segment {
  position: relative;
  min-width: 8px;
  height: 70px;
  box-sizing: border-box;
  flex: 0 0 auto;
  overflow: hidden;
  border: 2px solid transparent;
  border-radius: 5px;
  background: #222;
  cursor: pointer;
  user-select: none;
  transition: border-color .15s, opacity .15s;
}
.timeline-segment.active { border-color: var(--el-color-primary); }
.timeline-segment.dragging { opacity: .45; }
.segment-thumbnails { position: absolute; inset: 0; display: flex; overflow: hidden; }
.segment-thumbnails img {
  width: 0;
  min-width: 0;
  max-width: none;
  flex: 1 1 0;
  object-fit: cover;
}
.thumbnail-placeholder {
  width: 100%; display: grid; place-items: center; color: #909399; font-size: 12px;
  background: linear-gradient(110deg, #252525 30%, #353535 50%, #252525 70%);
}
.segment-label {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 28px;
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 0 7px;
  color: white;
  font-size: 11px;
  background: rgba(0, 0, 0, .72);
}
.segment-label span:last-child { margin-left: auto; }
.timeline-playhead {
  position: absolute;
  z-index: 4;
  top: 0;
  bottom: 6px;
  width: 28px;
  transform: translateX(-14px);
  cursor: ew-resize;
  touch-action: none;
}
.timeline-playhead::before {
  content: '';
  position: absolute;
  top: 0;
  bottom: 17px;
  left: 13px;
  width: 2px;
  background: #f56c6c;
  box-shadow: 0 0 0 1px rgba(255, 255, 255, .35);
}
.playhead-handle {
  position: absolute;
  left: 2px;
  bottom: 0;
  width: 24px;
  height: 16px;
  border-radius: 3px 3px 7px 7px;
  background: #f56c6c;
  box-shadow: 0 1px 4px rgba(0, 0, 0, .45);
}
.playhead-handle::before {
  content: '';
  position: absolute;
  top: 4px;
  left: 7px;
  width: 10px;
  height: 2px;
  border-top: 1px solid rgba(255, 255, 255, .8);
  border-bottom: 1px solid rgba(255, 255, 255, .8);
}
.timeline-playhead.dragging .playhead-handle { transform: scale(1.12); }
.transition-node {
  align-self: center;
  display: grid;
  place-items: center;
  flex: 0 0 58px;
  width: 58px;
  height: 48px;
  padding: 3px;
  border: 1px dashed var(--el-border-color-darker);
  border-radius: 8px;
  color: var(--el-text-color-secondary);
  background: var(--el-fill-color-light);
  cursor: pointer;
  transition: border-color .15s, color .15s, background .15s;
}
.transition-node:hover { border-color: var(--el-color-primary); color: var(--el-color-primary); }
.transition-node.active {
  border-style: solid;
  border-color: var(--el-color-primary);
  color: #fff;
  background: var(--el-color-primary);
}
.transition-node span {
  max-width: 50px;
  overflow: hidden;
  font-size: 11px;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.transition-node small { margin-top: 2px; font-size: 10px; }
.transition-editor { display: grid; gap: 12px; }
.transition-duration-row {
  display: grid;
  grid-template-columns: auto 1fr auto;
  gap: 8px;
  align-items: center;
}
.transition-duration-row :deep(.el-input-number) { width: 100%; }
.transition-audio-row {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 4px 10px;
  align-items: center;
}
.transition-audio-row small { grid-column: 1 / -1; color: var(--el-text-color-secondary); }
.timeline-thumbnail-loading {
  position: absolute;
  z-index: 8;
  top: 9px;
  left: 50%;
  display: flex;
  align-items: center;
  gap: 7px;
  max-width: calc(100% - 32px);
  padding: 7px 12px;
  border: 1px solid rgba(255, 255, 255, .16);
  border-radius: 16px;
  color: #fff;
  background: rgba(24, 28, 36, .88);
  box-shadow: 0 3px 12px rgba(0, 0, 0, .28);
  font-size: 12px;
  white-space: nowrap;
  pointer-events: none;
  transform: translateX(-50%);
}
.timeline-panel .el-alert { margin-top: auto; flex-shrink: 0; }
.editor-footer { justify-content: space-between; }
.editor-footer > span { color: var(--el-text-color-secondary); }
.editor-actions { display: flex; align-items: center; gap: 10px; }
.output-mode-label { color: var(--el-text-color-regular); white-space: nowrap; }
.output-mode-select { width: 150px; }
.output-file-name { width: 280px; }
@media (max-width: 900px) {
  .editor-layout { min-height: 520px; }
  .player-controls { gap: 7px; padding: 0 9px; }
  .volume-slider { flex-basis: 76px; width: 76px; }
  .time-label { min-width: 100px; }
  .preview-status { gap: 8px; }
  .preview-tip { display: none; }
  .timeline-toolbar { flex-wrap: wrap; }
  .timeline-toolbar > span { width: 100%; margin: 5px 0 0; }
  .editor-actions { flex-wrap: wrap; justify-content: flex-end; }
  .output-file-name { width: 220px; }
}
</style>
