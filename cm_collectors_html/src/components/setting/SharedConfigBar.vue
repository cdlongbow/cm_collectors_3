<template>
  <div class="shared-config-bar" v-loading="busy">
    <template v-if="state">
      <div class="actions">
        <el-switch
          :model-value="state.following"
          :disabled="!state.available || dialogOpen"
          active-text="跟随公共配置"
          @change="toggleFollow"
        />
        <el-tag :type="state.following ? 'primary' : 'info'">{{
          state.following ? '来源：公共配置' : '来源：本库设置'
        }}</el-tag>
        <el-button type="primary" plain @click="openEditor">{{
          state.available ? '打开公共配置' : '创建公共配置…'
        }}</el-button>
      </div>
      <p>
        {{
          state.following
            ? '本库的通用参数由公共配置提供，点击“打开公共配置”查看或修改。'
            : '下方参数仅保存到当前文件库。'
        }}
      </p>
      <p>{{ localHint }}</p>
    </template>
    <el-button v-else @click="retryLoad">重新加载公共配置状态</el-button>
  </div>

  <el-dialog
    v-model="dialogOpen"
    :title="'公共配置 · ' + moduleName"
    width="min(960px, 94vw)"
    top="5vh"
    append-to-body
    destroy-on-close
    :close-on-click-modal="false"
    :close-on-press-escape="!busy"
    :show-close="!busy"
    :before-close="closeEditor"
  >
    <div v-if="dialogOpen" class="public-editor" v-loading="busy">
      <el-alert
        :title="editorRevision === 0 ? '创建公共配置' : '正在编辑公共配置'"
        type="info"
        :closable="false"
        :description="
          editorRevision === 0
            ? '以当前库的通用参数为起点。保存后，各库可自行开启跟随。'
            : '保存后，以下跟随库会使用新参数；独立配置的库不受影响。'
        "
      />
      <div class="affected-libraries">
        <strong>跟随此配置的文件库（{{ state?.libraries.length || 0 }}）</strong>
        <div class="library-tags">
          <el-tag v-for="library in state?.libraries" :key="library.id">{{ library.name }}</el-tag>
          <span v-if="!state?.libraries.length">暂无，保存不会自动让任何库开启跟随。</span>
        </div>
      </div>
      <div class="public-form-scroll">
        <el-form label-width="auto" :disabled="busy">
          <SharedDisplayFields
            v-if="module === 'display'"
            :config="draft as unknown as I_config_app"
          />
          <SharedImportFields
            v-else-if="module === 'import'"
            :config="draft as unknown as I_config_scanDisk"
          />
          <SharedScraperFields v-else :config="draft as unknown as I_config_scraperData" />
        </el-form>
      </div>
    </div>
    <template #footer>
      <el-button :disabled="busy" @click="closeEditor()">取消</el-button>
      <el-button type="primary" :loading="busy" @click="savePublic">保存公共配置</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, h, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  sharedConfigServer,
  type SharedModule,
  type SharedConfigState,
} from '@/server/sharedConfig.server'
import { filesBasesServer } from '@/server/filesBases.server'
import {
  E_config_type,
  createDefaultConfigApp,
  defualtConfigScanDisk,
  defualtConfigScraperData,
  type I_config_app,
  type I_config_scanDisk,
  type I_config_scraperData,
} from '@/dataType/config.dataType'
import { appStoreData } from '@/storeData/app.storeData'
import SharedDisplayFields from './sharedConfig/SharedDisplayFields.vue'
import SharedImportFields from './sharedConfig/SharedImportFields.vue'
import SharedScraperFields from './sharedConfig/SharedScraperFields.vue'

const props = defineProps<{
  filesBasesId: string
  module: SharedModule
  config: object
  localHint: string
}>()
const emit = defineEmits<{ config: [value: object]; saved: [] }>()
const state = ref<SharedConfigState>()
const busy = ref(false)
const dialogOpen = ref(false)
const draft = ref<Record<string, unknown>>({})
const editorRevision = ref(0)
const moduleName = computed(
  () => ({ display: '基础展示', import: '导入规则', scraper: '刮削参数' })[props.module],
)
let serial = 0
const context = () => props.filesBasesId + ':' + props.module
const clone = <T,>(value: T): T => JSON.parse(JSON.stringify(value))

