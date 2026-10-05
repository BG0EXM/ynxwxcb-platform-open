<template>
  <div>
    <div class="no-print toolbar">
      <el-button type="primary" :icon="'Printer'" @click="printPage">打印传阅登记卡</el-button>
      <el-button :icon="'Back'" @click="goBack">返回</el-button>
    </div>

    <div class="a4-page" id="print-area">
      <div v-if="doc.need_return === 1" class="need-return-mark">
        {{ doc.returned === 1 ? '已退回' : '需退回' }}
      </div>
      <div v-if="doc.urgency === '紧急' || doc.urgency === '特急'" class="urgent-mark">
        {{ doc.urgency }}
      </div>
      <div class="doc-header">
        <span class="unit-name">中共伊宁县委宣传部</span>
      </div>
      <div class="doc-title">文件传阅登记卡</div>

      <table class="info-table">
        <tbody>
          <tr>
            <td class="label">收文编号</td>
            <td class="value">{{ doc.receive_no || '' }}</td>
            <td class="label">来文单位</td>
            <td class="value">{{ doc.from_unit || '' }}</td>
          </tr>
          <tr>
            <td class="label">来文字号</td>
            <td class="value">{{ doc.from_doc_no || '' }}</td>
            <td class="label">收文日期</td>
            <td class="value">{{ doc.received_date || '' }}</td>
          </tr>
          <tr>
            <td class="label">文件标题</td>
            <td class="value" colspan="3" style="font-weight:600">{{ doc.title || '' }}</td>
          </tr>
        </tbody>
      </table>

      <div class="note">
        <p>备注：本文件共 {{ doc.copies || 1 }} 份，传阅完毕后请及时签退归档。</p>
      </div>

      <!-- 传阅登记表：序号/签字手写，签退列 -->
      <div class="circ-wrap">
        <table class="circ-table">
          <thead>
            <tr>
              <th style="width:70px">序号</th>
              <th style="width:160px">传阅人</th>
              <th>签字</th>
              <th style="width:150px">传阅日期</th>
              <th style="width:90px">签退</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(c, i) in rows" :key="i" class="blank-row">
              <td class="center">{{ c.index }}</td>
              <td class="center">{{ c.user_name || '' }}</td>
              <td class="center"></td>
              <td class="center"></td>
              <td class="center"></td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="doc-footer">
        <span class="footer-left">打印日期：{{ today }}</span>
        <span class="footer-right">
          <div>中共伊宁县委宣传部办公室</div>
          <div>伊宁县委宣传部部务工作平台V1.6.1</div>
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
const doc = ref({})
const circs = ref([])

const today = computed(() => {
  const d = new Date()
  return `${d.getFullYear()}年${d.getMonth() + 1}月${d.getDate()}日`
})

// 默认 11 行，严格适配单页 A4 纸不溢出；已有传阅人则填充姓名，其余留空手写
const rows = computed(() => {
  const list = []
  const count = Math.max(11, circs.value.length)
  for (let i = 0; i < count; i++) {
    list.push({
      index: i + 1,
      user_name: circs.value[i] ? circs.value[i].user_name : ''
    })
  }
  return list
})

const goBack = () => router.push('/incoming')

const printPage = () => {
  window.print()
}

onMounted(async () => {
  const cached = sessionStorage.getItem('printCard') || sessionStorage.getItem('printDoc')
  if (cached) {
    try {
      const parsed = JSON.parse(cached)
      if (String(parsed.id) === String(route.params.id)) {
        doc.value = parsed
        circs.value = parsed.circulations || []
      }
    } catch (e) { console.error(e) }
  }
  try {
    const res = await request.get(`/incoming-docs/${route.params.id}`)
    doc.value = res
    circs.value = res.circulations || []
  } catch (e) { console.error(e) }
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
  min-height: 1100px;
  max-height: 1122px;
  margin: 0 auto;
  padding: 30px 48px 36px;
  background: #fff;
  box-shadow: 0 2px 12px rgba(0,0,0,.12);
  position: relative;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
}
.doc-header {
  text-align: center;
}
.need-return-mark {
  position: absolute;
  top: 24px;
  right: 24px;
  border: 3px solid #c8102e;
  color: #c8102e;
  font-size: 20px;
  font-weight: 700;
  padding: 5px 14px;
  letter-spacing: 4px;
  background: #fff;
}
.urgent-mark {
  position: absolute;
  top: 24px;
  left: 24px;
  border: 3px solid #c8102e;
  color: #c8102e;
  font-size: 20px;
  font-weight: 700;
  padding: 5px 14px;
  letter-spacing: 4px;
  transform: rotate(-12deg);
  background: #fff;
}
.unit-name {
  font-size: 18px;
  font-weight: 700;
  letter-spacing: 2px;
}
.doc-title {
  text-align: center;
  font-size: 22px;
  font-weight: 700;
  margin: 12px 0 16px;
  letter-spacing: 6px;
}
.info-table {
  width: 100%;
  border-collapse: collapse;
  border: 1px solid #000;
  margin-bottom: 10px;
}
.info-table td {
  border: 1px solid #000;
  padding: 6px 10px;
  font-size: 13.5px;
  line-height: 1.5;
}
.info-table .label {
  width: 100px;
  text-align: center;
  font-weight: 600;
  background: #f5f5f5;
}
.circ-wrap {
  /* 弹性填充头部与页脚之间的剩余空间，让表格延伸到底部 */
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 180px;
}
.circ-table {
  width: 100%;
  border-collapse: collapse;
  border: 1px solid #000;
  table-layout: fixed;
}
.circ-table th,
.circ-table td {
  border: 1px solid #000;
  padding: 6px;
  font-size: 13.5px;
}
.circ-table th {
  background: #f5f5f5;
  font-weight: 600;
}
.circ-table tbody tr {
  height: 36px;
}
.blank-row td {
  height: 36px;
}
.center {
  text-align: center;
}
.note {
  margin-bottom: 8px;
  font-size: 12.5px;
  color: #333;
  flex-shrink: 0;
  padding: 2px;
}
.doc-footer {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  font-size: 12.5px;
  color: #333;
  padding-top: 10px;
  flex-shrink: 0;
}
.footer-right {
  text-align: center;
  line-height: 1.6;
}
@media print {
  @page {
    size: A4 portrait;
    margin: 8mm 10mm;
  }
  html, body {
    background: #fff !important;
    height: auto !important;
    overflow: hidden !important;
  }
  .no-print {
    display: none !important;
  }
  .a4-page {
    width: 100% !important;
    max-width: 100% !important;
    min-height: auto !important;
    height: auto !important;
    box-shadow: none !important;
    padding: 0 !important;
    page-break-inside: avoid !important;
    page-break-after: avoid !important;
  }
  .circ-table tbody tr,
  .blank-row td {
    height: 32px !important;
    padding: 2px 4px !important;
  }
  .info-table td {
    padding: 4px 8px !important;
  }
  .doc-title {
    margin: 8px 0 12px !important;
  }
  .note {
    margin-bottom: 6px !important;
  }
  .doc-footer {
    padding-top: 8px !important;
  }
}
</style>
