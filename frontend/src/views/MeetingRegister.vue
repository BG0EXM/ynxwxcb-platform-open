<template>
  <div class="gov-reg-wrapper">
    <!-- 顶部深色政务天际线与天山微纹 -->
    <div class="skyline-bg">
      <div class="skyline-glow"></div>
      <!-- 天山金色山脉矢量剪影 -->
      <svg class="tianshan-peaks" viewBox="0 0 1440 220" fill="none" preserveAspectRatio="none">
        <path d="M0,220 L0,110 L120,60 L240,130 L380,40 L520,120 L660,20 L800,105 L960,35 L1120,115 L1280,55 L1440,95 L1440,220 Z" 
              fill="url(#peak-gold-gradient)" opacity="0.14" />
        <path d="M0,220 L0,140 L160,85 L320,155 L460,70 L620,145 L760,50 L900,130 L1080,65 L1240,140 L1440,110 L1440,220 Z" 
              fill="url(#peak-gold-gradient-2)" opacity="0.22" />
        <defs>
          <linearGradient id="peak-gold-gradient" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stop-color="#f5d499" />
            <stop offset="100%" stop-color="#9e1b1e" stop-opacity="0" />
          </linearGradient>
          <linearGradient id="peak-gold-gradient-2" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stop-color="#ffffff" />
            <stop offset="100%" stop-color="#24406e" stop-opacity="0" />
          </linearGradient>
        </defs>
      </svg>
      
      <!-- 官方组织标头 -->
      <div class="org-brand-bar">
        <img src="../assets/danghui.png" alt="党徽" class="brand-crest" />
        <div class="brand-titles">
          <div class="brand-sup">中共伊宁县委宣传部 · 数字化会务服务平台</div>
          <div class="brand-sub">官方会务通知与参会回执系统</div>
        </div>
      </div>
    </div>

    <!-- 主体容器 -->
    <div class="main-container">
      <template v-if="meeting">
        <!-- 核心会议通告卡片 -->
        <div class="dossier-card">
          <!-- 顶部红金装饰带 -->
          <div class="card-gold-crest-bar"></div>

          <!-- 会议状态与倒计时标签栏 -->
          <div class="status-ribbon-row">
            <div class="status-pill" :class="expired ? 'is-expired' : 'is-active'">
              <span class="pulse-dot" v-if="!expired"></span>
              <span>{{ expired ? '报名已截止' : '在线填报中 · 实时确认' }}</span>
            </div>
            <div class="rule-pill" v-if="unitLimitRuleText">
              <el-icon><User /></el-icon>
              <span>{{ unitLimitRuleText }}</span>
            </div>
            <div class="countdown-pill" v-if="!expired && countdownText">
              <el-icon><Clock /></el-icon>
              <span>{{ countdownText }}</span>
            </div>
          </div>

          <!-- 会议主标题 -->
          <h1 class="dossier-title font-serif">{{ meeting.title }}</h1>

          <!-- 会务三联信息看板（时间、地点、召集单位） -->
          <div class="meeting-dossier-grid">
            <div class="dossier-item">
              <div class="dossier-icon time-icon">
                <el-icon><Calendar /></el-icon>
              </div>
              <div class="dossier-text">
                <span class="dossier-label">会议时间</span>
                <span class="dossier-val font-serif">
                  {{ meetingDateTimeText || '时间待定' }}
                  <small v-if="meetingWeekDay" class="weekday-badge">{{ meetingWeekDay }}</small>
                </span>
              </div>
            </div>

            <div class="dossier-item">
              <div class="dossier-icon org-icon">
                <el-icon><OfficeBuilding /></el-icon>
              </div>
              <div class="dossier-text">
                <span class="dossier-label">组织单位</span>
                <span class="dossier-val font-serif">伊宁县委宣传部</span>
              </div>
            </div>

            <div class="dossier-item item-full-width">
              <div class="dossier-icon place-icon">
                <el-icon><Location /></el-icon>
              </div>
              <div class="dossier-text">
                <span class="dossier-label">会议地点</span>
                <span class="dossier-val font-serif">{{ meeting.location || '以会务通知为准' }}</span>
              </div>
            </div>
          </div>

          <!-- 会议要求与议程公文卡 -->
          <div class="notice-callout" v-if="meeting.content">
            <div class="callout-header">
              <span class="callout-flag">会务议程与参会要求</span>
            </div>
            <div class="callout-body">{{ meeting.content }}</div>
          </div>

          <!-- 已截止严正提示 -->
          <div v-if="expired" class="expired-banner">
            <div class="banner-icon-col">
              <el-icon :size="24"><Warning /></el-icon>
            </div>
            <div class="banner-text-col">
              <h4>会议已开始或截止填报</h4>
              <p>如需紧急调整参会领导或联络员，请直接致电宣传部办公室进行线下报备。</p>
            </div>
          </div>

          <!-- ================= 填报流程区 ================= -->
          <template v-if="!expired">
            <div class="flow-divider">
              <span class="divider-line"></span>
              <span class="divider-text font-serif">参 会 回 执 填 报</span>
              <span class="divider-line"></span>
            </div>

            <!-- 第一阶段：未选定单位 -->
            <div class="unit-select-card" v-if="!unitSelected">
              <div class="select-lead">
                <div class="lead-badge">第 1 步</div>
                <div class="lead-title">选择您的所属参会单位</div>
                <div class="lead-desc">全县各部门、乡镇场、企事业单位名单实时联网校验</div>
              </div>

              <div class="unit-picker-box">
                <el-select 
                  v-model="pendingUnit" 
                  filterable 
                  size="large"
                  placeholder="请在此输入或选择参会单位全称..." 
                  class="custom-unit-select"
                  @change="onUnitChosen"
                >
                  <template #prefix>
                    <el-icon class="select-search-icon"><OfficeBuilding /></el-icon>
                  </template>
                  <el-option 
                    v-for="u in units" 
                    :key="u" 
                    :label="u" 
                    :value="u" 
                  />
                </el-select>
              </div>

              <div class="select-foot-tips">
                <el-icon><Check /></el-icon>
                <span>选定单位后，系统将自动核验并显示该单位当前的报名及名额状态</span>
              </div>
            </div>

            <!-- 第二阶段：已选定单位 -->
            <div class="unit-active-area" v-else>
              <!-- 专属单位授牌栏 -->
              <div class="unit-honor-bar">
                <div class="honor-left">
                  <span class="honor-tag">已选定参会单位</span>
                  <h3 class="honor-name font-serif">{{ form.unit }}</h3>
                </div>
                <el-button 
                  type="primary" 
                  plain 
                  size="small" 
                  class="change-unit-btn"
                  @click="changeUnit"
                >
                  更换单位
                </el-button>
              </div>

              <!-- 官方正式回执凭证卡片（提交成功或已有填报时常驻） -->
              <transition name="fade-bounce">
                <div v-if="submitted" class="receipt-dossier">
                  <div class="receipt-stamp">
                    <div class="stamp-circle">
                      <span>已确认回执</span>
                      <small>CONFIRMED</small>
                    </div>
                  </div>
                  <div class="receipt-top">
                    <span class="receipt-badge">电子回执凭证</span>
                    <span class="receipt-time">{{ receiptTimeText }}</span>
                  </div>
                  <div class="receipt-main">
                    <h4 class="receipt-status-title font-serif">{{ submittedTitle }}</h4>
                    <p class="receipt-status-desc">{{ submittedText }}</p>
                    <div class="receipt-hint-bar" v-if="editHint">
                      <el-icon><InfoFilled /></el-icon>
                      <span>{{ editHint }}</span>
                    </div>
                  </div>

                  <!-- 回执快捷操作条 -->
                  <div class="receipt-quick-actions" v-if="registrations.length > 0">
                    <el-button 
                      size="small" 
                      type="success" 
                      plain 
                      :icon="'CopyDocument'" 
                      @click="copyReceiptSummary"
                    >
                      复制回执文本发工作群
                    </el-button>
                  </div>
                </div>
              </transition>

              <!-- 参会状态选择胶囊 -->
              <div class="switch-container">
                <div class="switch-lead">参会意向确认：</div>
                <el-radio-group v-model="notAttend" size="large" class="custom-radio-group" @change="watchNotAttend">
                  <el-radio-button :label="0">
                    <span class="radio-label-inner">
                      <el-icon><Check /></el-icon> 确认参加
                    </span>
                  </el-radio-button>
                  <el-radio-button :label="1">
                    <span class="radio-label-inner">
                      <el-icon><Close /></el-icon> 因故不参加
                    </span>
                  </el-radio-button>
                </el-radio-group>
              </div>

              <!-- 单位整体不参加状态横幅 -->
              <div v-if="notAttendAll && notAttend === 1" class="notattend-card">
                <div class="notattend-info">
                  <el-icon :size="20" class="notattend-icon"><Warning /></el-icon>
                  <div>
                    <div class="notattend-title">本单位已正式报备不参加本次会议</div>
                    <div class="notattend-reason" v-if="notAttendReason">报备原因：{{ notAttendReason }}</div>
                  </div>
                </div>
                <el-button type="primary" size="small" class="re-attend-btn" @click="switchToAttend">
                  撤销并改为参会
                </el-button>
              </div>

              <!-- ================= 参加模式：人员填报区 ================= -->
              <div v-if="notAttend === 0" class="attend-management-box">
                <!-- 多人模式：已报人员席位卡列表 -->
                <div class="seats-section" v-if="registrations.length > 0">
                  <div class="seats-header">
                    <div class="seats-title font-serif">
                      <span>已录入参会席位</span>
                      <span class="seats-count tabular-nums">（{{ registrations.length }}人）</span>
                    </div>
                    <div class="seats-quota" v-if="unitLimit > 0">
                      限额：{{ unitLimit }} 人
                    </div>
                  </div>

                  <div class="seat-cards-grid">
                    <div 
                      v-for="(rg, idx) in registrations" 
                      :key="rg.id" 
                      class="seat-card"
                      :class="{ 'is-editing': editingRegId === rg.id }"
                    >
                      <div class="seat-badge-no font-serif">席位 {{ idx + 1 }}</div>
                      <div class="seat-info">
                        <div class="seat-name font-serif">{{ rg.attendee_name }}</div>
                        <div class="seat-meta">
                          <span class="seat-title-pill">{{ rg.attendee_title }}</span>
                          <span class="seat-phone tabular-nums">{{ rg.phone }}</span>
                        </div>
                      </div>
                      <div class="seat-actions">
                        <el-button 
                          type="primary" 
                          link 
                          size="small" 
                          :icon="'Edit'"
                          @click="editReg(rg)"
                        >
                          修改
                        </el-button>
                        <el-button 
                          type="danger" 
                          link 
                          size="small" 
                          :icon="'Delete'"
                          @click="removeReg(rg)"
                        >
                          移除
                        </el-button>
                      </div>
                    </div>
                  </div>

                  <!-- 席位满额提示 -->
                  <div v-if="remain === 0" class="seats-full-alert">
                    <el-icon><Check /></el-icon>
                    <span>该单位参会席位已满（共 {{ unitLimit }} 人）。如需调整人员，可点击上方席位进行修改或移除。</span>
                  </div>
                </div>

                <!-- 填报 / 修改 人员表单卡 -->
                <div 
                  class="person-input-card"
                  v-if="editingRegId || (unitLimit === 1 && !notAttendAll) || (unitLimit !== 1 && remain !== 0 && remain !== undefined && !notAttendAll)"
                >
                  <div class="person-card-header">
                    <span class="header-mark"></span>
                    <h4 class="font-serif">
                      {{ editingRegId ? `修改参会人员信息（正在修改：${editingRegName}）` : (registrations.length > 0 ? '添加下一位参会人员' : '填写参会人员信息') }}
                    </h4>
                  </div>

                  <el-form :model="personForm" label-position="top" class="custom-gov-form">
                    <el-row :gutter="14">
                      <el-col :xs="24" :sm="12">
                        <el-form-item label="参会人员姓名" required>
                          <el-input 
                            v-model="personForm.attendee_name" 
                            placeholder="请填写干部真实姓名" 
                            size="large"
                          >
                            <template #prefix><el-icon><User /></el-icon></template>
                          </el-input>
                        </el-form-item>
                      </el-col>
                      <el-col :xs="24" :sm="12">
                        <el-form-item label="行政职务" required>
                          <el-input 
                            v-model="personForm.attendee_title" 
                            placeholder="如：副局长 / 科长 / 业务骨干" 
                            size="large"
                          >
                            <template #prefix><el-icon><Tickets /></el-icon></template>
                          </el-input>
                        </el-form-item>
                      </el-col>
                    </el-row>

                    <el-form-item label="联系电话" required>
                      <el-input 
                        v-model="personForm.phone" 
                        maxlength="11" 
                        :placeholder="phonePlaceholder || '请填写11位手机号'" 
                        size="large"
                      >
                        <template #prefix><el-icon><Phone /></el-icon></template>
                      </el-input>
                    </el-form-item>

                    <div class="form-btn-row">
                      <template v-if="editingRegId">
                        <el-button size="large" @click="cancelEditReg">取消修改</el-button>
                        <el-button type="primary" size="large" class="submit-action-btn" :loading="submitting" @click="savePerson">
                          保存并更新席位
                        </el-button>
                      </template>
                      <template v-else>
                        <el-button 
                          type="primary" 
                          size="large" 
                          class="submit-action-btn" 
                          :loading="submitting" 
                          @click="savePerson"
                        >
                          {{ unitLimit === 1 ? '确认参会并提交回执' : '确认并录入此席位' }}
                        </el-button>
                      </template>
                    </div>
                  </el-form>
                </div>
              </div>

              <!-- ================= 不参加模式：原因填报区 ================= -->
              <div v-if="notAttend === 1 && !notAttendAll" class="absent-input-card">
                <div class="absent-card-header">
                  <el-icon class="absent-icon"><Warning /></el-icon>
                  <div>
                    <h4 class="font-serif">说明不参加原因</h4>
                    <p>按县委宣传部会务规定，因故无法参会的单位需向会务组陈述请假事由备查。</p>
                  </div>
                </div>

                <el-form label-position="top" class="custom-gov-form absent-form-wrap">
                  <el-form-item label="请假事由与具体原因" required class="absent-reason-item">
                    <el-input 
                      v-model="absentReason" 
                      type="textarea" 
                      :rows="4" 
                      placeholder="请详细说明因公出差、专项任务或其他无法参会的原因..." 
                    />
                  </el-form-item>
                  <el-button 
                    type="danger" 
                    size="large" 
                    class="absent-submit-btn" 
                    :loading="submitting" 
                    @click="saveAbsent"
                  >
                    提交不参加回执
                  </el-button>
                </el-form>
              </div>
            </div>
          </template>
        </div>
      </template>

      <!-- 加载中动画骨架 -->
      <template v-else-if="loading">
        <div class="loading-card">
          <div class="loading-spinner"></div>
          <p class="font-serif">正在接入中共伊宁县委宣传部数字化会务中心...</p>
        </div>
      </template>

      <!-- 会议失效或未找到 -->
      <template v-else>
        <div class="error-card">
          <el-result 
            icon="error" 
            title="未检索到该会议通知" 
            sub-title="该会议链接可能已失效、被撤回，或未在系统中发布。请与县委宣传部会务组联系核对。" 
          />
        </div>
      </template>

      <!-- 尊贵政务页脚与技术背书 -->
      <footer class="gov-footer">
        <div class="footer-crest-row">
          <span class="crest-dot"></span>
          <span class="footer-org font-serif">中共伊宁县委宣传部</span>
          <span class="crest-dot"></span>
        </div>
        <div class="footer-system">部务工作平台 · 数字化会务服务终端 V1.6.2</div>
        <div class="footer-beian">
          <a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener">ICP备案号占位</a>
          <span class="sep">|</span>
          <a href="https://beian.mps.gov.cn/#/query/webSearch" target="_blank" rel="noopener">公网安备号占位</a>
          <span class="sep">|</span>
          <span class="ipv6-tag">本平台已全面支持 IPv6 访问</span>
        </div>
      </footer>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { 
  Calendar, Location, OfficeBuilding, Clock, User, Check, Close, 
  Warning, InfoFilled, Tickets, Phone, Edit, Delete, CopyDocument 
} from '@element-plus/icons-vue'
import request from '../utils/request'
import dayjs from 'dayjs'

