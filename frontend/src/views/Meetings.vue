<template>
  <div class="meetings-page">
    <!-- 页头 -->
    <page-header 
      title="会务管理" 
      subtitle="发布宣传思想文化重要会议，生成公开参会报名链接与二维码，实时统计报名与出勤"
      :tag="list.length ? `共 ${list.length} 场会议` : ''"
    >
      <template #actions>
        <el-button type="primary" :icon="'Plus'" @click="openCreate">录入新会议</el-button>
        <el-button :icon="'Refresh'" circle @click="loadData" />
      </template>
    </page-header>

    <el-card shadow="never" class="gov-card">
      <el-table :data="list" v-loading="loading" size="large">
        <template #empty>
          <empty-state description="暂无召开或安排的会议" />
        </template>

        <el-table-column prop="title" label="会议标题" min-width="220" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="meeting-title font-serif" @click="openDetail(row)">{{ row.title }}</span>
          </template>
        </el-table-column>

        <el-table-column prop="meeting_date" label="会议日期" width="115" />
        <el-table-column prop="meeting_time" label="具体时间" width="95" />
        <el-table-column prop="location" label="召开地点" width="150" show-overflow-tooltip />

        <el-table-column label="已报名" width="100" align="center">
          <template #default="{ row }">
            <status-dot type="success" :text="`${row.reg_count} 人`" />
          </template>
        </el-table-column>

        <el-table-column label="不参加" width="100" align="center">
          <template #default="{ row }">
            <span v-if="row.not_attend > 0" style="color:var(--el-color-warning);font-size:13px;font-weight:500;">
              {{ row.not_attend }} 单位
            </span>
            <span v-else style="color:var(--yx-text-4);font-size:12px;">无</span>
          </template>
        </el-table-column>

        <el-table-column label="操作" width="180" fixed="right" align="right">
          <template #default="{ row }">
            <div class="row-action-wrap">
              <el-button link type="primary" @click="openDetail(row)">报名情况</el-button>
              
              <el-dropdown @command="(cmd) => handleRowCommand(cmd, row)" trigger="click">
                <el-button link type="primary" class="more-link">
                  <span>更多</span>
                  <el-icon class="el-icon--right"><ArrowDown /></el-icon>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="copy" icon="Link">复制报名链接</el-dropdown-item>
                    <el-dropdown-item command="export" icon="Download">导出签到册</el-dropdown-item>
                    <el-dropdown-item command="edit" icon="Edit" divided>编辑会议</el-dropdown-item>
                    <el-dropdown-item command="delete" icon="Delete" class="text-danger-item">删除会议</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </template>
        </el-table-column>
      </el-table>
      <div class="pagination-wrap" v-if="total > 0" style="margin-top: 16px; display: flex; justify-content: flex-end;">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :page-sizes="[10, 20, 50, 100]"
          :layout="isMobile ? 'total, prev, pager, next' : 'total, sizes, prev, pager, next, jumper'"
          :total="total"
          @size-change="handleSizeChange"
          @current-change="handlePageChange"
        />
      </div>
    </el-card>

    <!-- 1. 录入/编辑会议：右侧滑出抽屉（Drawer） -->
    <el-drawer 
      v-model="dialogVisible" 
      :title="editId ? '编辑会议信息' : '创建会务通知'" 
      size="620px"
      destroy-on-close
    >
      <el-form :model="form" label-width="105px">
        <div class="form-section">
          <div class="form-section-title">会议基本信息</div>
          <el-form-item label="会议标题" required>
            <el-input v-model="form.title" placeholder="如：全县宣传思想文化工作推进会" />
          </el-form-item>

          <el-row :gutter="16">
            <el-col :xs="24" :sm="12">
              <el-form-item label="会议日期" required>
                <el-date-picker v-model="form.meeting_date" type="date" value-format="YYYY-MM-DD" style="width:100%" />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12">
              <el-form-item label="开会时间">
                <el-time-picker v-model="form.meeting_time" value-format="HH:mm" format="HH:mm" placeholder="如 10:30" style="width:100%" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-form-item label="会议地点" required>
            <el-input v-model="form.location" placeholder="如：县委二楼一号会议室" />
          </el-form-item>
        </div>

        <div class="form-section">
          <div class="form-section-title">参会单位与限额</div>
          <el-form-item label="参会单位范围" required>
            <el-input 
              v-model="form.units" 
              type="textarea" 
              :rows="5"
              placeholder="每行填写一个单位名称，如：&#10;各乡镇（片区）党委宣传委员&#10;县委宣传部各科室负责同志&#10;县融媒体中心负责同志" 
            />
            <div class="form-sub-tip">每行独立一个单位，公开报名端将按此列表供填报人选择。</div>
          </el-form-item>

          <el-form-item label="单位参会限额">
            <el-radio-group v-model="form.unit_limit" size="small">
              <el-radio-button :value="1">每单位 1 人</el-radio-button>
              <el-radio-button :value="2">每单位 2 人</el-radio-button>
              <el-radio-button :value="3">每单位 3 人</el-radio-button>
              <el-radio-button :value="0">不限人数</el-radio-button>
            </el-radio-group>
          </el-form-item>
        </div>

        <div class="form-section">
          <div class="form-section-title">议程与参会要求</div>
          <el-form-item label="会议内容要求">
            <el-input v-model="form.content" type="textarea" :rows="3" placeholder="会议议程、着装要求、席卡安排等提示（选填）" />
          </el-form-item>
        </div>
      </el-form>

      <template #footer>
        <div class="drawer-footer-wrap">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" @click="save">保存会务通知</el-button>
        </div>
      </template>
    </el-drawer>

    <!-- 2. 报名情况：右侧滑出抽屉（Drawer） -->
    <el-drawer 
      v-model="detailVisible" 
      :title="detailTitle" 
      size="760px"
      destroy-on-close
    >
      <el-tabs v-model="detailTab">
        <el-tab-pane :label="`参会人员名单 (${attendRegs.length}人)`" name="attend">
          <el-table :data="attendRegs" size="large" class="mt-8">
            <template #empty><empty-state description="暂无参会报名" /></template>
            <el-table-column prop="unit" label="参会单位" min-width="160" show-overflow-tooltip />
            <el-table-column prop="attendee_name" label="姓名" width="90" />
            <el-table-column prop="attendee_title" label="职务" width="110" />
            <el-table-column prop="phone" label="联系电话" width="120" />
            <el-table-column label="协调管理" width="170" align="center" fixed="right">
              <template #default="{ row }">
                <el-button link type="danger" size="small" @click="handleAdminDeleteSeat(row)">移除席位</el-button>
                <el-divider direction="vertical" />
                <el-button link type="warning" size="small" @click="handleAdminChangeAbsent(row.unit)">整单位转请假</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane :label="`请假不参加 (${notAttendRegs.length}个单位)`" name="absent">
          <el-table :data="notAttendRegs" size="large" class="mt-8">
            <template #empty><empty-state description="全部参会，无请假单位" /></template>
            <el-table-column prop="unit" label="单位名称" width="180" show-overflow-tooltip />
            <el-table-column prop="reason" label="不参加事由" min-width="200" show-overflow-tooltip />
            <el-table-column label="操作" width="120" align="center" fixed="right">
              <template #default="{ row }">
                <el-button link type="primary" size="small" @click="handleAdminResetUnit(row.unit)">恢复待确认</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane :label="`尚未确认 (${unconfirmedUnits.length}个单位)`" name="unconfirmed">
          <el-alert 
            v-if="unconfirmedUnits.length" 
            type="warning" 
            :closable="false" 
            class="mb-12"
            :title="`尚有 ${unconfirmedUnits.length} 个单位未进行网上报名确认。导出签到册时将自动留空供现场手填签到。`" 
          />
          <el-table :data="unconfirmedUnits.map(u => ({ unit: u }))" size="large">
            <template #empty><empty-state description="所有参会单位均已确认完成！" /></template>
            <el-table-column prop="unit" label="待确认单位" min-width="220" />
            <el-table-column label="操作" width="140" align="center" fixed="right">
              <template #default="{ row }">
                <el-button link type="warning" size="small" @click="handleAdminChangeAbsent(row.unit)">代登记请假</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>
      </el-tabs>

      <template #footer>
        <div class="drawer-footer-wrap">
          <el-button type="success" :icon="'Download'" @click="exportReg(detailMeeting)">导出会议签到册</el-button>
          <el-button @click="detailVisible = false">关闭</el-button>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import request, { exportFile } from '../utils/request'
