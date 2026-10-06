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
.page-header {
  margin-bottom: 20px;
  background: var(--yx-surface);
  padding: 18px 24px;
  border-radius: var(--yx-radius);
  border: 1px solid rgba(220, 223, 229, 0.6);
  border-bottom: 1px solid var(--el-border-color);
  box-shadow: 0 4px 18px rgba(19, 26, 38, 0.05);
  position: relative;
  overflow: hidden;
}

.page-header::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 3px;
  background: var(--yx-crest-gradient);
  z-index: 2;
}

.page-header.with-divider::after {
  content: '';
  position: absolute;
  left: 20px;
  right: 20px;
  bottom: 0;
  height: 1px;
  background: linear-gradient(90deg, var(--yx-gold-light) 0%, transparent 60%);
}

.page-header-main {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
}

.page-header-title-wrap {
  display: flex;
  flex-direction: column;
  gap: 4px;
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
}

.gov-serif {
  font-family: 'SimSun', 'Songti SC', serif;
}

.page-tag {
  display: inline-block;
  font-size: 12px;
  font-family: var(--yx-font-sans);
  font-weight: normal;
  padding: 2px 8px;
  border-radius: 12px;
  background: var(--yx-brand-bg);
  color: var(--yx-brand);
  border: 1px solid rgba(158, 27, 30, 0.2);
}

.page-subtitle {
  margin: 0;
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
  padding-top: 12px;
  border-top: 1px dashed var(--el-border-color-lighter);
}

@media (max-width: 768px) {
  .page-header {
    padding: 12px 14px;
    margin-bottom: 14px;
  }
  .page-title {
    font-size: 17px;
  }
  .page-header-main {
    flex-direction: column;
    align-items: flex-start;
  }
  .page-header-actions {
    width: 100%;
    justify-content: flex-start;
  }
}
</style>
