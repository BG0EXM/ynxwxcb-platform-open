<template>
  <div>
    <el-card shadow="never">
      <div class="toolbar">
        <div>
          <el-date-picker v-model="weekPicker" type="daterange" value-format="YYYY-MM-DD"
            start-placeholder="开始日期" end-placeholder="结束日期" style="width:260px" @change="onWeekChange" />
          <el-select v-if="authStore.isAdmin" v-model="deptFilter" placeholder="全部科室" clearable style="width:150px;margin-left:10px" @change="loadData">
            <el-option v-for="d in departments" :key="d.id" :label="d.name" :value="d.id" />
          </el-select>
        </div>
        <div>
          <el-button v-if="authStore.isAdmin" type="success" :icon="'Download'" @click="exportData">导出Word</el-button>
          <el-button type="primary" :icon="'Plus'" @click="openCreate">录入本周工作</el-button>
        </div>
      </div>

      <el-table :data="list" stripe v-loading="loading" empty-text="暂无每周工作总结">
        <el-table-column prop="department_name" label="科室" width="130" />
        <el-table-column label="本周" width="180">
          <template #default="{ row }">
            {{ row.week_start }} ~ {{ row.week_end }}
          </template>
        </el-table-column>
        <el-table-column prop="content" label="重点工作总结" min-width="280" show-overflow-tooltip />
        <el-table-column prop="created_name" label="录入人" width="100" />
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button link type="warning" size="small" @click="openEdit(row)">编辑</el-button>
            <el-button link type="danger" size="small" @click="removeSummary(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editId ? '编辑每周工作总结' : '录入本周工作总结'" width="600px">
      <el-form :model="form" label-width="110px">
        <el-form-item label="所属科室" required v-if="authStore.isAdmin">
          <el-select v-model="form.department_id" placeholder="选择科室" style="width:100%">
            <el-option v-for="d in departments" :key="d.id" :label="d.name" :value="d.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="本周日期" required>
          <el-date-picker v-model="weekRange" type="daterange" value-format="YYYY-MM-DD"
            start-placeholder="开始日期" end-placeholder="结束日期" style="width:100%" @change="onRangeChange" />
        </el-form-item>
        <el-form-item label="重点工作总结" required>
          <el-input v-model="form.content" type="textarea" :rows="6" placeholder="本周重点工作的完成情况、成效及下步计划" />
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
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request, { exportFile } from '../utils/request'
import dayjs from 'dayjs'
import { useAuthStore } from '../store/auth'

const authStore = useAuthStore()
const list = ref([])
const loading = ref(false)
const weekPicker = ref(currentWeekRange())
const deptFilter = ref('')
const departments = ref([])
const dialogVisible = ref(false)
const editId = ref(0)
const form = ref({ department_id: '', content: '' })
const weekRange = ref(currentWeekRange())

function currentWeekRange() {
  const now = dayjs()
  const day = now.day()
  const offset = day === 0 ? -6 : 1 - day
  const start = now.add(offset, 'day').format('YYYY-MM-DD')
  const end = now.add(offset + 6, 'day').format('YYYY-MM-DD')
  return [start, end]
}

const loadDepartments = async () => {
  try {
    const res = await request.get('/departments')
    departments.value = res.list || []
  } catch (e) {}
}

const onWeekChange = () => {
  loadData()
}

const onRangeChange = () => {}

const loadData = async () => {
  loading.value = true
  try {
    const params = {}
    if (weekPicker.value && weekPicker.value.length === 2) {
      params.week_start = weekPicker.value[0]
      params.week_end = weekPicker.value[1]
    }
    if (deptFilter.value) params.department_id = deptFilter.value
    const res = await request.get('/weekly-summaries', { params })
    list.value = res.list || []
  } catch (e) {
  } finally {
    loading.value = false
  }
}

const openCreate = () => {
  editId.value = 0
  weekRange.value = currentWeekRange()
  form.value = { department_id: authStore.isAdmin ? '' : (authStore.user?.department_id || ''), content: '' }
  dialogVisible.value = true
}

const openEdit = (row) => {
  editId.value = row.id
  weekRange.value = [row.week_start, row.week_end]
  form.value = { department_id: row.department_id, content: row.content || '' }
  dialogVisible.value = true
}

const save = async () => {
  if (!weekRange.value || weekRange.value.length !== 2) return ElMessage.warning('请选择本周日期')
  if (!form.value.content) return ElMessage.warning('请输入重点工作总结')
  try {
    const payload = { ...form.value, week_start: weekRange.value[0], week_end: weekRange.value[1] }
    if (editId.value) {
      await request.put('/weekly-summaries', { ...payload, id: editId.value })
      ElMessage.success('更新成功')
    } else {
      await request.post('/weekly-summaries', payload)
      ElMessage.success('录入成功')
    }
    dialogVisible.value = false
    loadData()
  } catch (e) {}
}

const removeSummary = async (row) => {
  try {
    await ElMessageBox.confirm('确认删除该周工作总结？', '删除确认', { type: 'warning', confirmButtonText: '删除' })
  } catch (e) { return }
  try {
    await request.delete(`/weekly-summaries/${row.id}`)
    ElMessage.success('删除成功')
    loadData()
  } catch (e) {}
}

const exportData = () => {
  const params = {}
  if (weekPicker.value && weekPicker.value.length === 2) {
    params.week_start = weekPicker.value[0]
    params.week_end = weekPicker.value[1]
  }
  exportFile('/export/weekly-summaries', params)
}

onMounted(() => {
  loadData()
  loadDepartments()
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
</style>
