<template>
  <div>
    <el-card shadow="never">
      <div class="toolbar">
        <div>
          <el-date-picker v-model="yearFilter" type="year" value-format="YYYY" placeholder="按年筛选"
            clearable style="width:130px" @change="loadData" />
          <el-date-picker v-model="monthFilter" type="month" value-format="YYYY-MM" placeholder="按月筛选"
            clearable style="width:130px;margin-left:10px" @change="loadData" />
          <el-select v-if="authStore.isAdmin" v-model="deptFilter" placeholder="全部科室" clearable style="width:150px;margin-left:10px" @change="loadData">
            <el-option v-for="d in departments" :key="d.id" :label="d.name" :value="d.id" />
          </el-select>
        </div>
        <div>
          <el-button v-if="authStore.isAdmin" type="success" :icon="'Download'" @click="exportData">导出Word（按年汇总）</el-button>
          <el-button type="primary" :icon="'Plus'" @click="openCreate">录入大事记</el-button>
        </div>
      </div>

      <el-table :data="list" stripe v-loading="loading" empty-text="暂无大事记">
        <el-table-column prop="department_name" label="科室" width="130" />
        <el-table-column prop="period" label="日期" width="110" />
        <el-table-column prop="title" label="重大事项" min-width="280" show-overflow-tooltip />
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button link type="warning" size="small" @click="openEdit(row)">编辑</el-button>
            <el-button link type="danger" size="small" @click="removeEvent(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editId ? '编辑大事记' : '录入大事记'" width="620px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="所属科室" required v-if="authStore.isAdmin">
          <el-select v-model="form.department_id" placeholder="选择科室" style="width:100%">
            <el-option v-for="d in departments" :key="d.id" :label="d.name" :value="d.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="日期" required>
          <el-date-picker v-model="form.period" type="date" value-format="YYYY-MM-DD" style="width:100%" placeholder="选择日期" />
        </el-form-item>
        <el-form-item label="重大事项" required>
          <el-input v-model="form.title" type="textarea" :rows="6" placeholder="请输入本日重大事项" />
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
const yearFilter = ref('')
const monthFilter = ref('')
const deptFilter = ref('')
const departments = ref([])
const dialogVisible = ref(false)
const editId = ref(0)
const form = ref({ department_id: '', period: dayjs().format('YYYY-MM-DD'), title: '' })

const loadDepartments = async () => {
  try {
    const res = await request.get('/departments')
    departments.value = res.list || []
  } catch (e) {}
}

const loadData = async () => {
  loading.value = true
  try {
    const params = {}
    if (yearFilter.value) params.year = yearFilter.value
    if (monthFilter.value) params.month = monthFilter.value
    if (deptFilter.value) params.department_id = deptFilter.value
    const res = await request.get('/major-events', { params })
    list.value = res.list || []
  } catch (e) {
  } finally {
    loading.value = false
  }
}

const openCreate = () => {
  editId.value = 0
  form.value = { department_id: authStore.isAdmin ? '' : (authStore.user?.department_id || ''), period: dayjs().format('YYYY-MM-DD'), title: '' }
  dialogVisible.value = true
}

const openEdit = (row) => {
  editId.value = row.id
  form.value = {
    department_id: row.department_id, period: row.period, title: row.title
  }
  dialogVisible.value = true
}

const save = async () => {
  if (!form.value.period) return ElMessage.warning('请选择日期')
  if (!form.value.title) return ElMessage.warning('请输入重大事项')
  try {
    if (editId.value) {
      await request.put('/major-events', { ...form.value, id: editId.value })
      ElMessage.success('更新成功')
    } else {
      await request.post('/major-events', form.value)
      ElMessage.success('录入成功')
    }
    dialogVisible.value = false
    loadData()
  } catch (e) {}
}

const removeEvent = async (row) => {
  try {
    await ElMessageBox.confirm(`确认删除大事记「${row.title}」？`, '删除确认', { type: 'warning', confirmButtonText: '删除' })
  } catch (e) { return }
  try {
    await request.delete(`/major-events/${row.id}`)
    ElMessage.success('删除成功')
    loadData()
  } catch (e) {}
}

const exportData = () => {
  const params = { year: yearFilter.value || dayjs().format('YYYY') }
  exportFile('/export/major-events', params)
}

onMounted(() => {
  loadData()
  loadDepartments()
})</script>

<style scoped>
.toolbar {
  display: flex;
  justify-content: space-between;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 8px;
}
</style>
