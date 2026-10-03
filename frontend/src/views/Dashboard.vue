<template>
  <div class="dashboard-container">
    <!-- 1. 顶部欢迎横幅（高大上政务 Banner） -->
    <div class="welcome-banner">
      <!-- 天山巍峨雪峰金色剪影与山脉暗纹 -->
      <div class="banner-mountain-wrap">
        <svg class="tianshan-svg" viewBox="0 0 960 140" fill="none" xmlns="http://www.w3.org/2000/svg" preserveAspectRatio="none">
          <defs>
            <linearGradient id="goldMountainFar" x1="0" y1="0" x2="0" y2="140" gradientUnits="userSpaceOnUse">
              <stop offset="0%" stop-color="#E3CFA6" stop-opacity="0.32" />
              <stop offset="100%" stop-color="#E3CFA6" stop-opacity="0.04" />
            </linearGradient>
            <linearGradient id="goldMountainNear" x1="0" y1="0" x2="0" y2="140" gradientUnits="userSpaceOnUse">
              <stop offset="0%" stop-color="#B8925A" stop-opacity="0.45" />
              <stop offset="100%" stop-color="#B8925A" stop-opacity="0.08" />
            </linearGradient>
          </defs>
          
          <!-- 远景：天山主峰与连绵雪山 -->
          <path d="M80 140 L260 55 L350 90 L480 20 L560 70 L660 35 L760 85 L850 48 L960 95 L960 140 Z" 
                fill="url(#goldMountainFar)" 
                stroke="rgba(227, 207, 166, 0.6)" 
                stroke-width="1.8" />
          
          <!-- 雪峰山脊与雪线刻画 -->
          <path d="M480 20 L465 65 L480 100 L475 140" stroke="rgba(227, 207, 166, 0.55)" stroke-width="1.4" stroke-dasharray="4 2" />
          <path d="M660 35 L648 80 L660 115 L655 140" stroke="rgba(227, 207, 166, 0.5)" stroke-width="1.4" stroke-dasharray="4 2" />
          <path d="M260 55 L272 90 L270 140" stroke="rgba(227, 207, 166, 0.45)" stroke-width="1.2" />
          <path d="M850 48 L840 85 L852 140" stroke="rgba(227, 207, 166, 0.45)" stroke-width="1.2" />

          <!-- 近景：伊宁山麓起伏山峦 -->
          <path d="M0 140 L140 85 L240 115 L360 65 L450 95 L570 50 L680 88 L790 60 L890 92 L960 75 L960 140 Z" 
                fill="url(#goldMountainNear)" 
                stroke="rgba(184, 146, 90, 0.7)" 
                stroke-width="2" />
        </svg>
      </div>
      <div class="banner-content">
        <div class="banner-greeting">
          <h1 class="greeting-title font-serif">
            {{ greetingText }}，{{ currentUserName }}同志
          </h1>
          <p class="greeting-subtitle">
            今天是 {{ todayFullDate }}，坚守宣传思想文化阵地，凝心聚力谱新篇。
          </p>
        </div>

        <!-- 横幅右侧：今日聚焦胶囊 -->
        <div class="banner-focus-group hidden-xs">
          <div 
            class="focus-pill" 
            v-if="stats.today_duty"
            @click="$router.push('/duty')"
          >
            <div class="focus-pill-icon"><el-icon><AlarmClock /></el-icon></div>
            <div class="focus-pill-info">
              <span class="focus-pill-label">今日值班</span>
              <span class="focus-pill-val">
                {{ stats.today_duty }}
                <small v-if="stats.today_duty_dawangyuan === 1" class="dawang-tag">大院</small>
              </span>
            </div>
          </div>

          <div 
            class="focus-pill" 
            @click="$router.push('/incoming')"
            v-if="authStore.hasPerm('incoming.view')"
          >
            <div class="focus-pill-icon"><el-icon><FolderOpened /></el-icon></div>
            <div class="focus-pill-info">
              <span class="focus-pill-label">待办收文</span>
              <span class="focus-pill-val tabular-nums">{{ stats.pending_incoming || 0 }} 份</span>
            </div>
          </div>

          <div 
            class="focus-pill" 
            @click="$router.push('/vehicles')"
            v-if="authStore.hasPerm('vehicle.view')"
          >
            <div class="focus-pill-icon"><el-icon><Van /></el-icon></div>
            <div class="focus-pill-info">
              <span class="focus-pill-label">今日派车</span>
              <span class="focus-pill-val tabular-nums">{{ stats.today_vehicle || 0 }} 辆</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 2. 核心指标卡片 -->
    <el-row :gutter="16" class="mt-20">
      <el-col :xs="24" :sm="12" :md="6" v-for="c in statCards" :key="c.label">
        <div 
          class="stat-box yx-card-crest" 
          :class="{ 'is-clickable': !!c.path }"
          @click="c.path && $router.push(c.path)"
        >
          <div class="stat-box-left">
            <span class="stat-box-label">{{ c.label }}</span>
            <div class="stat-box-val-wrap">
              <span class="stat-box-val tabular-nums">{{ c.value }}</span>
              <span class="stat-box-unit">{{ c.unit }}</span>
            </div>
            <span class="stat-box-desc">{{ c.desc }}</span>
          </div>
          <div class="stat-box-icon" :style="{ color: c.color, background: c.bg }">
            <el-icon :size="22"><component :is="c.icon" /></el-icon>
          </div>
        </div>
      </el-col>
    </el-row>

    <!-- 3. 主体工作区：左 16 列（最新收文 + 本周排班），右 8 列（今日值班 + 我的考勤 + 快捷操作） -->
    <el-row :gutter="16" class="mt-20">
      <!-- 左侧：核心业务流 -->
      <el-col :xs="24" :md="16">
        <!-- 最新收文卡片 -->
        <div class="panel-card">
          <div class="panel-header">
            <div class="panel-title-wrap">
              <span class="panel-title-bar"></span>
              <h3 class="panel-title font-serif">待办与最新收文</h3>
            </div>
            <el-button 
              link 
              type="primary" 
              @click="$router.push('/incoming')"
              v-if="authStore.hasPerm('incoming.view')"
            >
              全部公文 <el-icon class="ml-4"><ArrowRight /></el-icon>
            </el-button>
          </div>

          <div class="panel-body">
            <el-table 
              :data="latestIncoming" 
              :header-cell-style="{ background: 'var(--el-fill-color-light)', color: 'var(--yx-text-2)', fontWeight: '600' }"
              empty-text="暂无收文记录"
              class="incoming-table"
            >
              <el-table-column prop="status" label="状态" width="90">
                <template #default="{ row }">
                  <status-dot 
                    v-if="row.status < 5" 
                    type="warning" 
                    text="待办" 
                    pulse 
                  />
                  <status-dot 
                    v-else 
                    type="success" 
                    text="已办" 
                  />
                </template>
              </el-table-column>
              <el-table-column prop="received_date" label="收文日期" width="115" />
              <el-table-column prop="from_unit" label="来文单位" width="140" show-overflow-tooltip />
              <el-table-column prop="title" label="文件标题" min-width="200" show-overflow-tooltip>
                <template #default="{ row }">
                  <span class="doc-title" @click="$router.push('/incoming')">{{ row.title }}</span>
                </template>
              </el-table-column>
              <el-table-column label="操作" width="85" align="right">
                <template #default="{ row }">
                  <el-button 
                    link 
                    type="primary" 
                    @click="$router.push('/incoming')"
                  >
                    办理
                  </el-button>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </div>

        <!-- 本周排班 -->
        <div class="panel-card mt-20">
          <div class="panel-header">
            <div class="panel-title-wrap">
              <span class="panel-title-bar"></span>
              <h3 class="panel-title font-serif">本周部务值守安排</h3>
            </div>
            <el-button 
              link 
              type="primary" 
              @click="$router.push('/duty')"
              v-if="authStore.hasPerm('duty.view')"
            >
              完整排班表 <el-icon class="ml-4"><ArrowRight /></el-icon>
            </el-button>
          </div>

          <div class="panel-body">
            <div class="duty-week-grid" v-if="weekDuty.length">
              <div 
                v-for="d in weekDuty" 
                :key="d.duty_date"
                class="duty-week-item"
                :class="{ 'is-today': d.duty_date === todayStr }"
              >
                <div class="duty-week-badge" v-if="d.duty_date === todayStr">今日值守</div>
                <div class="duty-week-date">{{ formatDutyDate(d.duty_date) }}</div>
                <div class="duty-week-name font-serif">{{ d.user_name || '未排班' }}</div>
                <div class="duty-week-tag-wrap">
                  <span v-if="d.is_dawangyuan === 1" class="dawang-tag-sm">大院</span>
                  <span v-else class="normal-tag-sm">机关</span>
                </div>
              </div>
            </div>
            <el-empty v-else description="本周暂无排班记录" :image-size="60" />
          </div>
        </div>

        <!-- 近半年业务趋势 -->
        <div class="panel-card mt-20" v-if="trendSeries.length">
          <div class="panel-header">
            <div class="panel-title-wrap">
              <span class="panel-title-bar"></span>
              <h3 class="panel-title font-serif">近半年业务趋势</h3>
            </div>
            <span class="trend-sub">按月统计</span>
          </div>
          <div class="panel-body">
            <TrendChart :labels="trendLabels" :full-labels="trendFullLabels" :series="trendSeries" />
          </div>
        </div>
      </el-col>

      <!-- 右侧：值班、考勤与快捷功能 -->
      <el-col :xs="24" :md="8">
        <!-- 今日值班信息卡片 -->
        <div class="panel-card">
          <div class="panel-header">
            <div class="panel-title-wrap">
              <span class="panel-title-bar"></span>
              <h3 class="panel-title font-serif">今日值班干部</h3>
            </div>
          </div>
          <div class="panel-body">
            <div class="duty-card-content" v-if="stats.today_duty">
              <div class="duty-avatar-box">
                <el-icon :size="28"><UserFilled /></el-icon>
              </div>
              <div class="duty-details">
                <div class="duty-name-row">
                  <span class="duty-person font-serif">{{ stats.today_duty }}</span>
                  <span v-if="stats.today_duty_dawangyuan === 1" class="dawang-pill">县委大院值守</span>
                </div>
                <p class="duty-time-rule">
                  值守要求：值守至 21:00 收文，若当日无待收公文可提前离岗。
                </p>
              </div>
            </div>
            <el-empty v-else description="今日暂未设置值班安排" :image-size="60" />
          </div>
        </div>

        <!-- 我的考勤概况 -->
        <div class="panel-card mt-20">
          <div class="panel-header">
            <div class="panel-title-wrap">
              <span class="panel-title-bar"></span>
              <h3 class="panel-title font-serif">个人考勤休假</h3>
            </div>
            <el-button 
              link 
              type="primary" 
              @click="$router.push('/attendance')"
              v-if="authStore.hasPerm('attendance.view')"
            >
              考勤详情
            </el-button>
          </div>
          <div class="panel-body">
            <div class="att-status-box">
              <div class="att-status-left">
                <span class="att-label">今日考勤打卡</span>
                <span class="att-state" :class="`att-state-${attStatusType}`">
                  {{ attStatusText }}
                </span>
              </div>
              <el-button 
                size="small" 
                type="primary" 
                plain
                @click="$router.push('/attendance')"
                v-if="authStore.hasPerm('attendance.mark')"
              >
                点到打卡
              </el-button>
            </div>

            <div class="att-data-row mt-16">
              <div class="att-data-cell">
                <span class="att-data-title">本月出勤</span>
                <span class="att-data-num tabular-nums">{{ stats.month_present || 0 }} <small>天</small></span>
              </div>
              <div class="att-data-divider"></div>
              <div class="att-data-cell">
                <span class="att-data-title">本月休假</span>
                <span class="att-data-num tabular-nums">{{ stats.month_leave || 0 }} <small>天</small></span>
              </div>
            </div>
          </div>
        </div>

        <!-- 快捷操作入口 -->
        <div class="panel-card mt-20" v-if="quickActions.length">
          <div class="panel-header">
            <div class="panel-title-wrap">
              <span class="panel-title-bar"></span>
              <h3 class="panel-title font-serif">常用业务办理</h3>
            </div>
          </div>
          <div class="panel-body">
            <div class="quick-grid">
              <div 
                v-for="act in quickActions" 
                :key="act.label"
                class="quick-item"
                @click="act.path && $router.push(act.path)"
              >
                <div class="quick-icon-wrap" :style="{ color: act.color, background: act.bg }">
                  <el-icon :size="20"><component :is="act.icon" /></el-icon>
                </div>
                <span class="quick-label">{{ act.label }}</span>
              </div>
            </div>
          </div>
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useAuthStore } from '../store/auth'
import request from '../utils/request'
import dayjs from 'dayjs'
import StatusDot from '../components/StatusDot.vue'
import TrendChart from '../components/TrendChart.vue'

