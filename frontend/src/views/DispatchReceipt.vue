<template>
  <div class="gov-dispatch-wrapper">
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
          <div class="brand-sup font-serif">中共伊宁县委宣传部 · 数字化协同政务服务平台</div>
          <div class="brand-sub">官方文件与学习资料下发查收系统</div>
        </div>
      </div>
    </div>

    <!-- 主体容器 -->
    <div class="main-container">
      <template v-if="dispatch">
        <!-- 核心公文通告卡片 -->
        <div class="dossier-card">
          <!-- 顶部红金装饰带 -->
          <div class="card-gold-crest-bar"></div>

          <!-- 状态与字号标签栏 -->
          <div class="status-ribbon-row">
            <div class="status-pill" :class="expired ? 'is-expired' : 'is-active'">
              <span class="pulse-dot" v-if="!expired"></span>
              <span>{{ expired ? '查收时限已截止' : '下发中 · 在线签收查收' }}</span>
            </div>
            <div class="doc-no-pill" v-if="dispatch.doc_no">
              <el-icon><Tickets /></el-icon>
              <span>{{ dispatch.doc_no }}</span>
            </div>
            <div class="countdown-pill" v-if="!expired && countdownText">
              <el-icon><Clock /></el-icon>
              <span>{{ countdownText }}</span>
            </div>
          </div>

          <!-- 文件公文主标题 -->
          <h1 class="dossier-title font-serif">{{ dispatch.title }}</h1>

          <!-- 发文信息网格 -->
          <div class="dispatch-dossier-grid">
            <div class="dossier-item">
              <div class="dossier-icon time-icon">
                <el-icon><Calendar /></el-icon>
              </div>
              <div class="dossier-text">
                <span class="dossier-label">查收时限</span>
                <span class="dossier-val font-serif">{{ dispatch.deadline || '长期有效' }}</span>
              </div>
            </div>

            <div class="dossier-item">
              <div class="dossier-icon org-icon">
                <el-icon><OfficeBuilding /></el-icon>
              </div>
              <div class="dossier-text">
                <span class="dossier-label">发文单位</span>
                <span class="dossier-val font-serif">中共伊宁县委宣传部</span>
              </div>
            </div>

            <div class="dossier-item item-full-width" v-if="dispatch.content">
              <div class="dossier-icon place-icon">
                <el-icon><InfoFilled /></el-icon>
              </div>
              <div class="dossier-text">
                <span class="dossier-label">工作贯彻说明</span>
                <span class="dossier-val font-serif">{{ dispatch.content }}</span>
              </div>
            </div>
          </div>

          <!-- 免回函权威说明与保密提示 Banner -->
          <div class="no-reply-notice-banner">
            <div class="notice-badge">
              <el-icon><InfoFilled /></el-icon>
              <span>查收须知</span>
            </div>
            <div class="notice-text">
              本材料为中共伊宁县委宣传部下发公文，各单位在下方选定单位并确认查收即完成签收，<strong>无需报送回函公文或盖章凭证</strong>。<br>
              <strong>【保密红线提示】</strong>本系统为非涉密政务平台，<strong>严禁流转或截留任何标注为国家秘密（秘密、机密、绝密）及工作秘密、内部级文件资料！</strong>
            </div>
          </div>

          <!-- ================= 在线查看 PDF 底稿舱 ================= -->
          <div class="pdf-viewer-section">
            <div class="section-lead-bar">
              <div class="lead-left">
                <el-icon class="lead-icon"><Document /></el-icon>
                <span class="lead-text font-serif">正文公文查阅</span>
                <span class="file-name-tag">{{ dispatch.pdf_name || '正文文件.pdf' }}</span>
              </div>
              <div class="lead-actions">
                <el-button 
                  type="primary" 
                  plain 
                  size="small"
                  :icon="'FullScreen'"
                  @click="openPdfWindow"
                >
                  新窗口全屏阅览
                </el-button>
                <el-button 
                  type="default" 
                  size="small"
                  :icon="'Download'"
                  @click="downloadPdf"
                >
                  下载 PDF 原文
                </el-button>
                <el-button 
                  v-if="hasAttachment"
                  type="success" 
                  plain 
                  size="small"
                  :icon="'Document'"
                  @click="downloadAttachment"
                >
                  下载配套附件 ({{ dispatch.attachment_name || '附件' }})
                </el-button>
              </div>
            </div>

            <!-- 内嵌 PDF 预览框架 -->
            <div class="pdf-embed-wrapper">
              <iframe 
                :src="pdfEmbedUrl" 
                class="pdf-iframe-player" 
                title="正文文件预览"
              ></iframe>
              <div class="pdf-mobile-fallback hidden-sm-and-up">
                <p>手机端可在上方直接滑动预览，若遇显示限制可点击下方全屏打开：</p>
                <el-button type="primary" size="small" :icon="'View'" @click="openPdfWindow">
                  全屏打开 PDF 阅读器
                </el-button>
              </div>
            </div>
          </div>

          <!-- ================= 查收签收流程区 ================= -->
          <div class="flow-divider">
            <span class="divider-line"></span>
            <span class="divider-text font-serif">公 文 查 收 与 签 收 确 认</span>
            <span class="divider-line"></span>
          </div>

          <!-- 1. 未选定单位状态：引导选择单位 -->
          <div class="unit-select-card" v-if="!unitSelected">
            <div class="select-lead">
              <div class="lead-badge">第 1 步</div>
              <div class="lead-title">选择您的所属查收单位</div>
              <div class="lead-desc">全县各乡镇场、部门名单实时匹配校验</div>
            </div>

            <div class="unit-picker-box">
              <el-select 
                v-model="pendingUnit" 
                filterable 
                size="large"
                placeholder="请点击或输入搜索您的单位名称"
                class="unit-select-input"
              >
                <el-option 
                  v-for="u in units" 
                  :key="u" 
                  :label="u" 
                  :value="u" 
                />
              </el-select>
              <el-button 
                type="primary" 
                size="large" 
                class="btn-confirm-unit"
                :disabled="!pendingUnit"
                @click="confirmUnitSelection"
              >
                确定单位
              </el-button>
            </div>
          </div>

          <!-- 2. 已选定单位：签收表单 / 已签收凭证 -->
          <div v-else class="receipt-section-box">
            <!-- 当前单位标头栏 -->
            <div class="unit-header-bar">
              <div class="uh-left">
                <span class="uh-badge">查收单位</span>
                <span class="uh-unit font-serif">{{ currentUnit }}</span>
              </div>
              <el-button link type="primary" size="small" @click="reselectUnit">
                切换单位
              </el-button>
            </div>

            <!-- A. 若该单位已经签收查收：展示权威签收印鉴凭证 -->
            <div v-if="receipted" class="receipt-success-card">
              <!-- 红色官方电子签收印章 -->
              <div class="official-seal-wrap">
                <div class="official-seal">
                  <div class="seal-inner-circle">
                    <div class="seal-star">★</div>
                    <div class="seal-text-top">已确认查收</div>
                    <div class="seal-text-mid font-serif">伊宁县委宣传部</div>
                    <div class="seal-text-bot">RECEIVED</div>
                  </div>
                </div>
              </div>

              <div class="receipt-voucher-info">
                <div class="voucher-status-line">
                  <el-icon class="icon-ok"><CircleCheckFilled /></el-icon>
                  <span class="status-ok-title font-serif">贵单位已完成公文查收与签收登记</span>
                </div>
                <div class="voucher-sub">
                  查收状态已即时同步至县委宣传部管理后台，无需报送回函。
                </div>

                <div class="voucher-details-grid">
                  <div class="v-detail-item">
                    <span class="v-label">查收经办人</span>
                    <span class="v-val">{{ receiptData?.receiver_name }}</span>
                  </div>
                  <div class="v-detail-item">
                    <span class="v-label">联系电话</span>
                    <span class="v-val">{{ receiptData?.receiver_phone }}</span>
                  </div>
                  <div class="v-detail-item">
                    <span class="v-label">查收时间</span>
                    <span class="v-val">{{ formatDate(receiptData?.received_at) }}</span>
                  </div>
                  <div class="v-detail-item">
                    <span class="v-label">查收次数</span>
                    <span class="v-val">第 {{ receiptData?.read_count || 1 }} 次</span>
                  </div>
                </div>

                <div class="voucher-lock-tip">
                  <el-icon><Lock /></el-icon>
                  <span>查收记录已安全存证锁定。如需变更经办人信息，请联系宣传部办公室管理员在后台重置。</span>
                </div>
              </div>
            </div>

            <!-- B. 若该单位尚未签收：填写经办人并点击确认查收 -->
            <div v-else class="receipt-form-card">
              <div class="form-lead-box">
                <div class="lead-badge">第 2 步</div>
                <div class="lead-title font-serif">填写经办人信息并确认签收</div>
                <div class="lead-desc">签收信息将作为各单位贯彻落实该文件的工作依据</div>
              </div>

              <el-form :model="receiptForm" label-position="top" class="mt-16">
                <!-- 若材料查收已截止，显示明确的截止已过提示 -->
                <div v-if="expired" class="expired-alert-banner">
                  <el-alert
                    title="公文材料查收已截止"
                    type="warning"
                    description="该公文材料设定的查收截止时限已过，在线签收通道已关闭。如需补录或有紧急情况，请联系县委宣传部办公室核实。"
                    show-icon
                    :closable="false"
                  />
                </div>

                <el-row :gutter="20">
                  <el-col :xs="24" :sm="12">
                    <el-form-item label="经办人姓名" required>
                      <el-input 
                        v-model="receiptForm.receiver_name" 
                        size="large"
                        placeholder="请输入具体查收经办人姓名" 
                        :prefix-icon="'User'"
                        :disabled="expired"
                      />
                    </el-form-item>
                  </el-col>
                  <el-col :xs="24" :sm="12">
                    <el-form-item label="联系电话" required>
                      <el-input 
                        v-model="receiptForm.receiver_phone" 
                        size="large"
                        placeholder="请输入经办人手机号码" 
                        :prefix-icon="'Phone'"
                        :disabled="expired"
                      />
                    </el-form-item>
                  </el-col>
                </el-row>

                <div class="submit-action-row">
                  <el-button 
                    type="primary" 
                    size="large" 
                    class="btn-submit-receipt"
                    :loading="submitting"
                    :disabled="expired"
                    @click="submitReceipt"
                  >
                    {{ expired ? '材料查收已截止（签收通道关闭）' : '确认查收本期公文材料' }}
                  </el-button>
                  <div class="submit-tip">
                    <span v-if="expired" class="text-expired-tip">查收时限已截止，签收确认已被系统锁定</span>
                    <span v-else>点击确认后即刻打上查收印章，后台实时计入查收率，免报回函</span>
                  </div>
                </div>
              </el-form>
            </div>
          </div>
        </div>
      </template>

      <!-- 加载中或错误状态 -->
      <div v-else-if="loading" class="loading-box">
        <el-skeleton :rows="8" animated />
      </div>
      <div v-else class="error-box">
        <el-result icon="warning" title="材料未找到" sub-title="该文件可能已被撤回或链接有误，请联系县委宣传部办公室核实。">
        </el-result>
      </div>

      <!-- 统一权威政务页脚 -->
      <footer class="gov-footer">
        <div class="footer-crest-row">
          <span class="crest-dot"></span>
          <span class="footer-org font-serif">中共伊宁县委宣传部</span>
          <span class="crest-dot"></span>
        </div>
        <div class="footer-system">部务工作平台 · 数字化协同政务服务终端 V1.7.1</div>
        <div class="footer-beian">
          <a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener">[滇ICP备XXXXXXXX号-X]</a>
          <span class="sep">|</span>
          <a href="https://beian.mps.gov.cn/#/query/webSearch" target="_blank" rel="noopener">[滇公网安备XXXXXXXXXXXXXXXX号]</a>
          <span class="sep">|</span>
          <span class="ipv6-tag">本平台已全面支持 IPv6 访问</span>
        </div>
      </footer>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { 
  Calendar, OfficeBuilding, Clock, InfoFilled, Tickets, 
  Document, CircleCheckFilled, Lock
} from '@element-plus/icons-vue'
import request from '../utils/request'
import dayjs from 'dayjs'

