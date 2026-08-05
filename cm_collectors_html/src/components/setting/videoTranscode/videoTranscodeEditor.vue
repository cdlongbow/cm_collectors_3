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
        <videoPlay ref="videoPlayRef" class="editor-player" fit-container hide-controls />
        <div class="preview-status">
          <el-button size="small" type="primary" @click="togglePlayback">
            {{ isPlaying ? '暂停' : '播放' }}
          </el-button>
          <span>片段 {{ activeSegmentIndex + 1 }}/{{ segments.length }}</span>
          <span>成片 {{ formatTime(currentOutputTime) }} / {{ formatTime(editedDuration) }}</span>
          <span class="preview-tip">当前预览为保留片段按现有顺序拼接后的效果</span>
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
          <span>拖动片段可以调整成片顺序，点击片段可以定位播放。</span>
        </div>

        <div class="timeline-ruler">
          <span>00:00</span>
          <span>{{ formatTime(editedDuration / 2) }}</span>
          <span>{{ formatTime(editedDuration) }}</span>
        </div>
        <div v-if="segments.length" ref="timelineTrackRef" class="timeline-track">
          <div
            v-for="(segment, index) in segments"
            :key="segment.id"
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
const timelineTrackRef = ref<HTMLElement>();
const segments = ref<I_videoEditSegment[]>([]);
const undoStack = ref<I_videoEditSegment[][]>([]);
const redoStack = ref<I_videoEditSegment[][]>([]);
const thumbnailFrames = ref<ThumbnailFrame[]>([]);
const sourceDuration = ref(0);
const activeSegmentId = ref('');
const currentSourceTime = ref(0);
const initialSnapshot = ref('');
const initializing = ref(false);
const saving = ref(false);
const dragIndex = ref(-1);
const isPlaying = ref(false);
const draggingPlayhead = ref(false);
const playheadLeft = ref(2);
const outputMode = ref<'replace' | 'new_file'>('new_file');
const outputFileName = ref('');
let pollTimer: number | undefined;
let thumbnailLoadVersion = 0;
let initializedDurationFromPlayer = false;
let hadStoredEditPlan = false;
let closingAfterSave = false;

