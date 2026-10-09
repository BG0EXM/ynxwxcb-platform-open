<template>
  <div class="duty-page">
    <page-header 
      title="值守排班" 
      subtitle="值守干部当天值守至晚上21:00收文结束。若当日有县委大院排班，请在排班时勾选对应选项"
    >
      <template #actions>
        <el-tag type="warning" effect="plain" class="mr-8">值守至21:00收文</el-tag>
        <el-button type="primary" :icon="'Picture'" @click="openExportImageDialog">一键导出排班图片</el-button>
        <el-button type="success" :icon="'Download'" @click="exportData">导出Excel</el-button>
      </template>
    </page-header>

    <el-card shadow="never" class="mt-12">
      <div class="toolbar">
        <div>
          <el-date-picker v-model="currentMonth" type="month" value-format="YYYY-MM"
            placeholder="选择月份" @change="loadData" />
          <span class="ml-8 tip">{{ authStore.hasPerm('duty.manage') ? '点击日历格子安排当日值守人员' : '排班仅供查看，由管理员安排' }}</span>
        </div>
      </div>

      <div class="duty-calendar">
        <div class="week-header">
          <div v-for="w in weekNames" :key="w" class="week-cell">{{ w }}</div>
        </div>
        <div class="duty-grid">
          <div v-for="cell in calendarCells" :key="cell.dateStr" class="day-cell"
            :class="{ 'empty': !cell.date, 'today': cell.isToday }">
            <template v-if="cell.date">
              <div class="day-num">{{ cell.day }}</div>
              <div class="duty-box" :class="[cell.schedules.length ? 'assigned' : 'empty-shift', { 'clickable': authStore.hasPerm('duty.manage') }]"
                @click="authStore.hasPerm('duty.manage') && openEdit(cell.dateStr)">
                <template v-if="cell.schedules.length">
                  <div v-for="s in cell.schedules" :key="s.id" class="duty-person">
                    <div class="duty-person-line">
                      <span class="duty-name">{{ s.user_name }}</span>
                      <el-tag v-if="s.is_dawangyuan === 1" size="small" type="danger" effect="plain">大院</el-tag>
                    </div>
                    <div v-if="s.note" class="duty-note">{{ s.note }}</div>
                  </div>
                </template>
                <template v-else>
                  <div class="duty-empty">{{ authStore.hasPerm('duty.manage') ? '安排值守' : '未排班' }}</div>
                </template>
              </div>
            </template>
          </div>
        </div>
      </div>

      <!-- 图例 -->
      <div class="legend">
        <span class="legend-item"><el-tag size="small" type="danger" effect="plain">县委大院</el-tag> 当天同时有县委大院排班</span>
      </div>
    </el-card>

    <!-- 排班编辑对话框 -->
    <el-drawer v-model="dialogVisible" :title="`安排值守 - ${editDate}`" size="540px">
      <div class="dialog-rule">
        当天值守至 21:00（收文结束），无文件可提前回家。一天最多安排 3 人值守。
      </div>

      <!-- 当天已排班人员 -->
      <div class="sched-section">
        <div class="sched-title">已排班人员（{{ daySchedules.length }}人）</div>
        <el-table :data="daySchedules" size="small" empty-text="当日暂无排班">
          <el-table-column prop="user_name" label="值守人员" width="120" />
          <el-table-column label="县委大院" width="110">
            <template #default="{ row }">
              <el-checkbox :model-value="row.is_dawangyuan === 1" @change="(v) => toggleDaWangYuan(row, v)" />
            </template>
          </el-table-column>
          <el-table-column label="备注" min-width="140">
            <template #default="{ row }">
              <el-input v-model="row.note" size="small" placeholder="点击输入备注" @change="updateNote(row)" />
            </template>
          </el-table-column>
          <el-table-column label="操作" width="70">
            <template #default="{ row }">
              <el-button link type="danger" size="small" @click="removeOne(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <!-- 添加人员 -->
      <div class="add-section">
        <div class="sched-title">添加值守人员</div>
        <div class="add-row">
          <el-select v-model="addUser" filterable placeholder="选择值守人员" style="flex: 1">
            <el-option v-for="a in availableAssignees" :key="a.id"
              :label="a.real_name + (a.department ? ' (' + a.department + ')' : '')" :value="a.id" />
          </el-select>
          <el-button type="primary" :icon="'Plus'" @click="addSchedule" :disabled="daySchedules.length >= 3">添加</el-button>
        </div>
        <div v-if="daySchedules.length >= 3" class="max-tip">一天最多安排 3 人值守</div>
      </div>

      <template #footer>
        <el-button @click="dialogVisible = false">关闭</el-button>
      </template>
    </el-drawer>

    <!-- 一键导出选定周排班图片对话框 -->
    <el-dialog
      v-model="imageDialogVisible"
      title="导出周值守排班表图片"
      width="860px"
      destroy-on-close
      class="duty-image-dialog"
    >
      <div class="image-dialog-header">
        <div class="week-switch-bar">
          <el-button-group>
            <el-button size="small" @click="shiftWeek(-1)">上一周</el-button>
            <el-button size="small" @click="resetToCurrentWeek">本周</el-button>
            <el-button size="small" @click="shiftWeek(1)">下一周</el-button>
          </el-button-group>

          <el-date-picker
            v-model="targetWeekDate"
            type="date"
            value-format="YYYY-MM-DD"
            placeholder="定位基准日期"
            style="width: 140px; margin-left: 10px"
            size="small"
            @change="onTargetDateChange"
          />

          <div class="selected-week-info" v-if="weekInfo">
            <span class="week-range-text font-serif">{{ weekInfo.rangeTitle }}</span>
            <el-tag size="small" type="primary" effect="plain" class="ml-8">{{ weekInfo.weekNumberText }}</el-tag>
          </div>
        </div>

        <div class="style-config-bar">
          <el-radio-group v-model="posterTheme" size="small" @change="renderPoster">
            <el-radio-button value="red">经典政务红</el-radio-button>
            <el-radio-button value="blue">庄重藏青蓝</el-radio-button>
          </el-radio-group>
          <el-checkbox v-model="showNoticeCard" size="small" class="ml-12" @change="renderPoster">包含值守纪律要求</el-checkbox>
        </div>
      </div>

      <!-- 图片预览展示容器 -->
      <div class="poster-preview-container" v-loading="renderingPoster">
        <div v-if="previewImageUrl" class="poster-preview-box">
          <img :src="previewImageUrl" alt="值守排班表" class="poster-preview-img" />
        </div>
        <el-empty v-else description="正在生成高清排班图片..." />
      </div>

      <template #footer>
        <div class="dialog-footer-actions">
          <span class="footer-tip text-secondary">
            提示：图片已渲染为 2000px 视网膜高清分辨率，可一键复制或下载分享至工作群
          </span>
          <div class="footer-btns">
            <el-button @click="imageDialogVisible = false">关闭</el-button>
            <el-button :icon="'CopyDocument'" @click="copyPosterImage" :loading="copying">
              复制图片到剪贴板
            </el-button>
            <el-button type="primary" :icon="'Download'" @click="downloadPosterImage">
              一键下载高清图片
            </el-button>
          </div>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request, { exportFile } from '../utils/request'