const authStore = useAuthStore()

const stats = ref({})
const latestIncoming = ref([])
const weekDuty = ref([])

const trendMonths = computed(() => stats.value.trend?.months || [])
const trendLabels = computed(() => trendMonths.value.map(m => `${Number(m.slice(5))}月`))
const trendFullLabels = computed(() => trendMonths.value.map(m => `${m.slice(0, 4)}年${Number(m.slice(5))}月`))
const trendSeries = computed(() => {
  const t = stats.value.trend
  if (!t) return []
  const list = []
  if (authStore.hasPerm('incoming.view')) list.push({ name: '收文登记', color: '#9e1b1e', data: t.incoming || [] })
  if (authStore.hasPerm('vehicle.view')) list.push({ name: '公车出行', color: '#24406e', data: t.vehicle || [] })
  return list
})

const todayStr = dayjs().format('YYYY-MM-DD')

const currentUserName = computed(() => {
  return authStore.user?.real_name || '同志'
})

const greetingText = computed(() => {
  const hour = new Date().getHours()
  if (hour < 6) return '夜深了'
  if (hour < 9) return '早上好'
  if (hour < 12) return '上午好'
  if (hour < 14) return '中午好'
  if (hour < 18) return '下午好'
  return '晚上好'
})

const todayFullDate = computed(() => {
  return new Date().toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    weekday: 'long'
  })
})