const route = useRoute()
const dispatchId = route.params.id

const loading = ref(true)
const dispatch = ref(null)
const units = ref([])
const expired = ref(false)
const hasAttachment = ref(false)

const pendingUnit = ref('')
const currentUnit = ref('')
const unitSelected = computed(() => !!currentUnit.value)

const receipted = ref(false)
const receiptData = ref(null)
const submitting = ref(false)

const receiptForm = ref({
  receiver_name: '',
  receiver_phone: ''
})

// 倒计时
const countdownText = ref('')
let timer = null

const updateCountdown = () => {
  if (!dispatch.value?.deadline) {
    countdownText.value = ''
    return
  }
  const diff = dayjs(dispatch.value.deadline).diff(dayjs(), 'second')
  if (diff <= 0) {
    expired.value = true
    countdownText.value = '查收已截止'
    return
  }
  const days = Math.floor(diff / 86400)
  const hours = Math.floor((diff % 86400) / 3600)
  const mins = Math.floor((diff % 3600) / 60)
  if (days > 0) {
    countdownText.value = `距离截止还剩 ${days} 天 ${hours} 小时`
  } else {
    countdownText.value = `距离截止还剩 ${hours} 小时 ${mins} 分钟`
  }
}

const pdfEmbedUrl = computed(() => {
  return `/api/public/dispatches/${dispatchId}/file?type=pdf&mode=inline`
})

