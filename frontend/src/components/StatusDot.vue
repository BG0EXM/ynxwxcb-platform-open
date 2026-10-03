<template>
  <span class="status-dot-wrap" :class="[`is-${type}`, { 'has-pulse': pulse }]">
    <span class="status-dot"></span>
    <span class="status-text">{{ text }}</span>
  </span>
</template>

<script setup>
defineProps({
  type: {
    type: String,
    default: 'info',
    validator: (v) => ['primary', 'success', 'warning', 'danger', 'info'].includes(v)
  },
  text: {
    type: String,
    required: true
  },
  pulse: {
    type: Boolean,
    default: false
  }
})
</script>

<style scoped>
.status-dot-wrap {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: var(--yx-fs-sm);
  line-height: 1;
}

.status-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
  position: relative;
}

.status-text {
  color: var(--yx-text-2);
}

/* 颜色状态映射 */
.is-primary .status-dot { background-color: var(--el-color-primary); }
.is-primary .status-text { color: var(--el-color-primary-dark-2); }

.is-success .status-dot { background-color: var(--el-color-success); }
.is-success .status-text { color: var(--el-color-success-dark-2); }

.is-warning .status-dot { background-color: var(--el-color-warning); }
.is-warning .status-text { color: var(--el-color-warning-dark-2); }

.is-danger .status-dot { background-color: var(--el-color-danger); }
.is-danger .status-text { color: var(--el-color-danger-dark-2); font-weight: 500; }

.is-info .status-dot { background-color: var(--yx-text-4); }
.is-info .status-text { color: var(--yx-text-3); }

/* 呼吸动画 */
.has-pulse .status-dot::after {
  content: '';
  position: absolute;
  top: -2px;
  left: -2px;
  right: -2px;
  bottom: -2px;
  border-radius: 50%;
  background: inherit;
  opacity: 0.6;
  animation: status-pulse 2s infinite ease-out;
}

@keyframes status-pulse {
  0% {
    transform: scale(1);
    opacity: 0.6;
  }
  100% {
    transform: scale(2.2);
    opacity: 0;
  }
}
</style>
