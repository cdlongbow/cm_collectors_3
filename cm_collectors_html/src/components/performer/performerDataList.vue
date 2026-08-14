<template>
  <div class="performer-data-list">
    <performerInfo class="performer-info" v-if="props.showPerformerInfo && !batchMode" :performer="currentShowPerformer">
    </performerInfo>
    <aside v-else-if="props.showPerformerInfo" class="performer-batch-summary">
      <h3>批量管理</h3>
      <div class="batch-summary-count">已选择 <strong>{{ selectedPerformerIds.size }}</strong> 位演员</div>
      <el-scrollbar class="batch-selected-scrollbar">
        <div v-for="performer in selectedPerformers_C" :key="performer.id" class="batch-selected-item">
          <el-image :src="getPerformerPhoto(performer)" fit="cover" />
          <span :title="performer.name">{{ performer.name }}</span>
          <el-button link icon="Close" aria-label="取消选择" @click="togglePerformerSelection(performer)" />
        </div>
        <div v-if="selectedPerformerIds.size > selectedPerformers_C.length" class="batch-selected-overflow">
          还有 {{ selectedPerformerIds.size - selectedPerformers_C.length }} 位演员已选择
        </div>
        <el-empty v-if="selectedPerformerIds.size === 0" description="点击演员卡片进行选择" :image-size="70" />
      </el-scrollbar>
      <div class="batch-selection-tip">点击卡片选择，按住 Shift 可连续选择本页演员。</div>
    </aside>
    <div class="performer-container">
      <div class="performer-index">
        <span v-for="item, index in indexChars" :key="index" :class="{ 'select-index': item === selectIndex }"
          @click="selectCharIndexHandle(item)">
          {{ item }}
        </span>
      </div>
      <div class="performer-container-main">
        <performerSearch class="performer-search-toolbar" :admin="true" :performer-bases-id="props.performerBasesId"
          :batch-mode="batchMode" :selection-count="selectedPerformerIds.size" :total="dataCount" :selecting-all="selectingAll"
          @add="addPerformerHandle" @recycleBin="recycleBinHandle" @search="changeSearchHandle" @scraper="scraperHandle"
          @avatarBatch="avatarBatchHandle" @batchEnter="enterBatchMode" @batchExit="exitBatchMode"
          @selectPage="selectCurrentPage" @selectAll="selectAllFiltered" @invertPage="invertCurrentPage"
          @clearSelection="clearSelection" @batchStars="openBatchOperation('stars')"
          @batchMigrate="openBatchOperation('migrate')" @batchTags="openBatchOperation('tags')"
          @batchDelete="batchDeleteHandle">
        </performerSearch>
        <div class="performer-list-main" v-loading="loading">
          <el-scrollbar>
            <ul class="performer-list" :style="{ '--performer-block-size': performerBlockSize + 'px' }">
              <li v-for="(performer, index) in dataList" :key="performer.id"
                :class="{ 'batch-card-selected': selectedPerformerIds.has(performer.id) }">
                <div v-if="batchMode" class="performer-batch-card" @click="selectPerformerHandle(performer, index, $event)">
                  <el-checkbox class="batch-card-checkbox" :model-value="selectedPerformerIds.has(performer.id)"
                    @click.stop @change="togglePerformerSelection(performer)" />
                  <performerBlock :performer="performer" :attrAge="true" :attrNationality="true" />
                </div>
                <performerRightClickMenu v-else :performer="performer" @search="searchPerformerHandle"
                  @avatar="avatarHandle" @edit="editPerformerHandle" @migrate="migratePerformerHanadle" @delete="deletePerformerHandle">
                  <performerBlock :performer="performer" :tool="true" :admin="true" :attrAge="true"
                    :attrNationality="true" @search="searchPerformerHandle"
                    @click.stop="clickPerformerHandle(performer)" @edit="editPerformerHandle(performer)"
                    @delete="deletePerformerHandle(performer)">
                  </performerBlock>
                </performerRightClickMenu>
              </li>
            </ul>
          </el-scrollbar>
        </div>
        <div class="performer-paging">
          <el-pagination background layout="total, prev, pager, next, jumper" v-model:current-page="currentPage"
            :total="dataCount" :page-size="pageSize" @change="changePageHandle" size="small" />
          <div class="performer-size-slider">
            <el-slider v-model="performerBlockSize" :min="performerBlockSizeMin" :max="performerBlockSizeMax" :step="1"
              size="small" @change="changePerformerBlockSizeHandle" />
          </div>
        </div>
      </div>
    </div>
  </div>
  <performerFormDrawer ref="performerFormDrawerRef" :performerBasesId="props.performerBasesId"
    @success="getDataListAndCount" />
  <performerRecycleBinDialog ref="performerRecycleBinDialogRef" :performerBasesId="props.performerBasesId"
    @success="getDataListAndCount">
  </performerRecycleBinDialog>
  <scraperPerformerDialog ref="scraperPerformerDialogRef" @success="getDataListAndCount">
  </scraperPerformerDialog>
  <performerMigrateDialog ref="performerMigrateDialogRef" @success="getDataListAndCount" />
  <performerAvatarLibraryDialog ref="performerAvatarLibraryDialogRef" @success="getDataListAndCount" />
  <performerAvatarLibraryBatchDialog ref="performerAvatarLibraryBatchDialogRef" :page-size="pageSize"
    @success="getDataListAndCount" />
  <performerBatchOperationDialog ref="performerBatchOperationDialogRef" @success="batchOperationSuccess" />
