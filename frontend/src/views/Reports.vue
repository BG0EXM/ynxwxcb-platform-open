<template>
  <div>
    <el-card shadow="never">
      <div class="toolbar">
        <div>
          <el-radio-group v-model="reportType" @change="loadData">
            <el-radio-button value="">全部</el-radio-button>
            <el-radio-button value="weekly">周报</el-radio-button>
            <el-radio-button value="monthly">月报</el-radio-button>
            <el-radio-button value="yearly">年报</el-radio-button>
          </el-radio-group>
          <el-date-picker v-model="yearFilter" type="year" value-format="YYYY" placeholder="按年筛选"
            clearable style="width:130px;margin-left:10px" @change="loadData" />
        </div>
        <el-button type="primary" @click="openCreate">提交报表</el-button>
      </div>

      <el-table :data="list" stripe v-loading="loading" empty-text="暂无报表">
        <el-table-column label="类型" width="80">
          <template #default="{ row }">
            <el-tag size="small" :type="reportTypeTag(row.report_type)">{{ reportTypeName(row.report_type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="title" label="标题" min-width="180" show-overflow-tooltip />
        <el-table-column prop="period" label="周期" width="120" />
        <el-table-column prop="submitter" label="提交人" width="100" />
        <el-table-column prop="created_at" label="提交时间" width="160">
          <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="statusType(row.status)">{{ statusNames[row.status] }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click="openDetail(row)">查看</el-button>
            <el-button v-if="authStore.isAdmin || row.submitter_id === authStore.user?.id" link type="warning" size="small" @click="openEdit(row)">编辑</el-button>
            <el-button v-if="authStore.isAdmin" link type="success" size="small" @click="review(row)">审阅</el-button>
            <el-button v-if="authStore.isAdmin || row.submitter_id === authStore.user?.id" link type="danger" size="small" @click="removeReport(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-wrap" v-if="total > 0">
        <el-pagination background layout="total, prev, pager, next" :total="total" :page-size="pageSize"
          :current-page="page" @current-change="onPageChange" />
      </div>
    </el-card>

    <!-- 提交报表 -->
    <el-dialog v-model="dialogVisible" :title="editId ? '编辑报表' : '提交报表'" width="600px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="报表类型">
          <el-radio-group v-model="form.report_type">
            <el-radio value="weekly">周报</el-radio>
            <el-radio value="monthly">月报</el-radio>
            <el-radio value="yearly">年报</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="周期">
          <el-input v-model="form.period" placeholder="如：2026年第31周 / 2026年7月 / 2026年度" />
        </el-form-item>
        <el-form-item label="标题" required>
          <el-input v-model="form.title" />
        </el-form-item>
        <el-form-item label="内容" required>
          <el-input v-model="form.content" type="textarea" :rows="8" placeholder="工作情况、成效、问题及下步计划" />
        </el-form-item>
        <el-form-item label="提交方式">
          <el-radio-group v-model="form.status">
            <el-radio :label="1">存草稿</el-radio>
            <el-radio :label="2">直接提交</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <!-- 详情 -->
    <el-dialog v-model="detailVisible" :title="detail.title" width="640px">
      <div class="meta">
        <el-tag size="small">{{ reportTypeName(detail.report_type) }}</el-tag>
        <span>周期：{{ detail.period }}</span>
        <span>提交人：{{ detail.submitter }}</span>
      </div>
      <el-divider />
      <div class="content">{{ detail.content }}</div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '../utils/request'
import dayjs from 'dayjs'
import { useAuthStore } from '../store/auth'

const authStore = useAuthStore()
const list = ref([])
const loading = ref(false)
const reportType = ref('')
const yearFilter = ref('')
const page = ref(1)
const pageSize = 10
const total = ref(0)
const dialogVisible = ref(false)
const detailVisible = ref(false)
const editId = ref(0)
const form = reactive({ report_type: 'weekly', period: '', title: '', content: '', status: 2 })
const detail = ref({})

const statusNames = { 1: '草稿', 2: '已提交', 3: '已审阅' }
const statusType = (s) => ({ 1: 'info', 2: 'primary', 3: 'success' }[s])
const reportTypeName = (t) => ({ weekly: '周报', monthly: '月报', yearly: '年报' }[t] || t)
const reportTypeTag = (t) => ({ weekly: 'primary', monthly: 'success', yearly: 'warning' }[t] || 'info')
const formatDate = (d) => d ? dayjs(d).format('YYYY-MM-DD HH:mm') : '—'

const loadData = async () => {
  loading.value = true
  try {
    const params = { report_type: reportType.value, page: page.value, page_size: pageSize }
    if (yearFilter.value) params.year = yearFilter.value
    const res = await request.get('/reports', { params })
    list.value = res.list || []
    total.value = res.total || 0
  } catch (e) {
  } finally {
    loading.value = false
  }
}

const onPageChange = (p) => {
  page.value = p
  loadData()
}

const openCreate = () => {
  editId.value = 0
  Object.assign(form, { report_type: 'weekly', period: '', title: '', content: '', status: 2 })
  dialogVisible.value = true
}

const openEdit = async (row) => {
  editId.value = row.id
  Object.assign(form, {
    report_type: row.report_type, period: row.period || '', title: row.title,
    content: row.content || '', status: row.status
  })
  dialogVisible.value = true
}

const save = async () => {
  if (!form.title) return ElMessage.warning('请输入标题')
  if (!form.content) return ElMessage.warning('请输入内容')
  try {
    if (editId.value) {
      await request.put('/reports', { ...form, id: editId.value })
      ElMessage.success('修改成功')
    } else {
      await request.post('/reports', form)
      ElMessage.success(form.status === 2 ? '提交成功' : '已存草稿')
    }
    dialogVisible.value = false
    loadData()
  } catch (e) {}
}

const removeReport = async (row) => {
  try {
    await ElMessageBox.confirm(`确认删除报表「${row.title}」？`, '删除确认', { type: 'warning', confirmButtonText: '删除' })
  } catch (e) { return }
  try {
    await request.delete(`/reports/${row.id}`)
    ElMessage.success('删除成功')
    loadData()
  } catch (e) {}
}

const openDetail = async (row) => {
  try {
    const res = await request.get(`/reports/${row.id}`)
    detail.value = res
    detailVisible.value = true
  } catch (e) {}
}

const review = async (row) => {
  try {
    await ElMessageBox.confirm('确认审阅该报表？', '审阅确认', { type: 'success' })
  } catch (e) { return }
  try {
    await request.post(`/reports/${row.id}/status`, { status: 3 })
    ElMessage.success('已审阅')
    loadData()
  } catch (e) {}
}

onMounted(loadData)
</script>

<style scoped>
.toolbar {
  display: flex;
  justify-content: space-between;
  margin-bottom: 16px;
}
.meta {
  display: flex;
  gap: 16px;
  align-items: center;
  color: #909399;
  font-size: 13px;
}
.content {
  white-space: pre-wrap;
  line-height: 1.8;
}
</style>