const route = useRoute()
const meeting = ref(null)
const units = ref([])
const loading = ref(true)
const expired = ref(false)
const unitLimit = ref(1)
const unitSelected = ref(false)
const pendingUnit = ref('')
const notAttend = ref(0)
const notAttendAll = ref(false)
const notAttendReason = ref('')
const registrations = ref([])
const remain = ref(-1)
const submitting = ref(false)
const submitted = ref(false)
const submittedTitle = ref('')
const submittedText = ref('')
const editHint = ref('')

const form = ref({ unit: '' })
const personForm = ref({ attendee_name: '', attendee_title: '', phone: '' })
const phonePlaceholder = ref('')
const editingRegId = ref(0)
const editingRegName = ref('')
const absentReason = ref('')

const meetingId = route.params.id

// 格式化时间与日期展示
const meetingDateTimeText = computed(() => {
  if (!meeting.value) return ''
  const m = meeting.value
  if (m.meeting_date && m.meeting_time) return `${m.meeting_date}  ${m.meeting_time}`
  return m.meeting_date || m.meeting_time || ''
})

const meetingWeekDay = computed(() => {
  if (!meeting.value?.meeting_date) return ''
  try {
    const d = dayjs(meeting.value.meeting_date)
    if (!d.isValid()) return ''
    const weeks = ['星期日', '星期一', '星期二', '星期三', '星期四', '星期五', '星期六']
    return weeks[d.day()]
  } catch (e) {
    return ''
  }
})

