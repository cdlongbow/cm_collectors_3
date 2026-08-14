<template>
  <dialogCommon ref="dialogRef" :title="title_C" width="520px" :btn-submit-title="submitTitle_C" @submit="submitHandle">
    <div class="performer-batch-dialog" v-loading="loading">
      <el-alert :title="`将处理已选择的 ${performerIds.length} 位演员`" type="info" :closable="false" show-icon />
      <el-form label-position="top">
        <el-form-item v-if="action === 'stars'" label="评星">
          <selectStarSet v-model="stars" />
          <div class="form-tip">设置为 0 星可清除已有评星。</div>
        </el-form-item>
        <template v-else-if="action === 'migrate'">
          <el-form-item label="迁移至演员集">
            <selectDatabasePerformer v-model="targetPerformerBasesId" />
          </el-form-item>
          <el-alert title="迁移后原演员标签会被清除，演员头像会移动到目标演员集目录。" type="warning" :closable="false" show-icon />
        </template>
        <template v-else-if="action === 'tags'">
          <el-form-item label="设置方式">
            <el-radio-group v-model="tagMode">
              <el-radio-button value="add">添加标签</el-radio-button>
              <el-radio-button value="remove">移除标签</el-radio-button>
              <el-radio-button value="replace">替换标签</el-radio-button>
            </el-radio-group>
          </el-form-item>
          <el-form-item label="演员标签">
            <performerTagSelect v-model="tagIds" :performer-bases-id="performerBasesId" />
          </el-form-item>
          <div class="form-tip">替换模式未选择标签时，将清空所选演员的全部标签。</div>
        </template>
      </el-form>
    </div>
  </dialogCommon>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import dialogCommon from '@/components/com/dialog/dialog-common.vue';
import selectStarSet from '@/components/com/form/selectStarSet.vue';
import selectDatabasePerformer from '@/components/com/form/selectDatabasePerformer.vue';
import performerTagSelect from './performerTagSelect.vue';
import { performerServer } from '@/server/performer.server';

type BatchAction = 'stars' | 'migrate' | 'tags';
const emits = defineEmits<{ success: [] }>();
const dialogRef = ref<InstanceType<typeof dialogCommon>>();
const action = ref<BatchAction>('stars');
const performerIds = ref<string[]>([]);
const performerBasesId = ref('');
const targetPerformerBasesId = ref('');
const stars = ref(0);
const tagIds = ref<string[]>([]);
const tagMode = ref<'add' | 'remove' | 'replace'>('add');
const loading = ref(false);
const title_C = computed(() => ({ stars: '批量设置评星', migrate: '批量迁移演员集', tags: '批量设置演员标签' })[action.value]);
const submitTitle_C = computed(() => action.value === 'migrate' ? '确认迁移' : '应用设置');

const open = (newAction: BatchAction, ids: string[], baseId: string) => {
  action.value = newAction;
  performerIds.value = [...ids];
  performerBasesId.value = baseId;
  targetPerformerBasesId.value = '';
  stars.value = 0;
  tagIds.value = [];
  tagMode.value = 'add';
  dialogRef.value?.open();
};

const submitHandle = async () => {
  if (performerIds.value.length === 0) return ElMessage.warning('请先选择演员');
  if (action.value === 'migrate' && !targetPerformerBasesId.value) return ElMessage.warning('请选择目标演员集');
  if (action.value === 'migrate' && targetPerformerBasesId.value === performerBasesId.value) return ElMessage.warning('目标演员集不能与当前演员集相同');
  if (action.value === 'tags' && tagMode.value !== 'replace' && tagIds.value.length === 0) return ElMessage.warning('请选择演员标签');
  if (action.value === 'migrate') {
    try {
      await ElMessageBox.confirm(`将迁移 ${performerIds.value.length} 位演员，并清除其原演员标签。是否继续？`, '确认批量迁移', { type: 'warning' });
    } catch { return; }
  }
  loading.value = true;
  try {
    const result = action.value === 'stars'
      ? await performerServer.batchSetStars(performerIds.value, stars.value)
      : action.value === 'migrate'
        ? await performerServer.batchMigrate(performerIds.value, targetPerformerBasesId.value)
        : await performerServer.batchSetTags(performerIds.value, performerBasesId.value, tagIds.value, tagMode.value);
    if (!result.status) return ElMessage.error(result.msg || '批量操作失败');
    ElMessage.success(`已更新 ${result.data.updated} 位演员`);
    dialogRef.value?.close();
    emits('success');
  } catch {
    ElMessage.error('批量操作失败，请稍后重试');
  } finally {
    loading.value = false;
  }
};

defineExpose({ open });
</script>

<style scoped lang="scss">
.performer-batch-dialog { display: flex; flex-direction: column; gap: 16px; min-height: 120px; }
.performer-batch-dialog .el-form { margin-top: 4px; }
.performer-batch-dialog :deep(.performer-tag-select-list) { width: 100%; }
.form-tip { margin-top: 6px; color: var(--el-text-color-secondary); font-size: 12px; }
:global(html.dark) .performer-batch-dialog { color: #e5eaf3; }
:global(html.bright) .performer-batch-dialog { color: #303133; }
</style>