</template>
<script lang="ts" setup>
import { computed, ref, onMounted } from 'vue';
import performerFormDrawer from '@/components/performer/performerFormDrawer.vue';
import performerRecycleBinDialog from '@/components/performer/performerRecycleBinDialog.vue';
import scraperPerformerDialog from '../importResource/scraperPerformerDialog.vue';
import performerSearch from '@/components/performer/performerSearch.vue';
import performerInfo from '@/components/performer/performerInfo.vue';
import performerBlock from '@/components/performer/performerBlock.vue';
import type { I_performer, I_search_performer } from '@/dataType/performer.dataType';
import { performerServer } from '@/server/performer.server';
import { ElMessage, ElMessageBox } from 'element-plus';
import { messageBoxConfirm } from '../../common/messageBox';
import { searchStoreData } from '@/storeData/search.storeData';
import { useRouter } from 'vue-router';
import performerRightClickMenu from './performerRightClickMenu.vue';
import performerMigrateDialog from './performerMigrateDialog.vue';
import performerAvatarLibraryDialog from './performerAvatarLibraryDialog.vue';
import performerAvatarLibraryBatchDialog from './performerAvatarLibraryBatchDialog.vue';
import performerBatchOperationDialog from './performerBatchOperationDialog.vue';
import { getPerformerPhoto } from '@/common/photo';
const router = useRouter()
const store = {
  searchStoreData: searchStoreData(),
}