// 会议倒计时或召开状态
const countdownText = computed(() => {
  if (!meeting.value?.meeting_date) return ''
  try {
    const today = dayjs().startOf('day')
    const target = dayjs(meeting.value.meeting_date).startOf('day')
    const diff = target.diff(today, 'day')
    if (diff > 0) return `距会议召开还有 ${diff} 天`
    if (diff === 0) return '会议将于今日举行'
    return ''
  } catch (e) {
    return ''
  }
})

// 参会限额提示文字
const unitLimitRuleText = computed(() => {
  const limit = unitLimit.value
  if (limit === 1) return '每单位需参加 1 人'
  if (limit > 1) return `每单位需参加 ${limit} 人`
  return '每单位参会人数不限'
})

// 回执当前时间
const receiptTimeText = computed(() => {
  return dayjs().format('YYYY年MM月DD日 HH:mm')
})

const loadMeeting = async () => {
  try {
    const res = await request.get(`/public/meetings/${meetingId}`)
    meeting.value = res.meeting
    units.value = res.units || []
    unitLimit.value = res.meeting?.unit_limit ?? 1
    expired.value = !!res.expired
  } catch (e) {
    meeting.value = null
  } finally {
    loading.value = false
  }
}

// 选择单位后：查询该单位已报名情况
const onUnitChosen = (val) => {
  if (!val) return
  submitted.value = false
  form.value.unit = val
  unitSelected.value = true
  notAttend.value = 0
  notAttendAll.value = false
  notAttendReason.value = ''
  registrations.value = []
  remain.value = -1
  editingRegId.value = 0
  personForm.value = { attendee_name: '', attendee_title: '', phone: '' }
  absentReason.value = ''
  loadUnitStatus()
}

