<template>
  <div class="shared-config-bar" v-loading="busy">
    <template v-if="state">
      <div class="actions">
        <el-switch :model-value="state.following" :disabled="!state.available || editing" active-text="跟随公共配置"
          @change="toggleFollow" />
        <el-button v-if="!state.available" @click="savePublic">将当前配置设为公共配置</el-button>
        <el-button v-else-if="!editing" @click="editPublic">编辑公共配置</el-button>
        <template v-if="editing">
          <el-button type="primary" @click="savePublic">保存公共配置</el-button>
          <el-button @click="cancelEdit">取消编辑</el-button>
        </template>
      </div>
      <p>{{ editing ? '正在编辑公共配置，保存后影响所有跟随此分组的文件库。' : state.following ? '共享参数已锁定；关闭跟随后可独立修改，并保留当前值。' : '当前使用本库独立配置。' }}</p>
      <p>{{ localHint }}</p>
      <p v-if="editing">影响的文件库：{{ state.libraries.map(item => item.name).join('、') || '暂无' }}</p>
    </template>
    <el-button v-else @click="load">重新加载公共配置状态</el-button>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { sharedConfigServer, type SharedModule, type SharedConfigState } from '@/server/sharedConfig.server';
import { filesBasesServer } from '@/server/filesBases.server';
import { E_config_type } from '@/dataType/config.dataType';
import { appStoreData } from '@/storeData/app.storeData';

const props = defineProps<{ filesBasesId: string; module: SharedModule; config: object; localHint: string }>();
const emit = defineEmits<{ config: [value: object]; saved: [] }>();
const state = ref<SharedConfigState>();
const busy = ref(false);
const editing = ref(false);
let original: object | undefined;
let serial = 0;

const load = async () => {
  const requestSerial = ++serial;
  state.value = undefined;
  const result = await sharedConfigServer.status(props.filesBasesId, props.module);
  if (requestSerial !== serial) return;
  if (result.status) state.value = result.data;
  else ElMessage.error(result.msg);
};
const fieldDisabled = (key: string) => {
  if (!state.value || busy.value) return true;
  const shared = state.value.fields.includes(key);
  return editing.value ? !shared : state.value.following && shared;
};
const reloadConfig = async () => {
  const result = await filesBasesServer.getConfigById(props.filesBasesId, E_config_type.app);
  if (!result.status) throw new Error(result.msg);
  emit('config', { ...props.config, ...JSON.parse(result.data || '{}') });
  const app = appStoreData();
  if (app.currentFilesBases.id === props.filesBasesId) await app.initCurrentFilesBases(props.filesBasesId);
};
const toggleFollow = async (value: string | number | boolean) => {
  if (!state.value) return;
  try {
    await ElMessageBox.confirm(value ? '将使用公共配置替换本分组的共享参数。未保存的参数修改将丢弃，本库目录和关联项不受影响。' : '关闭跟随后保留当前生效配置，此后独立修改。', '切换配置来源');
    busy.value = true;
    const result = await sharedConfigServer.follow(props.filesBasesId, props.module, !!value, state.value.revision);
    if (!result.status) throw new Error(result.msg);
    await reloadConfig();
    await load();
    emit('saved');
  } catch (error) { if (error instanceof Error) ElMessage.error(error.message); }
  finally { busy.value = false; }
};
const editPublic = async () => {
  try {
    await load();
    if (!state.value) return;
    original = JSON.parse(JSON.stringify(props.config));
    emit('config', { ...props.config, ...state.value.config });
    editing.value = true;
  } catch (error) { if (error instanceof Error) ElMessage.error(error.message); }
};
const cancelEdit = () => {
  if (original) emit('config', original);
  original = undefined;
  editing.value = false;
};
const savePublic = async () => {
  if (!state.value) return;
  try {
    const names = state.value.libraries.map(item => item.name).join('、') || '暂无跟随库';
    await ElMessageBox.confirm(`保存公共配置，影响范围：${names}。本库只有开启跟随才会使用公共配置。`, '保存公共配置');
    busy.value = true;
    const result = await sharedConfigServer.save(props.module, state.value.revision, props.config);
    if (!result.status) throw new Error(result.msg);
    cancelEdit();
    await load();
    if (state.value?.following) await reloadConfig();
    ElMessage.success('公共配置已保存');
    emit('saved');
  } catch (error) { if (error instanceof Error) ElMessage.error(error.message); }
  finally { busy.value = false; }
};
watch(() => [props.filesBasesId, props.module], () => { cancelEdit(); void load(); }, { immediate: true });
defineExpose({ fieldDisabled, editing, savePublic, state });
</script>

<style scoped>
.shared-config-bar { padding: 12px; margin-bottom: 12px; border: 1px solid var(--el-border-color); border-radius: 6px; }
.actions { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
p { margin: 6px 0 0; font-size: 13px; color: var(--el-text-color-secondary); }
</style>
