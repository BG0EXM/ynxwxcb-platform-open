<template>
  <div class="leave-page">
    <!-- 页头 -->
    <page-header 
      title="请假管理" 
      subtitle="规范登记年休假及各类法定假期，支持请假审批条一键套打与年度统计"
      :tag="stats.total_days ? `本年累计请假 ${stats.total_days} 天` : ''"
    >
      <template #actions>
        <el-button v-if="authStore.hasPerm('leave.apply')" type="primary" :icon="'Plus'" @click="openCreate">请假申请</el-button>
        <el-button v-if="authStore.hasPerm('leave.export')" type="success" :icon="'Download'" @click="exportData">导出Excel</el-button>
      </template>
    </page-header>

    <!-- 请假分类统计指标卡 -->
    <el-card shadow="never" class="gov-card">
      <template #header>
        <div class="card-header-flex">
          <div class="card-title-wrap">
            <span class="card-title font-serif">{{ stats.year }} 年度请假汇总</span>
          </div>
          <div class="header-right">
            <el-date-picker 
              v-model="statsYear" 
              type="year" 
              value-format="YYYY" 
              placeholder="选择统计年份" 
              style="width: 140px" 
              @change="loadStats" 
            />
          </div>
        </div>
      </template>

      <el-row :gutter="12">
        <el-col :xs="12" :sm="6" :md="3" v-for="c in statItems" :key="c.key">
          <div class="stat-item" :style="{ borderTop: '3px solid ' + c.color }">
            <div class="stat-num tabular-nums">{{ c.days }}</div>
            <div class="stat-label">{{ c.label }}（{{ c.count }}次）</div>
          </div>
        </el-col>
        <el-col :xs="12" :sm="6" :md="3">
          <div class="stat-item total-item">
            <div class="stat-num total tabular-nums">{{ stats.total_days }}</div>
            <div class="stat-label">总计已休（{{ stats.total_count }}次）</div>
          </div>
        </el-col>
      </el-row>
    </el-card>

    <!-- 请假台账记录 -->
    <el-card shadow="never" class="mt-16 gov-card">
      <div class="toolbar">
        <div class="filter-left">
          <el-date-picker 
            v-model="monthFilter" 
            type="month" 
            value-format="YYYY-MM" 
            placeholder="按年月筛选" 
            clearable 
            style="width: 150px" 
            @change="reloadFirstPage" 
          />
          <el-select 
            v-model="filterType" 
            placeholder="全部假期类型" 
            clearable 
            style="width: 140px" 
            @change="reloadFirstPage"
          >
            <el-option v-for="(name, val) in leaveTypeNames" :key="val" :label="name" :value="val" />
          </el-select>
          <el-select 
            v-if="authStore.isAdmin" 
            v-model="userFilter" 
            placeholder="按人员筛选" 
            clearable 
            style="width: 140px" 
            @change="reloadFirstPage"
          >
            <el-option v-for="a in assignees" :key="a.id" :label="a.real_name" :value="a.id" />
          </el-select>
        </div>
        <div>
          <el-button :icon="'Refresh'" circle @click="loadData" />
        </div>
      </div>

      <el-table :data="list" v-loading="loading" class="mt-12" size="large">
        <template #empty>
          <empty-state description="暂无请假报备记录" />
        </template>

        <el-table-column prop="user_name" label="姓名" width="110" v-if="authStore.isAdmin" />
        
        <el-table-column label="假期类型" width="110">
          <template #default="{ row }">
            <el-tag size="small" effect="plain" :color="statColors[row.leave_type] ? `${statColors[row.leave_type]}15` : ''">
              {{ leaveTypeNames[row.leave_type] || row.leave_type }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column prop="start_date" label="起始日期" width="115" />
        <el-table-column prop="end_date" label="结束日期" width="115" />
        
        <el-table-column prop="days" label="休假天数" width="100" align="center">
          <template #default="{ row }">
            <b class="tabular-nums" style="color:var(--yx-text-1)">{{ row.days }} 天</b>
          </template>
        </el-table-column>

        <el-table-column prop="reason" label="请假事由" min-width="160" show-overflow-tooltip />

        <el-table-column label="操作" width="160" fixed="right" align="right">
          <template #default="{ row }">
            <div class="row-action-wrap">
              <el-button link type="primary" @click="openPrint(row)">打印假条</el-button>
              
              <el-dropdown 
                v-if="authStore.hasPerm('leave.manage') && (authStore.isAdmin || row.user_id === authStore.user?.id)"
                @command="(cmd) => handleRowCommand(cmd, row)" 
                trigger="click"
              >
                <el-button link type="primary" class="more-link">
                  <span>更多</span>
                  <el-icon class="el-icon--right"><ArrowDown /></el-icon>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="edit" icon="Edit">修改休假</el-dropdown-item>
                    <el-dropdown-item 
                      v-if="authStore.hasPerm('leave.delete')"
                      command="delete" 
                      icon="Delete" 
                      class="text-danger-item"
                    >
                      删除记录
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-wrap" v-if="total > 0">
        <el-pagination 
          background 
          layout="total, prev, pager, next" 
          :total="total" 
          :page-size="pageSize" 
          :current-page="page" 
          @current-change="onPageChange" 
        />
      </div>
    </el-card>

    <!-- 请假登记/编辑：右侧滑出抽屉（Drawer） -->
    <el-drawer 
      v-model="drawerVisible" 
      :title="editId ? '编辑休假申请' : (authStore.isAdmin ? '登记干部休假' : '提交请假申请')" 
      size="560px"
      destroy-on-close
    >
      <el-form ref="formRef" :model="form" :rules="rules" label-width="90px">
        <!-- 分组 1：申请主体与假期类型 -->
        <div class="form-section">
          <div class="form-section-title">休假主体与类型</div>
          <el-form-item label="休假人员" v-if="authStore.isAdmin" prop="user_id">
            <el-select v-model="form.user_id" filterable placeholder="选择干部职工" style="width:100%">
              <el-option v-for="a in assignees" :key="a.id" :label="a.real_name + (a.department ? ' (' + a.department + ')' : '')" :value="a.id" />
            </el-select>
          </el-form-item>

          <el-form-item label="假期类型" prop="leave_type">
            <el-select v-model="form.leave_type" placeholder="选择法定/因私假期" style="width:100%">
              <el-option v-for="(name, val) in leaveTypeNames" :key="val" :label="name" :value="val" />
            </el-select>
          </el-form-item>
        </div>

        <!-- 分组 2：起止日期与时长折算 -->
        <div class="form-section">
          <div class="form-section-title">休假时间与时长</div>
          <el-form-item label="时长计算">
            <el-radio-group v-model="form.duration_type" size="small" @change="onDurationChange">
              <el-radio-button value="day">按整天/半天</el-radio-button>
              <el-radio-button value="hour">按小时 (短假)</el-radio-button>
            </el-radio-group>
          </el-form-item>

          <template v-if="form.duration_type === 'day'">
            <el-row :gutter="16">
              <el-col :xs="24" :sm="12">
                <el-form-item label="起始日期" prop="start_date">
                  <el-date-picker v-model="form.start_date" type="date" value-format="YYYY-MM-DD" style="width:100%" @change="calcDays" />
                </el-form-item>
              </el-col>
              <el-col :xs="24" :sm="12">
                <el-form-item label="结束日期" prop="end_date">
                  <el-date-picker v-model="form.end_date" type="date" value-format="YYYY-MM-DD" style="width:100%" @change="calcDays" />
                </el-form-item>
              </el-col>
            </el-row>
            <el-form-item label="合计天数">
              <el-input-number v-model="form.days" :min="0.5" :max="365" :step="0.5" style="width:130px" />
              <span class="ml-8 text-secondary">天</span>
            </el-form-item>
          </template>

          <template v-else>
            <el-row :gutter="16">
              <el-col :xs="24" :sm="12">
                <el-form-item label="休假日期" prop="start_date">
                  <el-date-picker v-model="form.start_date" type="date" value-format="YYYY-MM-DD" style="width:100%" @change="syncHourDate" />
                </el-form-item>
              </el-col>
              <el-col :xs="24" :sm="12">
                <el-form-item label="请假小时">
                  <el-input-number v-model="form.leave_hours" :min="0.5" :max="8" :step="0.5" style="width:120px" @change="calcByHour" />
                  <span class="ml-8 text-secondary">小时</span>
                </el-form-item>
              </el-col>
            </el-row>
            <el-form-item label="快捷选择">
              <el-button size="small" @click="setHours(4)">半天 (4小时)</el-button>
              <el-button size="small" @click="setHours(2)">2小时</el-button>
              <el-button size="small" @click="setHours(1)">1小时</el-button>
            </el-form-item>
          </template>
        </div>

        <!-- 分组 3：事由 -->
        <div class="form-section">
          <div class="form-section-title">事由与审批备注</div>
          <el-form-item label="请假事由" prop="reason">
            <el-input 
              v-model="form.reason" 
              type="textarea" 
              :rows="3" 
              placeholder="请输入休假具体事由（如回原籍探亲、照料老人就医等）" 
            />
          </el-form-item>
        </div>
      </el-form>

      <template #footer>
        <div class="drawer-footer-wrap">
          <el-button @click="drawerVisible = false">取消</el-button>
          <el-button type="primary" @click="saveLeave">确认提交申请</el-button>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus'
import request, { exportFile } from '../utils/request'
import dayjs from 'dayjs'
import { useAuthStore } from '../store/auth'
import PageHeader from '../components/PageHeader.vue'
import EmptyState from '../components/EmptyState.vue'

const authStore = useAuthStore()
const list = ref([])
const loading = ref(false)
const page = ref(1)
const pageSize = 20
const total = ref(0)
const editId = ref(0)
const drawerVisible = ref(false)
const stats = reactive({ year: dayjs().format('YYYY'), total_count: 0, total_days: 0 })
const statsYear = ref(dayjs().format('YYYY'))
const filterType = ref('')
const userFilter = ref('')
const monthFilter = ref('')
const assignees = ref([])
const formRef = ref(null)
const form = reactive({ user_id: null, leave_type: 'annual', duration_type: 'day', start_date: dayjs().format('YYYY-MM-DD'), end_date: '', days: 1, leave_hours: 4, reason: '' })
const rules = reactive({
  user_id: [{ required: true, message: '请选择人员', trigger: 'change' }],
  leave_type: [{ required: true, message: '请选择假期类型', trigger: 'change' }],
  start_date: [{ required: true, message: '请选择开始日期', trigger: 'change' }],
  end_date: [{ 
    validator: (rule, value, callback) => {
      if (form.duration_type === 'day' && !value) callback(new Error('请选择结束日期'))
      else callback()
    }, trigger: 'change' 
  }],
  reason: [
    { required: true, message: '请填写请假事由', trigger: 'blur' },
    { max: 200, message: '事由不超过 200 个字符', trigger: 'blur' }
  ]
})

const leaveTypeNames = {
  annual: '年休假', sick: '病假', personal: '事假', marriage: '婚假',
  maternity: '产假', bereavement: '丧假', prenatal: '产检假', family: '探亲假',
  training: '培训假', comp: '调休补休', other: '其他假'
}

const statColors = {
  annual: '#24406e', sick: '#2e7d5b', personal: '#b7791f', marriage: '#c4312b',
  maternity: '#6b7785', bereavement: '#1f2329', prenatal: '#8c6a3a', family: '#3d619b',
  training: '#52796f', comp: '#409eff', other: '#8a94a0'
}

const statItems = ref([])

const loadStats = async () => {
  try {
    const res = await request.get('/leaves/stats', { params: { year: statsYear.value } })
    stats.year = res.year || statsYear.value
    stats.total_count = res.total_count || 0
    stats.total_days = res.total_days || 0

    let types = res.types
    if (!types || !types.length) {
      types = Object.keys(leaveTypeNames).map(k => ({
        type: k,
        days: res[k + '_days'] ?? 0,
        count: res[k + '_count'] ?? 0
      }))
    }
    statItems.value = types.map(t => ({
      key: t.type,
      label: leaveTypeNames[t.type] || t.type,
      days: t.days ?? 0,
      count: t.count ?? 0,
      color: statColors[t.type] || '#909399'
    }))
  } catch (e) { console.error(e) }
}

const loadAssignees = async () => {
  try {
    const res = await request.get('/assignees')
    assignees.value = res.list || []
  } catch (e) { console.error(e) }
}

const loadData = async () => {
  loading.value = true
  try {
    const params = { page: page.value, page_size: pageSize }
    if (filterType.value) params.leave_type = filterType.value
    if (userFilter.value) params.user_id = userFilter.value
    if (monthFilter.value) params.month = monthFilter.value
    const res = await request.get('/leaves', { params })
    list.value = res.list || []
    total.value = res.total || 0
  } catch (e) { console.error(e) } finally {
    loading.value = false
  }
}

const reloadFirstPage = () => {
  page.value = 1
  loadData()
}

const onPageChange = (p) => {
  page.value = p
  loadData()
}

const onDurationChange = (val) => {
  if (val === 'day') {
    calcDays()
  } else {
    calcByHour()
  }
}

const calcDays = () => {
  if (form.start_date && form.end_date) {
    const s = dayjs(form.start_date)
    const e = dayjs(form.end_date)
    const diff = e.diff(s, 'day') + 1
    if (diff > 0) form.days = diff
  }
}

const syncHourDate = () => {
  form.end_date = form.start_date
}

const calcByHour = () => {
  form.days = Math.round((form.leave_hours / 8) * 10) / 10
  form.end_date = form.start_date
}

const setHours = (h) => {
  form.leave_hours = h
  calcByHour()
}

const openCreate = () => {
  editId.value = 0
  Object.assign(form, {
    user_id: authStore.isAdmin ? null : authStore.user?.id,
    leave_type: 'annual',
    duration_type: 'day',
    start_date: dayjs().format('YYYY-MM-DD'),
    end_date: dayjs().format('YYYY-MM-DD'),
    days: 1,
    leave_hours: 4,
    reason: ''
  })
  drawerVisible.value = true
}

const openEdit = (row) => {
  editId.value = row.id
  Object.assign(form, {
    user_id: row.user_id,
    leave_type: row.leave_type,
    duration_type: row.days < 1 && row.days * 8 <= 8 ? 'hour' : 'day',
    start_date: row.start_date,
    end_date: row.end_date,
    days: row.days,
    leave_hours: Math.round(row.days * 8 * 10) / 10,
    reason: row.reason
  })
  drawerVisible.value = true
}

const saveLeave = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    try {
      const payload = {
        user_id: form.user_id || authStore.user?.id,
        leave_type: form.leave_type,
        start_date: form.start_date,
        end_date: form.duration_type === 'hour' ? form.start_date : form.end_date,
        days: form.days,
        leave_hours: form.duration_type === 'hour' ? form.leave_hours : 0,
        reason: form.reason
      }
      if (editId.value) {
        await request.put('/leaves', { ...payload, id: editId.value })
        ElMessage.success('修改成功')
      } else {
        await request.post('/leaves', payload)
        ElMessage.success('请假登记成功')
      }
      drawerVisible.value = false
      loadData()
      loadStats()
    } catch (e) { console.error(e) }
  })
}