const loadUnitStatus = async () => {
  try {
    const res = await request.get(`/public/meetings/${meetingId}?unit=${encodeURIComponent(form.value.unit)}`)
    registrations.value = res.registrations || []
    remain.value = res.remain
    notAttendAll.value = !!res.not_attend_all
    notAttendReason.value = res.not_attend_reason || ''

    if (notAttendAll.value) {
      notAttend.value = 1
      absentReason.value = notAttendReason.value
      submitted.value = true
      submittedTitle.value = '已确认不参加'
      submittedText.value = notAttendReason.value ? '已记录不参加原因：' + notAttendReason.value : '已记录不参加'
      editHint.value = '如需改为参加，请点击下方「确认参加」切换后录入参会人员。'
      return
    }

    if (unitLimit.value === 1 && registrations.value.length > 0) {
      const rg = registrations.value[0]
      editingRegId.value = rg.id
      editingRegName.value = rg.attendee_name
      personForm.value = { attendee_name: rg.attendee_name, attendee_title: rg.attendee_title, phone: '' }
      phonePlaceholder.value = rg.phone ? '原号码 ' + rg.phone + '（不修改请留空）' : ''
      submitted.value = true
      submittedTitle.value = '参会回执已确认'
      submittedText.value = `已成功登记「${rg.attendee_name}」同志参会，请准时出席。`
      editHint.value = '如需调整参会人员或更正联系电话，请在下方修改后重新提交。'
    } else if (unitLimit.value !== 1 && registrations.value.length > 0) {
      submitted.value = true
      submittedTitle.value = '参会回执已建立'
      submittedText.value = `该单位当前已落实 ${registrations.value.length} 个参会席位${remain.value > 0 ? '，尚余 ' + remain.value + ' 个名额可补报' : ''}。`
      editHint.value = '如需修改或移除参会人员，可在下方席位列表中点击对应按钮操作。'
    }
  } catch (e) { console.error(e) }
}

const changeUnit = () => {
  submitted.value = false
  unitSelected.value = false
  pendingUnit.value = ''
  form.value.unit = ''
  registrations.value = []
  remain.value = -1
  notAttendAll.value = false
  notAttendReason.value = ''
}

const switchToAttend = () => {
  submitted.value = false
  notAttendAll.value = false
  notAttend.value = 0
  absentReason.value = ''
}

const watchNotAttend = (val) => {
  submitted.value = false
  if (val === 1 && registrations.value.length > 0) {
    ElMessageBox.alert('该单位已有已确认的参会席位。如需全员变更为不参加，请联系会务组管理员协调调整。', '政务提示', { type: 'warning' })
    nextTick(() => {
      notAttend.value = 0
    })
    return
  }
  if (val === 0 && notAttendAll.value) {
    notAttendAll.value = false
  }
}

