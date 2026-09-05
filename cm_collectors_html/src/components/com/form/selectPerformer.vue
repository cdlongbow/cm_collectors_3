<template>
  <el-select-v2 v-model="selectVal" clearable :style="{ width: props.width }" @change="changeHandle"
    @clear="handleClear" :multiple="props.multiple" filterable :options="options" :loading="loading"
    :filter-method="filterMethod" :props="selectProps">
    <template v-if="editable" #label="{ label, value }">
      <span title="右键编辑演员" @contextmenu.prevent.stop="openPerformerMenu($event, value, label)">{{ label }}</span>
    </template>
    <template #default="{ item }">
      <div class="performer-item">
        <div class="name">{{ item.name }}</div>
        <div class="aliasName" v-if="item.aliasName != ''">({{ item.aliasName }})</div>
      </div>
    </template>
  </el-select-v2>
  <performerMenu v-if="editable" ref="performerMenuRef" :title="menuPerformer.name" :items="performerMenuItems" />
</template>
<script setup lang="ts">
import { debounce } from '@/assets/debounce';
import type { E_performerCareerType } from '@/dataType/app.dataType';
import type { I_performerBasic } from '@/dataType/performer.dataType';
import { performerServer } from '@/server/performer.server';
import { ElMessage } from 'element-plus';
import { ref, onMounted, type PropType, onActivated, computed, watch } from 'vue';
import performerMenu from '../tool/rightMenu/performerMenu.vue';

const selectVal = defineModel<string | string[]>({ type: [String, Array], default: "" as string | string[] });
const props = defineProps({
  width: {
    type: String,
    default: '100%',
  },
  multiple: {
    type: Boolean,
    default: false
  },
  performerBasesIds: {
    type: Array<string>,
    default: () => []
  },
  careerType: {
    type: String as PropType<E_performerCareerType>,
    default: 'all'
  },
  editable: { type: Boolean, default: false },
  editLoading: { type: Boolean, default: false },
  selectedData: { type: Array as PropType<I_performerBasic[]>, default: () => [] },
})
const emit = defineEmits(['change', 'edit'])
const list = ref<I_performerBasic[]>([]);
const queryText = ref('');
const savedNames = ref<Record<string, I_performerBasic>>({});
watch(() => props.selectedData, items => {
  items.forEach(item => { savedNames.value[item.id] = item; });
}, { immediate: true });
const resolvedList = computed(() => {
  const items = new Map(list.value.map(item => [item.id, savedNames.value[item.id] || item]));
  // 编辑职业后仍保留已关联演员，并优先展示最新保存的数据。
  props.selectedData.forEach(item => items.set(item.id, item));
  return [...items.values()];
});
const options = computed(() => resolvedList.value.filter(item => {
  const query = queryText.value;
  return !query || item.name.toLowerCase().includes(query) || item.aliasName.toLowerCase().includes(query) || item.keyWords.toLowerCase().includes(query);
}));
const performerMenuRef = ref<InstanceType<typeof performerMenu>>();
const menuPerformer = ref({ id: '', name: '' });
const performerMenuItems = computed(() => [{
  label: '演员编辑',
  icon: 'Edit',
  disabled: props.editLoading,
  handler: () => emit('edit', menuPerformer.value.id),
}]);
const openPerformerMenu = (event: MouseEvent, value: string, label: string) => {
  menuPerformer.value = { id: value, name: label };
  performerMenuRef.value?.show(event);
};
const loading = ref(false);

// 使用固定的对象引用，避免每次渲染时创建新对象导致滚动位置重置
const selectProps = computed(() => ({
  label: 'name',
  value: 'id'
}));


const init = async () => {
  list.value = [];
  queryText.value = '';
  await getPerformerList();
}
const getPerformerList = async () => {
  loading.value = true;
  console.log(props.performerBasesIds);
  const result = await performerServer.basicList(props.performerBasesIds, props.careerType)
  if (!result.status) {
    ElMessage.error(result.msg)
    return
  }
  list.value = result.data
  loading.value = false;
}

const changeHandle = () => {
  emit('change', selectVal.value || '')
}
const handleClear = () => {
  if (props.multiple) {
    selectVal.value = [];
  } else {
    selectVal.value = '';
  }
}

const filterMethod = debounce((query: string) => {
  queryText.value = query.toLowerCase();
}, 200)

const resetOptionsData = async () => {
  await getPerformerList();
}
const getOptionsData = (): I_performerBasic[] => {
  return options.value
}


onMounted(async () => {
  await init();
})
onActivated(async () => {
  await init();
})

defineExpose({
  resetOptionsData,
  getOptionsData
})

</script>
<style lang="scss" scoped>
.performer-item {
  display: flex;
  gap: 10px;
}
</style>