const openPdfWindow = () => {
  window.open(pdfEmbedUrl.value, '_blank')
}

const downloadPdf = () => {
  const downloadUrl = `/api/public/dispatches/${dispatchId}/file?type=pdf&mode=download`
  window.open(downloadUrl, '_blank')
}

const downloadAttachment = () => {
  const downloadUrl = `/api/public/dispatches/${dispatchId}/file?type=attachment&mode=download`
  window.open(downloadUrl, '_blank')
}

const formatDate = (val) => {
  if (!val) return ''
  return dayjs(val).format('YYYY年MM月DD日 HH:mm')
}

// 加载公文下发数据
const loadData = async () => {
  loading.value = true
  try {
    const unitParam = currentUnit.value ? `?unit=${encodeURIComponent(currentUnit.value)}` : ''
    const res = await request.get(`/public/dispatches/${dispatchId}${unitParam}`)
    dispatch.value = res.dispatch || null
    units.value = res.units || []
    expired.value = !!res.expired
    hasAttachment.value = !!res.has_attachment

    if (res.current_receipt) {
      receipted.value = true
      receiptData.value = res.current_receipt
      receiptForm.value.receiver_name = res.current_receipt.receiver_name || ''
      receiptForm.value.receiver_phone = res.current_receipt.receiver_phone || ''
    } else {
      receipted.value = false
      receiptData.value = null
    }

    updateCountdown()
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

// 确认选择所属单位
const confirmUnitSelection = async () => {
  if (!pendingUnit.value) return
  currentUnit.value = pendingUnit.value
  await loadData()
}

// 切换单位
const reselectUnit = () => {
  currentUnit.value = ''
  pendingUnit.value = ''
  receipted.value = false
  receiptData.value = null
}

// 提交确认查收
const submitReceipt = async () => {
  if (expired.value) {
    return ElMessage.warning('材料查收时限已截止，签收通道已关闭')
  }
  if (!receiptForm.value.receiver_name.trim()) {
    return ElMessage.warning('请填写经办人姓名')
  }
  if (!receiptForm.value.receiver_phone.trim()) {
    return ElMessage.warning('请填写经办人联系电话')
  }

  submitting.value = true
  try {
    const res = await request.post(`/public/dispatches/${dispatchId}/receipt`, {
      unit: currentUnit.value,
      receiver_name: receiptForm.value.receiver_name.trim(),
      receiver_phone: receiptForm.value.receiver_phone.trim()
    })
    ElMessage.success('查收签收成功！')
    receipted.value = true
    receiptData.value = res.receipt
  } catch (e) {
    console.error(e)
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  loadData()
  timer = setInterval(updateCountdown, 60000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.gov-dispatch-wrapper {
  min-height: 100vh;
  background-color: #f6f8fb;
  background-image: 
    radial-gradient(at 0% 0%, rgba(158, 27, 30, 0.05) 0px, transparent 50%),
    radial-gradient(at 100% 100%, rgba(36, 64, 110, 0.04) 0px, transparent 50%);
  display: flex;
  flex-direction: column;
  position: relative;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
}

/* 顶部深色政务天际线 - 与征求意见、会议填报页 100% 对齐 */
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

.org-brand-bar {
  position: relative;
  max-width: 960px;
  margin: 0 auto;
  display: flex;
  align-items: center;
  gap: 14px;
  z-index: 2;
}

.brand-crest {
  width: 50px;
  height: 50px;
  object-fit: contain;
  filter: drop-shadow(0 0 14px rgba(245, 212, 153, 0.65));
}

.brand-titles {
  display: flex;
  flex-direction: column;
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
  max-width: 960px;
  width: 100%;
  margin: -70px auto 40px;
  padding: 0 16px;
  position: relative;
  z-index: 3;
  flex: 1;
}

.dossier-card {
  background: #ffffff;
  border-radius: 12px;
  box-shadow: 0 8px 28px rgba(0, 0, 0, 0.08);
  overflow: hidden;
  padding: 28px 32px 36px;
  position: relative;
}

.card-gold-crest-bar {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 4px;
  background: linear-gradient(90deg, #d97706, #fbbf24, #b45309);
}

.status-ribbon-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}

.status-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  border-radius: 9999px;
  font-size: 12px;
  font-weight: 600;
}

.status-pill.is-active {
  background: #ecfdf5;
  color: #047857;
  border: 1px solid #a7f3d0;
}

.status-pill.is-expired {
  background: #f1f5f9;
  color: #64748b;
  border: 1px solid #cbd5e1;
}

.pulse-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #10b981;
  box-shadow: 0 0 0 0 rgba(16, 185, 129, 0.7);
  animation: pulse 1.8s infinite;
}

@keyframes pulse {
  0% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(16, 185, 129, 0.7); }
  70% { transform: scale(1); box-shadow: 0 0 0 6px rgba(16, 185, 129, 0); }
  100% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(16, 185, 129, 0); }
}

.doc-no-pill, .countdown-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  background: #f8fafc;
  color: #475569;
  border-radius: 6px;
  font-size: 12px;
  border: 1px solid #e2e8f0;
}