import dayjs from 'dayjs'
import { useAuthStore } from '../store/auth'
import PageHeader from '../components/PageHeader.vue'
import { getWeekRangeInfo, generateDutyPosterCanvas } from '../utils/dutyPoster'

const authStore = useAuthStore()
const currentMonth = ref(dayjs().format('YYYY-MM'))
const schedules = ref([])
const assignees = ref([])
const dialogVisible = ref(false)
const editDate = ref('')
const daySchedules = ref([])
const addUser = ref(null)

// 导出周值班图片相关状态
const imageDialogVisible = ref(false)
const targetWeekDate = ref(dayjs().format('YYYY-MM-DD'))
const weekInfo = ref(getWeekRangeInfo())
const posterTheme = ref('red')
const showNoticeCard = ref(true)
const previewImageUrl = ref('')
const renderingPoster = ref(false)
const copying = ref(false)
const currentPosterCanvas = ref(null)

const weekNames = ['一', '二', '三', '四', '五', '六', '日']

// 可添加的人员（未在该日排班的）
const availableAssignees = computed(() => {
  const assignedIds = new Set(daySchedules.value.map(s => s.user_id))
  return assignees.value.filter(a => !assignedIds.has(a.id))
})

const calendarCells = computed(() => {
  const [year, month] = currentMonth.value.split('-').map(Number)
  const firstDay = dayjs(`${year}-${month}-01`)
  // 星期一首：dayjs.day() 返回 0=周日...6=周六，转换为周一=0...周日=6
  const startOffset = (firstDay.day() + 6) % 7
  const daysInMonth = firstDay.daysInMonth()
  const cells = []
  for (let i = 0; i < startOffset; i++) {
    cells.push({ date: null })
  }
  const scheduleMap = {}
  schedules.value.forEach(s => {
    if (!scheduleMap[s.duty_date]) scheduleMap[s.duty_date] = []
    scheduleMap[s.duty_date].push(s)
  })
  for (let d = 1; d <= daysInMonth; d++) {
    const dateStr = `${year}-${String(month).padStart(2, '0')}-${String(d).padStart(2, '0')}`
    cells.push({
      date: dateStr,
      day: d,
      isToday: dayjs().format('YYYY-MM-DD') === dateStr,
      dateStr,
      schedules: scheduleMap[dateStr] || []
    })
  }
  return cells
})

