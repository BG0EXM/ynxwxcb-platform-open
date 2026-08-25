<template>
  <div>
    <el-alert type="info" :closable="false" class="rule-alert">
      <template #title>
        请假管理：登记年假及各类特殊假期，可查看各类型请假统计与年度总计。
      </template>
    </el-alert>

    <!-- 请假统计 -->
    <el-card shadow="never" class="mt-12">
      <template #header>
        <div class="card-header">
          <span class="card-title">请假统计（{{ stats.year }}）</span>
          <div class="header-right">
            <el-date-picker v-model="statsYear" type="year" value-format="YYYY" placeholder="选择年份" style="width:140px" @change="loadStats" />
          </div>
        </div>
      </template>
      <el-row :gutter="12">
        <el-col :xs="12" :sm="6" :md="3" v-for="c in statItems" :key="c.key">
          <div class="stat-item" :style="{ borderLeft: '4px solid ' + c.color }">
            <div class="stat-num">{{ c.days }}</div>
            <div class="stat-label">{{ c.label }}（{{ c.count }}次）</div>
          </div>
        </el-col>
        <el-col :xs="12" :sm="6" :md="3">
          <div class="stat-item" style="border-left: 4px solid #303133; background: #f5f7fa;">
            <div class="stat-num total">{{ stats.total_days }}</div>
            <div class="stat-label">总计（{{ stats.total_count }}次）</div>
          </div>
        </el-col>
      </el-row>
    </el-card>

    <!-- 请假登记 -->
    <el-card shadow="never" class="mt-12">
      <template #header>
        <span class="card-title">{{ editId ? '编辑请假' : (authStore.isAdmin ? '登记请假' : '我的请假申请') }}</span>
      </template>
      <el-form :model="form" inline label-width="70px">
        <el-form-item label="人员" v-if="authStore.isAdmin">
          <el-select v-model="form.user_id" filterable placeholder="选择人员" style="width:160px">
            <el-option v-for="a in assignees" :key="a.id" :label="a.real_name" :value="a.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="类型">
          <el-select v-model="form.leave_type" placeholder="请假类型" style="width:120px">
            <el-option v-for="(name, val) in leaveTypeNames" :key="val" :label="name" :value="val" />
          </el-select>
        </el-form-item>
        <el-form-item label="开始">
          <el-date-picker v-model="form.start_date" type="date" value-format="YYYY-MM-DD" style="width:150px" @change="calcDays" />
        </el-form-item>
        <el-form-item label="结束">
          <el-date-picker v-model="form.end_date" type="date" value-format="YYYY-MM-DD" style="width:150px" @change="calcDays" />
        </el-form-item>
        <el-form-item label="天数">
          <el-input-number v-model="form.days" :min="0.5" :max="365" :step="0.5" style="width:100px" />
        </el-form-item>
        <el-form-item label="事由">
          <el-input v-model="form.reason" placeholder="请假事由" style="width:200px" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="editId ? 'EditPen' : 'Plus'" @click="saveLeave">{{ editId ? '保存修改' : '登记' }}</el-button>
          <el-button v-if="editId" @click="cancelEdit">取消</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 请假记录 -->
    <el-card shadow="never" class="mt-12">
      <template #header>
        <div class="card-header">
          <span class="card-title">请假记录</span>
          <div class="header-right">
            <el-select v-model="filterType" placeholder="全部类型" clearable style="width:130px" @change="loadData">
              <el-option v-for="(name, val) in leaveTypeNames" :key="val" :label="name" :value="val" />
            </el-select>
            <el-select v-if="authStore.isAdmin" v-model="userFilter" placeholder="全部人员" clearable style="width:140px" class="ml-8" @change="loadData">
              <el-option v-for="a in assignees" :key="a.id" :label="a.real_name" :value="a.id" />
            </el-select>
            <el-button type="success" :icon="'Download'" class="ml-8" @click="exportData">导出Excel</el-button>
          </div>
        </div>
      </template>
      <el-table :data="list" stripe v-loading="loading" empty-text="暂无请假记录">
        <el-table-column prop="user_name" label="姓名" width="100" v-if="authStore.isAdmin" />
        <el-table-column label="请假类型" width="100">
          <template #default="{ row }">
            <el-tag size="small">{{ leaveTypeNames[row.leave_type] || row.leave_type }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="start_date" label="开始日期" width="110" />
        <el-table-column prop="end_date" label="结束日期" width="110" />
        <el-table-column prop="days" label="天数" width="80" />
        <el-table-column prop="reason" label="事由" min-width="150" show-overflow-tooltip />
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button v-if="authStore.isAdmin || row.user_id === authStore.user?.id" link type="warning" size="small" @click="openEdit(row)">编辑</el-button>
            <el-button link type="primary" size="small" @click="openPrint(row)">打印假条</el-button>
            <el-button v-if="authStore.isAdmin" link type="danger" size="small" @click="removeLeave(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request, { exportFile } from '../utils/request'
import dayjs from 'dayjs'
import { useAuthStore } from '../store/auth'

const authStore = useAuthStore()
const list = ref([])
const loading = ref(false)
const editId = ref(0)
const stats = reactive({ year: dayjs().format('YYYY'), total_count: 0, total_days: 0 })
const statsYear = ref(dayjs().format('YYYY'))
const filterType = ref('')
const userFilter = ref('')
const assignees = ref([])
const form = reactive({ user_id: null, leave_type: 'annual', start_date: dayjs().format('YYYY-MM-DD'), end_date: '', days: 1, reason: '' })

const leaveTypeNames = {
  annual: '年假', sick: '病假', personal: '事假', marriage: '婚假',
  maternity: '产假', bereavement: '丧假', other: '其他'
}

const statColors = {
  annual: '#409eff', sick: '#67c23a', personal: '#e6a23c', marriage: '#f56c6c',
  maternity: '#909399', bereavement: '#303133', other: '#b88230'
}

const statItems = computed(() => Object.keys(leaveTypeNames).map(k => ({
  key: k,
  label: leaveTypeNames[k],
  count: stats[k + '_count'] || 0,
  days: stats[k + '_days'] || 0,
  color: statColors[k]
})))

const loadData = async () => {
  loading.value = true
  try {
    const params = {}
    if (filterType.value) params.leave_type = filterType.value
    if (userFilter.value) params.user_id = userFilter.value
    const res = await request.get('/leave-records', { params })
    list.value = res.list || []
  } catch (e) {
  } finally {
    loading.value = false
  }
}

const loadStats = async () => {
  try {
    const res = await request.get('/leave-stats', { params: { year: statsYear.value } })
    Object.assign(stats, res)
  } catch (e) {}
}

const loadAssignees = async () => {
  try {
    const res = await request.get('/assignees')
    assignees.value = res.list || []
  } catch (e) {}
}

// 根据起止日期自动计算天数（含首尾），支持半天（0.5）
const calcDays = () => {
  const s = form.start_date
  const e = form.end_date
  if (!s) return
  if (!e) {
    form.end_date = s
    form.days = 1
    return
  }
  const start = dayjs(s)
  const end = dayjs(e)
  if (end.isBefore(start)) {
    ElMessage.warning('结束日期不能早于开始日期')
    form.end_date = s
    return
  }
  form.days = end.diff(start, 'day') + 1
}

const saveLeave = async () => {
  if (!authStore.isAdmin && !form.leave_type) return ElMessage.warning('请选择请假类型')
  if (authStore.isAdmin && !form.user_id) return ElMessage.warning('请选择人员')
  if (!form.leave_type) return ElMessage.warning('请选择请假类型')
  if (!form.end_date) form.end_date = form.start_date
  try {
    const payload = { ...form }
    if (!authStore.isAdmin) {
      // 普通用户只能为自己请假，由后端强制绑定当前账号
      payload.user_id = authStore.user?.id
    }
    if (editId.value) {
      await request.put('/leave-records', { ...payload, id: editId.value })
      ElMessage.success('修改成功')
    } else {
      await request.post('/leave-records', payload)
      ElMessage.success('请假登记成功')
    }
    Object.assign(form, { user_id: null, leave_type: 'annual', start_date: dayjs().format('YYYY-MM-DD'), end_date: '', days: 1, reason: '' })
    editId.value = 0
    loadData()
    loadStats()
  } catch (e) {}
}

// 编辑请假：填入表单
const openEdit = (row) => {
  editId.value = row.id
  Object.assign(form, {
    user_id: row.user_id, leave_type: row.leave_type, start_date: row.start_date,
    end_date: row.end_date || row.start_date, days: row.days, reason: row.reason || ''
  })
}

const cancelEdit = () => {
  editId.value = 0
  Object.assign(form, { user_id: null, leave_type: 'annual', start_date: dayjs().format('YYYY-MM-DD'), end_date: '', days: 1, reason: '' })
}

const openPrint = (row) => {
  sessionStorage.setItem('printLeave', JSON.stringify(row))
  window.open(`/leave/print/${row.id}`, '_blank')
}

const removeLeave = async (row) => {
  try {
    await ElMessageBox.confirm(`确认删除「${row.user_name}」的${leaveTypeNames[row.leave_type]}记录？`, '提示', { type: 'warning' })
  } catch (e) { return }
  try {
    await request.delete(`/leave-records/${row.id}`)
    ElMessage.success('删除成功')
    loadData()
    loadStats()
  } catch (e) {}
}

onMounted(() => {
  loadData()
  loadStats()
  if (authStore.isAdmin) loadAssignees()
})

const exportData = () => {
  const params = {}
  if (filterType.value) params.leave_type = filterType.value
  exportFile('/export/leave-records', params)
}
</script>

<style scoped>
.rule-alert {
  margin-bottom: 0;
}
.mt-12 {
  margin-top: 12px;
}
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.card-title {
  font-weight: 600;
}
.header-right {
  display: flex;
  align-items: center;
}
.ml-8 {
  margin-left: 8px;
}
.stat-item {
  text-align: center;
  padding: 12px 6px;
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 4px;
  margin-bottom: 8px;
}
.stat-num {
  font-size: 24px;
  font-weight: 700;
  color: #303133;
}
.stat-num.total {
  color: #c8102e;
}
.stat-label {
  color: #909399;
  font-size: 12px;
  margin-top: 4px;
}
</style>