const editReg = (rg) => {
  editingRegId.value = rg.id
  editingRegName.value = rg.attendee_name
  personForm.value = { attendee_name: rg.attendee_name, attendee_title: rg.attendee_title, phone: '' }
  phonePlaceholder.value = rg.phone ? '原号码 ' + rg.phone + '（不修改请留空）' : ''
  // 滚动至表单区域
  nextTick(() => {
    const el = document.querySelector('.person-input-card')
    if (el) el.scrollIntoView({ behavior: 'smooth' })
  })
}

const cancelEditReg = () => {
  editingRegId.value = 0
  editingRegName.value = ''
  personForm.value = { attendee_name: '', attendee_title: '', phone: '' }
  phonePlaceholder.value = ''
}

const removeReg = async (rg) => {
  if (registrations.value.length <= 1) {
    ElMessageBox.alert('该单位仅余一名参会人员，不可直接移除。如需更换人员请直接点击「修改」，如需取消参会请联系会务组管理员。', '提示', { type: 'warning' })
    return
  }
  try {
    await ElMessageBox.confirm(`确定移除「${rg.attendee_name}」同志的参会席位吗？`, '席位移除确认', { 
      type: 'warning', 
      confirmButtonText: '确定移除', 
      cancelButtonText: '取消' 
    })
  } catch (e) { console.error(e); return }

  try {
    await request.post(`/public/meetings/${meetingId}/remove`, { reg_id: rg.id, unit: form.value.unit })
    ElMessage.success('已成功移除该席位')
    loadUnitStatus()
  } catch (e) { console.error(e) }
}

const savePerson = async () => {
  if (!personForm.value.attendee_name) return ElMessage.warning('请填写参会人员真实姓名')
  if (!personForm.value.attendee_title) return ElMessage.warning('请填写行政职务')
  if (!editingRegId.value || personForm.value.phone) {
    if (!/^1[3-9]\d{9}$/.test(personForm.value.phone)) return ElMessage.warning('请输入正确的11位手机号码')
  }
  submitting.value = true
  try {
    const payload = {
      meeting_id: Number(meetingId),
      unit: form.value.unit,
      attendee_name: personForm.value.attendee_name,
      attendee_title: personForm.value.attendee_title,
      phone: personForm.value.phone,
      not_attend: 0,
      reg_id: editingRegId.value
    }
    await request.post(`/public/meetings/${meetingId}/register`, payload)
    await loadUnitStatus()
    if (editingRegId.value) {
      showSuccess('席位信息已更新', `「${personForm.value.attendee_name}」同志的信息已成功同步至会务名册。`, '如需再次修改，可在下方席位列表中点击操作。')
    } else if (unitLimit.value === 1) {
      showSuccess('参会回执确认成功', `已确认「${personForm.value.attendee_name}」同志代表本单位参会，请准时出席。`, '如需变更参会领导或信息，可直接在下方修改后重新提交。')
    } else {
      showSuccess('参会席位已录入', `该单位当前已落实 ${registrations.value.length} 人${remain.value > 0 ? '，尚可补报 ' + remain.value + ' 人' : ''}。`, '如需修改或移除，请在下方已报席位列表中操作。')
    }
    if (editingRegId.value) {
      editingRegId.value = 0
      editingRegName.value = ''
    }
    personForm.value = { attendee_name: '', attendee_title: '', phone: '' }
    phonePlaceholder.value = ''
  } catch (e) { console.error(e) } finally {
    submitting.value = false
  }
}

const showSuccess = (title, text, hint) => {
  submittedTitle.value = title
  submittedText.value = text
  editHint.value = hint || ''
  submitted.value = true
}

const saveAbsent = async () => {
  if (!absentReason.value) return ElMessage.warning('请详细说明不参加原因')
  submitting.value = true
  try {
    await request.post(`/public/meetings/${meetingId}/register`, {
      meeting_id: Number(meetingId), unit: form.value.unit, not_attend: 1, reason: absentReason.value
    })
    await loadUnitStatus()
    notAttend.value = 1
    showSuccess('已提交不参加报备', absentReason.value ? '已记录因故请假原因：' + absentReason.value : '已完成请假报备。', '如后续情况变更需参会，请点击上方「确认参加」后重新录入人员。')
  } catch (e) { console.error(e) } finally {
    submitting.value = false
  }
}

// 复制参会回执文本（供办公室转发工作群）
const copyReceiptSummary = () => {
  if (!meeting.value || !form.value.unit) return
  const attendees = registrations.value.map(r => `${r.attendee_name}（${r.attendee_title || '参会人员'}）`).join('、')
  const summary = `【参会回执】\n会议：${meeting.value.title}\n时间：${meetingDateTimeText.value}\n地点：${meeting.value.location || '见通知'}\n单位：${form.value.unit}\n参会人员：${attendees}\n状态：已完成数字化在线登记回执`
  
  navigator.clipboard.writeText(summary).then(() => {
    ElMessage.success('参会回执文本已复制到剪贴板，可直接粘贴发工作群')
  }).catch(() => {
    ElMessage.info('复制失败，请截图保存')
  })
}

onMounted(loadMeeting)
</script>

<style scoped>
/* 全局背景：尊贵墨蓝与红金政务渐变 */
.gov-reg-wrapper {
  min-height: 100vh;
  background-color: #f4f5f8;
  position: relative;
  overflow-x: hidden;
  display: flex;
  flex-direction: column;
}