import dayjs from 'dayjs'
import PageHeader from '../components/PageHeader.vue'
import StatusDot from '../components/StatusDot.vue'
import EmptyState from '../components/EmptyState.vue'
import { useMobile } from '../utils/useMobile'

const { isMobile } = useMobile()
const route = useRoute()

const list = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(10)
const loading = ref(false)
const dialogVisible = ref(false)
const editId = ref(0)
const form = ref({ title: '', meeting_date: dayjs().format('YYYY-MM-DD'), meeting_time: '', location: '', content: '', units: '', unit_limit: 1 })
const detailVisible = ref(false)
const detailTitle = ref('')
const detailTab = ref('attend')
const attendRegs = ref([])
const notAttendRegs = ref([])
const unconfirmedUnits = ref([])
const detailMeeting = ref(null)

const loadData = async () => {
  loading.value = true
  try {
    const res = await request.get('/meetings', { params: { page: page.value, page_size: pageSize.value } })
    list.value = res.list || []
    total.value = res.total || 0
  } catch (e) { console.error(e) } finally {
    loading.value = false
  }
}

const openCreate = () => {
const handlePageChange = (val) => {
  page.value = val
  loadData()
}

const handleSizeChange = (val) => {
  pageSize.value = val
  page.value = 1
  loadData()
}
  editId.value = 0
  form.value = { title: '', meeting_date: dayjs().format('YYYY-MM-DD'), meeting_time: '', location: '', content: '', units: '', unit_limit: 1 }
  dialogVisible.value = true
}