const loadData = async () => {
  try {
    const res = await request.get('/duty-schedules', { params: { month: currentMonth.value } })
    schedules.value = res.list || []
  } catch (e) { console.error(e) }
}

const loadAssignees = async () => {
  try {
    const res = await request.get('/assignees')
    assignees.value = res.list || []
  } catch (e) { console.error(e) }
}

const openEdit = (date) => {
  editDate.value = date
  daySchedules.value = schedules.value.filter(s => s.duty_date === date)
  addUser.value = null
  dialogVisible.value = true
}

const addSchedule = async () => {
  if (!addUser.value) return ElMessage.warning('请选择值守人员')
  if (daySchedules.value.length >= 3) return ElMessage.warning('一天最多安排 3 人值守')
  try {
    await request.post('/duty-schedules', {
      duty_date: editDate.value,
      user_id: addUser.value,
      is_dawangyuan: 0,
      note: ''
    })
    ElMessage.success('已添加')
    addUser.value = null
    await loadData()
    daySchedules.value = schedules.value.filter(s => s.duty_date === editDate.value)
  } catch (e) { console.error(e) }
}

const toggleDaWangYuan = async (row, val) => {
  try {
    await request.post('/duty-schedules', {
      duty_date: row.duty_date,
      user_id: row.user_id,
      is_dawangyuan: val ? 1 : 0,
      note: row.note || ''
    })
    ElMessage.success(val ? '已标记县委大院排班' : '已取消县委大院标记')
    await loadData()
    daySchedules.value = schedules.value.filter(s => s.duty_date === editDate.value)
  } catch (e) { console.error(e) }
}

// 更新排班备注
const updateNote = async (row) => {
  if (!row.id) return
  try {
    await request.post('/duty-schedules', {
      duty_date: row.duty_date,
      user_id: row.user_id,
      is_dawangyuan: row.is_dawangyuan || 0,
      note: row.note || ''
    })
    ElMessage.success('备注已更新')
    await loadData()
    daySchedules.value = schedules.value.filter(s => s.duty_date === editDate.value)
  } catch (e) { console.error(e) }
}

const removeOne = async (row) => {
  try {
    await ElMessageBox.confirm(`确认移除 ${editDate.value} 的「${row.user_name}」排班？`, '删除确认', { type: 'warning' })
  } catch (e) { console.error(e); return }
  try {
    await request.delete(`/duty-schedules/${row.id}`)
    ElMessage.success('已删除')
    await loadData()
    daySchedules.value = schedules.value.filter(s => s.duty_date === editDate.value)
  } catch (e) { console.error(e) }
}

onMounted(() => {
  loadData()
  loadAssignees()
})

// ===================== 周排班海报图片导出逻辑 =====================
const openExportImageDialog = () => {
  targetWeekDate.value = dayjs().format('YYYY-MM-DD')
  weekInfo.value = getWeekRangeInfo(targetWeekDate.value)
  imageDialogVisible.value = true
  renderPoster()
}

