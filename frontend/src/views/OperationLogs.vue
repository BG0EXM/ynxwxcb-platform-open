<template>
  <div>
    <el-card shadow="never">
      <div class="toolbar">
        <div>
          <el-select v-model="filters.module" placeholder="全部模块" clearable style="width:150px" @change="reloadFirstPage">
            <el-option v-for="m in modules" :key="m" :label="m" :value="m" />
          </el-select>
          <el-select v-model="filters.action" placeholder="全部动作" clearable style="width:120px" class="ml-8" @change="reloadFirstPage">
            <el-option v-for="a in actions" :key="a" :label="a" :value="a" />
          </el-select>
          <el-date-picker v-model="dateRange" type="daterange" value-format="YYYY-MM-DD"
            range-separator="至" start-placeholder="开始日期" end-placeholder="结束日期"
            class="ml-8" style="width:260px" @change="reloadFirstPage" />
          <el-input v-model="filters.keyword" placeholder="搜索操作人/内容" clearable style="width:200px" class="ml-8"
            @keyup.enter="reloadFirstPage" @clear="reloadFirstPage">
            <template #append><el-button :icon="'Search'" @click="reloadFirstPage" /></template>
          </el-input>
        </div>
        <div class="toolbar-right">
          <el-button type="danger" plain :icon="'Delete'" @click="handleClearLogs">清空日志</el-button>
          <el-button :icon="'Refresh'" @click="loadData">刷新</el-button>
        </div>
      </div>

      <el-table :data="list" stripe v-loading="loading" empty-text="暂无操作日志">
        <el-table-column prop="created_at" label="时间" width="170" />
        <el-table-column label="操作人" width="150">
          <template #default="{ row }">
            {{ row.user_name || '—' }}<span v-if="row.department" class="dept">（{{ row.department }}）</span>
          </template>
        </el-table-column>
        <el-table-column label="模块" width="120">
          <template #default="{ row }">
            <el-tag size="small" :type="row.module === '系统安全' || row.module === '保密防线' ? 'danger' : 'info'">{{ row.module }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="动作" width="120">
          <template #default="{ row }">
            <el-tag size="small" :type="actionType(row.action)">{{ row.action }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="detail" label="内容" min-width="300" show-overflow-tooltip />
        <el-table-column prop="ip" label="IP" width="130" />
      </el-table>

      <div class="pagination-wrap" v-if="total > 0">
        <el-pagination background layout="total, prev, pager, next" :total="total" :page-size="pageSize"
          :current-page="page" @current-change="onPageChange" />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessageBox, ElMessage } from 'element-plus'
import request from '../utils/request'

const modules = [
  '系统安全', '保密防线', '征求意见', '材料下发', '会务管理', '登录', '认证',
  '收文管理', '用车管理', '请假管理', '考勤管理', '加班管理',
  '年休假管理', '大事记', '每周工作总结', '工作日历', '公共资料',
  '通讯录', '值守排班', '用户管理', '常委管理'
]
const actions = ['新增', '修改', '删除', '导出', '登录', 'WAF拦截', '涉密阻断', '公开端涉密阻断']

const list = ref([])
const loading = ref(false)
const page = ref(1)
const pageSize = 20
const total = ref(0)
const filters = reactive({ module: '', action: '', keyword: '' })
const dateRange = ref([])

const actionType = (a) => ({
  新增: 'success',
  修改: 'warning',
  删除: 'danger',
  导出: 'primary',
  登录: 'info',
  WAF拦截: 'danger',
  涉密阻断: 'danger',
  公开端涉密阻断: 'danger'
}[a] || 'info')

const loadData = async () => {
  loading.value = true
  try {
    const params = { page: page.value, page_size: pageSize }
    if (filters.module) params.module = filters.module
    if (filters.action) params.action = filters.action
    if (filters.keyword) params.keyword = filters.keyword
    if (dateRange.value && dateRange.value.length === 2) {
      params.start = dateRange.value[0]
      params.end = dateRange.value[1]
    }
    const res = await request.get('/operation-logs', { params })
    list.value = res.list || []
    total.value = res.total || 0
  } catch (e) { console.error(e) } finally {
    loading.value = false
  }
}

const reloadFirstPage = () => { page.value = 1; loadData() }
const onPageChange = (p) => { page.value = p; loadData() }

const handleClearLogs = () => {
  ElMessageBox.confirm(
    '清空操作日志将永久删除所有历史审计记录，此操作不可撤销，确定清空吗？',
    '高危安全操作警告',
    {
      type: 'warning',
      confirmButtonText: '确定清空',
      cancelButtonText: '取消',
      confirmButtonClass: 'el-button--danger'
    }
  ).then(async () => {
    try {
      const res = await request.post('/operation-logs/clear')
      ElMessage.success(res.message || '操作日志已清空')
      reloadFirstPage()
    } catch (e) {
      console.error(e)
    }
  }).catch(() => {})
}

onMounted(loadData)
</script>

<style scoped>
.toolbar {
  display: flex;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 16px;
}
.ml-8 { margin-left: 8px; }
.dept { color: var(--el-text-color-secondary); font-size: 12px; }
.pagination-wrap {
  margin-top: 14px;
  display: flex;
  justify-content: flex-end;
}
</style>
