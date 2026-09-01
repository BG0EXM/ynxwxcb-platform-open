<template>
  <div>
    <el-card shadow="never">
      <div class="toolbar">
        <div>
          <el-date-picker v-model="year" type="year" value-format="YYYY" placeholder="按年筛选" style="width:130px" @change="loadAll" />
          <el-date-picker v-model="month" type="month" value-format="YYYY-MM" placeholder="按年+按月" clearable style="width:140px;margin-left:10px" @change="loadAll" />
        </div>
        <div>
          <el-button type="success" :icon="'Download'" @click="exportData">导出Excel</el-button>
          <el-button type="primary" :icon="'Plus'" @click="openCreate">录入加班</el-button>
        </div>
      </div>

      <!-- 加班统计表 -->
      <div class="section-title">{{ periodLabel }}加班与补休统计</div>
      <el-table :data="statsList" stripe v-loading="loading" empty-text="暂无统计数据">
        <el-table-column prop="user_name" label="姓名" width="120" />
        <el-table-column prop="department" label="部门" width="140" />
        <el-table-column label="加班小时" width="100">
          <template #default="{ row }">{{ row.overtime_hours }}</template>
        </el-table-column>
        <el-table-column label="折合补休(天)" width="110">
          <template #default="{ row }">{{ row.comp_days.toFixed(1) }}</template>
        </el-table-column>
        <el-table-column label="已补休(天)" width="110">
          <template #default="{ row }">
            <b style="color:#e6a23c">{{ row.used_days }}</b>
          </template>
        </el-table-column>
        <el-table-column label="剩余可补(天)" width="110">
          <template #default="{ row }">
            <b :style="{ color: row.remain_days < 0 ? '#f56c6c' : '#67c23a' }">{{ row.remain_days.toFixed(1) }}</b>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="110" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" size="small" :disabled="row.remain_days <= 0" @click="registerComp(row)">登记补休</el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 加班记录列表（当年全部，可按年/月筛选） -->
      <div class="section-title mt-16">{{ periodLabel }}加班记录</div>
      <el-table :data="records" stripe size="small" v-loading="recLoading" empty-text="暂无加班记录">
        <el-table-column prop="overtime_date" label="日期" width="110" />
        <el-table-column prop="user_name" label="姓名" width="100" />
        <el-table-column prop="hours" label="加班小时" width="90" />
        <el-table-column prop="reason" label="事由" min-width="200" show-overflow-tooltip />
        <el-table-column label="操作" width="80" fixed="right">
          <template #default="{ row }">
            <el-button link type="danger" size="small" @click="removeRecord(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 录入加班 -->
    <el-dialog v-model="dialogVisible" title="录入加班" width="500px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="人员" required>
          <el-select v-model="form.user_id" filterable placeholder="选择人员" style="width:100%">
            <el-option v-for="a in assignees" :key="a.id" :label="a.real_name" :value="a.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="加班日期" required>
          <el-date-picker v-model="form.overtime_date" type="date" value-format="YYYY-MM-DD" style="width:100%" />
        </el-form-item>
        <el-form-item label="加班时长" required>
          <el-input-number v-model="form.hours" :min="0.5" :max="24" :step="0.5" style="width:100%" />
          <div class="form-tip">单位：小时（8 小时 = 1 天补休）</div>
        </el-form-item>
        <el-form-item label="事由">
          <el-input v-model="form.reason" type="textarea" :rows="3" />
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

const year = ref(dayjs().format('YYYY'))
const month = ref('')
const statsList = ref([])
const records = ref([])
const loading = ref(false)
const recLoading = ref(false)
const assignees = ref([])
const dialogVisible = ref(false)
const form = ref({ user_id: '', overtime_date: dayjs().format('YYYY-MM-DD'), hours: 8, reason: '' })

// 统计期间标题：选月则 XXXX年X月，否则 XXXX年
const periodLabel = computed(() => {
  if (month.value) {
    const t = dayjs(month.value + '-01')
    return `${t.year()}年${t.month() + 1}月`
  }
  return `${year.value}年`
})

const loadAssignees = async () => {
  try {
    const res = await request.get('/assignees')
    assignees.value = res.list || []
  } catch (e) {}
}

const loadStats = async () => {
  loading.value = true
  try {
    const params = { year: year.value }
    if (month.value) params.month = month.value
    const res = await request.get('/overtime-stats', { params })
    statsList.value = res.list || []
  } catch (e) {
  } finally {
    loading.value = false
  }
}

const loadRecords = async () => {
  recLoading.value = true
  try {
    const params = { year: year.value }
    if (month.value) {
      params.start = month.value + '-01'
      params.end = dayjs(month.value + '-01').endOf('month').format('YYYY-MM-DD')
    }
    const res = await request.get('/overtime-records', { params })
    records.value = res.list || []
  } catch (e) {
  } finally {
    recLoading.value = false
  }
}

const loadAll = () => {
  loadStats()
  loadRecords()
}

const openCreate = () => {
  form.value = { user_id: '', overtime_date: dayjs().format('YYYY-MM-DD'), hours: 8, reason: '' }
  dialogVisible.value = true
}

const save = async () => {
  if (!form.value.user_id) return ElMessage.warning('请选择人员')
  if (!form.value.overtime_date) return ElMessage.warning('请选择日期')
  if (!form.value.hours || form.value.hours <= 0) return ElMessage.warning('请填写加班时长')
  try {
    await request.post('/overtime-records', form.value)
    ElMessage.success('录入成功')
    dialogVisible.value = false
    loadAll()
  } catch (e) {}
}

const removeRecord = async (row) => {
  try {
    await ElMessageBox.confirm(`确认删除 ${row.user_name} ${row.overtime_date} 的加班记录？`, '删除确认', { type: 'warning', confirmButtonText: '删除' })
  } catch (e) { return }
  try {
    await request.delete(`/overtime-records/${row.id}`)
    ElMessage.success('删除成功')
    loadAll()
  } catch (e) {}
}

// 登记补休：跳转到请假模块，并预填补休类型和人员
const registerComp = (row) => {
  if (row.remain_days <= 0) {
    ElMessage.warning('该人员当前无可补休天数')
    return
  }
  localStorage.setItem('compUser', JSON.stringify({
    user_id: row.user_id, user_name: row.user_name, remain_days: row.remain_days
  }))
  window.location.href = '/leave'
}

const exportData = () => {
  const params = { year: year.value }
  if (month.value) params.month = month.value
  exportFile('/export/overtime-records', params)
}

onMounted(() => {
  loadStats()
  loadRecords()
  loadAssignees()
})
</script>

<style scoped>
.toolbar {
  display: flex;
  justify-content: space-between;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 8px;
}
.section-title {
  font-weight: 600;
  margin: 8px 0 12px;
  color: #303133;
}
.mt-16 {
  margin-top: 16px;
}
.form-tip {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}
</style>