const cloneSegments = (items: I_videoEditSegment[]) => items.map(item => ({ ...item }));
const makeSegmentID = () => `segment-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
const snapshot = () => JSON.stringify({
  segments: segments.value.map(({ id, start, end }) => ({ id, start, end })),
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
const initializeEditor = async () => {
  initializing.value = true;
  closingAfterSave = false;
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
  updatePlayheadPosition();
  window.addEventListener('resize', updatePlayheadPosition);
  const { playUrl, playType } = await getPlayVideoURLAndType(props.task.dramaSeriesId);
  if (playUrl) {
    videoPlayRef.value?.setVideoSource(playUrl, playType, () => {
      seekToSegment(segments.value[0]);
    }, fileName(props.task.sourcePath));
  }
  startPolling();
  initializing.value = false;
  if (sourceDuration.value > 0) void ensureTimelineThumbnails(true);
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
      if (durationChanged || !thumbnailFrames.value.length) void ensureTimelineThumbnails(true);
    }
    const time = player.getCurrentTime();
    if (!Number.isFinite(time)) return;
    currentSourceTime.value = time;
    isPlaying.value = player.isPlaying();
    updatePlayheadPosition();
    const segment = activeSegment.value;
    if (!segment || !player.isPlaying()) return;
    if (time < segment.start - 0.05) {
      player.setCurrentTime(segment.start);
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

const desiredThumbnailTimes = (segment: I_videoEditSegment) => {
  const segmentDuration = segment.end - segment.start;
  const count = segments.value.length > 48
    ? (segment.id === activeSegmentId.value ? 1 : 0)
    : Math.max(1, Math.round(segmentDuration / Math.max(editedDuration.value, 0.1) * 18));
  return Array.from({ length: count }, (_, index) =>
    segment.start + segmentDuration * (index + 0.5) / count);
};
const ensureTimelineThumbnails = async (reset = false) => {
  if (reset) revokeThumbnails();
  const loadVersion = ++thumbnailLoadVersion;
  const targets = segments.value.flatMap(segment => {
    const count = Math.max(1, desiredThumbnailTimes(segment).length);
    const tolerance = Math.max(0.03, (segment.end - segment.start) / count * 0.2);
    return desiredThumbnailTimes(segment).map(time => ({ time, tolerance }));
  });
  const missingTimes = targets
    .filter(target => !thumbnailFrames.value.some(frame => Math.abs(frame.time - target.time) <= target.tolerance))
    .map(target => target.time);
  for (let index = 0; index < missingTimes.length; index += 3) {
    const batch = missingTimes.slice(index, index + 3);
    const results = await Promise.all(batch.map(async time => {
      const result = await videoTranscodeServer.thumbnail(props.task.id, time);
      if (!result.status || !result.data) return undefined;
      return { time, url: URL.createObjectURL(result.data) } satisfies ThumbnailFrame;
    }));
    const loaded = results.filter((item): item is ThumbnailFrame => Boolean(item));
    if (loadVersion !== thumbnailLoadVersion) {
      loaded.forEach(item => URL.revokeObjectURL(item.url));
      return;
    }
    thumbnailFrames.value.push(...loaded);
    thumbnailFrames.value.sort((left, right) => left.time - right.time);
  }
};

const revokeThumbnails = () => {
  thumbnailLoadVersion++;
  thumbnailFrames.value.forEach(item => URL.revokeObjectURL(item.url));
  thumbnailFrames.value = [];
};
const framesForSegment = (segment: I_videoEditSegment) => {
  const candidates = thumbnailFrames.value.filter(frame => frame.time >= segment.start && frame.time <= segment.end);
  if (!candidates.length) return [];
  const used = new Set<string>();
  return desiredThumbnailTimes(segment).flatMap(time => {
    const nearest = candidates
      .filter(frame => !used.has(frame.url))
      .sort((left, right) => Math.abs(left.time - time) - Math.abs(right.time - time))[0];
    if (!nearest) return [];
    used.add(nearest.url);
    return [nearest];
  });
};
const segmentStyle = (segment: I_videoEditSegment) => ({
  width: `${Math.max(4, (segment.end - segment.start) / Math.max(editedDuration.value, 0.1) * 100)}%`,
});
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
  const player = videoPlayRef.value;
  const segment = activeSegment.value;
  if (!player || !segment) return;
  if (player.isPlaying()) {
    player.pause();
    isPlaying.value = false;
    return;
  }
  if (currentSourceTime.value < segment.start || currentSourceTime.value >= segment.end - 0.04) {
    seekToSegment(segment);
  }
  void player.play();
  isPlaying.value = true;
};

const commitChange = (next: I_videoEditSegment[]) => {
  undoStack.value.push(cloneSegments(segments.value));
  redoStack.value = [];
  segments.value = next;
  if (!segments.value.some(item => item.id === activeSegmentId.value)) {
    activeSegmentId.value = segments.value[0]?.id || '';
  }
  void nextTick().then(() => {
    updatePlayheadPosition();
    void ensureTimelineThumbnails();
  });
};
const splitCurrentSegment = () => {
  if (!canSplit.value) return;
  const index = activeSegmentIndex.value;
  const segment = activeSegment.value;
  const splitAt = Math.round(currentSourceTime.value * 1000) / 1000;
  const left = { ...segment, id: makeSegmentID(), end: splitAt };
  const right = { ...segment, id: makeSegmentID(), start: splitAt };
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
  redoStack.value.push(cloneSegments(segments.value));
  segments.value = previous;
  seekToSegment(segments.value.find(item => item.id === activeSegmentId.value) || segments.value[0]);
  void ensureTimelineThumbnails();
};
const redo = () => {
  const next = redoStack.value.pop();
  if (!next) return;
  undoStack.value.push(cloneSegments(segments.value));
  segments.value = next;
  seekToSegment(segments.value.find(item => item.id === activeSegmentId.value) || segments.value[0]);
  void ensureTimelineThumbnails();
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
  const [moved] = next.splice(sourceIndex, 1);
  next.splice(targetIndex, 0, moved);
  commitChange(next);
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
  videoPlayRef.value?.releaseSource();
  stopPlayheadDrag();
  window.removeEventListener('resize', updatePlayheadPosition);
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
  videoPlayRef.value?.releaseSource();
  stopPlayheadDrag();
  window.removeEventListener('resize', updatePlayheadPosition);
  revokeThumbnails();
});
</script>

<style scoped lang="scss">
.editor-header,
.editor-footer,
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
.editor-player {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
.preview-status {
  min-height: 40px;
  flex-shrink: 0;
  gap: 20px;
  padding: 0 14px;
  color: #d6d6d6;
  font-size: 12px;
  background: #1f1f1f;
}
.preview-tip { margin-left: auto; color: #909399; }
.timeline-panel {
  flex: 0 0 auto;
  display: flex;
  flex-direction: column;
  padding: 10px 12px 4px;
  overflow: hidden;
}
.timeline-toolbar { gap: 4px; flex-shrink: 0; }
.timeline-toolbar > span { margin-left: auto; color: var(--el-text-color-secondary); font-size: 12px; }
.timeline-ruler {
  justify-content: space-between;
  margin-top: 8px;
  padding: 0 4px 5px;
  color: var(--el-text-color-secondary);
  font-size: 11px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.timeline-track {
  position: relative;
  min-height: 100px;
  box-sizing: border-box;
  flex-shrink: 0;
  display: flex;
  align-items: stretch;
  gap: 4px;
  overflow-x: auto;
  padding: 4px 14px 12px;
  scrollbar-width: thin;
}
.timeline-segment {
  position: relative;
  min-width: 78px;
  height: 70px;
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
  min-width: 70px;
  max-width: 240px;
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
.timeline-panel .el-alert { margin-top: auto; flex-shrink: 0; }
.editor-footer { justify-content: space-between; }
.editor-footer > span { color: var(--el-text-color-secondary); }
.editor-actions { display: flex; align-items: center; gap: 10px; }
.output-mode-label { color: var(--el-text-color-regular); white-space: nowrap; }
.output-mode-select { width: 150px; }
.output-file-name { width: 280px; }
@media (max-width: 900px) {
  .editor-layout { min-height: 520px; }
  .preview-status { gap: 8px; }
  .preview-tip { display: none; }
  .timeline-toolbar { flex-wrap: wrap; }
  .timeline-toolbar > span { width: 100%; margin: 5px 0 0; }
  .editor-actions { flex-wrap: wrap; justify-content: flex-end; }
  .output-file-name { width: 220px; }
}
</style>