const shiftWeek = (delta) => {
  targetWeekDate.value = dayjs(targetWeekDate.value).add(delta * 7, 'day').format('YYYY-MM-DD')
  weekInfo.value = getWeekRangeInfo(targetWeekDate.value)
  renderPoster()
}

const resetToCurrentWeek = () => {
  targetWeekDate.value = dayjs().format('YYYY-MM-DD')
  weekInfo.value = getWeekRangeInfo(targetWeekDate.value)
  renderPoster()
}

const onTargetDateChange = (val) => {
  if (!val) targetWeekDate.value = dayjs().format('YYYY-MM-DD')
  weekInfo.value = getWeekRangeInfo(targetWeekDate.value)
  renderPoster()
}

const renderPoster = async () => {
  renderingPoster.value = true
  try {
    const info = getWeekRangeInfo(targetWeekDate.value)
    weekInfo.value = info

    // 请求选定周对应的排班（支持跨月）
    const res = await request.get('/duty-schedules', {
      params: { start_date: info.mondayDate, end_date: info.sundayDate }
    })
    const list = res.list || []
    const scheduleMap = {}
    list.forEach(s => {
      if (!scheduleMap[s.duty_date]) scheduleMap[s.duty_date] = []
      scheduleMap[s.duty_date].push(s)
    })

    const weekDaysWithData = info.weekDays.map(d => ({
      ...d,
      schedules: scheduleMap[d.dateStr] || []
    }))

    const canvas = generateDutyPosterCanvas(weekDaysWithData, {
      theme: posterTheme.value,
      showNotice: showNoticeCard.value,
      weekRangeText: `${info.weekDays[0].dateStr.replace(/-/g, '.')} ~ ${info.weekDays[6].dateStr.replace(/-/g, '.')}`,
      weekNumberText: info.weekNumberText
    })

    currentPosterCanvas.value = canvas
    previewImageUrl.value = canvas.toDataURL('image/png')
  } catch (e) {
    console.error('渲染排班海报失败', e)
    ElMessage.error('渲染排班图片失败')
  } finally {
    renderingPoster.value = false
  }
}

const copyPosterImage = async () => {
  if (!currentPosterCanvas.value) return
  copying.value = true
  try {
    currentPosterCanvas.value.toBlob(async (blob) => {
      if (!blob) {
        ElMessage.warning('生成图片数据失败')
        copying.value = false
        return
      }
      if (navigator.clipboard && window.ClipboardItem) {
        try {
          await navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })])
          ElMessage.success('图片已成功复制到剪贴板，可直接在微信/钉钉中粘贴（Ctrl+V）发送！')
          copying.value = false
          return
        } catch (err) {
          console.warn('ClipboardItem write failed, trying fallback download', err)
        }
      }
      downloadPosterImage()
      ElMessage.info('当前浏览器限制直接写入图片剪贴板，已自动为您下载高清图片')
      copying.value = false
    }, 'image/png')
  } catch (e) {
    copying.value = false
    ElMessage.error('复制失败，请直接点击“下载图片”')
  }
}

const downloadPosterImage = () => {
  if (!currentPosterCanvas.value) return
  const link = document.createElement('a')
  const fileName = `中共伊宁县委宣传部值守排班表_${weekInfo.value.year}年第${weekInfo.value.weekNumber}周(${weekInfo.value.mondayDate.slice(5).replace('-', '.')}-${weekInfo.value.sundayDate.slice(5).replace('-', '.')}).png`
  link.download = fileName
  link.href = currentPosterCanvas.value.toDataURL('image/png')
  link.click()
  ElMessage.success('已开始下载高清排班表图片')
}
const exportData = () => {
  const params = {}
  if (currentMonth.value) params.month = currentMonth.value
  exportFile('/export/duty-schedules', params)
}
</script>