.countdown-pill {
  color: #b45309;
  background: #fffbeb;
  border-color: #fde68a;
}

.dossier-title {
  font-size: 24px;
  font-weight: 700;
  color: #1e293b;
  line-height: 1.4;
  margin: 0 0 20px;
}

.dispatch-dossier-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  background: #f8fafc;
  padding: 16px;
  border-radius: 8px;
  border: 1px solid #e2e8f0;
  margin-bottom: 20px;
}

.dossier-item {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.item-full-width {
  grid-column: 1 / -1;
}

.dossier-icon {
  width: 32px;
  height: 32px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  flex-shrink: 0;
}

.time-icon { background: #fee2e2; color: #b91c1c; }
.org-icon { background: #fef3c7; color: #b45309; }
.place-icon { background: #e0f2fe; color: #0369a1; }

.dossier-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.dossier-label {
  font-size: 12px;
  color: #64748b;
}

.dossier-val {
  font-size: 14px;
  color: #1e293b;
  font-weight: 500;
  line-height: 1.5;
}

/* 查收须知 Banner */
.no-reply-notice-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  background: #f0fdf4;
  border: 1px solid #bbf7d0;
  border-radius: 8px;
  padding: 12px 16px;
  margin-bottom: 24px;
}

.notice-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: #15803d;
  color: #fff;
  font-size: 12px;
  font-weight: 600;
  padding: 3px 8px;
  border-radius: 4px;
  flex-shrink: 0;
}

.notice-text {
  font-size: 13px;
  color: #166534;
  line-height: 1.5;
}

/* PDF 查阅舱 */
.pdf-viewer-section {
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  overflow: hidden;
  margin-bottom: 28px;
  background: #fff;
}

.section-lead-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #f8fafc;
  padding: 12px 16px;
  border-bottom: 1px solid #e2e8f0;
  flex-wrap: wrap;
  gap: 10px;
}

.lead-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.lead-icon {
  font-size: 18px;
  color: #b22222;
}

.lead-text {
  font-size: 15px;
  font-weight: 600;
  color: #1e293b;
}

.file-name-tag {
  background: #f1f5f9;
  color: #475569;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.lead-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.pdf-embed-wrapper {
  height: 520px;
  background: #525659;
  position: relative;
}

.pdf-iframe-player {
  width: 100%;
  height: 100%;
  border: none;
}

.pdf-mobile-fallback {
  background: #fff;
  padding: 12px;
  text-align: center;
  border-top: 1px solid #e2e8f0;
}

.pdf-mobile-fallback p {
  font-size: 12px;
  color: #64748b;
  margin-bottom: 8px;
}

/* 分隔线 */
.flow-divider {
  display: flex;
  align-items: center;
  gap: 16px;
  margin: 32px 0 24px;
}

.divider-line {
  flex: 1;
  height: 1px;
  background: #e2e8f0;
}

.divider-text {
  font-size: 16px;
  font-weight: 600;
  color: #b22222;
  letter-spacing: 2px;
}

/* 1. 选单位卡片 */
.unit-select-card {
  background: #fafafa;
  border: 1px dashed #cbd5e1;
  border-radius: 8px;
  padding: 24px;
  text-align: center;
}

.select-lead {
  margin-bottom: 20px;
}

.lead-badge {
  display: inline-block;
  background: #b22222;
  color: #fff;
  font-size: 11px;
  font-weight: 700;
  padding: 2px 8px;
  border-radius: 9999px;
  margin-bottom: 6px;
}

.lead-title {
  font-size: 18px;
  font-weight: 700;
  color: #1e293b;
  margin-bottom: 4px;
}

.lead-desc {
  font-size: 13px;
  color: #64748b;
}

.unit-picker-box {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  max-width: 480px;
  margin: 0 auto;
}

.unit-select-input {
  flex: 1;
}

.btn-confirm-unit {
  background-color: #b22222;
  border-color: #b22222;
}

/* 2. 签收区域 */
.receipt-section-box {
  background: #fff;
}

.unit-header-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #fdf2f2;
  border: 1px solid #fecaca;
  border-radius: 8px;
  padding: 10px 16px;
  margin-bottom: 20px;
}

