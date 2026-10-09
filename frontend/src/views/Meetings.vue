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
        <el-table-column prop="location" label="召开地点" width="140" show-overflow-tooltip />

        <el-table-column label="报名进度" width="190">
          <template #default="{ row }">
            <div class="progress-col">
              <div class="progress-label">
                <span>{{ calcConfirmedUnits(row) }} / {{ getUnitsCount(row.units) }} 单位</span>
                <span class="progress-pct">{{ calcUnitPct(row) }}%</span>
              </div>
              <el-progress 
                :percentage="calcUnitPct(row)" 
                :stroke-width="6" 
                :show-text="false"
                :color="isMeetingExpired(row.meeting_date, row.meeting_time) ? '#909399' : '#b22222'"
              />
              <div class="progress-sub-text">
                共 {{ row.reg_count || 0 }} 人参会
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="请假状态" width="125" align="center">
          <template #default="{ row }">
            <span class="stat-tag tag-opinion" v-if="row.not_attend > 0">
              请假 {{ row.not_attend }} 单位
            </span>
            <span class="stat-tag tag-none" v-else>全员参会</span>
          </template>
        </el-table-column>

        <el-table-column label="操作" width="260" fixed="right" align="right">
          <template #default="{ row }">
            <div class="row-action-wrap">
              <el-button link type="primary" @click="openDetail(row)">报名台账</el-button>
              <el-button link type="primary" @click="notifyMeeting(row)">一键通知会议</el-button>
              
              <el-dropdown @command="(cmd) => handleRowCommand(cmd, row)" trigger="click">
                <el-button link type="primary" class="more-link">
                  <span>更多</span>
                  <el-icon class="el-icon--right"><ArrowDown /></el-icon>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="copy_url" icon="Link">复制纯链接</el-dropdown-item>
                    <el-dropdown-item command="export" icon="Download">导出签到册 (Excel)</el-dropdown-item>
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
          <div class="preset-btn-bar">
            <span class="preset-label">快捷填充：</span>
            <el-button size="small" round @click="appendPreset('towns')">全县 16 个乡镇</el-button>
            <el-button size="small" round @click="appendPreset('county_orgs')">所有县直机关</el-button>
          </div>
          <el-form-item label="参会单位范围" required>
            <el-input 
              v-model="form.units" 
              type="textarea" 
              :rows="5"
              placeholder="每行填写一个单位" 
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

    <!-- 2. 报名台账详情：右侧滑出抽屉（Drawer） -->
    <el-drawer 
      v-model="detailVisible" 
      :title="detailTitle" 
      :size="isMobile ? '100%' : '820px'"
      destroy-on-close
    >
      <!-- 顶部数据看板卡片 -->
      <div v-if="detailMeeting" class="detail-header-card">
        <div class="detail-stats-bar">
          <div class="d-stat-box">
            <div class="stat-number">{{ attendRegs.length }}</div>
            <div class="stat-title">参会人员 (席位)</div>
          </div>
          <div class="d-stat-box">
            <div class="stat-number text-success">{{ confirmedUnitsCount }}</div>
            <div class="stat-title">已确认单位</div>
          </div>
          <div class="d-stat-box">
            <div class="stat-number text-warning">{{ notAttendRegs.length }}</div>
            <div class="stat-title">请假不参加</div>
          </div>
          <div class="d-stat-box">
            <div class="stat-number text-danger">{{ unconfirmedUnits.length }}</div>
            <div class="stat-title">待催报单位</div>
          </div>
        </div>

        <!-- 综合报名进度条 -->
        <div class="drawer-progress-box">
          <div class="progress-label">
            <span>单位报名进度：{{ confirmedUnitsCount }} / {{ totalUnitsCount }} 单位</span>
            <span class="progress-pct">{{ drawerProgressPct }}%</span>
          </div>
          <el-progress 
            :percentage="drawerProgressPct" 
            :stroke-width="7" 
            :show-text="false"
            :color="isMeetingExpired(detailMeeting.meeting_date, detailMeeting.meeting_time) ? '#909399' : '#b22222'"
          />
        </div>

        <div class="detail-quick-meta">
          <span><strong>会议时间：</strong>{{ detailMeeting.meeting_date }} {{ detailMeeting.meeting_time || '' }}</span>
          <span><strong>会议地点：</strong>{{ detailMeeting.location }}</span>
          <span><strong>单位限额：</strong>{{ detailMeeting.unit_limit ? `每单位 ${detailMeeting.unit_limit} 人` : '不限人数' }}</span>
        </div>
      </div>

      <el-tabs v-model="detailTab" class="mt-12">
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

        <el-tab-pane :label="`待催报单位 (${unconfirmedUnits.length})`" name="unconfirmed">
          <div class="reminder-header-bar">
            <el-alert 
              v-if="unconfirmedUnits.length" 
              type="warning" 
              :closable="false" 
              class="mb-12"
              :title="`尚有 ${unconfirmedUnits.length} 个单位未进行网上报名确认。可点击右侧快捷按钮复制催报名单。`" 
            />
            <el-button 
              type="warning" 
              :icon="CopyDocument" 
              @click="copyUrgeList"
              :disabled="!unconfirmedUnits.length"
            >
              一键复制微信催报名单
            </el-button>
          </div>

          <el-table :data="unconfirmedUnits.map(u => ({ unit: u }))" size="large">
            <template #empty><empty-state description="所有参会单位均已确认完成！" /></template>
            <el-table-column prop="unit" label="待催报单位" min-width="220" />
            <el-table-column label="状态" width="110" align="center">
              <template #default>
                <el-tag type="info" size="small">待报名</el-tag>
              </template>
            </el-table-column>
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
          <el-button type="success" :icon="'Download'" @click="exportReg(detailMeeting)">导出会议签到册 (Excel)</el-button>
          <el-button @click="detailVisible = false">关闭</el-button>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { CopyDocument } from '@element-plus/icons-vue'
