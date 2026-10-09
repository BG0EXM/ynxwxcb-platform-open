<template>
  <div class="attendance-page">
    <page-header 
      title="考勤点到" 
      subtitle="每日晨会规范手工点到（出勤/请假/出差/未到），支持月度与年度统计报表打印及导出"
    >
      <template #actions>
        <el-button v-if="authStore.hasPerm('attendance.stats')" type="primary" :icon="'DataAnalysis'" @click="goStats">考勤统计报表</el-button>
        <el-button type="success" :icon="'Download'" @click="exportData">导出Excel</el-button>
      </template>
    </page-header>

    <!-- 管理员点到操作区 -->
    <el-card shadow="never" v-if="authStore.hasPerm('attendance.mark')" class="mt-12">
      <template #header>
        <div class="card-header">
          <span class="card-title">晨会点到</span>
          <div class="header-right">
            <el-date-picker v-model="markDate" type="date" value-format="YYYY-MM-DD" placeholder="选择点到日期" style="width:160px" @change="loadMarkUsers" :cell-class-name="dateCellClass" />
            <el-button type="primary" :icon="'CircleCheck'" @click="markAllPresent" class="ml-8">全部出勤</el-button>
            <el-tag v-if="saved" type="success" class="ml-8">当日已点到</el-tag>
          </div>
        </div>
      </template>

      <div class="mark-area">
        <el-alert v-if="saved" type="info" :closable="false" class="saved-alert"
          title="当日点到结果已保存，可直接修改状态后重新保存（会覆盖原记录）。" show-icon />
        <div v-if="!markUsers.length" class="mark-empty">
          <el-empty description="暂无人员" :image-size="60" />
        </div>
        <el-table v-else :data="markUsers" size="small" :row-class-name="rowClass">
          <el-table-column prop="real_name" label="姓名" width="140">
            <template #default="{ row }">
              <span class="user-name-text">{{ row.real_name }}</span>
              <el-tag v-if="row.has_leave" type="warning" size="small" class="auto-leave-tag" effect="light">
                已请假{{ row.leave_record_type ? '·' + (leaveTypeNames[row.leave_record_type] || row.leave_record_type) : '' }}
              </el-tag>
              <el-tag v-else-if="row.auto_comp === 1" type="success" size="small" class="auto-leave-tag" effect="light">补休</el-tag>
              <el-tag v-else-if="row.status === 2 && !row.has_leave" type="danger" size="small" effect="plain" class="auto-leave-tag">未登记假条</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="department" label="部门" width="130" />
          <el-table-column label="状态" min-width="320">
            <template #default="{ row }">
              <div class="status-cell-wrap">
                <el-radio-group v-model="row.status" @change="(val) => handleStatusChange(row, val)">
                  <el-radio :label="1">出勤</el-radio>
                  <el-radio :label="2">请假</el-radio>
                  <el-radio :label="3">出差</el-radio>
                  <el-radio :label="4">未到</el-radio>
                  <el-radio :label="5">迟到</el-radio>
                  <el-radio :label="6">培训</el-radio>
                </el-radio-group>
                <div v-if="row.status === 2" class="leave-type-inline">
                  <el-select v-model="row.leave_type" size="small" placeholder="请假类型" style="width:110px">
                    <el-option v-for="(name, val) in leaveTypeNames" :key="val" :label="name" :value="val" />
                  </el-select>
                  <el-button v-if="!row.has_leave" link type="primary" size="small" @click="guideToLeave(row)">
                    去请假模块填写
                  </el-button>
                </div>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="备注" width="180">
            <template #default="{ row }">
              <el-input v-model="row.remark" size="small" placeholder="备注" />
            </template>
          </el-table-column>
        </el-table>
        <div class="mark-buttons" v-if="markUsers.length">
          <el-button type="primary" size="large" :loading="saving" @click="saveMark">保存点到结果</el-button>
        </div>
      </div>
    </el-card>

    <!-- 请假未配置引导对话框 -->
    <el-dialog 
      v-model="leaveGuideVisible" 
      title="请假模块未配置提示" 
      width="520px" 
      destroy-on-close
      :close-on-click-modal="false"
      class="leave-guide-dialog"
    >
      <div class="leave-guide-body">
        <div class="guide-header-tip">
          <el-icon class="guide-warn-icon" size="28" color="#e6a23c"><WarningFilled /></el-icon>
          <div class="guide-title">
            检测到【<b>{{ currentGuideRow?.real_name }}</b>】尚未在请假模块登记请假单
          </div>
        </div>
        <div class="guide-desc">
          <p>考勤点到与请假管理深度联动，点到中的请假数据应以<b>【请假管理】</b>模块的审批台账为基准。</p>
          <div class="guide-warn-box">
            <div class="guide-warn-title">⚠️ 若未在请假模块登记直接在点到中标记：</div>
            <ul class="guide-warn-list">
              <li>请假台账无对应记录，无法打印规范请假审批条；</li>
              <li>干部年休假/补休天数无法自动扣减核算；</li>
              <li>导致考勤统计与休假报表数据脱节、产生冲突。</li>
            </ul>
          </div>
          <p class="guide-action-tip">系统引导您先前往<b>【请假管理】</b>模块填写请假申请，保存后考勤点到将自动联动为请假状态。</p>
        </div>
      </div>

      <template #footer>
        <div class="guide-dialog-footer">
          <el-button @click="cancelGuide">取消</el-button>
          <el-button type="warning" plain @click="allowTempLeave">仍临时标记考勤</el-button>
          <el-button type="primary" @click="confirmGuideToLeave">前往请假模块填写</el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 考勤记录与统计 -->
    <el-row :gutter="16" class="mt-12">
      <el-col :xs="24" :md="8">
        <el-card shadow="never">
          <template #header>
            <span class="card-title">点到统计</span>
          </template>
          <div v-if="stats">
            <div class="stat-row"><span>日期</span><b>{{ stats.date }}</b></div>
            <div class="stat-row"><span>已点到</span><b>{{ stats.stats.total }} 人</b></div>
            <div class="stat-row"><span>出勤</span><b style="color:var(--el-color-success)">{{ stats.stats.present }} 人</b></div>
            <div class="stat-row"><span>请假</span><b style="color:var(--el-color-warning)">{{ stats.stats.leave }} 人</b></div>
            <div class="stat-row"><span>出差</span><b style="color:var(--el-color-primary)">{{ stats.stats.trip }} 人</b></div>
            <div class="stat-row"><span>未到</span><b style="color:var(--el-color-danger)">{{ stats.stats.absent }} 人</b></div>
            <div class="stat-row"><span>迟到</span><b style="color:var(--el-color-warning)">{{ stats.stats.late }} 人</b></div>
            <div class="stat-row"><span>培训</span><b style="color:#1d8a99">{{ stats.stats.training }} 人</b></div>
          </div>
        </el-card>
      </el-col>
      <el-col :xs="24" :md="16">
        <el-card shadow="never">
          <template #header>
            <div class="card-header">
              <span class="card-title">考勤记录</span>
              <div class="header-right">
                <el-date-picker v-model="queryDate" type="date" value-format="YYYY-MM-DD" placeholder="选择日期" style="width:160px" @change="reloadFirstPage" :cell-class-name="dateCellClass" />
                <el-select v-if="authStore.isAdmin" v-model="userFilter" placeholder="全部人员" clearable style="width:140px" class="ml-8" @change="reloadFirstPage">
                  <el-option v-for="a in assignees" :key="a.id" :label="a.real_name" :value="a.id" />
                </el-select>
              </div>
            </div>
          </template>
          <el-table :data="list" stripe v-loading="loading" empty-text="该日暂无考勤记录">
            <el-table-column prop="attend_date" label="日期" width="110" />
            <el-table-column v-if="authStore.isAdmin" prop="user_name" label="姓名" width="100" />
            <el-table-column label="状态" width="130">
              <template #default="{ row }">
                <el-tag size="small" :type="statusType(row.status)">{{ statusNames[row.status] }}</el-tag>
                <el-tag v-if="row.status === 2 && row.leave_type" size="small" type="info" class="ml-4">{{ leaveTypeNames[row.leave_type] }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="remark" label="备注" show-overflow-tooltip />
          </el-table>
          <div class="pagination-wrap" v-if="total > 0">
            <el-pagination background layout="total, prev, pager, next" :total="total" :page-size="pageSize" :current-page="page" @current-change="onPageChange" />
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import request, { exportFile } from '../utils/request'
import dayjs from 'dayjs'
import { useAuthStore } from '../store/auth'
import PageHeader from '../components/PageHeader.vue'

const authStore = useAuthStore()
const router = useRouter()
const markDate = ref(dayjs().format('YYYY-MM-DD'))
const queryDate = ref(dayjs().format('YYYY-MM-DD'))
const markUsers = ref([])
const list = ref([])
const page = ref(1)
const pageSize = 20
const total = ref(0)
const stats = ref(null)
const loading = ref(false)
const saving = ref(false)
const saved = ref(false)
const userFilter = ref('')
const assignees = ref([])
const recordedDates = ref([])

const statusNames = { 1: '出勤', 2: '请假', 3: '出差', 4: '未到', 5: '迟到', 6: '培训' }
const statusType = (s) => ({ 1: 'success', 2: 'warning', 3: 'primary', 4: 'danger', 5: 'warning', 6: 'primary' }[s] || 'info')
const leaveTypeNames = {
  annual: '年假', sick: '病假', personal: '事假', marriage: '婚假',
  maternity: '产假', bereavement: '丧假', prenatal: '产检假', family: '探亲假',
  training: '培训', other: '其他'
}

const leaveGuideVisible = ref(false)
const currentGuideRow = ref(null)

// 自动请假的用户行高亮
const rowClass = ({ row }) => {
  if (row.has_leave || row.auto_leave === 1) return 'auto-leave-row'
  return ''
}

const loadMarkUsers = async () => {
  if (!authStore.hasPerm('attendance.mark')) return
  try {
    const res = await request.get('/attendance/mark-users', { params: { date: markDate.value } })
    const userList = res.list || []
    userList.forEach(u => {
      u._lastStatus = u.status || 1
      if (!u.leave_type && u.leave_record_type) {
        u.leave_type = u.leave_record_type
      }
    })
    markUsers.value = userList
    saved.value = !!res.saved
  } catch (e) { console.error(e) }
}

const handleStatusChange = (row, val) => {
  if (val === 2) {
    // 切换到请假状态
    if (!row.has_leave) {
      // 未在请假模块配置：弹出引导对话框
      currentGuideRow.value = row
      leaveGuideVisible.value = true
      return
    }
    // 已在请假模块配置：自动带出请假类型
    if (row.leave_record_type && !row.leave_type) {
      row.leave_type = row.leave_record_type
    }
    row._lastStatus = 2
  } else {
    row._lastStatus = val
  }
}

const guideToLeave = (row) => {
  currentGuideRow.value = row
  confirmGuideToLeave()
}

const confirmGuideToLeave = () => {
  if (!currentGuideRow.value) return
  const row = currentGuideRow.value
  const draft = {
    user_id: row.id,
    real_name: row.real_name,
    date: markDate.value
  }
  localStorage.setItem('leaveApplyDraft', JSON.stringify(draft))
  leaveGuideVisible.value = false
  // 将状态还原为原状态，避免未填写前保存错误
  row.status = row._lastStatus || 1
  router.push('/leave')
}

const allowTempLeave = () => {
  if (!currentGuideRow.value) return
  const row = currentGuideRow.value
  row.status = 2
  row._lastStatus = 2
  if (!row.leave_type) row.leave_type = 'personal'
  leaveGuideVisible.value = false
  ElMessage.warning(`已临时标记【${row.real_name}】为请假，请后续务必前往请假管理模块补录请假条`)
}

const cancelGuide = () => {
  if (currentGuideRow.value) {
    currentGuideRow.value.status = currentGuideRow.value._lastStatus || 1
  }
  leaveGuideVisible.value = false
}

const markAllPresent = () => {
  let skipped = 0
  markUsers.value.forEach(u => {
    if (u.status === 2) {
      skipped++
      return
    }
    u.status = 1
    u._lastStatus = 1
  })
  if (skipped > 0) ElMessage.info(`${skipped} 人已请假，保持请假状态不变`)
}

const saveMark = async () => {
  if (!markUsers.value.length) return ElMessage.warning('没有人员')

  // 检查是否有未在请假模块登记却选择了请假的人员
  const unconfiguredLeaves = markUsers.value.filter(u => u.status === 2 && !u.has_leave)
  let confirmMsg = `确认保存 ${markDate.value} 的考勤点到？保存后仍可再次修改。`
  if (unconfiguredLeaves.length > 0) {
    const names = unconfiguredLeaves.map(u => u.real_name).join('、')
    confirmMsg += `<br><br><span style="color:var(--el-color-warning);font-size:13px;line-height:1.6;">⚠️ <b>注意</b>：【${names}】选择了请假，但尚未在请假模块登记请假单。建议前往请假管理补填假条，以确保考勤与休假台账数据一致。</span>`
  }

  try {
    await ElMessageBox.confirm(confirmMsg, '点到确认', {
      type: unconfiguredLeaves.length > 0 ? 'warning' : 'info',
      dangerouslyUseHTMLString: true,
      confirmButtonText: '确定保存',
      cancelButtonText: '取消'
    })
  } catch (e) { return }

  saving.value = true
  try {
    const records = markUsers.value.map(u => ({
      user_id: u.id, status: u.status, leave_type: u.leave_type || '', remark: u.remark || ''
    }))
    await request.post('/attendance/mark', { attend_date: markDate.value, records })
    ElMessage.success('点到完成')
    loadMarkUsers()
    loadStats()
    loadData()
    loadRecordedDates()
  } catch (e) { console.error(e) } finally {
    saving.value = false
  }
}

const reloadFirstPage = () => { page.value = 1; loadData() }
const onPageChange = (p) => { page.value = p; loadData() }

const loadData = async () => {
  loading.value = true
  try {
    const params = { date: queryDate.value, page: page.value, page_size: pageSize }
    if (userFilter.value) params.user_id = userFilter.value
    const res = await request.get('/attendance/list', { params })
    list.value = res.list || []
    total.value = res.total || 0
  } catch (e) { console.error(e) } finally {
    loading.value = false
  }
}

const loadStats = async () => {
  if (!authStore.hasPerm('attendance.stats')) return
  try {
    const res = await request.get('/attendance/stats', { params: { date: queryDate.value } })
    stats.value = res
  } catch (e) { console.error(e) }
}

const loadAssignees = async () => {
  try {
    const res = await request.get('/assignees')
    assignees.value = res.list || []
  } catch (e) { console.error(e) }
}

const goStats = () => {
  router.push('/attendance/print')
}

const exportData = () => {
  const params = {}
  if (queryDate.value) params.date = queryDate.value
  exportFile('/export/attendances', params)
}

// 加载已点到日期集合（用于日历标色）
const loadRecordedDates = async () => {
  try {
    const res = await request.get('/attendance/dates')
    recordedDates.value = res.dates || []
  } catch (e) { console.error(e) }
}

// 日历单元格样式：已点到标绿，未点到标红
// Element Plus cell-class-name 直接传入 Date 对象
const dateCellClass = (date) => {
  if (!date) return ''
  const d = dayjs(date).format('YYYY-MM-DD')
  if (recordedDates.value.includes(d)) return 'att-done'
  // 未来日期不标红
  if (d > dayjs().format('YYYY-MM-DD')) return ''
  return 'att-missing'
}

onMounted(() => {
  if (authStore.hasPerm('attendance.mark')) {
    loadMarkUsers()
    loadAssignees()
  }
  loadData()
  loadStats()
  loadRecordedDates()
})
</script>

<style scoped>
.rule-alert {
  margin-bottom: 0;
}
.stats-entry {
  margin-top: 12px;
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
.ml-4 {
  margin-left: 4px;
}
.mark-area {
  min-height: 100px;
}
.mark-empty {
  padding: 20px 0;
}
.mark-buttons {
  margin-top: 16px;
  text-align: right;
}
.stat-row {
  display: flex;
  justify-content: space-between;
  padding: 8px 0;
  border-bottom: 1px dashed var(--el-border-color-lighter);
  color: var(--el-text-color-regular);
}
.stat-row:last-child {
  border-bottom: none;
}
.saved-alert {
  margin-bottom: 12px;
}
.auto-leave-tag {
  margin-left: 4px;
}
:deep(.auto-leave-row) {
  background: var(--el-color-warning-light-9);
}
.pagination-wrap {
  margin-top: 14px;
  display: flex;
  justify-content: flex-end;
}
.user-name-text {
  font-weight: 500;
}
.status-cell-wrap {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.leave-type-inline {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  margin-left: 4px;
}
.leave-guide-body {
  padding: 4px 8px;
}
.guide-header-tip {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 14px;
}
.guide-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  line-height: 1.4;
}
.guide-desc {
  font-size: 14px;
  line-height: 1.6;
  color: var(--el-text-color-regular);
}
.guide-warn-box {
  background: var(--el-color-warning-light-9);
  border: 1px solid var(--el-color-warning-light-7);
  border-radius: 6px;
  padding: 10px 14px;
  margin: 12px 0;
}
.guide-warn-title {
  font-weight: 600;
  color: var(--el-color-warning-dark-2);
  margin-bottom: 6px;
}
.guide-warn-list {
  margin: 0;
  padding-left: 20px;
  color: var(--el-color-warning-dark-2);
  font-size: 13px;
  line-height: 1.6;
}
.guide-action-tip {
  margin-top: 10px;
  color: var(--el-text-color-primary);
}
.guide-dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>

<!-- 全局样式：日历面板渲染在 popper 中，scoped 样式无法穿透 -->
<style>
.el-date-table td.att-done {
  background-color: var(--el-color-success) !important;
  border-radius: 50%;
}
.el-date-table td.att-done .el-date-table-cell__text {
  color: #fff !important;
}
.el-date-table td.att-missing {
  background-color: var(--el-color-danger) !important;
  border-radius: 50%;
}
.el-date-table td.att-missing .el-date-table-cell__text {
  color: #fff !important;
}
</style>
