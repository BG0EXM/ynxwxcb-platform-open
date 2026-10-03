<template>
  <div 
    class="stat-card yx-card-gold" 
    :class="{ 'is-clickable': clickable }"
    @click="handleClick"
  >
    <div class="stat-card-body">
      <div class="stat-card-info">
        <span class="stat-card-label">{{ label }}</span>
        <div class="stat-card-value-wrap">
          <span class="stat-card-value tabular-nums">{{ value }}</span>
          <span v-if="unit" class="stat-card-unit">{{ unit }}</span>
        </div>
        <span v-if="hint" class="stat-card-hint">{{ hint }}</span>
      </div>
      <div v-if="icon" class="stat-card-icon" :style="{ color: iconColor || 'var(--yx-ink-2)' }">
        <el-icon :size="24"><component :is="icon" /></el-icon>
      </div>
    </div>
  </div>
</template>

<script setup>
const props = defineProps({
  label: {
    type: String,
    required: true
  },
  value: {
    type: [Number, String],
    default: 0
  },
  unit: {
    type: String,
    default: ''
  },
  icon: {
    type: String,
    default: ''
  },
  iconColor: {
    type: String,
    default: ''
  },
  hint: {
    type: String,
    default: ''
  },
  clickable: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['click'])

const handleClick = () => {
  if (props.clickable) {
    emit('click')
  }
}
</script>

<style scoped>
.stat-card {
  background: var(--yx-surface);
  border-radius: var(--yx-radius);
  border: 1px solid var(--el-border-color-lighter);
  box-shadow: var(--yx-shadow);
  padding: 18px 20px;
  transition: transform 0.2s ease, box-shadow 0.2s ease;
  position: relative;
}

.stat-card.is-clickable {
  cursor: pointer;
}

.stat-card.is-clickable:hover {
  transform: translateY(-2px);
  box-shadow: var(--yx-shadow-hover);
}

.stat-card-body {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.stat-card-info {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.stat-card-label {
  font-size: var(--yx-fs-xs);
  color: var(--yx-text-3);
  font-weight: 500;
  letter-spacing: 0.5px;
}

.stat-card-value-wrap {
  display: flex;
  align-items: baseline;
  gap: 4px;
}

.stat-card-value {
  font-size: var(--yx-fs-xxl);
  font-weight: 700;
  color: var(--yx-text-1);
  line-height: 1.1;
  letter-spacing: -0.5px;
}

.stat-card-unit {
  font-size: var(--yx-fs-xs);
  color: var(--yx-text-3);
  font-weight: normal;
}

.stat-card-hint {
  font-size: 11px;
  color: var(--yx-text-4);
}

.stat-card-icon {
  width: 44px;
  height: 44px;
  border-radius: 8px;
  background: var(--el-fill-color-light);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border: 1px solid var(--el-border-color-extra-light);
}
</style>