import request, { exportFile } from '../utils/request'
import dayjs from 'dayjs'
import PageHeader from '../components/PageHeader.vue'
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

// 预设单位名单
const PRESETS = {
  towns: [
    '吉里于孜镇', '喀什镇', '维吾尔玉其温镇', '曲鲁海乡', '墩麻扎镇',
    '萨木于孜镇', '萨地克于孜乡', '喀拉亚尕奇乡', '英塔木镇', '吐鲁番于孜乡',
    '麻扎乡', '温亚尔镇', '阿乌利亚乡', '武功乡', '巴依托海镇', '愉群翁回族乡'
  ],
  county_orgs: [
    '县委办', '人大办', '政府办', '政协办', '纪委监委', '组织部',
    '社会工作部', '宣传部', '统战部', '政法委', '巡察办', '公安局',
    '法院', '检察院', '司法局', '网信办', '应急管理局', '信访局',
    '退役军人事务局', '财政局', '交通局', '党研室', '编  办', '党  校',
    '机关工委', '民政局', '人社局', '工商联', '团  委', '妇  联',
    '总工会', '红十字会', '老干局', '残  联', '科  协', '发改委',
    '商工信局', '住建局', '审计局', '统计局', '供销社', '环保局',
    '税务局', '自然资源局', '伊东工业园区', '市场监督管理局', '农业农村局', '林草局',
    '水利局', '教育局', '卫健委', '融媒体中心', '文旅局', '医保局',
    '科技局', '档案馆'
  ]
}

const appendPreset = (key) => {
  const arr = PRESETS[key] || []
  const current = form.value.units ? form.value.units.split('\n').map(s => s.trim()).filter(Boolean) : []
  const set = new Set([...current, ...arr])
  form.value.units = Array.from(set).join('\n')
  ElMessage.success(`已追加填充 ${arr.length} 个单位`)
}

// 抽屉精确数据指标统计
const attendUnitsCount = computed(() => {
  return new Set(attendRegs.value.map(r => r.unit)).size
})

const confirmedUnitsCount = computed(() => {
  return attendUnitsCount.value + notAttendRegs.value.length
})

const totalUnitsCount = computed(() => {
  return confirmedUnitsCount.value + unconfirmedUnits.value.length
})

const drawerProgressPct = computed(() => {
  if (totalUnitsCount.value === 0) return 0
  return Math.min(100, Math.round((confirmedUnitsCount.value / totalUnitsCount.value) * 100))
})

// 列表页计算辅助
const getUnitsCount = (unitsStr) => {
  if (!unitsStr) return 0
  return unitsStr.split('\n').map(s => s.trim()).filter(Boolean).length
}

const isMeetingExpired = (date, time) => {
  if (!date) return false
  const full = time ? `${date} ${time}` : `${date} 23:59`
  return dayjs().isAfter(dayjs(full))
}

const calcConfirmedUnits = (row) => {
  const total = getUnitsCount(row.units)
  if (total === 0) return 0
  const unitLimit = row.unit_limit > 0 ? row.unit_limit : 1
  const attendUnitsEst = Math.min(total, Math.ceil((row.reg_count || 0) / unitLimit))
  const notAttendUnits = row.not_attend || 0
  return Math.min(total, attendUnitsEst + notAttendUnits)
}

const calcUnitPct = (row) => {
  const total = getUnitsCount(row.units)
  if (total === 0) return 0
  const confirmed = calcConfirmedUnits(row)
  const pct = Math.round((confirmed / total) * 100)
  return Math.min(100, Math.max(0, pct))
}

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

const handlePageChange = (val) => {
  page.value = val
  loadData()
}

const handleSizeChange = (val) => {
  pageSize.value = val
  page.value = 1
  loadData()
}