const handleRowCommand = (cmd, row) => {
  if (cmd === 'edit') openEdit(row)
  else if (cmd === 'delete') removeLeave(row)
}

const removeLeave = async (row) => {
  try {
    await ElMessageBox.confirm('确认删除该请假记录？', '提示', {
      type: 'warning',
      confirmButtonText: '确认删除',
      confirmButtonClass: 'el-button--danger'
    })
    await request.delete(`/leaves/${row.id}`)
    ElMessage.success('已删除')
    loadData()
    loadStats()
  } catch (e) { console.error(e) }
}

const openPrint = (row) => {
  sessionStorage.setItem('printLeave', JSON.stringify(row))
  const url = `/leave/print/${row.id}`
  window.open(url, '_blank')
}

const exportData = async () => {
  try {
    const params = {}
    if (filterType.value) params.leave_type = filterType.value
    if (userFilter.value) params.user_id = userFilter.value
    if (monthFilter.value) params.month = monthFilter.value
    await exportFile('/export/leaves', params, '请假登记台账.xlsx')
  } catch (e) { console.error(e) }
}

onMounted(() => {
  loadData()
  loadStats()
  loadAssignees()

  // 检查是否有从加班管理带来的“登记补休”人员信息
  const compUserRaw = localStorage.getItem('compUser')
  if (compUserRaw) {
    try {
      const comp = JSON.parse(compUserRaw)
      localStorage.removeItem('compUser')
      openCreate()
      if (comp.user_id) {
        form.user_id = comp.user_id
      }
      form.leave_type = 'comp'
      ElNotification({
        title: '已联动补休人员',
        message: `已自动载入【${comp.user_name || '补休人员'}】（剩余补休 ${comp.remain_days || 0} 天），请选择休假起止日期`,
        type: 'info',
        duration: 4500
      })
    } catch (e) {
      localStorage.removeItem('compUser')
    }
  }

  // 检查是否有从考勤点到带来的“未配置请假，引导填写”人员信息
  const leaveDraftRaw = localStorage.getItem('leaveApplyDraft')
  if (leaveDraftRaw) {
    try {
      const draft = JSON.parse(leaveDraftRaw)
      localStorage.removeItem('leaveApplyDraft')
      openCreate()
      if (draft.user_id) {
        form.user_id = draft.user_id
      }
      if (draft.date) {
        form.start_date = draft.date
        form.end_date = draft.date
        form.days = 1
      }
      ElNotification({
        title: '已自动联动点到人员',
        message: `正在为【${draft.real_name || '干部职工'}】填写 ${draft.date || ''} 的请假单，填写保存后返回考勤点到即可自动联动！`,
        type: 'success',
        duration: 5500
      })
    } catch (e) {
      localStorage.removeItem('leaveApplyDraft')
    }
  }
})
</script>