const props = defineProps({
  performerBasesId: {
    type: String,
    default: '',
  },
  showPerformerInfo: {
    type: Boolean,
    default: true,
  },
  countFilesBasesId: {
    type: String,
    default: '',
  },
})
const performerFormDrawerRef = ref<InstanceType<typeof performerFormDrawer>>();
const performerRecycleBinDialogRef = ref<InstanceType<typeof performerRecycleBinDialog>>();
const scraperPerformerDialogRef = ref<InstanceType<typeof scraperPerformerDialog>>();
const performerMigrateDialogRef = ref<InstanceType<typeof performerMigrateDialog>>();
const performerAvatarLibraryDialogRef = ref<InstanceType<typeof performerAvatarLibraryDialog>>();
const performerAvatarLibraryBatchDialogRef = ref<InstanceType<typeof performerAvatarLibraryBatchDialog>>();
const performerBatchOperationDialogRef = ref<InstanceType<typeof performerBatchOperationDialog>>();
const loading = ref(false);
const dataList = ref<I_performer[]>([]);
const dataCount = ref(0);
let fetchCount = true;
const currentPage = ref(1);
const pageSize = ref(90);
const performerBlockSizeMin = 100;
const performerBlockSizeMax = 200;
const performerBlockSizeStorageKey = `performer-block-size-${props.performerBasesId || 'default'}`;
const getStoragePerformerBlockSize = () => {
  const storageValue = parseInt(localStorage.getItem(performerBlockSizeStorageKey) || '', 10);
  if (Number.isNaN(storageValue)) {
    return performerBlockSizeMin;
  }
  return Math.min(performerBlockSizeMax, Math.max(performerBlockSizeMin, storageValue));
}
const performerBlockSize = ref(getStoragePerformerBlockSize());
const indexChars = ref(['ALL', 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z'])
const selectIndex = ref('ALL');
let searchCondition: I_search_performer = {
  search: '',
  star: '',
  cup: '',
  charIndex: '',
  tagIds: [],
  tagMatchMode: 'any',
  sort: 'createdAtDesc',
}

const currentShowPerformer = ref<I_performer | undefined>(undefined);
const batchMode = ref(false);
const selectingAll = ref(false);
const selectedPerformerIds = ref<Set<string>>(new Set());
const selectedPerformerMap = ref<Map<string, I_performer>>(new Map());
const lastSelectedPageIndex = ref<number | null>(null);
const selectedPerformers_C = computed(() => Array.from(selectedPerformerMap.value.values()).slice(0, 100));

const init = async () => {
  await getDataListAndCount(true);
  if (dataList.value.length > 0) {
    currentShowPerformer.value = dataList.value[0];
  }
}

const getDataListAndCount = async (fetchCountStatus: boolean = true) => {
  fetchCount = fetchCountStatus;
  await getDataList();
}
const getDataList = async () => {
  loading.value = true;
  searchCondition.charIndex = selectIndex.value;
  const result = await performerServer.dataList(props.performerBasesId, fetchCount, currentPage.value, pageSize.value, searchCondition, props.countFilesBasesId);
  if (result && result.status) {
    dataList.value = result.data.dataList;
    if (currentShowPerformer.value) {
      currentShowPerformer.value = dataList.value.find(item => item.id === currentShowPerformer.value?.id) || dataList.value[0];
    }
    if (fetchCount) {
      dataCount.value = result.data.total;
      fetchCount = false;
    }
  } else {
    ElMessage.error(result.msg);
  }
  loading.value = false;
}

const changePageHandle = () => {
  lastSelectedPageIndex.value = null;
  getDataList();
}

const clickPerformerHandle = (data: I_performer) => {
  currentShowPerformer.value = data;
}

const setPerformerSelected = (performer: I_performer, selected: boolean) => {
  const ids = new Set(selectedPerformerIds.value);
  const performers = new Map(selectedPerformerMap.value);
  if (selected) {
    ids.add(performer.id);
    performers.set(performer.id, performer);
  } else {
    ids.delete(performer.id);
    performers.delete(performer.id);
  }
  selectedPerformerIds.value = ids;
  selectedPerformerMap.value = performers;
}

const togglePerformerSelection = (performer: I_performer) => {
  setPerformerSelected(performer, !selectedPerformerIds.value.has(performer.id));
}

const selectPerformerHandle = (performer: I_performer, index: number, event: MouseEvent) => {
  if (event.shiftKey && lastSelectedPageIndex.value !== null) {
    const start = Math.min(lastSelectedPageIndex.value, index);
    const end = Math.max(lastSelectedPageIndex.value, index);
    const shouldSelect = !selectedPerformerIds.value.has(performer.id);
    dataList.value.slice(start, end + 1).forEach(item => setPerformerSelected(item, shouldSelect));
  } else {
    togglePerformerSelection(performer);
  }
  lastSelectedPageIndex.value = index;
}

const enterBatchMode = () => { batchMode.value = true; };
const clearSelection = () => {
  selectedPerformerIds.value = new Set();
  selectedPerformerMap.value = new Map();
  lastSelectedPageIndex.value = null;
};
const exitBatchMode = () => { batchMode.value = false; clearSelection(); };
const selectCurrentPage = () => dataList.value.forEach(item => setPerformerSelected(item, true));
const invertCurrentPage = () => dataList.value.forEach(item => togglePerformerSelection(item));

const selectAllFiltered = async () => {
  if (dataCount.value === 0) return;
  selectingAll.value = true;
  try {
    const pageCount = Math.ceil(dataCount.value / pageSize.value);
    const selected = new Map(selectedPerformerMap.value);
    for (let page = 1; page <= pageCount; page++) {
      const result = await performerServer.dataList(props.performerBasesId, false, page, pageSize.value, searchCondition, props.countFilesBasesId);
      if (!result?.status) throw new Error(result?.msg || '读取筛选结果失败');
      result.data.dataList.forEach(item => selected.set(item.id, item));
    }
    selectedPerformerMap.value = selected;
    selectedPerformerIds.value = new Set(selected.keys());
    ElMessage.success(`已选择全部 ${dataCount.value} 位筛选结果`);
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '读取筛选结果失败');
  } finally {
    selectingAll.value = false;
  }
}

const selectedIDsOrWarn = () => {
  const ids = Array.from(selectedPerformerIds.value);
  if (ids.length === 0) ElMessage.warning('请先选择演员');
  return ids;
}

const openBatchOperation = (action: 'stars' | 'migrate' | 'tags') => {
  const ids = selectedIDsOrWarn();
  if (ids.length > 0) performerBatchOperationDialogRef.value?.open(action, ids, props.performerBasesId);
}

const batchDeleteHandle = async () => {
  const ids = selectedIDsOrWarn();
  if (ids.length === 0) return;
  try {
    await ElMessageBox.confirm(`将把选中的 ${ids.length} 位演员移入回收站，是否继续？`, '批量删除演员', {
      type: 'warning', confirmButtonText: '移入回收站',
    });
  } catch { return; }
  try {
    const result = await performerServer.batchUpdateStatus(ids, false);
    if (!result.status) return ElMessage.error(result.msg || '批量删除失败');
    ElMessage.success(`已将 ${result.data.updated} 位演员移入回收站`);
    await batchOperationSuccess();
  } catch {
    ElMessage.error('批量删除失败，请稍后重试');
  }
}

const batchOperationSuccess = async () => {
  clearSelection();
  await getDataListAndCount(true);
}

const searchPerformerHandle = (data: I_performer) => {
  store.searchStoreData.setQueryPerformer(data.id, data.name)
  router.push(`/`)
}

const addPerformerHandle = () => {
  performerFormDrawerRef.value?.open('add')
}
const editPerformerHandle = async (data: I_performer) => {
  const result = await performerServer.infoById(data.id);
  if (!result?.status) {
    ElMessage.error(result?.msg || '加载演员详情失败');
    return;
  }
  performerFormDrawerRef.value?.open('edit', result.data)
}

const migratePerformerHanadle = (data: I_performer) => {
  performerMigrateDialogRef.value?.open(data)
}

const deletePerformerHandle = (performer: I_performer) => {
  messageBoxConfirm({
    text: '确定要删除吗？',
    successCallBack: async () => {
      const result = await performerServer.updateStatus(performer.id, false);
      if (result && result.status) {
        getDataListAndCount()
      } else {
        ElMessage.error(result.msg);
      }
    },
    failCallBack: () => {
      //console.log('取消删除')
    },
  })
}

const recycleBinHandle = () => {
  performerRecycleBinDialogRef.value?.open()
}

const changeSearchHandle = (search: I_search_performer) => {
  searchCondition = search;
  currentPage.value = 1;
  fetchCount = true;
  clearSelection();
  getDataList();
}
const selectCharIndexHandle = (charIndex: string) => {
  selectIndex.value = charIndex;
  currentPage.value = 1;
  fetchCount = true;
  clearSelection();
  getDataList();
}

const scraperHandle = () => {
  scraperPerformerDialogRef.value?.open(props.performerBasesId)
}

const avatarHandle = (performer: I_performer) => {
  performerAvatarLibraryDialogRef.value?.open(performer)
}

const avatarBatchHandle = () => {
  performerAvatarLibraryBatchDialogRef.value?.open(props.performerBasesId)
}

const changePerformerBlockSizeHandle = (newVal: number | number[]) => {
  if (typeof newVal === 'number') {
    localStorage.setItem(performerBlockSizeStorageKey, newVal.toString());
  }
}

onMounted(async () => {
  await init()
})

</script>
<style lang="scss" scoped>
.performer-data-list {
  width: 100%;
  height: 100%;
  overflow: hidden;
  display: flex;
  gap: 5px;

  .performer-info {
    flex-shrink: 0;
    width: 260px;
    height: 100%;
  }

  .performer-batch-summary {
    flex: 0 0 260px;
    min-width: 0;
    height: 100%;
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 14px 10px;
    border: 1px solid var(--el-border-color);
    border-radius: 6px;
    background: var(--el-bg-color-overlay);
    color: var(--el-text-color-primary);

    h3 { margin: 0; color: var(--el-color-primary); }
    .batch-summary-count { padding-bottom: 8px; border-bottom: 1px solid var(--el-border-color); color: var(--el-text-color-regular); }
    .batch-selected-scrollbar { flex: 1; min-height: 0; }
    .batch-selected-item {
      display: grid;
      grid-template-columns: 38px minmax(0, 1fr) 28px;
      align-items: center;
      gap: 8px;
      margin-bottom: 7px;
      padding: 6px;
      border: 1px solid var(--el-border-color-lighter);
      border-radius: 5px;
      background: var(--el-bg-color);
      font-size: 13px;
    }
    .batch-selected-item .el-image { width: 38px; height: 38px; border-radius: 50%; }
    .batch-selected-item span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .batch-selected-overflow { padding: 8px; text-align: center; color: var(--el-text-color-secondary); font-size: 12px; }
    .batch-selection-tip { color: var(--el-text-color-secondary); font-size: 12px; line-height: 1.5; }
  }

  .performer-container {
    flex: 1;
    overflow: hidden;
    display: flex;
    gap: 5px;

    .performer-index {
      width: 40px;
      display: flex;
      flex-direction: column;
      gap: 3px;

      span {
        display: block;
        width: 100%;
        height: 21px;
        line-height: 21px;
        text-align: center;
        background-color: #262727;
        border-radius: 2px;
        cursor: pointer;
        /*禁止选择 */
        -webkit-user-select: none;
        -moz-user-select: none;
        -ms-user-select: none;
        user-select: none;
      }

      span:hover {
        background-color: #3d3f3f;
      }

      .select-index {
        background-color: #E67F23;
      }

      .select-index:hover {
        background-color: #E67F23;
      }
    }

    .performer-container-main {
      flex: 1;
      display: flex;
      flex-direction: column;

      .performer-search-toolbar {
        flex-shrink: 0;
        display: flex;
      }

      .performer-list-main {
        flex-grow: 1;
        overflow: hidden;
        padding: 0.5em 0;

        .performer-list {
          list-style-type: none;
          display: flex;
          flex-wrap: wrap;
          align-content: flex-start;
          gap: 0.5em;

          li {
            width: var(--performer-block-size);
          }

          .performer-batch-card {
            position: relative;
            border: 2px solid transparent;
            border-radius: 7px;
            box-sizing: border-box;
            transition: border-color 0.16s ease, background-color 0.16s ease, box-shadow 0.16s ease;
          }

          .performer-batch-card:hover { border-color: var(--el-color-primary-light-5); }
          .batch-card-selected .performer-batch-card {
            border-color: var(--el-color-primary);
            background: var(--el-color-primary-light-9);
            box-shadow: 0 0 0 1px var(--el-color-primary-light-7);
          }

          .batch-card-checkbox {
            position: absolute;
            top: 7px;
            left: 7px;
            z-index: 5;
            width: 19px;
            height: 19px;
            padding: 1px;
            border-radius: 3px;
            background: var(--el-bg-color-overlay);
            box-shadow: 0 1px 4px rgba(0, 0, 0, 0.35);
          }
        }
      }

      .performer-paging {
        flex-shrink: 0;
        padding-top: 5px;
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 16px;

        .performer-size-slider {
          flex: 0 0 160px;
          margin-right: 16px;
        }
      }
    }
  }
}

:global(html.dark) .performer-batch-summary { border-color: #414243; background: #202225; color: #e5eaf3; }
:global(html.dark) .performer-batch-summary .batch-selected-item { border-color: #414243; background: #1d1e1f; }
:global(html.dark) .batch-card-selected .performer-batch-card { border-color: #409eff; background: #18222c; box-shadow: 0 0 0 1px #245b86; }
:global(html.bright) .performer-batch-summary { border-color: #dcdfe6; background: #ffffff; color: #303133; }
:global(html.bright) .performer-batch-summary .batch-selected-item { border-color: #e4e7ed; background: #f7f8fa; }
:global(html.bright) .batch-card-selected .performer-batch-card { border-color: #409eff; background: #ecf5ff; box-shadow: 0 0 0 1px #a0cfff; }
</style>
