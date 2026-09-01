<template>
  <div>
    <el-card shadow="never">
      <div class="toolbar">
        <div>
          <el-date-picker v-model="year" type="year" value-format="YYYY" placeholder="选择年份" style="width:140px" @change="loadData" />
        </div>
        <div>
          <el-button v-if="authStore.isAdmin" type="success" :icon="'Download'" @click="exportData">导出Excel</el-button>
          <el-button v-if="authStore.isAdmin" type="primary" :icon="'Plus'" @click="openCreate">配置年休假</el-button>
        </div>
      </div>

      <div class="section-title">{{ year }}年 年休假统计</div>
      <el-table :data="list" stripe v-loading="loading" empty-text="暂无数据">
        <el-table-column prop="user_name" label="姓名" width="120" />
        <el-table-column prop="department" label="部门" width="140" />
        <el-table-column label="配置天数" width="110">
          <template #default="{ row }">{{ row.config_days }}</template>
        </el-table-column>
        <el-table-column label="已休天数" width="110">
          <template #default="{ row }">
            <b style="color:#e6a23c">{{ row.used_days }}</b>
          </template>
        </el-table-column>
        <el-table-column label="剩余天数" width="110">
          <template #default="{ row }">
            <b :style="{ color: row.remain_days < 0 ? '#f56c6c' : '#67c23a' }">{{ row.remain_days }}</b>
          </template>
        </el-table-column>
        <el-table-column v-if="authStore.isAdmin" label="操作" width="100" fixed="right">
          <template #default="{ row }">
            <el-button link type="warning" size="small" @click="openEdit(row)">配置</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 配置年休假 -->
    <el-dialog v-model="dialogVisible" title="配置年休假" width="460px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="人员" required>
          <el-select v-model="form.user_id" filterable placeholder="选择人员" style="width:100%">
            <el-option v-for="a in assignees" :key="a.id" :label="a.real_name" :value="a.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="年份" required>
          <el-date-picker v-model="form.year" type="year" value-format="YYYY" style="width:100%" />
        </el-form-item>
        <el-form-item label="年休假天数" required>
          <el-input-number v-model="form.days" :min="0" :max="60" :step="0.5" style="width:100%" />
          <div class="form-tip">单位：天（默认年假 5-15 天，按工龄可调整）</div>
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
import { ElMessage } from 'element-plus'
import request, { exportFile } from '../utils/request'
import dayjs from 'dayjs'
import { useAuthStore } from '../store/auth'

const authStore = useAuthStore()
const year = ref(dayjs().format('YYYY'))
const list = ref([])
const loading = ref(false)
const assignees = ref([])
const dialogVisible = ref(false)
const form = ref({ user_id: '', year: dayjs().format('YYYY'), days: 0 })

const loadAssignees = async () => {
  try {
    const res = await request.get('/assignees')
    assignees.value = res.list || []
  } catch (e) {}
}

const loadData = async () => {
  loading.value = true
  try {
    const res = await request.get('/annual-leave-configs', { params: { year: year.value } })
    list.value = res.list || []
  } catch (e) {
  } finally {
    loading.value = false
  }
}

const openCreate = () => {
  form.value = { user_id: '', year: dayjs().format('YYYY'), days: 0 }
  dialogVisible.value = true
}

const openEdit = (row) => {
  form.value = { user_id: row.user_id, year: year.value, days: row.config_days }
  dialogVisible.value = true
}

const save = async () => {
  if (!form.value.user_id) return ElMessage.warning('请选择人员')
  if (!form.value.year) return ElMessage.warning('请选择年份')
  try {
    await request.post('/annual-leave-configs', form.value)
    ElMessage.success('保存成功')
    dialogVisible.value = false
    loadData()
  } catch (e) {}
}

const exportData = () => {
  const params = { year: year.value }
  exportFile('/export/annual-leave-configs', params)
}

onMounted(() => {
  loadData()
  if (authStore.isAdmin) loadAssignees()
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
.form-tip {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}
</style>