/* 顶部天际线背景区 */
.skyline-bg {
  position: relative;
  background: linear-gradient(135deg, #131a26 0%, #1b2433 45%, #7a1417 100%);
  padding: 32px 20px 110px;
  color: #fff;
  overflow: hidden;
}

.skyline-glow {
  position: absolute;
  top: -60px;
  right: 10%;
  width: 320px;
  height: 320px;
  background: radial-gradient(circle, rgba(227, 207, 166, 0.22) 0%, rgba(158, 27, 30, 0) 70%);
  pointer-events: none;
}

.tianshan-peaks {
  position: absolute;
  bottom: 0;
  left: 0;
  width: 100%;
  height: 100px;
  pointer-events: none;
}

/* 官方标头 */
.org-brand-bar {
  max-width: 720px;
  margin: 0 auto;
  display: flex;
  align-items: center;
  gap: 14px;
  position: relative;
  z-index: 2;
}

.brand-crest {
  width: 50px;
  height: 50px;
  object-fit: contain;
  filter: drop-shadow(0 0 14px rgba(245, 212, 153, 0.65));
}

.brand-sup {
  font-family: var(--yx-font-serif);
  font-size: 17px;
  font-weight: 600;
  letter-spacing: 1.2px;
  color: #f7ecec;
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.5);
}

.brand-sub {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.72);
  margin-top: 3px;
  letter-spacing: 0.8px;
}

/* 主体容器 */
.main-container {
  max-width: 720px;
  width: 100%;
  margin: -70px auto 40px;
  padding: 0 16px;
  box-sizing: border-box;
  position: relative;
  z-index: 3;
  flex: 1;
}

/* 核心公文通告卡片 */
.dossier-card {
  background: #ffffff;
  border-radius: 14px;
  box-shadow: 0 12px 40px rgba(19, 26, 38, 0.12), 0 2px 6px rgba(19, 26, 38, 0.04);
  padding: 32px 30px;
  box-sizing: border-box;
  position: relative;
  border: 1px solid rgba(220, 223, 229, 0.6);
}

.card-gold-crest-bar {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 4px;
  border-top-left-radius: 14px;
  border-top-right-radius: 14px;
  background: linear-gradient(90deg, #9e1b1e 0%, #b8925a 50%, #24406e 100%);
}

/* 状态标签栏 */
.status-ribbon-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin-bottom: 18px;
}

.status-pill {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 4px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 600;
}

.status-pill.is-active {
  background: #eef7f2;
  color: #1e7048;
  border: 1px solid #c7e8d6;
}

.status-pill.is-expired {
  background: #fff4e5;
  color: #b7791f;
  border: 1px solid #ffd8a8;
}

.pulse-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background-color: #2e7d5b;
  box-shadow: 0 0 0 0 rgba(46, 125, 91, 0.7);
  animation: pulse-ring 1.8s infinite;
}

@keyframes pulse-ring {
  0% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(46, 125, 91, 0.7); }
  70% { transform: scale(1); box-shadow: 0 0 0 6px rgba(46, 125, 91, 0); }
  100% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(46, 125, 91, 0); }
}

.rule-pill, .countdown-pill {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 4px 11px;
  border-radius: 20px;
  font-size: 12px;
  background: #f4f6fa;
  color: #4e5969;
  border: 1px solid #e1e4ea;
}

.countdown-pill {
  background: #fdf6ec;
  color: #b8925a;
  border-color: #faecd8;
}

/* 会议大标题 */
.dossier-title {
  font-size: 24px;
  line-height: 1.45;
  color: #1b2433;
  margin: 0 0 22px;
  letter-spacing: 0.6px;
  word-break: break-all;
}

/* 会务三联看板 */
.meeting-dossier-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
  margin-bottom: 22px;
}

.item-full-width {
  grid-column: 1 / -1;
}

.dossier-item {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  background: #f8f9fb;
  border: 1px solid #e9ecf2;
  border-radius: 8px;
  padding: 12px 14px;
  min-width: 0;
  box-sizing: border-box;
}

.dossier-icon {
  width: 38px;
  height: 38px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  flex-shrink: 0;
  margin-top: 2px;
}

