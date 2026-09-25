<template>
  <div class="mode-scan-disk" v-loading="loading">
    <SharedConfigBar v-if="configReady" ref="sharedBar" :files-bases-id="store.appStoreData.currentFilesBases.id" module="import"
      :config="formData" local-hint="扫描目录和本库封面预设独立保存；公共参数不会同步目录或已有资源。" @config="applySharedConfig" />
    <div class="block">
      <el-alert title="本库扫描目录" type="success" :closable="false" />
      <ul class="scan-list">
        <li v-for="(item, index) in formData.scanDiskPaths" :key="index">
          <el-input v-model="formData.scanDiskPaths[index]" :disabled="true">
            <template #append>
              <el-button icon="Delete" @click="deleteDiskLocationHandle(index)" />
            </template>
          </el-input>
        </li>
      </ul>
      <div class="tool">
        <el-button type="primary" plain @click="addDiskLocationHandle">添加文件夹位置</el-button>
        <el-button v-if="store.appStoreData.runtimeBridgeStatus" icon="Folder" type="primary" plain
          @click="selectLocalDirectoryHandle">选择本地文件夹</el-button>
      </div>
    </div>
    <el-form :model="formData" label-width="160px">
      <el-form-item label="封面海报类型">
          <el-select v-model="formData.coverPosterType">
            <el-option label="自适应尺寸" :value="-1" />
            <el-option v-for="item, index in store.appStoreData.currentConfigApp.coverPosterData" :key="index"
              :label="item.name" :value="index" />
          </el-select>
        </el-form-item>
      <SharedImportFields v-if="!sharedBar?.state?.following" :config="formData" />
    </el-form>
  </div>
  <serverFileManagementDialog ref="serverFileManagementDialogRef" @selectedFiles="selectedFilesHandle"
    :show="[E_sfm_FileType.Directory]">
  </serverFileManagementDialog>
  <modeScanDiskImportDataDialog ref="modeScanDiskImportDataDialogRef" @success="successHandle">
  </modeScanDiskImportDataDialog>
</template>
<script setup lang="ts">
import { ref } from 'vue';
import SharedConfigBar from '@/components/setting/SharedConfigBar.vue';
import SharedImportFields from '@/components/setting/sharedConfig/SharedImportFields.vue';
import type { I_sfm_FileEntry } from '@/components/serverFileManagement/com/dataType';
import { E_sfm_FileType } from '@/components/serverFileManagement/com/dataType';
import serverFileManagementDialog from '@/components/serverFileManagement/serverFileManagementDialog.vue';
import modeScanDiskImportDataDialog from './modeScanDiskImportDataDialog.vue';
import { appStoreData } from '@/storeData/app.storeData';
import { defualtConfigScanDisk, E_config_type, type I_config_scanDisk } from '@/dataType/config.dataType';
import { filesBasesServer } from '@/server/filesBases.server';
import { ElMessage } from 'element-plus';
import { debounceNow } from '@/assets/debounce';
import { importDataServer } from '@/server/importData.server';
import { openDirectoryDialog } from '@/common/runtimeBridge';
const store = {
  appStoreData: appStoreData(),
}
const emits = defineEmits(['success'])

const serverFileManagementDialogRef = ref<InstanceType<typeof serverFileManagementDialog>>();
const modeScanDiskImportDataDialogRef = ref<InstanceType<typeof modeScanDiskImportDataDialog>>();

const sharedBar = ref<InstanceType<typeof SharedConfigBar>>();
const configReady = ref(false);
const applySharedConfig = (config: object) => { formData.value = config as I_config_scanDisk; };
const loading = ref(false)
const formData = ref<I_config_scanDisk>(JSON.parse(JSON.stringify(defualtConfigScanDisk)))

const init = async () => {
  configReady.value = false;
  await getConfig();
}