const formatDutyDate = (dStr) => {
  if (!dStr) return ''
  const d = dayjs(dStr)
  const weekdays = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
  return `${d.format('MM/DD')} ${weekdays[d.day()]}`
}

const statCards = computed(() => {
  const all = [
    { 
      label: '本月出勤', 
      value: stats.value.month_present || 0, 
      unit: '天', 
      desc: '正常考勤出勤天数',
      icon: 'Calendar', 
      color: 'var(--el-color-success)', 
      bg: 'var(--el-color-success-light-9)', 
      path: '/attendance', 
      perm: 'attendance.view' 
    },
    { 
      label: '本月请假', 
      value: stats.value.month_leave_days || 0, 
      unit: '天', 
      desc: '已核准休假总天数',
      icon: 'Tickets', 
      color: 'var(--el-color-warning)', 
      bg: 'var(--el-color-warning-light-9)', 
      path: '/leave', 
      perm: 'leave.view' 
    },
    { 
      label: '今日用车', 
      value: stats.value.today_vehicle || 0, 
      unit: '辆', 
      desc: '已调度出行公务用车',
      icon: 'Van', 
      color: 'var(--el-color-primary)', 
      bg: 'var(--el-color-primary-light-9)', 
      path: '/vehicles', 
      perm: 'vehicle.view' 
    },
    { 
      label: '待办收文', 
      value: stats.value.pending_incoming || 0, 
      unit: '份', 
      desc: '等待批示或办理的公文',
      icon: 'FolderOpened', 
      color: 'var(--el-color-danger)', 
      bg: 'var(--el-color-danger-light-9)', 
      path: '/incoming', 
      perm: 'incoming.view' 
    }
  ]
  return all.filter(c => !c.perm || authStore.hasPerm(c.perm))
})