.time-icon { background: #fdf2f2; color: #9e1b1e; }
.place-icon { background: #eef3f9; color: #24406e; }
.org-icon { background: #fbf6ec; color: #b8925a; }

.dossier-text {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

.dossier-label {
  font-size: 11px;
  color: #8a94a0;
  margin-bottom: 3px;
  letter-spacing: 0.5px;
}

.dossier-val {
  font-size: 14.5px;
  font-weight: 600;
  color: #1f2329;
  line-height: 1.55;
  white-space: normal;
  word-break: break-word;
  overflow-wrap: break-word;
}

.weekday-badge {
  font-size: 11px;
  font-weight: 500;
  background: #e9ecf2;
  color: #4e5969;
  padding: 1px 6px;
  border-radius: 4px;
  margin-left: 6px;
  white-space: nowrap;
  display: inline-flex;
  align-items: center;
  vertical-align: 1.5px;
  line-height: 1.4;
}

/* 议程公文卡 */
.notice-callout {
  background: #fafaf8;
  border: 1px solid #ebe9e3;
  border-left: 4px solid #b8925a;
  border-radius: 6px;
  padding: 14px 16px;
  margin-bottom: 24px;
}

.callout-header {
  margin-bottom: 8px;
}

.callout-flag {
  font-family: var(--yx-font-serif);
  font-size: 13px;
  font-weight: 600;
  color: #7a5d30;
}

.callout-body {
  font-size: 13.5px;
  color: #4e5969;
  line-height: 1.75;
  white-space: pre-wrap;
  word-break: break-all;
}

/* 流程分割线 */
.flow-divider {
  display: flex;
  align-items: center;
  margin: 30px 0 24px;
}

.divider-line {
  flex: 1;
  height: 1px;
  background: linear-gradient(90deg, transparent, #dcdfe5, transparent);
}

.divider-text {
  padding: 0 16px;
  font-size: 15px;
  font-weight: 600;
  letter-spacing: 2px;
  color: #24406e;
}

/* 第一步：单位选择卡 */
.unit-select-card {
  background: #ffffff;
  border: 1.5px dashed #d0d7e5;
  border-radius: 12px;
  padding: 24px;
  text-align: center;
}

.lead-badge {
  display: inline-block;
  background: #24406e;
  color: #ffffff;
  font-size: 11px;
  font-weight: 600;
  padding: 2px 10px;
  border-radius: 12px;
  margin-bottom: 8px;
}

.lead-title {
  font-family: var(--yx-font-serif);
  font-size: 18px;
  font-weight: 600;
  color: #1f2329;
  margin-bottom: 4px;
}

.lead-desc {
  font-size: 13px;
  color: #8a94a0;
  margin-bottom: 20px;
}

.unit-picker-box {
  max-width: 480px;
  margin: 0 auto;
}

.custom-unit-select {
  width: 100%;
}

.select-search-icon {
  color: #24406e;
  font-size: 16px;
}

.select-foot-tips {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  margin-top: 14px;
  font-size: 12px;
  color: #6b7785;
}

/* 第二步：单位授牌栏 */
.unit-honor-bar {
  background: linear-gradient(135deg, #f7f9fc 0%, #eef3f9 100%);
  border: 1px solid #d4e0f0;
  border-radius: 10px;
  padding: 16px 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
}

.honor-tag {
  font-size: 11px;
  color: #24406e;
  background: #e0e9f6;
  padding: 2px 8px;
  border-radius: 4px;
  font-weight: 500;
}

.honor-name {
  font-size: 18px;
  color: #1b2433;
  margin: 6px 0 0;
  letter-spacing: 0.5px;
}

.change-unit-btn {
  border-radius: 6px;
}

/* 官方正式回执凭证卡片 */
.receipt-dossier {
  background: #ffffff;
  border: 1px solid #b7e1cd;
  border-left: 4px solid #2e7d5b;
  border-radius: 10px;
  padding: 18px 22px;
  margin-bottom: 22px;
  position: relative;
  box-shadow: 0 4px 16px rgba(46, 125, 91, 0.08);
  overflow: hidden;
}

.receipt-stamp {
  position: absolute;
  top: 12px;
  right: 18px;
  pointer-events: none;
  opacity: 0.78;
}

.stamp-circle {
  width: 74px;
  height: 74px;
  border: 2px dashed #2e7d5b;
  border-radius: 50%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  transform: rotate(-12deg);
  color: #2e7d5b;
  font-weight: bold;
  font-size: 11px;
}

.stamp-circle small {
  font-size: 8px;
  letter-spacing: 0.5px;
}

.receipt-top {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 6px;
}

.receipt-badge {
  background: #2e7d5b;
  color: #ffffff;
  font-size: 10.5px;
  padding: 2px 8px;
  border-radius: 4px;
  font-weight: 600;
}

.receipt-time {
  font-size: 12px;
  color: #8a94a0;
}

.receipt-status-title {
  font-size: 17px;
  color: #1b2433;
  margin: 4px 0;
}

.receipt-status-desc {
  font-size: 13.5px;
  color: #4e5969;
  line-height: 1.6;
  margin: 0;
}

.receipt-hint-bar {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 10px;
  font-size: 12px;
  color: #6b7785;
}

.receipt-quick-actions {
  margin-top: 12px;
  padding-top: 10px;
  border-top: 1px dashed #d5eedf;
}

/* 参加/不参加胶囊切换 */
.switch-container {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #f8f9fb;
  border-radius: 10px;
  padding: 10px 16px;
  margin-bottom: 20px;
  border: 1px solid #ebeef5;
}

.switch-lead {
  font-size: 14px;
  font-weight: 600;
  color: #24406e;
}

.radio-label-inner {
  display: flex;
  align-items: center;
  gap: 5px;
}

/* 单位整体不参加提示卡 */
.notattend-card {
  background: #fdf6ec;
  border: 1px solid #faecd8;
  border-radius: 8px;
  padding: 14px 18px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 20px;
}

.notattend-info {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.notattend-icon {
  color: #e6a23c;
  margin-top: 2px;
}

.notattend-title {
  font-size: 14px;
  font-weight: 600;
  color: #b88230;
}

.notattend-reason {
  font-size: 12.5px;
  color: #6b7785;
  margin-top: 2px;
}

/* 席位卡片列表 */
.seats-section {
  margin-bottom: 24px;
}

.seats-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.seats-title {
  font-size: 16px;
  font-weight: 600;
  color: #1f2329;
}

.seats-count {
  color: #24406e;
}

.seats-quota {
  font-size: 12px;
  color: #8a94a0;
  background: #f0f2f5;
  padding: 2px 8px;
  border-radius: 4px;
}

.seat-cards-grid {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.seat-card {
  background: #ffffff;
  border: 1px solid #e2e6ed;
  border-radius: 10px;
  padding: 14px 18px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.03);
  transition: all 0.2s ease;
}

.seat-card.is-editing {
  border-color: #b8925a;
  background: #fdfaf5;
  box-shadow: 0 0 0 2px rgba(184, 146, 90, 0.2);
}

.seat-badge-no {
  font-size: 12px;
  font-weight: 600;
  background: #eef2f9;
  color: #24406e;
  padding: 4px 8px;
  border-radius: 6px;
  margin-right: 14px;
  flex-shrink: 0;
}

.seat-info {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.seat-name {
  font-size: 16px;
  font-weight: 600;
  color: #1f2329;
}

.seat-meta {
  display: flex;
  align-items: center;
  gap: 8px;
}

.seat-title-pill {
  font-size: 12px;
  color: #4e5969;
  background: #f2f3f5;
  padding: 2px 8px;
  border-radius: 4px;
}

.seat-phone {
  font-size: 13px;
  color: #8a94a0;
}

.seat-actions {
  display: flex;
  gap: 6px;
  flex-shrink: 0;
}

.seats-full-alert {
  margin-top: 10px;
  background: #f0f9eb;
  border: 1px solid #e1f3d8;
  color: #67c23a;
  border-radius: 6px;
  padding: 8px 12px;
  font-size: 12.5px;
  display: flex;
  align-items: center;
  gap: 8px;
}

/* 填写人员信息卡 */
.person-input-card {
  background: #ffffff;
  border: 1px solid #dcdfe6;
  border-radius: 12px;
  padding: 22px 24px;
  margin-top: 14px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.04);
}

.person-card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 18px;
  padding-bottom: 12px;
  border-bottom: 1px solid #edf0f5;
}

.header-mark {
  width: 4px;
  height: 16px;
  background: #b8925a;
  border-radius: 2px;
}

.person-card-header h4 {
  margin: 0;
  font-size: 16px;
  color: #1f2329;
}

.form-btn-row {
  display: flex;
  gap: 12px;
  margin-top: 10px;
}

.submit-action-btn {
  flex: 1;
  background: linear-gradient(135deg, #24406e 0%, #1b2433 100%);
  border: none;
  font-size: 15px;
  letter-spacing: 1px;
}

.submit-action-btn:hover {
  background: linear-gradient(135deg, #2f538e 0%, #24406e 100%);
}

/* 不参加原因卡 */
.absent-input-card {
  background: #fef8f8;
  border: 1px solid #fad2d2;
  border-radius: 12px;
  padding: 22px 24px;
}

.absent-card-header {
  display: flex;
  gap: 12px;
}

.absent-icon {
  font-size: 24px;
  color: #c4312b;
  margin-top: 2px;
}

.absent-card-header h4 {
  margin: 0 0 4px;
  font-size: 16px;
  color: #c4312b;
}

.absent-card-header p {
  margin: 0;
  font-size: 13px;
  color: #6b7785;
  line-height: 1.5;
}

.absent-form-wrap {
  margin-top: 22px;
  padding-top: 18px;
  border-top: 1px dashed #f0c5c5;
}

.absent-reason-item :deep(.el-form-item__label) {
  font-size: 14px;
  font-weight: 600;
  color: #2c384e;
  margin-bottom: 8px;
}

.absent-submit-btn {
  width: 100%;
}

/* 截止横幅 */
.expired-banner {
  background: #fdf6ec;
  border: 1px solid #faecd8;
  border-radius: 8px;
  padding: 16px 20px;
  display: flex;
  gap: 14px;
  margin-bottom: 24px;
}

.banner-icon-col {
  color: #e6a23c;
  margin-top: 2px;
}

.banner-text-col h4 {
  margin: 0 0 4px;
  font-size: 15px;
  color: #b88230;
}

.banner-text-col p {
  margin: 0;
  font-size: 13px;
  color: #6b7785;
}

/* 加载与错误 */
.loading-card, .error-card {
  background: #fff;
  border-radius: 14px;
  padding: 60px 20px;
  text-align: center;
  box-shadow: 0 8px 30px rgba(0, 0, 0, 0.08);
}

.loading-spinner {
  width: 40px;
  height: 40px;
  border: 3px solid #eef2f9;
  border-top-color: #24406e;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  margin: 0 auto 16px;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.loading-card p {
  font-size: 15px;
  color: #4e5969;
}

/* 尊贵页脚 */
.gov-footer {
  margin-top: 36px;
  text-align: center;
  color: #6b7785;
  font-size: 12px;
}

.footer-crest-row {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin-bottom: 6px;
}

.crest-dot {
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: #b8925a;
}

.footer-org {
  font-size: 14px;
  color: #1f2329;
  letter-spacing: 1px;
}

.footer-system {
  color: #8a94a0;
  margin-bottom: 8px;
}

.footer-beian {
  color: #8a94a0;
  line-height: 1.8;
}

.footer-beian a {
  color: #6b7785;
  text-decoration: none;
}

.footer-beian .sep {
  margin: 0 8px;
  color: #dcdfe6;
}

.ipv6-tag {
  color: #6b7785;
}

/* 动效 */
.fade-bounce-enter-active {
  transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}
.fade-bounce-enter-from {
  opacity: 0;
  transform: translateY(-8px);
}

/* 移动端全屏响应式适配 */
@media (max-width: 680px) {
  .skyline-bg {
    padding: 24px 14px 90px;
  }
  .brand-crest {
    width: 42px;
    height: 42px;
  }
  .brand-sup {
    font-size: 15px;
  }
  .main-container {
    padding: 0 10px;
    margin-top: -65px;
  }
  .dossier-card {
    padding: 22px 16px;
    border-radius: 12px;
  }
  .dossier-title {
    font-size: 19px;
  }
  .meeting-dossier-grid {
    grid-template-columns: 1fr;
  }
  .unit-honor-bar {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }
  .change-unit-btn {
    align-self: flex-end;
  }
  .switch-container {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }
  .custom-radio-group {
    width: 100%;
    display: flex;
  }
  .custom-radio-group :deep(.el-radio-button) {
    flex: 1;
  }
  .custom-radio-group :deep(.el-radio-button__inner) {
    width: 100%;
  }
  .seat-card {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }
  .seat-actions {
    align-self: flex-end;
  }
  .footer-beian {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .footer-beian .sep {
    display: none;
  }
}
</style>
