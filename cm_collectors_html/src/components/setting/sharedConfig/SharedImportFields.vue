<template>
  <div class="shared-fields">
    <el-divider content-position="left">导入配置</el-divider>
    <el-form-item label="监控文件后缀名">
      <selectVideoSuffixName
        v-model="config.videoSuffixName"
        multiple
        filterable
        allow-create
        default-first-option
      />
      <div><el-checkbox v-model="config.autoGetVideoDefinition" label="自动获取视频清晰度" /></div>
    </el-form-item>
    <el-form-item label="资源命名方式">
      <el-radio-group v-model="config.resourceNamingMode" size="small">
        <el-radio-button label="文件名" value="fileName" />
        <el-radio-button label="目录名" value="dirName" />
        <el-radio-button label="目录名+文件名" value="dirFileName" />
        <el-radio-button label="全路径名" value="fullPathName" />
      </el-radio-group>
    </el-form-item>
    <el-form-item label="导入方式">
      <div class="form-column-list">
        <el-radio-group v-model="config.importMode" size="small">
          <el-radio-button label="追加导入" value="append" />
          <el-radio-button label="覆盖导入" value="cover" />
        </el-radio-group>
        <div><el-text type="warning">覆盖导入会更新已存在的数据并导入新资源</el-text></div>
        <div>
          <el-text type="warning"
            >覆盖导入当多个资源指向同一视频地址时，仅更新最后的资源记录</el-text
          >
        </div>
      </div>
    </el-form-item>
    <el-form-item label="封面海报匹配名">
      <el-select
        v-model="config.coverPosterMatchName"
        multiple
        filterable
        allow-create
        default-first-option
      >
        <el-option
          v-for="(item, key) in dataset.coverPosterMatchName"
          :key="key"
          :label="item"
          :value="item"
        />
      </el-select>
      <el-switch
        v-model="config.coverPosterFuzzyMatch"
        active-text="模糊匹配"
        inactive-text="严格匹配"
      />
      <el-checkbox
        v-model="config.coverPosterUseRandomImageIfNoMatch"
        label="匹配的封面失败时，使用目录下随机图片"
      />
      <div>
        <el-text type="warning">
          以regex:开头，可以使用正则表达式匹配。例如：regex:^@fileName-poster$
          其中@fileName代表文件名</el-text
        >
      </div>
    </el-form-item>
    <el-form-item label="封面海报后缀名">
      <selectImageSuffixName
        v-model="config.coverPosterSuffixName"
        multiple
        filterable
        allow-create
        default-first-option
      />
    </el-form-item>
    <el-form-item>
      <el-checkbox
        v-model="config.autoCreatePoster"
        label="(未找到封面海报) 自动截取视频内容作封面海报"
      />
    </el-form-item>
    <el-form-item>
      <el-checkbox v-model="config.folderToSeries" label="将同一文件夹下的多个视频文件合并为剧集" />
    </el-form-item>
    <el-form-item>
      <div class="form-column-list">
        <el-checkbox
          v-model="config.similarNameToSeries"
          label="将同一文件夹下名称相近的视频文件合并为剧集"
        />
        <div>
          <el-text type="warning"
            >适合连续剧文件名只差集数、分段号或少量字符的情况；开启“同一文件夹合并”时会优先使用完整文件夹合并。</el-text
          >
        </div>
      </div>
    </el-form-item>
    <el-form-item
      v-if="config.folderToSeries || config.similarNameToSeries"
      label="合并后的剧集排序"
    >
      <div class="form-column-list">
        <el-select v-model="config.folderToSeriesSortMode">
          <el-option label="保持现有顺序（新增分集追加到末尾）" value="keep" />
          <el-option label="文件名称正序" value="nameAsc" />
          <el-option label="文件名称倒序" value="nameDesc" />
          <el-option label="文件大小正序（小文件在前）" value="sizeAsc" />
          <el-option label="文件大小倒序（大文件在前）" value="sizeDesc" />
        </el-select>
        <div>
          <el-text type="warning">新增视频后会按所选方式重新排列该资源的全部分集。</el-text>
        </div>
      </div>
    </el-form-item>
    <el-form-item>
      <div class="form-column-list">
        <el-checkbox v-model="config.enableNfoFuzzyMatch" label="开启nfo模糊匹配" />
        <div><el-text type="warning">例如：abc.mp4 可以匹配到：abc-C.nfo</el-text></div>
      </div>
    </el-form-item>
    <el-form-item>
      <div class="form-column-list">
        <el-checkbox
          v-model="config.useRandomNfoIfNoneMatch"
          label="开启nfo无法匹配时，使用目录下随机nfo文件"
        />
        <div>
          <el-text type="warning"
            >如果找不到与视频名称相同的nfo，自动使用视频目录下的一个nfo文件</el-text
          >
        </div>
      </div>
    </el-form-item>
    <el-divider content-position="left">nfo配置</el-divider>
    <el-form-item>
      <div>
        <div><el-checkbox v-model="config.nfo.nfoStatus" label="导入nfo文件" /></div>
        <div><el-text type="warning">次级节点标签请使用 . 链接</el-text></div>
      </div>
    </el-form-item>
    <el-form-item label="根节点">
      <el-select v-model="config.nfo.roots" multiple filterable allow-create default-first-option>
        <el-option
          v-for="(item, index) in dataset.nfo.roots"
          :key="index"
          :label="item"
          :value="item"
        />
      </el-select>
    </el-form-item>
    <el-form-item label="标题">
      <el-select v-model="config.nfo.titles" multiple filterable allow-create default-first-option>
        <el-option
          v-for="(item, index) in dataset.nfo.titles"
          :key="index"
          :label="item"
          :value="item"
        />
      </el-select>
    </el-form-item>
    <el-form-item label="版号番号">
      <el-select
        v-model="config.nfo.issueNumbers"
        multiple
        filterable
        allow-create
        default-first-option
      >
        <el-option
          v-for="(item, index) in dataset.nfo.issueNumbers"
          :key="index"
          :label="item"
          :value="item"
        />
      </el-select>
    </el-form-item>
    <el-form-item label="发行日期">
      <el-select
        v-model="config.nfo.issuingDates"
        multiple
        filterable
        allow-create
        default-first-option
      >
        <el-option
          v-for="(item, index) in dataset.nfo.issuingDates"
          :key="index"
          :label="item"
          :value="item"
        />
      </el-select>
    </el-form-item>
    <el-form-item label="评分">
      <el-select v-model="config.nfo.score" multiple filterable allow-create default-first-option>
        <el-option
          v-for="(item, index) in dataset.nfo.score"
          :key="index"
          :label="item"
          :value="item"
        />
      </el-select>
    </el-form-item>
    <el-form-item label="摘要简介">
      <el-select
        v-model="config.nfo.abstracts"
        multiple
        filterable
        allow-create
        default-first-option
      >
        <el-option
          v-for="(item, index) in dataset.nfo.abstracts"
          :key="index"
          :label="item"
          :value="item"
        />
      </el-select>
    </el-form-item>
    <el-form-item label="标签">
      <el-select v-model="config.nfo.tags" multiple filterable allow-create default-first-option>
        <el-option
          v-for="(item, index) in dataset.nfo.tags"
          :key="index"
          :label="item"
          :value="item"
        />
      </el-select>
      <el-checkbox v-model="config.nfo.tagAutoCreate" label="自动添加标签" />
    </el-form-item>
    <el-form-item label="演员姓名">
      <el-select
        v-model="config.nfo.performerNames"
        multiple
        filterable
        allow-create
        default-first-option
      >
        <el-option
          v-for="(item, index) in dataset.nfo.performerNames"
          :key="index"
          :label="item"
          :value="item"
        />
      </el-select>
      <el-checkbox v-model="config.nfo.performerMatchAliasName" label="同时匹配别名" />
      <el-checkbox v-model="config.nfo.performerAutoCreate" label="自动添加演员" />
    </el-form-item>
    <el-form-item label="演员头像">
      <el-select
        v-model="config.nfo.performerThumbs"
        multiple
        filterable
        allow-create
        default-first-option
      >
        <el-option
          v-for="(item, index) in dataset.nfo.performerThumbs"
          :key="index"
          :label="item"
          :value="item"
        />
      </el-select>
    </el-form-item>
  </div>
</template>
<script setup lang="ts">
import type { I_config_scanDisk } from '@/dataType/config.dataType'
import selectVideoSuffixName from '@/components/com/form/selectVideoSuffixName.vue'
import selectImageSuffixName from '@/components/com/form/selectImageSuffixName.vue'
import dataset from '@/assets/dataset'
defineProps<{ config: I_config_scanDisk }>()
</script>
<style scoped>
.shared-fields {
  min-width: 0;
}
.shared-fields :deep(.el-form-item__content) {
  gap: 8px;
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