const attStatusText = computed(() => {
  const s = stats.value.today_attendance
  if (s === 1) return '已正常出勤'
  if (s === 2) return '已准假休假'
  if (s === 3) return '外出公干/出差'
  if (s === 4) return '尚未打卡'
  if (s === 5) return '迟到'
  if (s === 6) return '外出培训'
  return '未点到'
})

const attStatusType = computed(() => {
  const s = stats.value.today_attendance
  if (s === 1) return 'success'
  if (s === 2) return 'warning'
  if (s === 3) return 'primary'
  if (s === 4) return 'danger'
  if (s === 5) return 'warning'
  return 'info'
})

const quickActions = computed(() => {
  const acts = [
    { label: '收文登记', icon: 'FolderAdd', color: 'var(--yx-ink-2)', bg: 'var(--el-color-primary-light-9)', path: '/incoming', perm: 'incoming.manage' },
    { label: '用车报备', icon: 'Van', color: 'var(--yx-gold-dark)', bg: 'rgba(184, 146, 90, 0.12)', path: '/vehicles', perm: 'vehicle.apply' },
    { label: '录入大事记', icon: 'DocumentAdd', color: 'var(--el-color-success)', bg: 'var(--el-color-success-light-9)', path: '/reports', perm: 'event.manage' },
    { label: '考勤点到', icon: 'AlarmClock', color: 'var(--yx-brand)', bg: 'var(--yx-brand-bg)', path: '/attendance', perm: 'attendance.mark' }
  ]
  return acts.filter(a => !a.perm || authStore.hasPerm(a.perm))
})