<style scoped>
.rule-alert {
  margin-bottom: 0;
}
.rule-content {
  line-height: 1.8;
  color: var(--el-text-color-regular);
}
.mt-12 {
  margin-top: 12px;
}
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 16px;
}
.ml-8 { margin-left: 8px; }
.tip { color: var(--el-text-color-secondary); font-size: 13px; }
.duty-calendar {
  border: 1px solid var(--el-border-color-light);
  border-radius: 4px;
}
.week-header {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  background: var(--el-fill-color-light);
  border-bottom: 1px solid var(--el-border-color-light);
}
.week-cell {
  padding: 10px;
  text-align: center;
  font-weight: 600;
  color: var(--el-text-color-regular);
  border-right: 1px solid var(--el-border-color-light);
}
.week-cell:last-child { border-right: none; }
.duty-grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
}
.day-cell {
  min-height: 100px;
  border-right: 1px solid var(--el-border-color-light);
  border-bottom: 1px solid var(--el-border-color-light);
  padding: 6px;
  position: relative;
}
.day-cell:nth-child(7n) { border-right: none; }
.day-cell.empty { background: var(--el-fill-color-lighter); }
.day-cell.today {
  background: var(--yx-brand-bg);
}
.day-num {
  font-size: 14px;
  color: var(--el-text-color-primary);
  margin-bottom: 6px;
  font-weight: 600;
}
.today .day-num {
  color: var(--yx-brand);
}
.duty-box {
  border-radius: 4px;
  padding: 6px;
  min-height: 46px;
  display: flex;
  flex-direction: column;
  justify-content: center;
}
.duty-box.clickable {
  cursor: pointer;
}
.duty-box.assigned {
  background: var(--el-color-primary-light-9);
  border: 1px solid var(--el-color-primary-light-5);
}
.duty-box.empty-shift {
  background: var(--el-fill-color-light);
  border: 1px dashed var(--el-border-color);
  align-items: center;
  justify-content: center;
}
.duty-name {
  font-weight: 600;
  color: var(--el-text-color-primary);
  font-size: 13px;
}
.duty-person {
  display: flex;
  flex-direction: column;
  padding: 2px 0;
  margin-bottom: 4px;
}
.duty-person-line {
  display: flex;
  align-items: center;
  gap: 6px;
}
.duty-note {
  font-size: 11px;
  color: var(--el-text-color-secondary);
  line-height: 1.4;
  margin-top: 2px;
  word-break: break-all;
}
.duty-empty {
  color: var(--el-text-color-placeholder);
  font-size: 12px;
}
.legend {
  margin-top: 12px;
  display: flex;
  gap: 24px;
  font-size: 13px;
  color: var(--el-text-color-regular);
}
.legend-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.dialog-rule {
  background: var(--el-color-warning-light-9);
  border: 1px solid var(--el-color-warning-light-8);
  color: var(--el-color-warning-dark-2);
  border-radius: 4px;
  padding: 8px 12px;
  font-size: 13px;
  margin-bottom: 16px;
}
.sched-section {
  margin-bottom: 16px;
}
.sched-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  margin-bottom: 8px;
}
.add-section {
  border-top: 1px solid var(--el-border-color-lighter);
  padding-top: 12px;
}
.add-row {
  display: flex;
  gap: 8px;
  align-items: center;
}
.max-tip {
  margin-top: 6px;
  color: var(--el-color-warning);
  font-size: 12px;
}

@media (max-width: 768px) {
  .duty-calendar {
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
  }
  .week-header,
  .duty-grid {
    min-width: 580px;
  }
}

/* 海报导出弹窗样式 */
.image-dialog-header {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.week-switch-bar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}
.selected-week-info {
  display: inline-flex;
  align-items: center;
  margin-left: 12px;
}
.week-range-text {
  font-size: 15px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}
.style-config-bar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}
.ml-12 {
  margin-left: 12px;
}
.poster-preview-container {
  min-height: 280px;
  max-height: 520px;
  overflow-y: auto;
  padding: 16px;
  background: var(--el-fill-color-light);
  border-radius: 6px;
  margin-top: 14px;
  display: flex;
  justify-content: center;
  align-items: flex-start;
}
.poster-preview-box {
  width: 100%;
  max-width: 720px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
  border-radius: 8px;
  overflow: hidden;
  background: #ffffff;
}
.poster-preview-img {
  display: block;
  width: 100%;
  height: auto;
}
.dialog-footer-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
  flex-wrap: wrap;
  gap: 12px;
}
.footer-tip {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.footer-btns {
  display: flex;
  gap: 8px;
}
@media (max-width: 640px) {
  .dialog-footer-actions {
    flex-direction: column;
    align-items: stretch;
  }
  .footer-btns {
    justify-content: flex-end;
  }
}
</style>
