<template>
  <div class="empty-state">
    <div class="empty-icon-wrap">
      <svg class="empty-svg" viewBox="0 0 120 100" fill="none" xmlns="http://www.w3.org/2000/svg">
        <!-- 卷宗文件夹底层 -->
        <rect x="18" y="24" width="84" height="60" rx="8" fill="var(--el-fill-color)" stroke="var(--el-border-color)" stroke-width="2" />
        <!-- 卷宗标签折角 -->
        <path d="M26 24 L26 16 C26 13.8 27.8 12 30 12 L50 12 C52.2 12 54 13.8 54 16 L54 24 Z" fill="var(--el-fill-color-light)" stroke="var(--el-border-color)" stroke-width="2" />
        <!-- 内部公文纸张 -->
        <rect x="30" y="32" width="60" height="42" rx="4" fill="#ffffff" stroke="var(--el-border-color-lighter)" stroke-width="1.5" />
        <!-- 公文横线条 -->
        <path d="M38 42 H72" stroke="var(--el-border-color)" stroke-width="2" stroke-linecap="round" />
        <path d="M38 50 H82" stroke="var(--el-border-color-lighter)" stroke-width="2" stroke-linecap="round" />
        <path d="M38 58 H64" stroke="var(--el-border-color-lighter)" stroke-width="2" stroke-linecap="round" />
        <!-- 鎏金微章印迹 -->
        <circle cx="76" cy="62" r="6" fill="rgba(184, 146, 90, 0.15)" stroke="var(--yx-gold)" stroke-width="1.2" />
      </svg>
    </div>
    <p class="empty-text">{{ description || '暂无相关记录' }}</p>
    <div v-if="$slots.action" class="empty-action">
      <slot name="action"></slot>
    </div>
  </div>
</template>

<script setup>
defineProps({
  description: {
    type: String,
    default: '暂无相关记录'
  }
})
</script>

<style scoped>
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 20px;
  text-align: center;
}

.empty-icon-wrap {
  width: 120px;
  height: 100px;
  margin-bottom: 12px;
}

.empty-svg {
  width: 100%;
  height: 100%;
}

.empty-text {
  margin: 0;
  font-size: var(--yx-fs-sm);
  color: var(--yx-text-3);
  letter-spacing: 0.5px;
}

.empty-action {
  margin-top: 14px;
}
</style>