onMounted(async () => {
  try {
    const res = await request.get('/dashboard-stats')
    stats.value = res
    latestIncoming.value = res.latest_incoming || []
    weekDuty.value = res.week_duty || []
  } catch (e) {}
})
</script>

<style scoped>
.trend-sub {
  font-size: 12px;
  color: var(--yx-text-3);
}
.dashboard-container {
  display: flex;
  flex-direction: column;
}

/* ===== 1. 顶部庄重红色渐变横幅 ===== */
.welcome-banner {
  background: linear-gradient(135deg, #131a26 0%, #1b2433 45%, #7a1417 100%);
  border-radius: var(--yx-radius);
  position: relative;
  overflow: hidden;
  box-shadow: 0 8px 32px rgba(19, 26, 38, 0.16);
  color: #ffffff;
  padding: 26px 30px;
  border: 1px solid rgba(227, 207, 166, 0.28);
}

.welcome-banner::after {
  content: '';
  position: absolute;
  top: -80px;
  right: 12%;
  width: 340px;
  height: 340px;
  background: radial-gradient(circle, rgba(227, 207, 166, 0.22) 0%, rgba(158, 27, 30, 0) 70%);
  pointer-events: none;
  z-index: 1;
}

/* 天山巍峨金色山脉多层次背景 */
.banner-mountain-wrap {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  left: 0;
  pointer-events: none;
  z-index: 1;
  overflow: hidden;
  display: flex;
  align-items: flex-end;
}

.tianshan-svg {
  width: 100%;
  height: 100%;
  min-width: 800px;
  object-fit: fill;
  opacity: 0.95;
  filter: drop-shadow(0 -2px 8px rgba(227, 207, 166, 0.25));
}

.banner-content {
  position: relative;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 16px;
}

.banner-greeting {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.greeting-title {
  margin: 0;
  font-size: 22px;
  font-weight: 600;
  letter-spacing: 1px;
  color: #ffffff;
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.25);
}

.greeting-subtitle {
  margin: 0;
  font-size: 13px;
  color: rgba(255, 255, 255, 0.86);
  letter-spacing: 0.5px;
}

/* 横幅内聚焦胶囊 */
.banner-focus-group {
  display: flex;
  align-items: center;
  gap: 12px;
}

.focus-pill {
  background: rgba(255, 255, 255, 0.12);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  border: 1px solid rgba(227, 207, 166, 0.3);
  border-radius: 8px;
  padding: 8px 14px;
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.focus-pill:hover {
  background: rgba(255, 255, 255, 0.2);
  transform: translateY(-1px);
  border-color: rgba(227, 207, 166, 0.6);
}

.focus-pill-icon {
  font-size: 18px;
  color: var(--yx-gold-light);
  display: flex;
  align-items: center;
}

.focus-pill-info {
  display: flex;
  flex-direction: column;
  line-height: 1.2;
}

.focus-pill-label {
  font-size: 11px;
  color: rgba(255, 255, 255, 0.75);
}

.focus-pill-val {
  font-size: 14px;
  font-weight: 600;
  color: #ffffff;
  margin-top: 2px;
  display: flex;
  align-items: center;
  gap: 4px;
}

.dawang-tag {
  font-size: 10px;
  background: #ffffff;
  color: var(--yx-brand);
  padding: 1px 4px;
  border-radius: 4px;
  font-weight: bold;
}

/* ===== 2. 统计卡片 ===== */
.mt-20 {
  margin-top: 20px;
}
.ml-4 {
  margin-left: 4px;
}

.stat-box {
  background: var(--yx-surface);
  border-radius: var(--yx-radius);
  border: 1px solid rgba(220, 223, 229, 0.6);
  box-shadow: 0 4px 18px rgba(19, 26, 38, 0.05);
  padding: 18px 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  transition: transform 0.2s ease, box-shadow 0.2s ease;
  position: relative;
  overflow: hidden;
}

.stat-box::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 3.5px;
  background: var(--yx-crest-gradient);
  z-index: 2;
}

.stat-box.is-clickable {
  cursor: pointer;
}

.stat-box.is-clickable:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 28px rgba(19, 26, 38, 0.12);
}

