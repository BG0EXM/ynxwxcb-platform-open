<template>
  <!-- 工作台“今日待办”：汇总各模块当天要处理的事，点一条直接打开对应记录 -->
  <div class="todo-panel" v-loading="todoState.loading && !todoState.loaded">
    <div class="todo-header">
      <div class="todo-title-wrap">
        <span class="todo-title-bar"></span>
        <h3 class="todo-title font-serif">今日待办</h3>
        <span v-if="todoState.total" class="todo-count tabular-nums">{{ todoState.total }}</span>
      </div>
      <el-button link type="primary" :icon="'Refresh'" :loading="todoState.loading" @click="refresh">刷新</el-button>
    </div>

    <div v-if="todoState.loaded && !todoState.groups.length" class="todo-empty">
      <el-icon class="todo-empty-icon"><CircleCheckFilled /></el-icon>
      <span>今天没有待办事项</span>
    </div>

    <div v-else class="todo-groups">
      <section v-for="g in todoState.groups" :key="g.key" class="todo-group">
        <div class="group-head">
          <div class="group-name">
            <el-icon><component :is="g.icon" /></el-icon>
            <span>{{ g.title }}</span>
            <span class="group-num tabular-nums">{{ g.total }}</span>
          </div>
          <el-button v-if="g.total > g.items.length" link type="primary" @click="go(g.path)">
            还有 {{ g.total - g.items.length }} 条，查看全部
          </el-button>
          <el-button v-else link type="primary" @click="go(g.path)">查看</el-button>
        </div>
        <ul class="todo-list">
          <li v-for="it in g.items" :key="g.key + it.id" class="todo-item" :class="'lv-' + it.level" @click="go(it.link)">
            <span class="lv-bar"></span>
            <div class="todo-main">
              <div class="todo-item-title">{{ it.title }}</div>
              <div class="todo-item-desc" v-if="it.desc">{{ it.desc }}</div>
            </div>
            <span class="todo-due" v-if="it.due">{{ it.due }}</span>
            <el-icon class="todo-arrow"><ArrowRight /></el-icon>
          </li>
        </ul>
      </section>
    </div>
  </div>
</template>

<script setup>
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { CircleCheckFilled, ArrowRight } from '@element-plus/icons-vue'
import { todoState, loadTodos } from '../utils/todos'

const router = useRouter()
const go = (path) => path && router.push(path)
const refresh = () => loadTodos({ force: true }).catch(() => {})

// 每次进入工作台都重新加载，办完的事回到工作台即消失
onMounted(refresh)
</script>

<style scoped>
.todo-panel {
  background: var(--yx-surface);
  border-radius: var(--yx-radius);
  border: 1px solid var(--yx-border-soft);
  box-shadow: var(--yx-shadow);
  min-height: 96px;
}
.todo-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 20px;
  border-bottom: 1px solid var(--yx-border-soft);
}
.todo-title-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
}
.todo-title-bar {
  width: 4px;
  height: 18px;
  background: var(--yx-brand);
  border-radius: 2px;
}
.todo-title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--yx-text-1);
  letter-spacing: 1px;
}
.todo-count {
  min-width: 22px;
  height: 22px;
  line-height: 22px;
  padding: 0 7px;
  border-radius: 11px;
  background: var(--yx-brand);
  color: #fff;
  font-size: 13px;
  text-align: center;
}

.todo-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 28px 16px;
  color: var(--yx-text-3);
  font-size: var(--yx-fs-sm);
}
.todo-empty-icon {
  font-size: 22px;
  color: var(--el-color-success);
}

.todo-groups {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 0 24px;
  padding: 6px 20px 14px;
}
.todo-group {
  padding-top: 10px;
}
.group-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 4px;
}
.group-name {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: var(--yx-fs-sm);
  font-weight: 600;
  color: var(--yx-text-1);
}
.group-name .el-icon {
  color: var(--yx-ink-2);
}
.group-num {
  color: var(--yx-text-3);
  font-weight: 500;
}

.todo-list {
  list-style: none;
  margin: 0;
  padding: 0;
}
.todo-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 8px 8px 0;
  border-bottom: 1px dashed var(--yx-border-soft);
  cursor: pointer;
  border-radius: var(--yx-radius-sm);
}
.todo-item:hover {
  background: var(--yx-surface-muted);
}
.todo-item:last-child {
  border-bottom: 0;
}
.lv-bar {
  width: 3px;
  align-self: stretch;
  border-radius: 2px;
  background: var(--yx-border);
  flex: none;
}
.lv-danger .lv-bar { background: var(--yx-brand); }
.lv-warning .lv-bar { background: var(--el-color-warning); }
.todo-main {
  flex: 1;
  min-width: 0;
}
.todo-item-title {
  font-size: var(--yx-fs-sm);
  color: var(--yx-text-1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.todo-item-desc {
  font-size: var(--yx-fs-xs);
  color: var(--yx-text-3);
  margin-top: 2px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.todo-due {
  flex: none;
  font-size: var(--yx-fs-xs);
  color: var(--yx-text-3);
  white-space: nowrap;
}
.lv-danger .todo-due { color: var(--yx-brand); font-weight: 600; }
.lv-warning .todo-due { color: var(--el-color-warning-dark-2); font-weight: 600; }
.todo-arrow {
  flex: none;
  color: var(--yx-text-4);
}

@media (max-width: 768px) {
  .todo-header { padding: 12px 14px; }
  .todo-groups {
    grid-template-columns: 1fr;
    padding: 4px 14px 12px;
  }
  /* 手机端时限放到第二行，标题不被挤压 */
  .todo-item { flex-wrap: wrap; }
  .todo-main { flex-basis: calc(100% - 40px); }
  .todo-due { order: 3; margin-left: 13px; }
  .todo-arrow { display: none; }
}
</style>