const openEdit = (row) => {
  editId.value = row.id
  form.value = {
    title: row.title,
    meeting_date: row.meeting_date,
    meeting_time: row.meeting_time || '',
    location: row.location || '',
    content: row.content || '',
    units: row.units || '',
    unit_limit: row.unit_limit ?? 1
  }
  dialogVisible.value = true
}

const save = async () => {
  if (!form.value.title) return ElMessage.warning('请输入会议标题')
  if (!form.value.meeting_date) return ElMessage.warning('请选择会议日期')
  if (!form.value.units) return ElMessage.warning('请输入参会单位')
  try {
    if (editId.value) {
      await request.put('/meetings', { ...form.value, id: editId.value })
      ElMessage.success('更新成功')
    } else {
      await request.post('/meetings', form.value)
      ElMessage.success('录入成功')
    }
    dialogVisible.value = false
    loadData()
  } catch (e) { console.error(e) }
}

const handleRowCommand = (cmd, row) => {
  if (cmd === 'copy') copyLink(row)
  else if (cmd === 'export') exportReg(row)
  else if (cmd === 'edit') openEdit(row)
  else if (cmd === 'delete') removeMeeting(row)
}

const copyLink = (row) => {
  const url = `${window.location.origin}/meeting/${row.id}`
  navigator.clipboard.writeText(url).then(() => {
    ElMessage.success('公开参会报名链接已复制到剪贴板')
  }).catch(() => {
    ElMessage.info(`报名链接：${url}`)
  })
}

const openDetail = async (row) => {
  try {
    detailMeeting.value = row
    detailTitle.value = `会务报名统计 - ${row.title}`
    const res = await request.get(`/meetings/${row.id}`)
    const regs = res.registrations || res.list || []
    attendRegs.value = regs.filter(r => Number(r.not_attend) === 0)
    notAttendRegs.value = regs.filter(r => Number(r.not_attend) === 1)
    unconfirmedUnits.value = res.unconfirmed_units || res.unconfirmed || []
    detailTab.value = 'attend'
    detailVisible.value = true
  } catch (e) { console.error(e) }
}