.stat-box-left {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.stat-box-label {
  font-size: 13px;
  color: var(--yx-text-3);
  font-weight: 500;
}

.stat-box-val-wrap {
  display: flex;
  align-items: baseline;
  gap: 4px;
}

.stat-box-val {
  font-size: 28px;
  font-weight: 700;
  color: var(--yx-text-1);
  line-height: 1.1;
}

.stat-box-unit {
  font-size: 12px;
  color: var(--yx-text-3);
}

.stat-box-desc {
  font-size: 11px;
  color: var(--yx-text-4);
}

.stat-box-icon {
  width: 44px;
  height: 44px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.05);
}

/* ===== 3. 面板卡片统一样式 ===== */
.panel-card {
  background: var(--yx-surface);
  border-radius: var(--yx-radius);
  border: 1px solid rgba(220, 223, 229, 0.6);
  box-shadow: 0 4px 18px rgba(19, 26, 38, 0.05);
  overflow: hidden;
  position: relative;
}

.panel-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 3px;
  background: var(--yx-crest-gradient);
  z-index: 2;
}

.panel-header {
  padding: 14px 20px;
  border-bottom: 1px solid var(--el-border-color-lighter);
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.panel-title-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
}

.panel-title-bar {
  width: 3.5px;
  height: 16px;
  background: linear-gradient(180deg, var(--yx-brand), var(--yx-gold));
  border-radius: 2px;
}

.panel-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--yx-text-1);
  letter-spacing: 0.5px;
}

.panel-body {
  padding: 16px 20px;
}

/* 收文表格 */
.incoming-table :deep(.el-table__row) {
  cursor: pointer;
}

.incoming-table :deep(.el-table__row:hover > td) {
  background-color: var(--el-fill-color-light) !important;
}

.doc-title {
  color: var(--yx-text-1);
  font-weight: 500;
  transition: color 0.2s ease;
}

.doc-title:hover {
  color: var(--yx-ink-2);
}

/* 本周排班网格 */
.duty-week-grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 10px;
}

.duty-week-item {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  padding: 12px 6px;
  text-align: center;
  background: var(--yx-surface);
  position: relative;
  transition: all 0.2s ease;
}

.duty-week-item.is-today {
  border-color: var(--yx-gold);
  background: rgba(184, 146, 90, 0.08);
  box-shadow: 0 0 10px rgba(184, 146, 90, 0.2);
}

