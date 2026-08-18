<template>
  <dialogCommon ref="dialogCommonRef" title="视频关键帧截图" width="915px" top="10vh" btnSubmitTitle="设置封面海报"
    @submit="submitHandle">
    <div class="video-keyframe-poster-dialog" v-loading="loading">
      <button v-for="item, index in base64Images" :key="index" type="button" class="keyframe-option"
        :class="{ selected: selectedIndex === index }" :aria-label="`选择第 ${index + 1} 个视频关键帧`"
        :aria-pressed="selectedIndex === index" @click="selectImageHandle(index)">
        <el-image :src="item" fit="contain" />
        <span v-if="selectedIndex === index" class="selection-badge">
          <el-icon>
            <Check />
          </el-icon>
          已选择
        </span>
      </button>
    </div>
  </dialogCommon>
</template>
<script lang="ts" setup>
import { debounceNow } from '@/assets/debounce';
import dialogCommon from '@/components/com/dialog/dialog-common.vue';
import { ffmpegServer } from '@/server/ffmpeg.server';
import { Check } from '@element-plus/icons-vue';
import { ElMessage } from 'element-plus';
import { nextTick, ref } from 'vue';

const emits = defineEmits(['selectImage'])

const dialogCommonRef = ref<InstanceType<typeof dialogCommon>>();

const loading = ref(false);
const base64Images = ref<string[]>([]);
const selectedIndex = ref<number>(0);

const init = async (_videoPath: string, frameCount: number) => {
  base64Images.value = [];
  selectedIndex.value = 0;
  await getVKeyFramePosters(_videoPath, frameCount)
}

const getVKeyFramePosters = async (_videoPath: string, frameCount: number) => {
  try {
    loading.value = true;
    const result = await ffmpegServer.getVideoThumbnails(_videoPath, frameCount);
    if (!result.status) {
      ElMessage.error(result.msg);
      return;
    }
    base64Images.value = result.data;
  } catch (error) {
    console.log(error);
  } finally {
    loading.value = false;
  }
}

const selectImageHandle = (index: number) => {
  selectedIndex.value = index;
}

const submitHandle = debounceNow(() => {
  close();
  emits('selectImage', base64Images.value[selectedIndex.value])
})

const open = (_videoPath: string, frameCount: number) => {
  dialogCommonRef.value?.open();
  nextTick(() => {
    init(_videoPath, frameCount);
  });
}
const close = () => {
  dialogCommonRef.value?.close();
}

defineExpose({ open })
</script>
<style scoped lang="scss">
.video-keyframe-poster-dialog {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  width: 100%;
  height: 70vh;
  overflow-y: auto;

  .keyframe-option {
    position: relative;
    display: flex;
    align-self: flex-start;
    padding: 3px;
    overflow: hidden;
    background: var(--el-fill-color-light);
    border: 2px solid var(--el-border-color-darker);
    border-radius: 8px;
    cursor: pointer;
    transition: border-color 0.15s ease, box-shadow 0.15s ease, transform 0.15s ease;

    &:hover {
      border-color: var(--el-color-primary);
      transform: translateY(-1px);
    }

    &:focus-visible {
      outline: 3px solid var(--el-color-primary-light-5);
      outline-offset: 2px;
    }

    &.selected {
      background: var(--el-color-primary-light-9);
      border-color: var(--el-color-primary);
      box-shadow: 0 0 0 3px var(--el-color-primary-light-5), 0 4px 12px rgb(0 0 0 / 18%);
    }

    .el-image {
      display: block;
      max-width: 200px;
      border-radius: 4px;
    }

    .selection-badge {
      position: absolute;
      top: 8px;
      right: 8px;
      display: inline-flex;
      gap: 4px;
      align-items: center;
      padding: 4px 8px;
      color: #fff;
      font-size: 13px;
      font-weight: 600;
      line-height: 1;
      background: var(--el-color-primary);
      border: 1px solid rgb(255 255 255 / 80%);
      border-radius: 999px;
      box-shadow: 0 2px 8px rgb(0 0 0 / 35%);
      pointer-events: none;
    }
  }
}

:global(html.bright) .video-keyframe-poster-dialog .keyframe-option {
  background: #fff;
  border-color: #909399;
}

:global(html.bright) .video-keyframe-poster-dialog .keyframe-option.selected {
  background: #ecf5ff;
  border-color: #337ecc;
  box-shadow: 0 0 0 3px #a0cfff, 0 4px 12px rgb(0 0 0 / 20%);
}

:global(html.dark) .video-keyframe-poster-dialog .keyframe-option {
  background: #1d1e1f;
  border-color: #606266;
}

:global(html.dark) .video-keyframe-poster-dialog .keyframe-option:hover {
  border-color: #79bbff;
}

:global(html.dark) .video-keyframe-poster-dialog .keyframe-option.selected {
  background: #18222c;
  border-color: #79bbff;
  box-shadow: 0 0 0 3px rgb(64 158 255 / 55%), 0 4px 14px rgb(0 0 0 / 55%);
}
</style>
