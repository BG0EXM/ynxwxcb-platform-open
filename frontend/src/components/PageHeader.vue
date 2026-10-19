<template>
  <div class="page-header" :class="{ 'with-divider': divider }">
    <div class="page-header-main">
      <div class="page-header-title-wrap">
        <h1 class="page-title gov-serif">
          {{ title }}
          <span v-if="tag" class="page-tag">{{ tag }}</span>
        </h1>
        <p v-if="subtitle" class="page-subtitle">{{ subtitle }}</p>
      </div>
      <div v-if="$slots.actions" class="page-header-actions">
        <slot name="actions"></slot>
      </div>
    </div>
    <div v-if="$slots.extra" class="page-header-extra">
      <slot name="extra"></slot>
    </div>
  </div>
</template>

<script setup>
defineProps({
  title: {
    type: String,
    required: true
  },
  subtitle: {
    type: String,
    default: ''
  },
  tag: {
    type: String,
    default: ''
  },
  divider: {
    type: Boolean,
    default: true
  }
})
</script>

<style scoped>
/* 页面标题区：不再做成带阴影和三色条的卡片，直接落在页面底色上，标题左侧一道品牌红竖线 */
.page-header {
  margin-bottom: 16px;
  padding: 4px 2px 0;
}

.page-header-main {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
}

.page-header-title-wrap {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.page-title {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: var(--yx-text-1);
  letter-spacing: 0.5px;
  display: flex;
  align-items: center;
  gap: 10px;
  line-height: 1.3;
  border-left: 3px solid var(--yx-brand);
  padding-left: 10px;
}

.gov-serif {
  font-family: var(--yx-font-serif);
}

.page-tag {
  display: inline-block;
  font-size: 12px;
  font-family: var(--yx-font-sans);
  font-weight: normal;
  padding: 2px 8px;
  border-radius: 12px;
  background: var(--yx-ink-2-bg);
  color: var(--yx-ink-2);
}

.page-subtitle {
  margin: 0;
  padding-left: 13px;
  font-size: var(--yx-fs-xs);
  color: var(--yx-text-3);
  line-height: 1.4;
}

.page-header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.page-header-extra {
  margin-top: 12px;
}

/* 手机端：标题区收紧；操作按钮固定在屏幕底部，单手可点 */
@media (max-width: 768px) {
  .page-header {
    margin-bottom: 12px;
    padding-top: 0;
  }
  .page-title {
    font-size: 17px;
  }
  .page-subtitle {
    display: none;
  }
  .page-header-actions {
    position: fixed;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: 30;
    padding: 10px 16px calc(10px + env(safe-area-inset-bottom));
    background: var(--yx-surface);
    border-top: 1px solid var(--yx-border-soft);
    box-shadow: 0 -4px 16px rgba(19, 26, 38, 0.06);
    flex-wrap: nowrap;
  }
  .page-header-actions :deep(.el-button:not(.is-circle)) {
    flex: 1;
    height: 40px;
    margin-left: 0;
  }
}
</style>