.uh-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.uh-badge {
  background: #b22222;
  color: #fff;
  font-size: 12px;
  padding: 2px 6px;
  border-radius: 4px;
}

.uh-unit {
  font-size: 16px;
  font-weight: 700;
  color: #991b1b;
}

/* A. 已签收印章凭证卡片 */
.receipt-success-card {
  position: relative;
  background: #fcfdfd;
  border: 2px dashed #86efac;
  border-radius: 12px;
  padding: 28px;
  overflow: hidden;
}

.official-seal-wrap {
  position: absolute;
  top: 16px;
  right: 24px;
  pointer-events: none;
}

/* 权威公章印鉴 */
.official-seal {
  width: 110px;
  height: 110px;
  border: 3px solid #dc2626;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  transform: rotate(-15deg);
  opacity: 0.85;
}

.seal-inner-circle {
  width: 96px;
  height: 96px;
  border: 1px dashed #dc2626;
  border-radius: 50%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  color: #dc2626;
}

.seal-star {
  font-size: 16px;
  line-height: 1;
}

.seal-text-top {
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 1px;
}

.seal-text-mid {
  font-size: 10px;
  font-weight: 700;
}

.seal-text-bot {
  font-size: 8px;
  letter-spacing: 1px;
  transform: scale(0.9);
}