const getConfig = async () => {
  try {
    loading.value = true;
    const result = await filesBasesServer.getConfigById(store.appStoreData.currentFilesBases.id, E_config_type.importScanDisk);
    if (!result.status) {
      ElMessage.error(result.msg);
      return;
    }
    const configStr = result.data;
    if (configStr != '') {
      const config = JSON.parse(configStr);
      const legacySortMode = config.folderToSeriesSort === true ? 'nameAsc' : 'keep';
      formData.value = {
        ...JSON.parse(JSON.stringify(defualtConfigScanDisk)),
        ...config,
        folderToSeriesSortMode: config.folderToSeriesSortMode ?? legacySortMode,
      };
      delete formData.value.folderToSeriesSort;
    } else {
      formData.value = JSON.parse(JSON.stringify(defualtConfigScanDisk));
    }
    configReady.value = true;
  } catch (error) {
    console.log(error);
  } finally {
    loading.value = false;
  }
}

const getConfigScanDisk = (): I_config_scanDisk => {
  const configData = JSON.parse(JSON.stringify(formData.value));
  if (configData.coverPosterType == -1) {
    configData.coverPosterWidth = 0;
    configData.coverPosterHeight = 0;
  } else {
    const coverPosterData = store.appStoreData.currentConfigApp.coverPosterData[configData.coverPosterType];
    if (coverPosterData) {
      configData.coverPosterWidth = coverPosterData.width;
      configData.coverPosterHeight = coverPosterData.height;
    }
  }
  return configData
}

const submit = debounceNow(async () => {
  if (!configReady.value || !sharedBar.value?.state) { ElMessage.warning('请等待配置加载完成'); return; }
  if (sharedBar.value.dialogOpen) { ElMessage.warning('请先保存或取消公共配置编辑，再执行任务'); return; }
  if (formData.value.scanDiskPaths.length == 0) {
    ElMessage.error('请先设置监控路径');
    return;
  }
  try {
    const configData = JSON.parse(JSON.stringify(getConfigScanDisk()));

    const taskFilesBasesId = store.appStoreData.currentFilesBases.id;
    loading.value = true;
    const result = await importDataServer.scanDiskImportPaths(taskFilesBasesId, configData);
    if (!result.status) {
      ElMessage.error(result.msg);
      return;
    } else if (result.data.length == 0) {
      ElMessage.error('没有可导入的数据');
      return;
    } else {
      modeScanDiskImportDataDialogRef.value?.open(result.data, configData, taskFilesBasesId);
    }
  } catch (error) {
    ElMessage.error(String(error));
  } finally {
    loading.value = false;
  }
});

const saveConfig = debounceNow(async () => {
  if (!configReady.value || !sharedBar.value?.state) { ElMessage.warning('请等待配置加载完成'); return; }
  if (sharedBar.value.dialogOpen) { ElMessage.warning('请在公共配置弹窗中保存'); return; }
  try {
    loading.value = true;
    const configData = JSON.parse(JSON.stringify(getConfigScanDisk()));
    const result = await importDataServer.updateScanDiskConfig(store.appStoreData.currentFilesBases.id, configData);
    if (!result.status) {
      ElMessage.error(result.msg);
      return;
    } else {
      ElMessage.success('保存成功');
    }
  } catch (error) {
    ElMessage.error(String(error));
  } finally {
    loading.value = false;
  }
})

const addDiskLocationHandle = () => {
  serverFileManagementDialogRef.value?.open();
}
const selectedFilesHandle = (slc: I_sfm_FileEntry[]) => {
  if (slc.length == 0) {
    return;
  }
  slc.forEach(item => {
    if (item.is_dir && !formData.value.scanDiskPaths.includes(item.path)) {
      formData.value.scanDiskPaths.push(item.path);
    }
  });
}

const selectLocalDirectoryHandle = async () => {
  const path = await openDirectoryDialog('选择导入文件夹');
  if (path && !formData.value.scanDiskPaths.includes(path)) {
    formData.value.scanDiskPaths.push(path);
  }
}

const deleteDiskLocationHandle = (index: number) => {
  formData.value.scanDiskPaths.splice(index, 1);
}

const successHandle = () => {
  emits('success')
}

defineExpose({ init, submit, saveConfig })

</script>
<style lang="scss" scoped>
.mode-scan-disk {
  width: 100%;
  height: 100%;
  overflow-y: auto;

  .block {
    .form-column-list {
      line-height: normal;
    }

    .el-alert {
      margin: 0 0 10px 0;
    }

    .scan-list {
      list-style-type: none;
      display: flex;
      flex-direction: column;
      gap: 5px;
    }

    .tool {
      padding: 10px 0;
    }

    .el-form {
      width: 90%;

    }
  }
}
</style>
