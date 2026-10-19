<template>
  <!-- 列表页顶部统计条：常看的数字一眼可见；点击可按该口径筛选（可选） -->
  <div class="stat-strip" :style="{ '--mobile-cols': items.length === 3 ? 3 : 2 }">
    <div
      v-for="item in items"
      :key="item.label"
      class="stat-strip-item"
      :class="[item.tone ? 'tone-' + item.tone : '', { 'is-clickable': !!item.onClick }]"
      @click="item.onClick && item.onClick()"
    >
      <div class="stat-strip-label">{{ item.label }}</div>
      <div class="stat-strip-value tabular-nums">
        {{ item.value ?? 0 }}<span v-if="item.unit" class="stat-strip-unit">{{ item.unit }}</span>
      </div>
    </div>
  </div>
</template>

<script setup>
defineProps({
  items: { type: Array, default: () => [] }
})
</script>

<style scoped>
.stat-strip {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}
.stat-strip-item {
  background: var(--yx-surface);
  border: 1px solid var(--yx-border-soft);
  border-radius: var(--yx-radius);
  padding: 12px 16px;
  transition: border-color 0.15s ease;
}
.stat-strip-item.is-clickable {
  cursor: pointer;
}
.stat-strip-item.is-clickable:hover {
  border-color: var(--el-color-primary-light-5);
}
.stat-strip-label {
  font-size: 12px;
  color: var(--yx-text-3);
}
.stat-strip-value {
  margin-top: 4px;
  font-size: 22px;
  font-weight: 600;
  color: var(--yx-text-1);
  line-height: 1.2;
}
.stat-strip-unit {
  margin-left: 3px;
  font-size: 12px;
  font-weight: 400;
  color: var(--yx-text-3);
}
.tone-warning .stat-strip-value {
  color: var(--el-color-warning);
}
.tone-danger .stat-strip-value {
  color: var(--el-color-danger);
}
.tone-primary .stat-strip-value {
  color: var(--yx-ink-2);
}
@media (max-width: 768px) {
  .stat-strip {
    /* 3 项时一行排满，其余按两列排，避免出现落单的格子 */
    grid-template-columns: repeat(var(--mobile-cols, 2), minmax(0, 1fr));
    gap: 8px;
    margin-bottom: 12px;
  }
  .stat-strip-item {
    padding: 10px 12px;
  }
  .stat-strip-value {
    font-size: 19px;
  }
}
</style>