.receipt-voucher-info {
  position: relative;
  z-index: 2;
  max-width: 600px;
}

.voucher-status-line {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}

.icon-ok {
  font-size: 24px;
  color: #16a34a;
}

.status-ok-title {
  font-size: 18px;
  font-weight: 700;
  color: #15803d;
}

.voucher-sub {
  font-size: 13px;
  color: #4b5563;
  margin-bottom: 18px;
}

.voucher-details-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  background: #f0fdf4;
  padding: 14px 18px;
  border-radius: 8px;
  border: 1px solid #dcfce7;
  margin-bottom: 16px;
}

.v-detail-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.v-label {
  font-size: 11px;
  color: #6b7280;
}

.v-val {
  font-size: 14px;
  font-weight: 600;
  color: #1f2937;
}

.voucher-lock-tip {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: #6b7280;
}

/* B. 签收表单卡片 */
.receipt-form-card {
  background: #fafafa;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 24px;
}

.form-lead-box {
  border-bottom: 1px solid #e2e8f0;
  padding-bottom: 12px;
}

.submit-action-row {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  margin-top: 24px;
}

.btn-submit-receipt {
  background-color: #b22222;
  border-color: #b22222;
  width: 280px;
  height: 48px;
  font-size: 16px;
  font-weight: 600;
  letter-spacing: 1px;
}