const load = async () => {
  const requestSerial = ++serial
  const result = await sharedConfigServer.status(props.filesBasesId, props.module)
  if (requestSerial !== serial) return
  if (!result.status) throw new Error(result.msg)
  state.value = result.data
  return result.data
}
const retryLoad = () => {
  void load().catch((error) => ElMessage.error(String(error)))
}
const refreshApp = async () => {
  const app = appStoreData()
  if (props.module === 'display' && app.currentFilesBases.id === props.filesBasesId)
    await app.initCurrentFilesBases(props.filesBasesId)
}
const reloadConfig = async () => {
  const before = context()
  const result = await filesBasesServer.getConfigById(
    props.filesBasesId,
    props.module === 'display'
      ? E_config_type.app
      : props.module === 'import'
        ? E_config_type.importScanDisk
        : E_config_type.scraper,
  )
  if (before !== context()) return
  if (!result.status) throw new Error(result.msg)
  const defaults = props.module === 'display' ? createDefaultConfigApp()
    : props.module === 'import' ? defualtConfigScanDisk : defualtConfigScraperData
  emit('config', { ...clone(defaults), ...JSON.parse(result.data || '{}') })
  await refreshApp()
}
const toggleFollow = async (value: string | number | boolean) => {
  if (!state.value || busy.value) return
  const before = context()
  try {
    let detachMode: 'keep' | 'restore' = 'keep'
    const option = (mode: 'keep' | 'restore', label: string, disabled = false) =>
      h('label', { style: 'display:flex;gap:8px;align-items:center;margin:14px 0;cursor:pointer;opacity:' + (disabled ? '0.5' : '1') }, [
        h('input', { type: 'radio', name: 'detach-shared-config', value: mode, checked: mode === detachMode, disabled, onChange: () => { detachMode = mode } }),
        h('span', label),
      ])
    await ElMessageBox.confirm(
      value
        ? '开启后使用公共配置，并备份本库已保存的通用参数，关闭时可选择恢复。页面上未保存的修改会丢弃，请先保存需要保留的修改。'
        : h('div', [
            h('p', '关闭后，本分组将独立设置。请选择通用参数的来源：'),
            option('keep', '保留当前公共配置，作为本库配置'),
            option('restore', '恢复跟随前的本库配置', !state.value.canRestore),
            !state.value.canRestore ? h('p', '此次跟随没有历史快照，无法恢复开启前的参数。') : null,
            h('p', '本库已保存的目录和关联项保持当前值；页面上未保存的修改会丢弃。'),
          ]),
      '切换配置来源',
    )
    if (before !== context()) return
    busy.value = true
    const result = await sharedConfigServer.follow(
      props.filesBasesId,
      props.module,
      !!value,
      state.value.revision,
      detachMode,
    )
    if (!result.status) throw new Error(result.msg)
    if (before !== context()) return
    await reloadConfig()
    await load()
    emit('saved')
  } catch (error) {
    if (error instanceof Error) ElMessage.error(error.message)
  } finally {
    busy.value = false
  }
}
const openEditor = async () => {
  if (busy.value) return
  try {
    busy.value = true
    const latest = await load()
    if (!latest) return
    const defaults =
      props.module === 'display'
        ? createDefaultConfigApp()
        : props.module === 'import'
          ? defualtConfigScanDisk
          : defualtConfigScraperData
    const source = {
      ...clone(defaults),
      ...clone(latest.available ? latest.config : props.config),
    } as Record<string, unknown>
    // 弹窗只持有公共字段的深复制，不能带入或修改本库目录、标签引用及未保存草稿。
    draft.value = Object.fromEntries(
      latest.fields.filter((key) => key in source).map((key) => [key, source[key]]),
    )
    editorRevision.value = latest.revision
    dialogOpen.value = true
  } catch (error) {
    if (error instanceof Error) ElMessage.error(error.message)
  } finally {
    busy.value = false
  }
}
const closeEditor = (done?: () => void) => {
  if (busy.value) return
  dialogOpen.value = false
  draft.value = {}
  done?.()
}
const savePublic = async () => {
  if (!state.value || !dialogOpen.value || busy.value) return
  const before = context()
  const revision = editorRevision.value
  const payload = clone(draft.value)
  try {
    busy.value = true
    const latest = await load()
    if (!latest || before !== context()) return
    if (latest.revision !== revision)
      throw new Error('公共配置已被其他页面修改，请关闭弹窗后重新打开。当前草稿仍保留。')
    const names = latest.libraries.map((item) => item.name).join('、') || '暂无跟随库'
    await ElMessageBox.confirm(
      '保存' + moduleName.value + '公共配置，影响范围：' + names + '。',
      '保存公共配置',
    )
    if (before !== context()) return
    const result = await sharedConfigServer.save(props.module, revision, payload)
    if (!result.status) throw new Error(result.msg)
    const saved = await load()
    if (!saved || before !== context()) return
    if (saved.following) {
      // 只更新生效公共值，保留页面中尚未保存的本库独立字段。
      emit('config', { ...props.config, ...clone(saved.config) })
      await refreshApp()
    }
    dialogOpen.value = false
    draft.value = {}
    ElMessage.success('公共配置已保存')
    emit('saved')
  } catch (error) {
    if (error instanceof Error) ElMessage.error(error.message)
  } finally {
    busy.value = false
  }
}
watch(
  () => [props.filesBasesId, props.module],
  () => {
    serial++
    dialogOpen.value = false
    draft.value = {}
    state.value = undefined
    retryLoad()
  },
  { immediate: true },
)
defineExpose({ dialogOpen, state })
</script>

<style scoped>
.shared-config-bar {
  padding: 14px;
  margin-bottom: 16px;
  border: 1px solid var(--el-border-color);
  border-radius: 8px;
  background: var(--el-fill-color-light);
}
.actions,
.library-tags {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
p {
  margin: 8px 0 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}
.affected-libraries {
  padding: 16px 0;
}
.library-tags {
  margin-top: 8px;
  max-height: 80px;
  overflow-y: auto;
}
.library-tags span {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.public-form-scroll {
  max-height: clamp(180px, calc(90vh - 290px), 58vh);
  overflow-y: auto;
  padding: 0 14px 0 0;
}
</style>
