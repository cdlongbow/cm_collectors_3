<template>
  <div class="shared-fields">
    <SettingSectionTitle>刮削配置</SettingSectionTitle>
    <el-form-item label="监控文件后缀名">
      <selectVideoSuffixName
        v-model="config.videoSuffixName"
        multiple
        filterable
        allow-create
        default-first-option
      />
    </el-form-item>
    <el-form-item label="应用刮削器配置文件">
      <selectScraperConfig v-model="config.scraperConfigs" multiple />
    </el-form-item>
    <el-form-item label="并发处理数量">
      <el-input-number v-model="config.concurrency" :min="1" :max="10" />
    </el-form-item>
    <el-form-item label="重试次数">
      <el-input-number v-model="config.retryCount" :min="0" :max="10" />
    </el-form-item>
    <el-form-item label="超时时间">
      <el-input-number v-model="config.timeout" :min="1" :max="300" />
    </el-form-item>
    <el-form-item>
      <el-checkbox v-model="config.skipIfNfoExists" label="已存在nfo文件时跳过" />
      <el-checkbox v-model="config.saveNfo" label="保存元数据为nfo文件" />
    </el-form-item>
    <el-form-item>
      <div>
        <el-checkbox v-model="config.enableDownloadImages" label="下载元数据中的图片链接" />
        <el-checkbox v-model="config.useTagAsImageName" label="使用标签名作为图片名" />
        <el-checkbox v-model="config.enableUserSimulation" label="开启用户模拟操作" />
      </div>
      <div class="warning-list">
        <el-text type="warning">下载图片会耗费更多时间进行反反爬</el-text>
        <el-text type="warning">开启模拟操作会降低刮削速度，增加反反爬能力</el-text>
      </div>
    </el-form-item>
  </div>
</template>
<script setup lang="ts">
import SettingSectionTitle from '@/components/setting/SettingSectionTitle.vue';
import type { I_config_scraperData } from '@/dataType/config.dataType'
import selectVideoSuffixName from '@/components/com/form/selectVideoSuffixName.vue'
import selectScraperConfig from '@/components/com/form/selectScraperConfig.vue'
defineProps<{ config: I_config_scraperData }>()
</script>
<style scoped>
.shared-fields {
  min-width: 0;
}
.warning-list {
  display: flex;
  flex-direction: column;
  width: 100%;
}
.shared-fields :deep(.el-select) {
  min-width: 180px;
}
.color-picker-block,
.color-picker-btn,
.module-block,
.module-block-value-k {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}
.module-block-group {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.form-column-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
</style>
