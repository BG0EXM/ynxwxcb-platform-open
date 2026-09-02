<template>
  <div class="reg-page">
    <div class="reg-card">
      <template v-if="meeting">
        <div class="reg-header">
          <img src="../assets/danghui.png" alt="党徽" class="danghui" />
          <h1 class="meeting-title">{{ meeting.title }}</h1>
          <div class="meeting-meta">
            <span v-if="meeting.meeting_date || meeting.meeting_time">会议时间：{{ meeting.meeting_date }}<span v-if="meeting.meeting_date && meeting.meeting_time">&nbsp;&nbsp;</span>{{ meeting.meeting_time }}</span>
            <span v-if="meeting.location">会议地点：{{ meeting.location }}</span>
          </div>
          <div v-if="meeting.content" class="meeting-content">{{ meeting.content }}</div>
        </div>

        <!-- 已截止提示 -->
        <el-alert v-if="expired" type="warning" :closable="false" title="会议已开始，报名已截止" description="如需调整报名情况，请联系会议组织单位。" class="expired-alert" />

        <!-- 报名区（未截止且未选单位时的初始切换） -->
        <template v-if="!expired">
          <!-- 单位选择 -->
          <div class="unit-select" v-if="!unitSelected">
            <div class="step-tip">第一步：请选择你的参会单位</div>
            <el-alert v-if="unitLimit > 1" type="info" :closable="false" class="limit-tip"
              :title="'本会议每单位需参加 ' + unitLimit + ' 人'" description="如单位人员不足，可少报" />
            <el-alert v-else-if="unitLimit === 0" type="info" :closable="false" class="limit-tip"
              title="本会议每单位参会人数不限" description="同一单位可报多位参会人员" />
            <el-select v-model="pendingUnit" filterable placeholder="请选择参会单位" style="width:100%" @change="onUnitChosen">
              <el-option v-for="u in units" :key="u" :label="u" :value="u" />
            </el-select>
          </div>

          <!-- 已选单位后的操作区 -->
          <div v-else class="unit-area">
            <div class="unit-head">
              <span class="unit-name">参会单位：{{ form.unit }}</span>
              <el-button link type="info" size="small" @click="changeUnit">更换单位</el-button>
            </div>

            <!-- 提交成功提示条 -->
            <transition name="fade">
              <el-alert v-if="submitted" type="success" :closable="true" show-icon class="success-alert"
                :title="submittedTitle" :description="submittedText" @close="submitted = false" />
            </transition>

            <!-- 参加/不参加切换（所有模式可用） -->
            <div class="attend-switch">
              <el-radio-group v-model="notAttend" size="large" @change="watchNotAttend">
                <el-radio-button :label="0">参加</el-radio-button>
                <el-radio-button :label="1">不参加</el-radio-button>
              </el-radio-group>
            </div>

            <!-- 该单位整体不参加提示（点"改为参加"可切换回来） -->
            <div v-if="notAttendAll && notAttend === 1" class="notattend-banner">
              <el-tag type="info">该单位已确认不参加{{ notAttendReason ? '：' + notAttendReason : '' }}</el-tag>
              <el-button link type="primary" size="small" @click="switchToAttend">改为参加</el-button>
            </div>

            <!-- 参加填写区（未整体不参加时） -->
            <div v-if="notAttend === 0">
              <!-- 多人与会：已报人员列表 -->
              <template v-if="unitLimit > 1">
                <div class="regs-list" v-if="registrations.length > 0">
                  <div class="list-title">已报参会人员（{{ registrations.length }} 人）</div>
                  <div class="reg-item" v-for="(rg, idx) in registrations" :key="rg.id">
                    <div class="reg-info">
                      <b>{{ rg.attendee_name }}</b>
                      <span class="reg-title">{{ rg.attendee_title }}</span>
                      <span class="reg-phone">{{ rg.phone }}</span>
                    </div>
                    <div class="reg-actions">
                      <el-button link type="warning" size="small" @click="editReg(rg)">修改</el-button>
                      <el-button link type="danger" size="small" @click="removeReg(rg)">移除</el-button>
                    </div>
                  </div>
                </div>

                <!-- 满员提示 -->
                <div v-if="remain === 0" class="full-tip">
                  <el-alert type="success" :closable="false" title="该单位报名人数已满" :description="`（最多 ${unitLimit} 人），可移除后重新添加`" />
                </div>
              </template>

              <!-- 新增/修改人员表单 -->
              <!-- 单人：显示（已有则预填替换）；多人：未满(remain!==0 或不限)或正在编辑时显示 -->
              <el-form v-if="editingRegId || (unitLimit === 1 && !notAttendAll) || (unitLimit !== 1 && remain !== 0 && remain !== undefined && !notAttendAll)"
                :model="personForm" label-width="90px" class="person-form">
                <div v-if="editingRegId" class="edit-tip">正在修改「{{ editingRegName }}」的信息</div>
                <el-form-item label="参会人员" required>
                  <el-input v-model="personForm.attendee_name" placeholder="填写参会人员姓名" />
                </el-form-item>
                <el-form-item label="职务" required>
                  <el-input v-model="personForm.attendee_title" placeholder="填写职务" />
                </el-form-item>
                <el-form-item label="联系电话" required>
                  <el-input v-model="personForm.phone" maxlength="11" placeholder="填写11位手机号" />
                </el-form-item>
                <el-form-item v-if="editingRegId">
                  <div class="form-actions">
                    <el-button @click="cancelEditReg">取消修改</el-button>
                    <el-button type="primary" @click="savePerson">保存修改</el-button>
                  </div>
                </el-form-item>
                <el-form-item v-else>
                  <el-button type="primary" class="submit-btn" :loading="submitting" @click="savePerson">
                    {{ unitLimit === 1 ? '确认参会' : '添加参会人员' }}
                  </el-button>
                </el-form-item>
              </el-form>
            </div>

            <!-- 不参加填写 -->
            <div v-if="notAttend === 1" class="absent-form">
              <el-form label-width="90px" class="person-form">
                <el-form-item label="不参加原因" required>
                  <el-input v-model="absentReason" type="textarea" :rows="3" placeholder="请说明不参加原因" />
                </el-form-item>
                <el-form-item>
                  <el-button type="primary" class="submit-btn" :loading="submitting" @click="saveAbsent">确认不参加</el-button>
                </el-form-item>
              </el-form>
            </div>
          </div>
        </template>
      </template>
      <template v-else-if="loading">
        <div class="loading-tip">加载中...</div>
      </template>
      <template v-else>
        <el-result icon="error" title="未找到该会议" sub-title="链接可能已失效，请联系会议组织单位" />
      </template>
    </div>

    <div class="reg-footer">
      <div class="reg-footer-platform">伊宁县委宣传部部务工作平台 V1.4.0</div>
      <div class="reg-footer-beian">
        <a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener">ICP备案号占位</a>
        <span class="footer-sep">|</span>
        <a href="https://beian.mps.gov.cn/#/query/webSearch" target="_blank" rel="noopener">公网安备号占位</a>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '../utils/request'

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
const submittedShowBack = ref(false) // 成功后显示"返回修改"

