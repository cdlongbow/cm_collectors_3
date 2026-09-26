<template>
  <h3 class="setting-section-title"><span><slot /></span></h3>
</template>

<style scoped>
.setting-section-title {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 28px 0 18px;
  color: var(--el-text-color-primary);
  font-size: 14px;
  font-weight: 600;
  line-height: 1.5;
}

.setting-section-title:first-child {
  margin-top: 16px;
}

.setting-section-title::before {
  content: '';
  width: 3px;
  height: 14px;
  flex: 0 0 3px;
  border-radius: 2px;
  background: var(--el-color-primary);
}

.setting-section-title::after {
  content: '';
  flex: 1;
  min-width: 16px;
  height: 1px;
  background: var(--el-border-color-lighter);
}

.setting-section-title span {
  min-width: 0;
  overflow-wrap: anywhere;
}
</style>