const openCreate = () => {
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
  if (cmd === 'copy_url') copyLink(row)
  else if (cmd === 'export') exportReg(row)
  else if (cmd === 'edit') openEdit(row)
  else if (cmd === 'delete') removeMeeting(row)
}

const notifyMeeting = (row) => {
  const url = `${window.location.origin}/meeting/${row.id}`
  const timeDesc = row.meeting_time ? `${row.meeting_date} ${row.meeting_time}` : row.meeting_date
  const unitsArr = (row.units || '').split('\n').map(s => s.trim()).filter(Boolean)
  const unitsText = unitsArr.join('、')
  const limitText = row.unit_limit ? `（每单位限 ${row.unit_limit} 人）` : ''

  let text = `【会议通知】\n`
  text += `各有关单位：\n`
  text += `定于 ${timeDesc} 在【${row.location || '指定会议室'}】召开《${row.title}》。\n\n`
  text += `【参会范围】${unitsText || '详见通知'}${limitText}\n`
  if (row.content) {
    text += `【会议要求】${row.content}\n`
  }
  text += `【在线报名】请上述参会单位负责同志点击下方链接，及时填报参会人员信息：\n`
  text += `${url}`

  navigator.clipboard.writeText(text).then(() => {
    ElMessage.success('已复制会议通知文案，可直接发送微信工作群')
  }).catch(() => {
    ElMessage.info(`通知文案已生成，请手动复制：\n${text}`)
  })
}

const copyLink = (row) => {
  const url = `${window.location.origin}/meeting/${row.id}`
  navigator.clipboard.writeText(url).then(() => {
    ElMessage.success('公开参会报名纯链接已复制到剪贴板')
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

const copyUrgeList = () => {
  if (!unconfirmedUnits.value.length || !detailMeeting.value) return
  const nowStr = dayjs().format('MM月DD日 HH:mm')
  const m = detailMeeting.value
  const timeDesc = m.meeting_time ? `${m.meeting_date} ${m.meeting_time}` : m.meeting_date
  
  let text = `【会务催报提醒 · ${nowStr}】\n`
  text += `关于《${m.title}》参会报名工作，会议将于 ${timeDesc} 在 ${m.location || '指定会议室'} 召开。\n`
  text += `截至目前，以下 ${unconfirmedUnits.value.length} 个单位尚未完成参会人员网上报名：\n\n`
  unconfirmedUnits.value.forEach((u, i) => {
    text += `${i + 1}. ${u}\n`
  })
  text += `\n请上述单位主要负责同志高度重视，尽快点击以下专属链接填报参会人员：\n`
  text += `${window.location.origin}/meeting/${m.id}`

  navigator.clipboard.writeText(text)
    .then(() => ElMessage.success('已复制微信催报名单至剪贴板，可直接发送工作群'))
    .catch(() => ElMessage.error('复制失败'))
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
  gap: 8px;
  white-space: nowrap;
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
.mt-12 {
  margin-top: 12px;
}

/* 进度条与状态标签 */
.progress-col {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.progress-label {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: #606266;
}

.progress-pct {
  font-weight: 600;
  color: #303133;
}

.progress-sub-text {
  font-size: 11px;
  color: #909399;
}

.stat-tag {
  font-size: 12px;
  padding: 2px 8px;
  border-radius: 12px;
}

.tag-opinion {
  background: #fff3e0;
  color: #e65100;
  font-weight: 500;
}

.tag-none {
  background: #f4f5f7;
  color: #909399;
}

/* 抽屉顶部数据看板卡片 */
.detail-header-card {
  background: #f9fafb;
  border: 1px solid #eaedf1;
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 16px;
}

.detail-stats-bar {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  text-align: center;
  margin-bottom: 12px;
}

.d-stat-box {
  background: #fff;
  border: 1px solid #edf0f5;
  border-radius: 6px;
  padding: 10px;
}

.stat-number {
  font-size: 22px;
  font-weight: 700;
  color: #24292e;
}

.stat-title {
  font-size: 12px;
  color: #888;
  margin-top: 2px;
}

.text-warning {
  color: #e6a23c !important;
}

.text-success {
  color: #67c23a !important;
}

.text-danger {
  color: #f56c6c !important;
}

.drawer-progress-box {
  background: #fff;
  border: 1px solid #edf0f5;
  border-radius: 6px;
  padding: 10px 14px;
  margin-bottom: 12px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.detail-quick-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 20px;
  font-size: 13px;
  color: #606266;
  border-top: 1px dashed #e4e7ed;
  padding-top: 10px;
}

.reminder-header-bar {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 12px;
}

.preset-btn-bar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
  background: #fff;
  padding: 8px 12px;
  border-radius: 6px;
  border: 1px dashed #cbd5e1;
}

.preset-label {
  font-size: 12px;
  color: #64748b;
}
</style>