const form = ref({ unit: '' })
const personForm = ref({ attendee_name: '', attendee_title: '', phone: '' })
const editingRegId = ref(0)
const editingRegName = ref('')
const absentReason = ref('')

const meetingId = route.params.id

const loadMeeting = async () => {
  try {
    const res = await request.get(`/public/meetings/${meetingId}`)
    meeting.value = res.meeting
    units.value = res.units || []
    unitLimit.value = res.meeting.unit_limit || 1
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
    // 若单位已整体不参加：默认停在不参加态（横幅提示），但允许切回"参加"
    if (notAttendAll.value) {
      notAttend.value = 1
      absentReason.value = notAttendReason.value
    }
    // 单人模式已有报名：回显
    if (unitLimit.value === 1 && registrations.value.length > 0) {
      const rg = registrations.value[0]
      personForm.value = { attendee_name: rg.attendee_name, attendee_title: rg.attendee_title, phone: rg.phone }
    }
  } catch (e) {}
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

// 单位已不参加时点"改为参加"：切到参加态，此时不参加标记保留在后端，
// 待用户提交任一参会人员时后端自动清除不参加标记
const switchToAttend = () => {
  submitted.value = false
  notAttendAll.value = false
  notAttend.value = 0
  absentReason.value = ''
}

// 用户在切换条上切到"参加"时，若后端还留着不参加标记也视为想改参加
const watchNotAttend = (val) => {
  submitted.value = false
  if (val === 0 && notAttendAll.value) {
    notAttendAll.value = false
  }
}

// 修改某条报名
const editReg = (rg) => {
  editingRegId.value = rg.id
  editingRegName.value = rg.attendee_name
  personForm.value = { attendee_name: rg.attendee_name, attendee_title: rg.attendee_title, phone: rg.phone }
}

const cancelEditReg = () => {
  editingRegId.value = 0
  editingRegName.value = ''
  personForm.value = { attendee_name: '', attendee_title: '', phone: '' }
}

// 移除某条报名（询问确认）
const removeReg = async (rg) => {
  try {
    await ElMessageBox.confirm(`确定移除「${rg.attendee_name}」的参会报名吗？`, '移除确认', { type: 'warning', confirmButtonText: '确定移除', cancelButtonText: '取消' })
  } catch (e) { return }
  try {
    await request.post(`/public/meetings/${meetingId}/remove`, { reg_id: rg.id, unit: form.value.unit })
    ElMessage.success('已移除')
    loadUnitStatus()
  } catch (e) {}
}

// 保存人员（新增/单人确认/修改）
const savePerson = async () => {
  if (!personForm.value.attendee_name) return ElMessage.warning('请填写参会人员姓名')
  if (!personForm.value.attendee_title) return ElMessage.warning('请填写职务')
  if (!/^1[3-9]\d{9}$/.test(personForm.value.phone)) return ElMessage.warning('请输入正确的11位手机号')
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
    const res = await request.post(`/public/meetings/${meetingId}/register`, payload)
    await loadUnitStatus()
    if (editingRegId.value) {
      showSuccess('修改成功', `「${personForm.value.attendee_name}」的信息已更新`)
    } else if (unitLimit.value === 1) {
      showSuccess('报名成功', `已确认「${personForm.value.attendee_name}」参会，请准时参加。`)
    } else {
      showSuccess('添加成功', `该单位当前已报 ${registrations.value.length} 人${remain.value > 0 ? '，还可补报 ' + remain.value + ' 人' : ''}`)
    }
    if (editingRegId.value) {
      editingRegId.value = 0
      editingRegName.value = ''
    }
    personForm.value = { attendee_name: '', attendee_title: '', phone: '' }
  } catch (e) {
  } finally {
    submitting.value = false
  }
}