.duty-week-badge {
  position: absolute;
  top: -8px;
  left: 50%;
  transform: translateX(-50%);
  background: var(--yx-brand);
  color: #fff;
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 10px;
  white-space: nowrap;
}

.duty-week-date {
  font-size: 11px;
  color: var(--yx-text-3);
  margin-top: 4px;
}

.duty-week-name {
  font-size: 15px;
  font-weight: 600;
  color: var(--yx-text-1);
  margin: 6px 0;
}

.duty-week-tag-wrap {
  display: flex;
  justify-content: center;
}

.dawang-tag-sm {
  font-size: 11px;
  color: var(--yx-brand);
  background: var(--yx-brand-bg);
  padding: 1px 6px;
  border-radius: 4px;
  font-weight: 500;
}

.normal-tag-sm {
  font-size: 11px;
  color: var(--yx-text-3);
  background: var(--el-fill-color);
  padding: 1px 6px;
  border-radius: 4px;
}

/* 今日值班大卡片 */
.duty-card-content {
  display: flex;
  align-items: flex-start;
  gap: 14px;
}

.duty-avatar-box {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  background: var(--yx-brand-bg);
  color: var(--yx-brand);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border: 1px solid rgba(158, 27, 30, 0.2);
}

.duty-details {
  flex: 1;
}

.duty-name-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.duty-person {
  font-size: 20px;
  font-weight: 600;
  color: var(--yx-text-1);
}

.dawang-pill {
  font-size: 11px;
  background: var(--yx-brand-bg);
  color: var(--yx-brand);
  border: 1px solid rgba(158, 27, 30, 0.3);
  padding: 2px 8px;
  border-radius: 10px;
  font-weight: 500;
}

.duty-time-rule {
  margin: 8px 0 0 0;
  font-size: 12px;
  color: var(--yx-text-3);
  line-height: 1.5;
}

/* 个人考勤板块 */
.att-status-box {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: var(--yx-bg);
  padding: 12px 14px;
  border-radius: 8px;
  border: 1px solid var(--el-border-color-lighter);
}

.att-status-left {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.att-label {
  font-size: 12px;
  color: var(--yx-text-3);
}

.att-state {
  font-size: 15px;
  font-weight: 600;
}

.att-state-success { color: var(--el-color-success); }
.att-state-warning { color: var(--el-color-warning); }
.att-state-primary { color: var(--yx-ink-2); }
.att-state-danger { color: var(--el-color-danger); }
.att-state-info { color: var(--yx-text-3); }

.att-data-row {
  display: flex;
  align-items: center;
  justify-content: space-around;
  padding: 8px 0;
}

.att-data-cell {
  text-align: center;
}

.att-data-title {
  font-size: 12px;
  color: var(--yx-text-3);
  display: block;
}

.att-data-num {
  font-size: 22px;
  font-weight: 700;
  color: var(--yx-text-1);
  margin-top: 4px;
  display: block;
}

.att-data-num small {
  font-size: 12px;
  font-weight: normal;
  color: var(--yx-text-3);
}

.att-data-divider {
  width: 1px;
  height: 30px;
  background: var(--el-border-color-lighter);
}

/* 快捷操作网格 */
.quick-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
}

.quick-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 8px;
  background: var(--yx-bg);
  border: 1px solid var(--el-border-color-lighter);
  cursor: pointer;
  transition: all 0.2s ease;
}

.quick-item:hover {
  background: var(--yx-surface);
  box-shadow: var(--yx-shadow);
  border-color: var(--yx-gold-light);
  transform: translateY(-1px);
}

.quick-icon-wrap {
  width: 36px;
  height: 36px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.quick-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--yx-text-1);
}

/* 响应式断点 */
@media (max-width: 991px) {
  .duty-week-grid {
    grid-template-columns: repeat(4, 1fr);
  }
}

@media (max-width: 767px) {
  .hidden-xs {
    display: none !important;
  }
  .welcome-banner {
    padding: 18px 16px;
  }
  .greeting-title {
    font-size: 18px;
  }
  .duty-week-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