<style scoped>
.card-header-flex {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--yx-text-1);
}

.stat-item {
  background: var(--yx-surface);
  border: 1px solid var(--el-border-color-lighter);
  border-radius: var(--yx-radius-sm);
  padding: 12px 14px;
  text-align: center;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.02);
}

.stat-item.total-item {
  border-top: 3px solid var(--yx-brand);
  background: var(--yx-brand-bg);
}

.stat-num {
  font-size: 20px;
  font-weight: 700;
  color: var(--yx-text-1);
}

.stat-label {
  font-size: 12px;
  color: var(--yx-text-3);
  margin-top: 4px;
}

.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}

.filter-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.row-action-wrap {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

.more-link {
  font-size: 13px;
  color: var(--yx-text-3);
}

.more-link:hover {
  color: var(--yx-ink-2);
}

:deep(.text-danger-item) {
  color: var(--el-color-danger) !important;
}

.text-secondary {
  color: var(--yx-text-3);
  font-size: 13px;
}

.pagination-wrap {
  margin-top: 18px;
  display: flex;
  justify-content: flex-end;
}

.drawer-footer-wrap {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.mt-12 {
  margin-top: 12px;
}
.mt-16 {
  margin-top: 16px;
}
.ml-8 {
  margin-left: 8px;
}
</style>