.submit-tip {
  font-size: 12px;
  color: #94a3b8;
}

.expired-alert-banner {
  margin-bottom: 20px;
}

.text-expired-tip {
  color: #d97706;
  font-weight: 500;
}

/* 页脚 */
.gov-footer {
  text-align: center;
  padding: 36px 16px 24px;
  color: #64748b;
  font-size: 13px;
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
  background: #b22222;
  border-radius: 50%;
}

.footer-org {
  font-size: 14px;
  font-weight: 600;
  color: #475569;
}

.footer-system {
  font-size: 12px;
  color: #94a3b8;
  margin-bottom: 6px;
}

.footer-beian {
  font-size: 12px;
  color: #94a3b8;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  flex-wrap: wrap;
}

.footer-beian a {
  color: #94a3b8;
  text-decoration: none;
}

.footer-beian a:hover {
  color: #b22222;
}

.sep {
  color: #cbd5e1;
}

.ipv6-tag {
  color: #94a3b8;
}

@media (max-width: 640px) {
  .dossier-card {
    padding: 20px 16px;
  }
  .dossier-title {
    font-size: 20px;
  }
  .dispatch-dossier-grid {
    grid-template-columns: 1fr;
  }
  .unit-picker-box {
    flex-direction: column;
    width: 100%;
  }
  .btn-confirm-unit {
    width: 100%;
  }
  .btn-submit-receipt {
    width: 100%;
  }
  .voucher-details-grid {
    grid-template-columns: 1fr;
  }
  .official-seal-wrap {
    top: 8px;
    right: 8px;
    transform: scale(0.8);
  }
}
</style>