const showSuccess = (title, text) => {
  submittedTitle.value = title
  submittedText.value = text
  submitted.value = true
}

// 确认不参加
const saveAbsent = async () => {
  if (!absentReason.value) return ElMessage.warning('请填写不参加原因')
  submitting.value = true
  try {
    await request.post(`/public/meetings/${meetingId}/register`, {
      meeting_id: Number(meetingId), unit: form.value.unit, not_attend: 1, reason: absentReason.value
    })
    // 刷新状态：切换为"已不参加"横幅展示（notAttend=1），隐藏填写表单
    await loadUnitStatus()
    notAttend.value = 1
    showSuccess('已确认不参加', absentReason.value ? '已记录不参加原因。' : '')
  } catch (e) {
  } finally {
    submitting.value = false
  }
}

onMounted(loadMeeting)
</script>

<style scoped>
.reg-page {
  min-height: 100vh;
  background: linear-gradient(160deg, #f4f6fa 0%, #eceff5 100%);
  padding: 24px 16px 40px;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  align-items: center;
}
.reg-card {
  width: 100%;
  max-width: 560px;
  background: #fff;
  border-radius: 16px;
  box-shadow: 0 10px 40px rgba(31, 45, 61, 0.12);
  padding: 28px 24px;
  box-sizing: border-box;
}
.reg-header {
  text-align: center;
  margin-bottom: 20px;
}
.danghui {
  width: 52px;
  height: 52px;
  object-fit: contain;
  margin-bottom: 10px;
}
.meeting-title {
  font-size: 20px;
  color: #c8102e;
  margin: 0 0 10px;
  letter-spacing: 1px;
  word-break: break-all;
}
.meeting-meta {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 6px 16px;
  color: #606266;
  font-size: 14px;
  margin-bottom: 10px;
}
.meeting-content {
  color: #606266;
  font-size: 13px;
  line-height: 1.7;
  white-space: pre-wrap;
  text-align: left;
  padding: 10px 12px;
  background: #f7f8fc;
  border-radius: 8px;
}
.expired-alert {
  margin-bottom: 8px;
}
.success-alert {
  margin-bottom: 12px;
}
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
.limit-tip {
  margin-bottom: 12px;
}
.step-tip {
  color: #909399;
  font-size: 13px;
  margin-bottom: 12px;
}
.unit-select {
  margin: 8px 0;
}
.unit-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 4px 0 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid #f0f0f0;
}
.unit-name {
  font-weight: 600;
  color: #303133;
}
.attend-switch {
  text-align: center;
  margin: 12px 0 16px;
}
.notattend-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  background: #fdf6ec;
  padding: 10px 14px;
  border-radius: 8px;
  margin-bottom: 12px;
}
.regs-list {
  margin-bottom: 6px;
}
.list-title {
  font-size: 13px;
  color: #606266;
  font-weight: 600;
  margin: 10px 0 6px;
}
.add-title {
  color: #67c23a;
}
.reg-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #f7f8fc;
  border-radius: 8px;
  padding: 8px 12px;
  margin-bottom: 6px;
}
.reg-info {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.reg-info b {
  color: #303133;
}
.reg-title, .reg-phone {
  color: #909399;
  font-size: 13px;
}
.reg-actions {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}
.full-tip {
  margin: 8px 0;
}
.person-form {
  max-width: 100%;
  margin-top: 10px;
}
.edit-tip {
  color: #e6a23c;
  font-size: 13px;
  margin-bottom: 8px;
}
.absent-form {
  margin-top: 8px;
}
.form-actions {
  display: flex;
  gap: 10px;
  width: 100%;
}
.form-actions .el-button {
  flex: 1;
}
.submit-btn {
  width: 100%;
  height: 42px;
  font-size: 15px;
  letter-spacing: 4px;
}
.toast-tip {
  margin-top: 8px;
}
.loading-tip {
  text-align: center;
  color: #909399;
  padding: 40px 0;
}
.reg-footer {
  width: 100%;
  max-width: 560px;
  margin: 16px auto 0;
  text-align: center;
  font-size: 12px;
  color: #606266;
  box-sizing: border-box;
}
.reg-footer-platform {
  margin-bottom: 4px;
  color: #909399;
}
.reg-footer-beian {
  white-space: nowrap;
}
.reg-footer a {
  color: #606266;
  text-decoration: none;
}
.reg-footer .footer-sep {
  margin: 0 8px;
  color: #c0c4cc;
}

/* 响应式 */
@media (max-width: 600px) {
  .reg-page {
    padding: 12px 10px 30px;
  }
  .reg-card {
    padding: 20px 16px;
    border-radius: 12px;
  }
  .meeting-title {
    font-size: 17px;
  }
  .meeting-meta {
    font-size: 13px;
  }
  .el-form-item {
    display: block;
    margin-bottom: 14px;
  }
  .el-form-item__label {
    float: none;
    text-align: left;
    padding: 0 0 4px;
    line-height: 1.5;
  }
  .el-form-item__content {
    margin-left: 0 !important;
  }
  input, select, textarea, button {
    font-size: 16px !important;
  }
  .reg-item {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }
  .reg-actions {
    align-self: flex-end;
  }
  .reg-footer-beian {
    white-space: normal;
    line-height: 1.6;
  }
}
</style>
