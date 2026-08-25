<template>
  <div class="calendar-page">
    <div class="toolbar">
      <el-radio-group v-model="viewMode">
        <el-radio-button value="month">按月展示</el-radio-button>
        <el-radio-button value="year">按年展示</el-radio-button>
      </el-radio-group>
      <el-button-group>
        <el-button :icon="'ArrowLeft'" @click="prevPeriod" />
        <el-button :icon="'ArrowRight'" @click="nextPeriod" />
      </el-button-group>
      <el-date-picker v-if="viewMode === 'month'" v-model="month" type="month" value-format="YYYY-MM" placeholder="选择月份" style="width:140px" @change="loadData" />
      <el-date-picker v-else v-model="year" type="year" value-format="YYYY" placeholder="选择年份" style="width:120px" @change="loadData" />
      <el-button type="primary" :icon="'Plus'" @click="openCreate">添加工作</el-button>
      <el-select v-model="exportDept" placeholder="全部科室" clearable style="width:150px" @change="reloadExport">
        <el-option v-for="d in departments" :key="d.id" :label="d.name" :value="d.id" />
      </el-select>
      <el-button type="success" :icon="'Download'" @click="exportData">导出工作</el-button>
      <el-button :icon="'List'" @click="showList = !showList">列表展示</el-button>
    </div>

    <!-- 列表展示 -->
    <el-card v-if="showList" shadow="never" class="list-card">
      <template #header>工作列表</template>
      <el-table :data="tasks" stripe size="small" v-loading="loading">
        <el-table-column prop="department_name" label="科室" width="120" />
        <el-table-column prop="title" label="工作内容" min-width="180" show-overflow-tooltip />
        <el-table-column prop="start_date" label="开始日期" width="110" />
        <el-table-column prop="end_date" label="结束日期" width="110" />
        <el-table-column prop="created_name" label="录入人" width="90" />
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button link type="warning" size="small" @click="openEdit(row)">编辑</el-button>
            <el-button link type="danger" size="small" @click="removeTask(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 按月全屏日历 -->
    <div v-if="viewMode === 'month'" class="month-calendar" v-loading="loading">
      <div class="week-header">
        <div v-for="w in weekDays" :key="w" class="week-cell">{{ w }}</div>
      </div>
      <div class="grid">
        <div v-for="d in monthCells" :key="d.date" class="day-cell" :class="{ 'other-month': !d.current }" @click="openCreateOnDate(d.date)">
          <div class="day-num">{{ d.dayNum }}</div>
          <div class="task-list">
            <div v-for="t in tasksOfDay(d.date)" :key="t.id" class="task-chip"
              :class="{ 'span-task': t.span }" @click.stop="openEdit(t)">
              <el-tooltip :content="`${t.title}（${t.department_name}）`" placement="top">
                <span>{{ t.title }}</span>
              </el-tooltip>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 按年全屏日历（12 月网格） -->
    <div v-if="viewMode === 'year'" class="year-calendar" v-loading="loading">
      <div v-for="m in 12" :key="m" class="year-month">
        <div class="year-month-title">{{ m }} 月</div>
        <div class="year-week-header">
          <div v-for="w in ['日','一','二','三','四','五','六']" :key="w" class="year-week-cell">{{ w }}</div>
        </div>
        <div class="year-grid">
          <div v-for="d in monthCellsOf(m)" :key="d.date" class="year-day" :class="{ 'other-month': !d.current }">
            <div class="day-num">{{ d.dayNum }}</div>
            <div class="task-list">
              <div v-for="t in tasksOfDay(d.date)" :key="t.id" class="task-chip" @click.stop="openEdit(t)">
                <span>{{ t.title }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 添加/编辑工作 -->
    <el-dialog v-model="dialogVisible" :title="editId ? '编辑工作' : '添加工作'" width="520px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="所属科室" required>
          <el-select v-model="form.department_id" placeholder="选择科室" style="width:100%">
            <el-option v-for="d in departments" :key="d.id" :label="d.name" :value="d.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="工作内容" required>
          <el-input v-model="form.title" placeholder="如：筹备专题会议" />
        </el-form-item>
        <el-form-item label="详情">
          <el-input v-model="form.content" type="textarea" :rows="3" placeholder="工作详情（选填）" />
        </el-form-item>
        <el-form-item label="开始日期" required>
          <el-date-picker v-model="form.start_date" type="date" value-format="YYYY-MM-DD" style="width:100%" />
        </el-form-item>
        <el-form-item label="结束日期">
          <el-date-picker v-model="form.end_date" type="date" value-format="YYYY-MM-DD" style="width:100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request, { exportFile } from '../utils/request'
import dayjs from 'dayjs'

const viewMode = ref('month')
const month = ref(dayjs().format('YYYY-MM'))
const year = ref(dayjs().format('YYYY'))
const tasks = ref([])
const loading = ref(false)
const departments = ref([])
const exportDept = ref('')
const showList = ref(false)
const dialogVisible = ref(false)
const editId = ref(0)
const form = ref({ department_id: '', title: '', content: '', start_date: '', end_date: '' })

const weekDays = ['日', '一', '二', '三', '四', '五', '六']

const loadDepartments = async () => {
  try {
    const res = await request.get('/departments')
    departments.value = res.list || []
  } catch (e) {}
}

// 当月所有日历格子（含前后补位）
const monthCells = computed(() => {
  const m = month.value
  const first = dayjs(m + '-01')
  const daysInMonth = first.daysInMonth()
  const startOffset = first.day()
  const cells = []
  for (let i = startOffset - 1; i >= 0; i--) {
    const d = first.subtract(i + 1, 'day')
    cells.push({ date: d.format('YYYY-MM-DD'), dayNum: d.date(), current: false })
  }
  for (let i = 1; i <= daysInMonth; i++) {
    const d = first.date(i)
    cells.push({ date: d.format('YYYY-MM-DD'), dayNum: i, current: true })
  }
  let endIdx = cells.length % 7
  if (endIdx !== 0) {
    const last = dayjs(month.value + '-' + daysInMonth)
    for (let i = 1; i <= 7 - endIdx; i++) {
      const d = last.add(i, 'day')
      cells.push({ date: d.format('YYYY-MM-DD'), dayNum: d.date(), current: false })
    }
  }
  return cells
})

// 某月的格子（按年视图用）
const monthCellsOf = (m) => {
  const first = dayjs(year.value + '-' + String(m).padStart(2, '0') + '-01')
  const daysInMonth = first.daysInMonth()
  const startOffset = first.day()
  const cells = []
  for (let i = startOffset - 1; i >= 0; i--) {
    const d = first.subtract(i + 1, 'day')
    cells.push({ date: d.format('YYYY-MM-DD'), dayNum: d.date(), current: false })
  }
  for (let i = 1; i <= daysInMonth; i++) {
    const d = first.date(i)
    cells.push({ date: d.format('YYYY-MM-DD'), dayNum: i, current: true })
  }
  let endIdx = cells.length % 7
  if (endIdx !== 0) {
    const last = first.date(daysInMonth)
    for (let i = 1; i <= 7 - endIdx; i++) {
      const d = last.add(i, 'day')
      cells.push({ date: d.format('YYYY-MM-DD'), dayNum: d.date(), current: false })
    }
  }
  return cells
}

// 判断任务是否跨天（用于样式）
const markSpan = (t) => {
  t.span = t.start_date !== t.end_date
  return t
}

// 查询日期范围：月视图用当月，年视图用全年
const range = computed(() => {
  if (viewMode.value === 'month') {
    const start = month.value + '-01'
    const end = dayjs(start).endOf('month').format('YYYY-MM-DD')
    return { start, end }
  }
  return { start: year.value + '-01-01', end: year.value + '-12-31' }
})

const loadData = async () => {
  loading.value = true
  try {
    const res = await request.get('/calendar-tasks', { params: { start: range.value.start, end: range.value.end } })
    tasks.value = (res.list || []).map(markSpan)
  } catch (e) {
  } finally {
    loading.value = false
  }
}

// 某个日期上显示的任务
const tasksOfDay = (date) => {
  return tasks.value.filter(t => date >= t.start_date && date <= t.end_date)
}

const prevPeriod = () => {
  if (viewMode.value === 'month') month.value = dayjs(month.value + '-01').subtract(1, 'month').format('YYYY-MM')
  else year.value = String(Number(year.value) - 1)
  loadData()
}

const nextPeriod = () => {
  if (viewMode.value === 'month') month.value = dayjs(month.value + '-01').add(1, 'month').format('YYYY-MM')
  else year.value = String(Number(year.value) + 1)
  loadData()
}

const openCreate = () => {
  editId.value = 0
  form.value = { department_id: '', title: '', content: '', start_date: dayjs().format('YYYY-MM-DD'), end_date: dayjs().format('YYYY-MM-DD') }
  dialogVisible.value = true
}

const openCreateOnDate = (date) => {
  if (!date) return
  editId.value = 0
  form.value = { department_id: '', title: '', content: '', start_date: date, end_date: date }
  dialogVisible.value = true
}

const openEdit = (row) => {
  editId.value = row.id
  form.value = {
    department_id: row.department_id, title: row.title, content: row.content || '',
    start_date: row.start_date, end_date: row.end_date
  }
  dialogVisible.value = true
}

const save = async () => {
  if (!form.value.department_id) return ElMessage.warning('请选择科室')
  if (!form.value.title) return ElMessage.warning('请输入工作内容')
  if (!form.value.start_date) return ElMessage.warning('请选择开始日期')
  try {
    const payload = { ...form.value }
    if (!payload.end_date) payload.end_date = payload.start_date
    if (editId.value) {
      await request.put('/calendar-tasks', { ...payload, id: editId.value })
      ElMessage.success('更新成功')
    } else {
      await request.post('/calendar-tasks', payload)
      ElMessage.success('添加成功')
    }
    dialogVisible.value = false
    loadData()
  } catch (e) {}
}

const removeTask = async (row) => {
  try {
    await ElMessageBox.confirm(`确认删除工作「${row.title}」？`, '删除确认', { type: 'warning', confirmButtonText: '删除' })
  } catch (e) { return }
  try {
    await request.delete(`/calendar-tasks/${row.id}`)
    ElMessage.success('删除成功')
    loadData()
  } catch (e) {}
}

const reloadExport = () => {}

const exportData = () => {
  const params = {}
  if (exportDept.value) params.department_id = exportDept.value
  exportFile('/export/calendar-tasks', params)
}

onMounted(() => {
  loadData()
  loadDepartments()
})
</script>

<style scoped>
.calendar-page {
  display: flex;
  flex-direction: column;
}
.toolbar {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-bottom: 12px;
  flex-wrap: wrap;
}
.list-card {
  margin-bottom: 12px;
}
.month-calendar {
  background: #fff;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  overflow: hidden;
}
.week-header,
.year-week-header {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  background: #f5f7fa;
  border-bottom: 1px solid #dcdfe6;
}
.week-cell,
.year-week-cell {
  padding: 8px 0;
  text-align: center;
  font-weight: 600;
  color: #606266;
  font-size: 13px;
}
.grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
}
.day-cell {
  min-height: 120px;
  border-right: 1px solid #ebeef5;
  border-bottom: 1px solid #ebeef5;
  padding: 4px;
  cursor: pointer;
  overflow: hidden;
}
.day-cell:hover {
  background: #f5f7fa;
}
.day-cell.other-month {
  background: #fafafa;
  color: #c0c4cc;
}
.day-num {
  font-size: 13px;
  font-weight: 600;
  margin-bottom: 2px;
}
.task-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.task-chip {
  background: #ecf5ff;
  color: #409eff;
  border-radius: 3px;
  font-size: 12px;
  padding: 1px 4px;
  cursor: pointer;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.task-chip.span-task {
  background: #fdf6ec;
  color: #e6a23c;
}
.year-calendar {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}
.year-month {
  background: #fff;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  overflow: hidden;
}
.year-month-title {
  text-align: center;
  font-weight: 600;
  padding: 6px 0;
  background: #f5f7fa;
  color: #303133;
}
.year-grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
}
.year-day {
  min-height: 52px;
  border-right: 1px solid #ebeef5;
  border-bottom: 1px solid #ebeef5;
  padding: 2px;
  cursor: pointer;
  overflow: hidden;
}
.year-day:hover {
  background: #f5f7fa;
}
.year-day.other-month {
  background: #fafafa;
}
@media (max-width: 767px) {
  .year-calendar {
    grid-template-columns: repeat(1, 1fr);
  }
  .day-cell {
    min-height: 80px;
  }
  .task-chip {
    font-size: 11px;
  }
  .toolbar {
    gap: 6px;
  }
}
</style>
