<template>
  <!-- 手机端列表：表格在窄屏只剩两三列无法阅读，改为一行一张卡片 -->
  <div class="mcl" v-loading="loading">
    <div v-for="(row, index) in data" :key="row[rowKey] ?? index" class="mcl-card" @click="$emit('row-click', row)">
      <div class="mcl-head">
        <div class="mcl-title">
          <slot name="title" :row="row" :index="index" />
        </div>
        <div v-if="$slots.tag" class="mcl-tag">
          <slot name="tag" :row="row" :index="index" />
        </div>
      </div>
      <div v-if="$slots.meta" class="mcl-meta">
        <slot name="meta" :row="row" :index="index" />
      </div>
      <div v-if="$slots.actions" class="mcl-actions" @click.stop>
        <slot name="actions" :row="row" :index="index" />
      </div>
    </div>
    <EmptyState v-if="!loading && !data.length" :description="emptyText" />
  </div>
</template>

<script setup>
import EmptyState from './EmptyState.vue'

defineProps({
  data: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false },
  rowKey: { type: String, default: 'id' },
  emptyText: { type: String, default: '暂无记录' }
})
defineEmits(['row-click'])
</script>

<style scoped>
.mcl {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 80px;
}
.mcl-card {
  background: var(--yx-surface);
  border: 1px solid var(--yx-border-soft);
  border-radius: var(--yx-radius);
  padding: 12px 14px;
}
.mcl-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}
.mcl-title {
  font-size: 15px;
  font-weight: 500;
  color: var(--yx-text-1);
  line-height: 1.45;
  min-width: 0;
  word-break: break-word;
}
.mcl-tag {
  flex-shrink: 0;
}
.mcl-meta {
  margin-top: 6px;
  font-size: 13px;
  color: var(--yx-text-3);
  line-height: 1.6;
  display: flex;
  flex-wrap: wrap;
  column-gap: 12px;
}
.mcl-actions {
  margin-top: 10px;
  padding-top: 8px;
  border-top: 1px solid var(--el-border-color-extra-light);
  display: flex;
  flex-wrap: wrap;
  gap: 4px 16px;
  align-items: center;
}
.mcl-actions :deep(.el-button.is-link) {
  font-size: 14px;
  margin-left: 0;
  padding: 4px 0;
}
</style>
