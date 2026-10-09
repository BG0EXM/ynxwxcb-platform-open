<template>
  <el-drawer
    :model-value="visible"
    @update:model-value="val => emit('update:visible', val)"
    :title="`干部出勤与休假全息档案 - ${userName || profileData?.user?.real_name || '个人档案'}`"
    size="860px"
    destroy-on-close
    append-to-body
    class="attendance-profile-drawer"
    transition="el-fade-in-linear"
  >
    <template #header>
      <div class="drawer-header-bar">
        <div class="header-left">
          <span class="header-title">干部出勤与休假个人全息档案</span>
          <el-tag size="small" type="danger" effect="plain" class="header-badge">档案查验</el-tag>
        </div>
        <div class="header-right no-print">
          <span class="year-label">统计年度：</span>
          <el-select v-model="selectedYear" style="width: 110px" size="small" @change="loadProfile">
            <el-option v-for="y in yearOptions" :key="y" :label="`${y}年`" :value="y" />
          </el-select>
          <el-button type="primary" size="small" :icon="'Printer'" class="ml-8" @click="handlePrint">
            打印登记表
          </el-button>
          <el-button size="small" :icon="'Refresh'" circle class="ml-4" @click="loadProfile" />
        </div>
      </div>
    </template>

    <div v-loading="loading" class="profile-drawer-body">
      <!-- 1. 干部名片 / 基本信息条 -->
      <div class="cadre-card yx-card-gold">
        <div class="cadre-avatar">
          <div class="avatar-badge">
            {{ (profileData?.user?.real_name || userName || '干').slice(-2) }}
          </div>
        </div>
        <div class="cadre-info">
          <div class="cadre-title-row">
            <span class="cadre-name font-serif">{{ profileData?.user?.real_name || userName || '未知' }}</span>
            <el-tag size="small" type="primary" class="cadre-tag">{{ profileData?.user?.department || '宣传部' }}</el-tag>
            <el-tag size="small" :type="roleTagType(profileData?.user?.role_name)" class="cadre-tag">
              {{ profileData?.user?.role_name || '工作人员' }}
            </el-tag>
            <el-tag size="small" :type="profileData?.user?.status === 1 ? 'success' : 'danger'" class="cadre-tag">
              {{ profileData?.user?.status === 1 ? '在职在岗' : '已停用' }}
            </el-tag>
          </div>
          <div class="cadre-details-grid">
            <div class="detail-item">
              <span class="label">账号名称：</span>
              <span class="value">{{ profileData?.user?.username || '-' }}</span>
            </div>
            <div class="detail-item">
              <span class="label">联系电话：</span>
              <span class="value">{{ profileData?.user?.phone || '未填报' }}</span>
            </div>
            <div class="detail-item">
              <span class="label">档案编号：</span>
              <span class="value font-mono">CADRE-{{ String(userId || '').padStart(4, '0') }}</span>
            </div>
            <div class="detail-item">
              <span class="label">统计周期：</span>
              <span class="value">{{ selectedYear }}年度（01/01 - 12/31）</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 2. 年度核心指标统计卡（4列） -->
      <div class="core-metrics-grid">
        <!-- 2.1 年休假核定与剩余 -->
        <div class="metric-card gold-border">
          <div class="metric-card-header">
            <span class="metric-title">法定年休假</span>
            <el-tag size="small" type="success" effect="plain">额度制</el-tag>
          </div>
          <div class="metric-highlight">
            <span class="num">{{ profileData?.annual_leave?.remain_days ?? 0 }}</span>
            <span class="unit">天剩余</span>
          </div>
          <div class="metric-desc">
            核定 <b>{{ profileData?.annual_leave?.config_days ?? 0 }}</b> 天 / 已休 <b>{{ profileData?.annual_leave?.used_days ?? 0 }}</b> 天
          </div>
          <div class="metric-progress-bar">
            <el-progress
              :percentage="annualLeavePercent"
              :color="annualLeaveProgressColor"
              :show-text="false"
              :stroke-width="6"
            />
          </div>
        </div>

        <!-- 2.2 加班工时与折算补休 -->
        <div class="metric-card gold-border">
          <div class="metric-card-header">
            <span class="metric-title">加班与补休</span>
            <el-tag size="small" type="warning" effect="plain">8h/天</el-tag>
          </div>
          <div class="metric-highlight text-warning">
            <span class="num">{{ profileData?.overtime?.remain_comp_days ?? 0 }}</span>
            <span class="unit">天可补休</span>
          </div>
          <div class="metric-desc">
            年度加班 <b>{{ profileData?.overtime?.year_hours ?? 0 }}</b> 小时
          </div>
          <div class="metric-sub-desc">
            累计补休 <b>{{ profileData?.overtime?.total_comp_days ?? 0 }}</b> 天 (已休 {{ profileData?.overtime?.total_comp_used ?? 0 }} 天)
          </div>
        </div>

        <!-- 2.3 考勤实绩到会分布 -->
        <div class="metric-card gold-border">
          <div class="metric-card-header">
            <span class="metric-title">考勤到会分布</span>
            <el-tag size="small" type="info" effect="plain">点到台账</el-tag>
          </div>
          <div class="metric-highlight text-primary">
            <span class="num">{{ profileData?.attendance?.present ?? 0 }}</span>
            <span class="unit">天出勤</span>
          </div>
          <div class="attendance-badges">
            <span class="att-badge att-present" title="正常出勤">出勤 {{ profileData?.attendance?.present ?? 0 }}</span>
            <span class="att-badge att-trip" title="外出出差">出差 {{ profileData?.attendance?.trip ?? 0 }}</span>
            <span class="att-badge att-training" title="参训学习">培训 {{ profileData?.attendance?.training ?? 0 }}</span>
            <span class="att-badge att-late" title="考勤迟到">迟到 {{ profileData?.attendance?.late ?? 0 }}</span>
            <span class="att-badge att-leave" title="请假记录">请假 {{ profileData?.attendance?.leave ?? 0 }}</span>
          </div>
        </div>

        <!-- 2.4 年度请假总天数 -->
        <div class="metric-card gold-border">
          <div class="metric-card-header">
            <span class="metric-title">年度请假总览</span>
            <el-tag size="small" type="danger" effect="plain">全流程批假</el-tag>
          </div>
          <div class="metric-highlight text-danger">
            <span class="num">{{ profileData?.total_leave_days ?? 0 }}</span>
            <span class="unit">天总休假</span>
          </div>
          <div class="metric-desc">
            批准流水 <b>{{ profileData?.leave_records?.length ?? 0 }}</b> 笔
          </div>
          <div class="metric-sub-desc">
            请假总计包含当年各类法定假期
          </div>
        </div>
      </div>

      <!-- 3. 各类法定假明细分布（11 类） -->
      <div class="section-card">
        <div class="section-header">
          <div class="section-title">
            <el-icon class="mr-4"><Calendar /></el-icon>
            各类法定假明细分布（11 类法定假）
          </div>
          <div class="section-tip">单位：天（含已审批生效各假别统计）</div>
        </div>
        <div class="leave-types-grid">
          <div
            v-for="item in leaveSummaryList"
            :key="item.type"
            :class="['leave-type-chip', { 'has-days': item.days > 0 }]"
          >
            <div class="chip-name">{{ item.label }}</div>
            <div class="chip-days">
              <b>{{ item.days }}</b>
              <span class="chip-unit">天</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 4. 年度请假台账流水表格 -->
      <div class="section-card">
        <div class="section-header">
          <div class="section-title">
            <el-icon class="mr-4"><Document /></el-icon>
            年度请假台账流水（{{ selectedYear }}年度）
          </div>
          <div class="section-tip">已按起止日期倒序排列</div>
        </div>

        <el-table
          :data="profileData?.leave_records || []"
          stripe
          border
          size="small"
          class="leave-records-table"
          empty-text="当年暂无已审批的请假流水记录"
        >
          <el-table-column type="index" label="序号" width="55" align="center" />
          <el-table-column prop="leave_type" label="假种类型" width="100" align="center">
            <template #default="{ row }">
              <el-tag size="small" :type="leaveTagType(row.leave_type)">
                {{ leaveLabel(row.leave_type) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="起止时间" width="200" align="center">
            <template #default="{ row }">
              <span class="font-mono text-sm">{{ row.start_date }} 至 {{ row.end_date }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="days" label="请假天数" width="90" align="center">
            <template #default="{ row }">
              <span class="days-pill">{{ row.days }} 天</span>
            </template>
          </el-table-column>
          <el-table-column prop="reason" label="请假事由" min-width="180" show-overflow-tooltip>
            <template #default="{ row }">
              <span>{{ row.reason || '未填写事由' }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="created_at" label="登记时间" width="150" align="center">
            <template #default="{ row }">
              <span class="text-secondary text-xs">{{ formatTime(row.created_at) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="85" align="center">
            <template #default>
              <el-tag size="small" type="success">已批准</el-tag>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <!-- 5. 打印排版模版（支持预览与标准公文打印） -->
      <div class="section-card print-preview-card no-print">
        <div class="section-header">
          <div class="section-title">
            <el-icon class="mr-4"><Printer /></el-icon>
            公文打印登记表预览
          </div>
          <div>
            <el-button type="primary" size="small" :icon="'Printer'" @click="handlePrint">
              立即打印《干部职工年度出勤与休假登记表》
            </el-button>
          </div>
        </div>
        <div class="print-instruction">
          说明：点击上方按钮将调用浏览器打印引擎，系统已自动配置红头公文样式、标准 A4 黑白表格排版及会签栏。
        </div>
      </div>

      <!-- 标准 A4 打印区域 (纯公文排版, 在普通界面折叠展示或供打印捕获) -->
      <div id="attendance-profile-print-area" class="attendance-profile-print-sheet">
        <div class="print-redhead">
          <div class="redhead-title">中共伊宁县委宣传部</div>
          <div class="redhead-line"></div>
        </div>

        <div class="print-doc-title">
          干部职工年度出勤与休假登记表
        </div>

        <div class="print-meta-bar">
          <span><b>姓名：</b>{{ profileData?.user?.real_name || userName || '未填' }}</span>
          <span><b>所属科室：</b>{{ profileData?.user?.department || '宣传部' }}</span>
          <span><b>统计年度：</b>{{ selectedYear }} 年度</span>
          <span><b>制表日期：</b>{{ todayStr }}</span>
        </div>

        <table class="print-gov-table">
          <tbody>
            <tr>
              <th class="tbl-head" style="width: 14%;">干部姓名</th>
              <td style="width: 20%;">{{ profileData?.user?.real_name || userName }}</td>
              <th class="tbl-head" style="width: 14%;">所属科室</th>
              <td style="width: 20%;">{{ profileData?.user?.department || '宣传部' }}</td>
              <th class="tbl-head" style="width: 14%;">系统角色</th>
              <td style="width: 18%;">{{ profileData?.user?.role_name || '工作人员' }}</td>
            </tr>
            <tr>
              <th class="tbl-head">联系电话</th>
              <td>{{ profileData?.user?.phone || '未登记' }}</td>
              <th class="tbl-head">年休假核定</th>
              <td>核定 {{ profileData?.annual_leave?.config_days ?? 0 }} 天 (已休 {{ profileData?.annual_leave?.used_days ?? 0 }} 天)</td>
              <th class="tbl-head">年休假剩余</th>
              <td><b>{{ profileData?.annual_leave?.remain_days ?? 0 }} 天</b></td>
            </tr>
            <tr>
              <th class="tbl-head">加班与补休</th>
              <td colspan="5">
                当年累计加班工时：{{ profileData?.overtime?.year_hours ?? 0 }} 小时；
                折算累计可补休：{{ profileData?.overtime?.total_comp_days ?? 0 }} 天；
                累计已用补休：{{ profileData?.overtime?.total_comp_used ?? 0 }} 天；
                <b>目前剩余可用补休：{{ profileData?.overtime?.remain_comp_days ?? 0 }} 天</b>。
              </td>
            </tr>
            <tr>
              <th class="tbl-head">考勤点到实绩</th>
              <td colspan="5">
                正常出勤：{{ profileData?.attendance?.present ?? 0 }} 天；
                外出出差：{{ profileData?.attendance?.trip ?? 0 }} 天；
                迟到记录：{{ profileData?.attendance?.late ?? 0 }} 次；
                培训学习：{{ profileData?.attendance?.training ?? 0 }} 天；
                未到记录：{{ profileData?.attendance?.absent ?? 0 }} 天；
                年度请假总计：<b>{{ profileData?.total_leave_days ?? 0 }} 天</b>。
              </td>
            </tr>
            <tr>
              <th class="tbl-head" rowspan="2">法定假统计<br/>(11类假种)</th>
              <td colspan="5" class="p-0">
                <table class="nested-mini-table">
                  <thead>
                    <tr>
                      <th>年假</th>
                      <th>病假</th>
                      <th>事假</th>
                      <th>婚假</th>
                      <th>产假</th>
                      <th>丧假</th>
                      <th>产检假</th>
                      <th>探亲假</th>
                      <th>培训假</th>
                      <th>补休</th>
                      <th>其他</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr>
                      <td v-for="lt in leaveSummaryList" :key="lt.type">
                        {{ lt.days }}
                      </td>
                    </tr>
                  </tbody>
                </table>
              </td>
            </tr>
            <tr>
              <td colspan="5" class="text-xs text-muted" style="padding: 4px 8px;">
                注：各项假期天数均按自然年内实际批准天数核算，单位为自然日/工作日折算。
              </td>
            </tr>
            <tr>
              <th class="tbl-head">
                年度请假明细<br/>
                流水台账
              </th>
              <td colspan="5" class="p-0">
                <table class="nested-flow-table">
                  <thead>
                    <tr>
                      <th style="width: 36px;">序号</th>
                      <th style="width: 64px;">类别</th>
                      <th style="width: 140px;">起止时间</th>
                      <th style="width: 48px;">天数</th>
                      <th>请假事由</th>
                      <th style="width: 80px;">审批状态</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="(rec, idx) in (profileData?.leave_records || []).slice(0, 10)" :key="rec.id">
                      <td>{{ idx + 1 }}</td>
                      <td>{{ leaveLabel(rec.leave_type) }}</td>
                      <td>{{ rec.start_date }} ~ {{ rec.end_date }}</td>
                      <td>{{ rec.days }}天</td>
                      <td class="text-left">{{ rec.reason || '-' }}</td>
                      <td>已批准</td>
                    </tr>
                    <tr v-if="!profileData?.leave_records || profileData?.leave_records.length === 0">
                      <td colspan="6" class="text-center text-muted" style="padding: 10px;">当年无请假记录</td>
                    </tr>
                    <tr v-if="(profileData?.leave_records || []).length > 10">
                      <td colspan="6" class="text-center text-muted" style="padding: 4px;">
                        (余下 {{ profileData.leave_records.length - 10 }} 笔记录略，详见电子台账)
                      </td>
                    </tr>
                  </tbody>
                </table>
              </td>
            </tr>
            <!-- 签批审批与归档栏 -->
            <tr>
              <th class="tbl-head">干部本人确认</th>
              <td colspan="2" class="sign-td">
                <div class="sign-placeholder">本人签字：</div>
                <div class="sign-date-placeholder">年&nbsp;&nbsp;&nbsp;&nbsp;月&nbsp;&nbsp;&nbsp;&nbsp;日</div>
              </td>
              <th class="tbl-head">科室负责人审核</th>
              <td colspan="2" class="sign-td">
                <div class="sign-placeholder">签名：</div>
                <div class="sign-date-placeholder">年&nbsp;&nbsp;&nbsp;&nbsp;月&nbsp;&nbsp;&nbsp;&nbsp;日</div>
              </td>
            </tr>
            <tr>
              <th class="tbl-head">分管领导审签</th>
              <td colspan="2" class="sign-td">
                <div class="sign-placeholder">签名：</div>
                <div class="sign-date-placeholder">年&nbsp;&nbsp;&nbsp;&nbsp;月&nbsp;&nbsp;&nbsp;&nbsp;日</div>
              </td>
              <th class="tbl-head">办公室考勤备档</th>
              <td colspan="2" class="sign-td">
                <div class="sign-placeholder">专管员签章：</div>
                <div class="sign-date-placeholder">年&nbsp;&nbsp;&nbsp;&nbsp;月&nbsp;&nbsp;&nbsp;&nbsp;日</div>
              </td>
            </tr>
          </tbody>
        </table>

        <div class="print-footer-info">
          <span>单位：中共伊宁县委宣传部办公室</span>
          <span>系统依据：伊宁县委宣传部部务工作平台V1.7.0</span>
          <span>打印存档专用</span>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="drawer-footer-row no-print">
        <el-button @click="emit('update:visible', false)">关 闭</el-button>
        <el-button type="primary" :icon="'Printer'" @click="handlePrint">打印出勤登记表</el-button>
      </div>
    </template>
  </el-drawer>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../utils/request'
import dayjs from 'dayjs'

const props = defineProps({
  visible: {
    type: Boolean,
    default: false
  },
  userId: {
    type: [Number, String],
    default: ''
  },
  userName: {
    type: String,
    default: ''
  }
})

const emit = defineEmits(['update:visible'])

const currentYearNum = new Date().getFullYear()
const selectedYear = ref(String(currentYearNum))
const yearOptions = computed(() => {
  return [currentYearNum - 2, currentYearNum - 1, currentYearNum, currentYearNum + 1].map(String)
})

const loading = ref(false)
const profileData = ref(null)

const todayStr = computed(() => dayjs().format('YYYY年MM月DD日'))

const leaveTypeLabels = {
  annual: '年假',
  sick: '病假',
  personal: '事假',
  marriage: '婚假',
  maternity: '产假',
  bereavement: '丧假',
  prenatal: '产检假',
  family: '探亲假',
  training: '培训假',
  comp: '补休',
  other: '其他'
}

const leaveSummaryList = computed(() => {
  if (profileData.value && Array.isArray(profileData.value.leave_summary) && profileData.value.leave_summary.length > 0) {
    return profileData.value.leave_summary
  }
  const defaultKeys = ['annual', 'sick', 'personal', 'marriage', 'maternity', 'bereavement', 'prenatal', 'family', 'training', 'comp', 'other']
  return defaultKeys.map(k => ({
    type: k,
    label: leaveTypeLabels[k] || k,
    days: 0
  }))
})

const annualLeavePercent = computed(() => {
  const config = profileData.value?.annual_leave?.config_days || 0
  const used = profileData.value?.annual_leave?.used_days || 0
  if (config <= 0) return 0
  const p = Math.round((used / config) * 100)
  return p > 100 ? 100 : p
})

const annualLeaveProgressColor = computed(() => {
  const p = annualLeavePercent.value
  if (p >= 80) return '#e6a23c'
  return '#1890ff'
})

const roleTagType = (role) => {
  if (!role) return 'info'
  if (role.includes('管理')) return 'danger'
  if (role.includes('部长') || role.includes('主任')) return 'warning'
  return 'primary'
}

const leaveLabel = (type) => {
  return leaveTypeLabels[type] || type || '请假'
}

const leaveTagType = (type) => {
  switch (type) {
    case 'annual': return 'primary'
    case 'sick': return 'warning'
    case 'personal': return 'info'
    case 'marriage': return 'danger'
    case 'maternity': return 'success'
    case 'comp': return 'warning'
    case 'training': return 'success'
    default: return 'info'
  }
}

const formatTime = (t) => {
  if (!t) return '-'
  return dayjs(t).format('YYYY-MM-DD HH:mm')
}

const loadProfile = async () => {
  if (!props.userId) return
  loading.value = true
  try {
    const res = await request.get(`/users/${props.userId}/attendance-profile`, {
      params: { year: selectedYear.value }
    })
    profileData.value = res
  } catch (e) {
    // 错误已被 axios 拦截器提示
  } finally {
    loading.value = false
  }
}

watch(
  [() => props.visible, () => props.userId],
  ([visible, uid]) => {
    if (visible && uid) {
      loadProfile()
    } else if (!visible) {
      profileData.value = null
    }
  },
  { immediate: true }
)

const handlePrint = () => {
  window.print()
}
</script>

<style scoped>
.drawer-header-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding-right: 12px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.header-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--yx-text-1, #1f2329);
}

.header-badge {
  font-weight: 500;
}

.header-right {
  display: flex;
  align-items: center;
}

.year-label {
  font-size: 13px;
  color: var(--yx-text-2, #4e5969);
  margin-right: 4px;
}

.profile-drawer-body {
  padding: 4px 6px 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* 1. 干部名片 */
.cadre-card {
  display: flex;
  align-items: center;
  padding: 16px 20px;
  background: linear-gradient(135deg, #ffffff 0%, #faf8f5 100%);
  border: 1px solid rgba(184, 146, 90, 0.4);
  border-radius: var(--yx-radius, 8px);
  box-shadow: 0 2px 10px rgba(184, 146, 90, 0.08);
  gap: 18px;
}

.cadre-avatar {
  flex-shrink: 0;
}

.avatar-badge {
  width: 58px;
  height: 58px;
  border-radius: 50%;
  background: linear-gradient(135deg, #9e1b1e 0%, #7a1417 100%);
  border: 2px solid #e3cfa6;
  color: #fff;
  font-size: 19px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 2px 8px rgba(158, 27, 30, 0.25);
  font-family: var(--yx-font-serif, serif);
  letter-spacing: 1px;
}

.cadre-info {
  flex: 1;
}

.cadre-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}

.cadre-name {
  font-size: 22px;
  font-weight: 700;
  color: #1b2433;
  letter-spacing: 1px;
}

.cadre-details-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 6px 20px;
  font-size: 13px;
}

.detail-item {
  display: flex;
  align-items: center;
}

.detail-item .label {
  color: #6b7785;
  width: 70px;
  flex-shrink: 0;
}

.detail-item .value {
  color: #1f2329;
  font-weight: 500;
}

/* 2. 核心指标统计卡 */
.core-metrics-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
}

.metric-card {
  background: #ffffff;
  border-radius: 8px;
  padding: 14px 14px 12px;
  border: 1px solid #e6e8eb;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.03);
  display: flex;
  flex-direction: column;
  transition: all 0.2s ease;
}

.metric-card.gold-border {
  border-top: 3px solid #b8925a;
}

.metric-card:hover {
  box-shadow: 0 4px 12px rgba(27, 36, 51, 0.08);
  transform: translateY(-1px);
}

.metric-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.metric-title {
  font-size: 13px;
  font-weight: 600;
  color: #4e5969;
}

.metric-highlight {
  display: flex;
  align-items: baseline;
  gap: 4px;
  margin-bottom: 6px;
}

.metric-highlight .num {
  font-size: 26px;
  font-weight: 700;
  line-height: 1;
  color: #1b2433;
  font-family: var(--yx-font-mono, monospace);
}

.metric-highlight .unit {
  font-size: 12px;
  color: #8a94a0;
}

.metric-desc {
  font-size: 12px;
  color: #4e5969;
  margin-bottom: 4px;
  line-height: 1.4;
}

.metric-sub-desc {
  font-size: 11px;
  color: #8a94a0;
  line-height: 1.3;
}

.metric-progress-bar {
  margin-top: 6px;
}

.attendance-badges {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 4px;
}

.att-badge {
  font-size: 11px;
  padding: 1px 5px;
  border-radius: 4px;
  font-weight: 500;
}

.att-present { background: #e8f5e9; color: #2e7d32; }
.att-trip { background: #e3f2fd; color: #1565c0; }
.att-training { background: #ede7f6; color: #512da8; }
.att-late { background: #fff3e0; color: #e65100; }
.att-leave { background: #ffebee; color: #c62828; }

.text-warning { color: #d97706 !important; }
.text-primary { color: #24406e !important; }
.text-danger { color: #9e1b1e !important; }

/* 3 & 4. 章节容器卡片 */
.section-card {
  background: #ffffff;
  border-radius: 8px;
  border: 1px solid #e6e8eb;
  padding: 14px 16px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.02);
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.section-title {
  font-size: 14px;
  font-weight: 700;
  color: #1f2329;
  display: flex;
  align-items: center;
}

.section-tip {
  font-size: 12px;
  color: #8a94a0;
}

/* 3. 各类法定假明细网格 (11类) */
.leave-types-grid {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 8px;
}

.leave-type-chip {
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 6px;
  padding: 8px 10px;
  text-align: center;
  transition: all 0.2s ease;
}

.leave-type-chip.has-days {
  background: #fbf6ec;
  border-color: #e3cfa6;
  box-shadow: 0 1px 4px rgba(184, 146, 90, 0.12);
}

.leave-type-chip.has-days .chip-name {
  color: #8c6a3a;
  font-weight: 600;
}

.leave-type-chip.has-days .chip-days b {
  color: #9e1b1e;
  font-size: 16px;
}

.chip-name {
  font-size: 12px;
  color: #6b7785;
  margin-bottom: 4px;
}

.chip-days {
  font-size: 14px;
  color: #1f2329;
}

.chip-days b {
  font-size: 15px;
  font-family: var(--yx-font-mono, monospace);
}

.chip-unit {
  font-size: 11px;
  color: #8a94a0;
  margin-left: 2px;
}

/* 4. 流水表格 */
.leave-records-table {
  width: 100%;
}

.days-pill {
  font-weight: 600;
  color: #24406e;
}

.print-preview-card {
  background: #faf8f5;
  border-color: #e3cfa6;
}

.print-instruction {
  font-size: 12px;
  color: #6b7785;
  line-height: 1.5;
}

/* 5. 打印专用公文排版模版 */
.attendance-profile-print-sheet {
  background: #ffffff;
  border: 1px solid #dcdfe5;
  border-radius: 6px;
  padding: 24px 28px;
  margin-top: 8px;
}

.print-redhead {
  text-align: center;
  margin-bottom: 12px;
}

.redhead-title {
  font-size: 22px;
  font-weight: 700;
  color: #c00000;
  font-family: var(--yx-font-serif, "SimSun", serif);
  letter-spacing: 4px;
}

.redhead-line {
  height: 2px;
  background-color: #c00000;
  margin-top: 8px;
}

.print-doc-title {
  text-align: center;
  font-size: 18px;
  font-weight: 700;
  color: #000;
  letter-spacing: 2px;
  margin: 14px 0 10px;
  font-family: 'SimSun', 'Songti SC', serif !important;
}

.print-meta-bar {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: #333;
  margin-bottom: 8px;
  padding: 0 4px;
}

.print-gov-table {
  width: 100%;
  border-collapse: collapse;
  border: 1px solid #000;
  font-size: 12px;
}

.print-gov-table th,
.print-gov-table td {
  border: 1px solid #000;
  padding: 6px 8px;
  vertical-align: middle;
}

.print-gov-table th.tbl-head {
  background: #f7f7f7;
  font-weight: 600;
  text-align: center;
  color: #000;
}

.nested-mini-table {
  width: 100%;
  border-collapse: collapse;
  text-align: center;
  font-size: 11px;
}

.nested-mini-table th,
.nested-mini-table td {
  border: 1px solid #333;
  padding: 4px 2px;
}

.nested-mini-table th {
  background: #fafafa;
  font-weight: 600;
}

.nested-flow-table {
  width: 100%;
  border-collapse: collapse;
  text-align: center;
  font-size: 11px;
}

.nested-flow-table th,
.nested-flow-table td {
  border: 1px solid #333;
  padding: 4px 4px;
}

.nested-flow-table th {
  background: #fafafa;
}

.p-0 {
  padding: 0 !important;
}

.text-left {
  text-align: left;
}

.text-center {
  text-align: center;
}

.sign-td {
  height: 60px;
  vertical-align: top;
  position: relative;
  padding: 6px 8px;
}

.sign-placeholder {
  font-size: 12px;
  color: #333;
}

.sign-date-placeholder {
  position: absolute;
  right: 12px;
  bottom: 6px;
  font-size: 11px;
  color: #666;
}

.print-footer-info {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  color: #666;
  margin-top: 10px;
  padding: 0 4px;
}

.drawer-footer-row {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.ml-8 { margin-left: 8px; }
.ml-4 { margin-left: 4px; }
.mr-4 { margin-right: 4px; }
.text-xs { font-size: 12px; }
.text-sm { font-size: 13px; }
.text-secondary { color: #8a94a0; }
.text-muted { color: #909399; }

/* 移动端响应式适配 */
@media (max-width: 768px) {
  .drawer-header-bar {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
  .drawer-header-bar .header-right {
    width: 100%;
    justify-content: flex-start;
    flex-wrap: wrap;
    gap: 6px;
  }
  .cadre-card {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }
  .cadre-details-grid {
    grid-template-columns: 1fr;
    gap: 6px;
  }
  .core-metrics-grid {
    grid-template-columns: repeat(2, 1fr);
    gap: 8px;
  }
  .leave-types-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}

@media (max-width: 480px) {
  .core-metrics-grid {
    grid-template-columns: 1fr;
  }
  .leave-types-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

/* 打印样式适配 */
@media print {
  @page {
    size: A4 portrait;
    margin: 12mm 10mm;
  }

  body {
    background: #ffffff !important;
  }

  body * {
    visibility: hidden !important;
  }

  .attendance-profile-print-sheet,
  .attendance-profile-print-sheet * {
    visibility: visible !important;
  }

  .attendance-profile-print-sheet {
    position: fixed !important;
    left: 0 !important;
    top: 0 !important;
    width: 100% !important;
    margin: 0 !important;
    padding: 0 !important;
    border: none !important;
    background: #ffffff !important;
    z-index: 999999 !important;
    display: block !important;
  }

  .no-print {
    display: none !important;
  }
}
</style>
