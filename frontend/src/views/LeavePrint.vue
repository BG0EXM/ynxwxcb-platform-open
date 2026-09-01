<template>
  <div>
    <div class="no-print toolbar">
      <el-button type="primary" :icon="'Printer'" @click="printPage">打印假条</el-button>
      <el-button :icon="'Back'" @click="goBack">返回</el-button>
    </div>

    <div class="a4-page" id="print-area">
      <div class="doc-header">
        <span class="unit-name">中共伊宁县委宣传部</span>
      </div>
      <div class="doc-title">请假条</div>

      <div class="leave-intro">
        兹有{{ record.user_name || '' }}同志（{{ leaveTypeNames[record.leave_type] || record.leave_type }}），
        因{{ record.reason || '____________________' }}，需请假
        {{ formatDate(record.start_date) }}至{{ formatDate(record.end_date) }}，共计{{ record.days }}天。
        请假期间，其工作由<span class="blank-line">________</span>同志代管，特此申请，望领导批准。
      </div>

      <table class="leave-table">
        <tbody>
          <tr>
            <td class="label">请假人</td>
            <td class="value">{{ record.user_name || '' }}</td>
            <td class="label">部门</td>
            <td class="value">{{ record.department_name || '' }}</td>
          </tr>
          <tr>
            <td class="label">请假类型</td>
            <td class="value">{{ leaveTypeNames[record.leave_type] || record.leave_type }}</td>
            <td class="label">天数</td>
            <td class="value">{{ record.days }} 天</td>
          </tr>
          <tr>
            <td class="label">请假时间</td>
            <td class="value" colspan="3">{{ formatDate(record.start_date) }} 至 {{ formatDate(record.end_date) }}</td>
          </tr>
          <tr>
            <td class="label">请假事由</td>
            <td class="value reason" colspan="3">{{ record.reason || '' }}</td>
          </tr>
          <tr>
            <td class="label">审批流程</td>
            <td class="value" colspan="3">{{ approvalFlow }}</td>
          </tr>
        </tbody>
      </table>

      <div class="sign-area">
        <div class="sign-row" v-for="(row, rIdx) in signRows" :key="rIdx">
          <div class="sign-item" v-for="item in row" :key="item.label">
            <div class="sign-label">{{ item.label }}</div>
            <div class="sign-line"></div>
            <div class="sign-date">{{ item.date }}</div>
          </div>
        </div>
      </div>

      <div class="doc-footer">
        <span class="footer-left">打印日期：{{ today }}</span>
        <span class="footer-right">
          <div>中共伊宁县委宣传部办公室</div>
          <div>伊宁县委宣传部部务工作平台V1.3.7</div>
        </span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import request from '../utils/request'

const route = useRoute()
const router = useRouter()
const record = ref({})

const leaveTypeNames = {
  annual: '年假', sick: '病假', personal: '事假', marriage: '婚假',
  maternity: '产假', bereavement: '丧假', prenatal: '产检假', family: '探亲假',
  comp: '补休', other: '其他'
}

const today = computed(() => {
  const d = new Date()
  return `${d.getFullYear()}年${d.getMonth() + 1}月${d.getDate()}日`
})

const formatDate = (val) => {
  if (!val) return ''
  const parts = String(val).split('-')
  if (parts.length === 3) return `${parts[0]}年${parseInt(parts[1])}月${parseInt(parts[2])}日`
  return val
}

// 审批链：按请假天数决定审批层级
// 半天以内：科室主任；1-3天：科室主任+分管领导；3天以上：科室主任+分管领导+县分管领导
const approvalFlow = computed(() => {
  const days = Number(record.value.days) || 0
  let flow = '科室主任审批'
  if (days >= 1) flow += ' → 分管领导审批'
  if (days > 3) flow += ' → 县分管领导审批'
  return flow
})

const signRows = computed(() => {
  const days = Number(record.value.days) || 0
  const approvals = ['科室主任审批']
  if (days >= 1) approvals.push('分管领导审批')
  if (days > 3) approvals.push('县分管领导审批')
  const items = [{ label: '申请人签字', date: `日期：${today.value}` }]
  approvals.forEach(a => items.push({ label: a, date: '日期：____年__月__日' }))
  const rows = []
  for (let i = 0; i < items.length; i += 2) {
    rows.push(items.slice(i, i + 2))
  }
  return rows
})

const goBack = () => router.push('/leave')

const printPage = () => {
  window.print()
}

onMounted(async () => {
  const cached = sessionStorage.getItem('printLeave')
  if (cached) {
    record.value = JSON.parse(cached)
  } else {
    try {
      const res = await request.get('/leave-records', { params: { page: 1, page_size: 100 } })
      const found = (res.list || []).find(x => String(x.id) === String(route.params.id))
      if (found) record.value = found
    } catch (e) {}
  }
})
</script>

<style scoped>
.toolbar {
  max-width: 794px;
  margin: 0 auto 12px;
  padding: 8px;
  background: #fff;
  border-radius: 4px;
  box-shadow: 0 1px 4px rgba(0,0,0,.1);
  display: flex;
  gap: 8px;
}
.a4-page {
  width: 794px;
  min-height: 1122px;
  margin: 0 auto;
  padding: 36px 56px 76px;
  background: #fff;
  box-shadow: 0 2px 12px rgba(0,0,0,.12);
  position: relative;
  box-sizing: border-box;
}
.doc-header {
  text-align: center;
}
.unit-name {
  font-size: 18px;
  font-weight: 700;
  letter-spacing: 2px;
}
.doc-title {
  text-align: center;
  font-size: 26px;
  font-weight: 700;
  margin: 24px 0 28px;
  letter-spacing: 8px;
}
.leave-intro {
  font-size: 16px;
  line-height: 2.2;
  text-indent: 2em;
  margin-bottom: 20px;
}
.blank-line {
  letter-spacing: 1px;
}
.leave-table {
  width: 100%;
  border-collapse: collapse;
  border: 1px solid #000;
}
.leave-table td {
  border: 1px solid #000;
  padding: 10px 12px;
  font-size: 15px;
  line-height: 1.8;
}
.label {
  width: 100px;
  text-align: center;
  font-weight: 600;
  background: #f5f5f5;
}
.value {
  width: 190px;
}
.reason {
  min-height: 70px;
  white-space: pre-wrap;
}
.sign-area {
  margin-top: 40px;
}
.sign-row {
  display: flex;
  justify-content: space-between;
  margin-bottom: 28px;
}
.sign-item {
  width: 48%;
}
.sign-label {
  font-size: 15px;
  font-weight: 600;
}
.sign-line {
  border-bottom: 1px solid #000;
  margin-top: 32px;
  height: 30px;
}
.sign-date {
  font-size: 13px;
  color: #333;
  margin-top: 8px;
}
.doc-footer {
  position: absolute;
  bottom: 36px;
  left: 56px;
  right: 56px;
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  font-size: 13px;
  color: #333;
}
.footer-right {
  text-align: center;
  line-height: 1.7;
}
@media print {
  @page {
    size: A4;
    margin: 0;
  }
  body {
    background: #fff !important;
  }
  .no-print {
    display: none !important;
  }
  .a4-page {
    width: 100%;
    min-height: 100vh;
    box-shadow: none;
    padding: 30px 44px 70px;
  }
}
</style>