const reloadDetail = async () => {
  if (!detailMeeting.value) return
  const res = await request.get(`/meetings/${detailMeeting.value.id}`)
  const regs = res.registrations || res.list || []
  attendRegs.value = regs.filter(r => Number(r.not_attend) === 0)
  notAttendRegs.value = regs.filter(r => Number(r.not_attend) === 1)
  unconfirmedUnits.value = res.unconfirmed_units || res.unconfirmed || []
  loadData()
}

const handleAdminDeleteSeat = async (row) => {
  try {
    await ElMessageBox.confirm(`确定从参会名单中移除【${row.unit}】的参会人【${row.attendee_name}】席位吗？`, '移除席位确认', {
      type: 'warning',
      confirmButtonText: '确定移除',
      cancelButtonText: '取消'
    })
    await request.post(`/meetings/${detailMeeting.value.id}/registrations/delete`, { reg_id: row.id })
    ElMessage.success('已移除该参会席位')
    reloadDetail()
  } catch (e) { console.error(e) }
}

const handleAdminChangeAbsent = async (unit) => {
  try {
    const { value: reason } = await ElMessageBox.prompt(
      `请输入【${unit}】请假不参加本次会议的具体事由或依据：`,
      '协调设置为请假不参加',
      {
        confirmButtonText: '确认变更',
        cancelButtonText: '取消',
        inputPlaceholder: '例如：主要领导因公出差，已向县委分管领导请假',
        inputValidator: (val) => !val || !val.trim() ? '请填写请假事由' : true
      }
    )
    await request.post(`/meetings/${detailMeeting.value.id}/registrations/change-absent`, {
      unit,
      reason: reason.trim()
    })
    ElMessage.success(`已成功将【${unit}】协调变更为请假不参加`)
    reloadDetail()
  } catch (e) { console.error(e) }
}

const handleAdminResetUnit = async (unit) => {
  try {
    await ElMessageBox.confirm(`确定重置【${unit}】的报名状态吗？重置后该单位将恢复为「待确认」，允许单位重新进入报名系统填报。`, '状态重置确认', {
      type: 'warning',
      confirmButtonText: '确定恢复',
      cancelButtonText: '取消'
    })
    await request.post(`/meetings/${detailMeeting.value.id}/registrations/reset-unit`, { unit })
    ElMessage.success(`已重置【${unit}】报名状态，恢复为待确认`)
    reloadDetail()
  } catch (e) { console.error(e) }
}

const exportReg = async (row) => {
  if (!row) return
  try {
    await exportFile(`/export/meeting-registrations?meeting_id=${row.id}`, {}, `${row.title}-签到册.xlsx`)
  } catch (e) { console.error(e) }
}

const removeMeeting = async (row) => {
  try {
    await ElMessageBox.confirm(`确认删除会议「${row.title}」及其全部报名记录？此操作不可恢复。`, '高危操作确认', {
      type: 'warning',
      confirmButtonText: '确认删除',
      confirmButtonClass: 'el-button--danger'
    })
    await request.delete(`/meetings/${row.id}`)
    ElMessage.success('已删除')
    loadData()
  } catch (e) { console.error(e) }
}

onMounted(() => {
  loadData()
  if (route.query.id) {
    openDetail({ id: route.query.id, title: '' })
  }
})

watch(() => route.query.id, (newId) => {
  if (newId) {
    openDetail({ id: newId, title: '' })
  }
})
</script>

<style scoped>
.meeting-title {
  color: var(--yx-text-1);
  font-weight: 600;
  cursor: pointer;
  transition: color 0.2s ease;
}

.meeting-title:hover {
  color: var(--yx-ink-2);
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

.form-sub-tip {
  font-size: 12px;
  color: var(--yx-text-3);
  margin-top: 4px;
}

.drawer-footer-wrap {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.mb-12 {
  margin-bottom: 12px;
}
.mt-8 {
  margin-top: 8px;
}
</style>
