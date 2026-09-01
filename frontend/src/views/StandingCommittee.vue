<template>
  <div>
    <el-card shadow="never">
      <div class="toolbar">
        <div>
          <el-date-picker v-model="year" type="year" value-format="YYYY" placeholder="按年筛选" style="width:130px" @change="loadData" />
          <el-date-picker v-model="month" type="month" value-format="YYYY-MM" placeholder="按月筛选" clearable style="width:150px;margin-left:10px" @change="loadData" />
        </div>
        <div>
          <el-button type="success" :icon="'Download'" @click="exportData">导出Word</el-button>
          <el-button type="primary" :icon="'Plus'" @click="openCreate">录入大事记</el-button>
        </div>
      </div>

      <el-table :data="list" stripe v-loading="loading" empty-text="暂无常委大事记">
        <el-table-column prop="event_date" label="日期" width="120" />
        <el-table-column prop="title" label="事项" min-width="280" show-overflow-tooltip />
        <el-table-column prop="created_name" label="录入人" width="100" />
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button link type="warning" size="small" @click="openEdit(row)">编辑</el-button>
            <el-button link type="danger" size="small" @click="removeEvent(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editId ? '编辑大事记' : '录入大事记'" width="560px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="日期" required>
          <el-date-picker v-model="form.event_date" type="date" value-format="YYYY-MM-DD" style="width:100%" />
        </el-form-item>
        <el-form-item label="事项" required>
          <el-input v-model="form.title" type="textarea" :rows="4" placeholder="如：赴 XX 县调研" />
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

const list = ref([])
const loading = ref(false)
const year = ref(dayjs().format('YYYY'))
const month = ref('')
const dialogVisible = ref(false)
const editId = ref(0)
const form = ref({ event_date: dayjs().format('YYYY-MM-DD'), title: '' })

const loadData = async () => {
  loading.value = true
  try {
    const params = { year: year.value }
    if (month.value) params.month = month.value
    const res = await request.get('/standing-events', { params })
    list.value = res.list || []
  } catch (e) {
  } finally {
    loading.value = false
  }
}

const openCreate = () => {
  editId.value = 0
  form.value = { event_date: dayjs().format('YYYY-MM-DD'), title: '' }
  dialogVisible.value = true
}

const openEdit = (row) => {
  editId.value = row.id
  form.value = { event_date: row.event_date, title: row.title }
  dialogVisible.value = true
}

const save = async () => {
  if (!form.value.event_date) return ElMessage.warning('请选择日期')
  if (!form.value.title) return ElMessage.warning('请输入事项')
  try {
    if (editId.value) {
      await request.put('/standing-events', { ...form.value, id: editId.value })
      ElMessage.success('更新成功')
    } else {
      await request.post('/standing-events', form.value)
      ElMessage.success('录入成功')
    }
    dialogVisible.value = false
    loadData()
  } catch (e) {}
}

const removeEvent = async (row) => {
  try {
    await ElMessageBox.confirm(`确认删除该大事记「${row.title}」？`, '删除确认', { type: 'warning', confirmButtonText: '删除' })
  } catch (e) { return }
  try {
    await request.delete(`/standing-events/${row.id}`)
    ElMessage.success('删除成功')
    loadData()
  } catch (e) {}
}

const exportData = () => {
  const params = { year: year.value }
  if (month.value) params.month = month.value
  exportFile('/export/standing-events', params)
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
</style>
