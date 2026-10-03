<template>
  <div class="action-menu-wrap">
    <!-- 主操作区 -->
    <div class="action-primary">
      <slot name="primary"></slot>
    </div>

    <!-- 更多操作下拉 -->
    <el-dropdown v-if="$slots.more || items?.length" @command="handleCommand" trigger="click">
      <el-button link type="primary" class="action-more-btn">
        <span>更多</span>
        <el-icon class="el-icon--right"><ArrowDown /></el-icon>
      </el-button>
      <template #dropdown>
        <el-dropdown-menu class="action-dropdown-menu">
          <slot name="more">
            <template v-for="item in items" :key="item.key">
              <el-dropdown-item 
                :command="item.key" 
                :divided="item.divided"
                :disabled="item.disabled"
                :class="{ 'is-danger': item.danger }"
              >
                <el-icon v-if="item.icon"><component :is="item.icon" /></el-icon>
                <span>{{ item.label }}</span>
              </el-dropdown-item>
            </template>
          </slot>
        </el-dropdown-menu>
      </template>
    </el-dropdown>
  </div>
</template>

<script setup>
defineProps({
  items: {
    type: Array,
    default: () => []
  }
})

const emit = defineEmits(['command'])

const handleCommand = (cmd) => {
  emit('command', cmd)
}
</script>

<style scoped>
.action-menu-wrap {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  white-space: nowrap;
}

.action-primary {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.action-more-btn {
  font-size: var(--yx-fs-sm);
  color: var(--yx-text-3);
  padding: 2px 4px;
}

.action-more-btn:hover {
  color: var(--el-color-primary);
}

:deep(.is-danger) {
  color: var(--el-color-danger) !important;
}

:deep(.is-danger:hover) {
  background-color: var(--el-color-danger-light-9) !important;
  color: var(--el-color-danger-dark-2) !important;
}
</style>
