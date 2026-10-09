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
          <div class="brand-sup font-serif">中共伊宁县委宣传部 · 数字化协同政务服务平台</div>
          <div class="brand-sub">官方文件征求意见与盖章回函系统</div>
        </div>
      </div>
    </div>

    <!-- 主体容器 -->
    <div class="main-container">
      <template v-if="solicit">
        <!-- 核心公文通告卡片 -->
        <div class="dossier-card">
          <!-- 顶部红金装饰带 -->
          <div class="card-gold-crest-bar"></div>

          <!-- 状态与倒计时标签栏 -->
          <div class="status-ribbon-row">
            <div class="status-pill" :class="expired ? 'is-expired' : 'is-active'">
              <span class="pulse-dot" v-if="!expired"></span>
              <span>{{ expired ? '意见征集已截止' : '征集中 · 实时在线反馈' }}</span>
            </div>
            <div class="doc-no-pill" v-if="solicit.doc_no">
              <el-icon><Tickets /></el-icon>
              <span>{{ solicit.doc_no }}</span>
            </div>
            <div class="countdown-pill" v-if="!expired && countdownText">
              <el-icon><Clock /></el-icon>
              <span>{{ countdownText }}</span>
            </div>
          </div>

          <!-- 文件公文主标题 -->
          <h1 class="dossier-title font-serif">{{ solicit.title }}</h1>

          <!-- 发文信息网格 -->
          <div class="meeting-dossier-grid">
            <div class="dossier-item">
              <div class="dossier-icon time-icon">
                <el-icon><Calendar /></el-icon>
              </div>
              <div class="dossier-text">
                <span class="dossier-label">征集截止时间</span>
                <span class="dossier-val font-serif">{{ solicit.deadline }}</span>
              </div>
            </div>

            <div class="dossier-item">
              <div class="dossier-icon org-icon">
                <el-icon><OfficeBuilding /></el-icon>
              </div>
              <div class="dossier-text">
                <span class="dossier-label">牵头单位</span>
                <span class="dossier-val font-serif">中共伊宁县委宣传部</span>
              </div>
            </div>

            <div class="dossier-item item-full-width" v-if="solicit.content">
              <div class="dossier-icon place-icon">
                <el-icon><InfoFilled /></el-icon>
              </div>
              <div class="dossier-text">
                <span class="dossier-label">审阅要求与说明</span>
                <span class="dossier-val font-serif">{{ solicit.content }}</span>
              </div>
            </div>
          </div>

          <!-- 已截止严正提示 -->
          <div v-if="expired" class="expired-banner">
            <div class="banner-icon-col">
              <el-icon :size="24"><Warning /></el-icon>
            </div>
            <div class="banner-text-col">
              <h4>本次文件材料意见征集已截止</h4>
              <p>如需紧急补报或调整盖章回函，请直接致电宣传部办公室进行线下联系。</p>
            </div>
          </div>

          <!-- ================= 在线查看 PDF 底稿舱 ================= -->
          <div class="pdf-viewer-section">
            <div class="section-lead-bar">
              <div class="lead-left">
                <el-icon class="lead-icon"><Document /></el-icon>
                <span class="lead-text font-serif">正文底稿在线查阅</span>
                <span class="file-name-tag">{{ solicit.pdf_name || '正文底稿.pdf' }}</span>
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
                  v-if="solicit.word_path"
                  type="success" 
                  plain 
                  size="small"
                  :icon="'Document'"
                  @click="downloadWord"
                >
                  下载 Word 修改稿
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

          <!-- ================= 填报流程区 ================= -->
          <template v-if="!expired">
            <div class="flow-divider">
              <span class="divider-line"></span>
              <span class="divider-text font-serif">意 见 与 盖 章 回 函 填 报</span>
              <span class="divider-line"></span>
            </div>

            <!-- 第一阶段：未选定单位 -->
            <div class="unit-select-card" v-if="!unitSelected">
              <div class="select-lead">
                <div class="lead-badge">第 1 步</div>
                <div class="lead-title">选择您的所属填报单位</div>
                <div class="lead-desc">全县各乡镇场、各部门名单实时匹配校验</div>
              </div>

              <div class="unit-picker-box">
                <el-select 
                  v-model="pendingUnit" 
                  filterable 
                  size="large"
                  placeholder="请在此输入或选择您的单位全称..." 
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
                <span>选定单位后，系统将自动核验并显示该单位当前的反馈与回函状态</span>
              </div>
            </div>

            <!-- 第二阶段：已选定单位 -->
            <div class="unit-active-area" v-else>
              <!-- 专属单位授牌栏 -->
              <div class="unit-honor-bar">
                <div class="honor-left">
                  <span class="honor-tag">已选定填报单位</span>
                  <h3 class="honor-name font-serif">{{ currentUnit }}</h3>
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

              <!-- 官方正式回执凭证卡片（提交成功或已有填报时展示，锁定防篡改） -->
              <transition name="fade-bounce">
                <div v-if="submitted" class="receipt-dossier">
                  <div class="receipt-stamp">
                    <div class="stamp-circle">
                      <span>已归档存证</span>
                      <small>ARCHIVED</small>
                    </div>
                  </div>
                  <div class="receipt-top">
                    <span class="receipt-badge">电子回执凭证</span>
                    <span class="receipt-badge-locked">
                      <el-icon><Lock /></el-icon> 提交即锁定 · 防篡改保护
                    </span>
                    <span class="receipt-time">{{ receiptTimeText }}</span>
                  </div>
                  <div class="receipt-main">
                    <h4 class="receipt-status-title font-serif">{{ currentUnit }} · 已正式回函</h4>
                    
                    <div class="receipt-detail-grid">
                      <div class="detail-row">
                        <span class="detail-label">审阅结论：</span>
                        <span class="detail-val" :class="submittedFeedback?.has_opinion === 1 ? 'has-opinion' : 'no-opinion'">
                          <el-icon><Check v-if="submittedFeedback?.has_opinion === 0" /><Edit v-else /></el-icon>
                          {{ submittedFeedback?.has_opinion === 1 ? '提出修改意见与建议' : '原则同意，无修改意见' }}
                        </span>
                      </div>

                      <div class="detail-row" v-if="submittedFeedback?.has_opinion === 1 && submittedFeedback?.opinion_detail">
                        <span class="detail-label">意见明细：</span>
                        <div class="detail-box">{{ submittedFeedback.opinion_detail }}</div>
                      </div>

                      <div class="detail-row" v-if="submittedFeedback?.reply_doc_name">
                        <span class="detail-label">盖章公函：</span>
                        <span class="detail-val file-tag">
                          <el-icon><DocumentChecked /></el-icon>
                          {{ submittedFeedback.reply_doc_name }}
                        </span>
                      </div>

                      <div class="detail-row" v-if="submittedFeedback?.attachment_name">
                        <span class="detail-label">修改稿附件：</span>
                        <span class="detail-val file-tag">
                          <el-icon><Document /></el-icon>
                          {{ submittedFeedback.attachment_name }}
                        </span>
                      </div>

                      <div class="detail-row" v-if="submittedFeedback?.contact_name">
                        <span class="detail-label">经办人员：</span>
                        <span class="detail-val">
                          {{ submittedFeedback.contact_name }}（{{ submittedFeedback.contact_phone }}）
                        </span>
                      </div>
                    </div>

                    <!-- 防篡改锁定公告横幅 -->
                    <div class="receipt-lock-notice">
                      <div class="lock-notice-header">
                        <el-icon class="lock-icon"><WarningFilled /></el-icon>
                        <span class="lock-title">正式公文防恶意修改与归档锁定声明</span>
                      </div>
                      <p class="lock-desc">
                        为确保公文征求意见的法律效力与严肃性，防止外部他人随意篡改，已提交的盖章回函与修改意见已即时<strong>存证归档并锁定</strong>。公开端已关闭自行撤回或编辑；如确因重大特殊情况需变更意见或补充材料，请联系<strong>宣传部办公室</strong>核实，由管理员在后台安全重置解锁后方可重新填报。
                      </p>
                    </div>
                  </div>

                </div>
              </transition>

              <!-- 填报表单区（仅未提交时展示） -->
              <div v-if="!submitted" class="feedback-form-box">

                <!-- 步骤 2：意见类型确认 -->
                <div class="switch-container">
                  <div class="switch-lead font-serif">1. 审阅意见结论：</div>
                  <el-radio-group v-model="form.has_opinion" size="large" class="custom-radio-group">
                    <el-radio-button :value="0">
                      <span class="radio-label-inner">
                        <el-icon><Check /></el-icon> 原则同意，无修改意见
                      </span>
                    </el-radio-button>
                    <el-radio-button :value="1">
                      <span class="radio-label-inner">
                        <el-icon><Edit /></el-icon> 提出修改意见与建议
                      </span>
                    </el-radio-button>
                  </el-radio-group>
                </div>

                <!-- 步骤 3：修改意见输入（有意见时展开） -->
                <transition name="fade">
                  <div v-if="form.has_opinion === 1" class="opinion-input-card">
                    <div class="input-card-header font-serif">
                      <span class="header-mark"></span>
                      <h4>填写具体修改建议（可分条列明）：</h4>
                    </div>
                    <el-input 
                      v-model="form.opinion_detail" 
                      type="textarea" 
                      :rows="5"
                      placeholder="例：&#10;1. 建议将第 3 页第 2 段中的“...”修改为“...”；&#10;2. 建议在第 5 条保障措施中增加各乡镇宣传骨干队伍建设内容。"
                      class="custom-opinion-textarea"
                    />

                    <!-- 可选上传标红修改稿 -->
                    <div class="modify-doc-uploader">
                      <div class="doc-upload-label">
                        <span>标红修改稿文档（选填）：</span>
                        <small>若有本地排版标红的修改稿，可一并上传</small>
                      </div>
                      <div v-if="form.attachment_path" class="uploaded-doc-badge">
                        <el-icon><Document /></el-icon>
                        <span class="badge-text">{{ form.attachment_name }}</span>
                        <el-button link type="danger" size="small" @click="removeModifyDoc">删除</el-button>
                      </div>
                      <el-upload
                        v-else
                        class="doc-upload-btn"
                        :action="`/api/public/solicits/${solicit.id}/upload`"
                        :show-file-list="false"
                        :before-upload="beforeUploadDoc"
                        :on-success="handleUploadDocSuccess"
                        :on-error="handleUploadError"
                        accept=".doc,.docx,.pdf"
                      >
                        <el-button size="small" plain :icon="'Upload'">上传修改稿附件 (Word/PDF)</el-button>
                      </el-upload>
                    </div>
                  </div>
                </transition>

                <!-- 步骤 4：上传盖章红头回函（核心必填卡片） -->
                <div class="reply-upload-card">
                  <div class="reply-card-header">
                    <div class="header-badge-tag font-serif">核心凭证 · 必须上传</div>
                    <h4 class="font-serif">2. 上传经主要领导审签并加盖公章的红头回函</h4>
                    <p class="reply-card-sub">
                      请将本单位书面公函或意见反馈表（经主要领导签字、加盖单位党委/政府公章），扫描或清晰拍照后上传（支持 PDF、JPG、PNG 格式）。
                    </p>
                  </div>

                  <!-- 高亮加粗保密红线警示条 -->
                  <div class="secrecy-public-banner">
                    <el-icon class="sec-warn-icon"><WarningFilled /></el-icon>
                    <div class="sec-warn-text">
                      <strong>【保密红线 · 严正警示】</strong>
                      本平台为非涉密互联网政务系统，<strong>严禁上传任何标注为国家秘密（秘密、机密、绝密）以及工作秘密、内部级的文件资料！</strong>
                      各单位公函及修改稿务必做脱密处理。系统已部署涉密特征智能监测，违规上传将触发<strong>安全熔断、阻断并锁定会话</strong>，违规行为即时上报安全审计中心！
                    </div>
                  </div>

                  <div class="reply-upload-area">
                    <div v-if="form.reply_doc_path" class="uploaded-reply-box">
                      <div class="reply-file-info">
                        <el-icon class="reply-file-icon"><DocumentChecked /></el-icon>
                        <div class="reply-file-text">
                          <span class="reply-name">{{ form.reply_doc_name }}</span>
                          <span class="reply-status-ok">已成功上传</span>
                        </div>
                      </div>
                      <div class="reply-actions">
                        <el-button link type="danger" size="small" @click="removeReplyDoc">重新上传</el-button>
                      </div>
                    </div>

                    <el-upload
                      v-else
                      class="reply-upload-dropzone"
                      drag
                      :action="`/api/public/solicits/${solicit.id}/upload`"
                      :show-file-list="false"
                      :before-upload="beforeUploadReply"
                      :on-success="handleUploadReplySuccess"
                      :on-error="handleUploadError"
                      accept=".pdf,.jpg,.jpeg,.png,.doc,.docx"
                    >
                      <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
                      <div class="el-upload__text">
                        将盖章公函扫描件/照片 <em>拖到此处，或点击上传</em>
                      </div>
                      <template #tip>
                        <div class="el-upload__tip">
                          支持手机拍照上传或扫描件 PDF/JPG/PNG，单文件不得超过 20MB
                        </div>
                      </template>
                    </el-upload>
                  </div>
                </div>

                <!-- 步骤 5：经办人姓名与电话 -->
                <div class="contact-card">
                  <div class="contact-header font-serif">3. 经办人联系方式（便于宣传部对口沟通）</div>
                  <el-row :gutter="14">
                    <el-col :xs="24" :sm="12">
                      <el-form-item label="经办人真实姓名" required>
                        <el-input 
                          v-model="form.contact_name" 
                          placeholder="请填写干部姓名" 
                          size="large"
                          class="gov-input"
                        >
                          <template #prefix><el-icon><User /></el-icon></template>
                        </el-input>
                      </el-form-item>
                    </el-col>
                    <el-col :xs="24" :sm="12">
                      <el-form-item label="联系手机电话" required>
                        <el-input 
                          v-model="form.contact_phone" 
                          maxlength="11" 
                          placeholder="请填写11位有效手机号" 
                          size="large"
                          class="gov-input"
                        >
                          <template #prefix><el-icon><Phone /></el-icon></template>
                        </el-input>
                      </el-form-item>
                    </el-col>
                  </el-row>
                </div>

                <!-- 提交按钮 -->
                <div class="submit-action-wrap">
                  <el-button 
                    type="primary" 
                    size="large" 
                    class="gov-submit-btn font-serif" 
                    :loading="submitting"
                    @click="handleSubmitFeedback"
                  >
                    确认提交盖章红头回函与意见
                  </el-button>
                  <div class="submit-tip-text">
                    点击提交后，宣传部平台将即时生成正式电子回执凭证
                  </div>
                </div>
              </div>
            </div>
          </template>
        </div>
      </template>

      <!-- 尊贵政务页脚与技术背书 -->
      <footer class="gov-footer">
        <div class="footer-crest-row">
          <span class="crest-dot"></span>
          <span class="footer-org font-serif">中共伊宁县委宣传部</span>
          <span class="crest-dot"></span>
        </div>
        <div class="footer-system">部务工作平台 · 数字化协同政务服务终端 V1.7.0</div>
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
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { 
  Calendar, OfficeBuilding, Clock, User, Check, Warning, WarningFilled, Lock,
  InfoFilled, Tickets, Phone, Edit, Document, 
  DocumentChecked, FullScreen, Download, UploadFilled, View 
} from '@element-plus/icons-vue'
import request from '../utils/request'
import { validateUploadFileSecrecy, handleSecrecyUploadError } from '../utils/secrecyGuard'
import dayjs from 'dayjs'

const route = useRoute()
const solicitId = route.params.id

const solicit = ref(null)
const units = ref([])
const expired = ref(false)

const pendingUnit = ref('')
const currentUnit = ref('')
const unitSelected = computed(() => !!currentUnit.value)

const submitted = ref(false)
const submittedFeedback = ref(null)
const submitting = ref(false)

const form = ref({
  has_opinion: 0,
  opinion_detail: '',
  attachment_path: '',
  attachment_name: '',
  reply_doc_path: '',
  reply_doc_name: '',
  contact_name: '',
  contact_phone: ''
})

const pdfEmbedUrl = computed(() => {
  if (!solicit.value?.id) return ''
  return `/api/public/solicits/${solicit.value.id}/file?type=pdf&mode=inline`
})

const countdownText = computed(() => {
  if (!solicit.value?.deadline) return ''
  const end = dayjs(solicit.value.deadline)
  const now = dayjs()
  if (now.isAfter(end)) return ''
  const diffHours = end.diff(now, 'hour')
  const diffDays = Math.floor(diffHours / 24)
  const remainHours = diffHours % 24
  if (diffDays > 0) {
    return `距截止还剩 ${diffDays} 天 ${remainHours} 小时`
  }
  const diffMins = end.diff(now, 'minute')
  if (diffHours > 0) {
    return `距截止还剩 ${diffHours} 小时`
  }
  return `距截止仅剩 ${diffMins} 分钟`
})

const receiptTimeText = computed(() => {
  if (!submittedFeedback.value?.created_at) return dayjs().format('YYYY-MM-DD HH:mm')
  return dayjs(submittedFeedback.value.created_at).format('YYYY-MM-DD HH:mm')
})

const loadSolicit = async (unitParam = '') => {
  try {
    const res = await request.get(`/public/solicits/${solicitId}`, {
      params: unitParam ? { unit: unitParam } : {}
    })
    solicit.value = res.solicit
    units.value = res.units || []
    expired.value = res.expired

    if (res.current_feedback) {
      submittedFeedback.value = res.current_feedback
      submitted.value = true
      form.value = {
        has_opinion: res.current_feedback.has_opinion || 0,
        opinion_detail: res.current_feedback.opinion_detail || '',
        attachment_path: res.current_feedback.attachment_path || '',
        attachment_name: res.current_feedback.attachment_name || '',
        reply_doc_path: res.current_feedback.reply_doc_path || '',
        reply_doc_name: res.current_feedback.reply_doc_name || '',
        contact_name: res.current_feedback.contact_name || '',
        contact_phone: res.current_feedback.contact_phone || ''
      }
    } else if (unitParam) {
      submitted.value = false
      submittedFeedback.value = null
    }
  } catch (e) {
    console.error(e)
    ElMessage.error('无法加载该征求意见文件，可能不存在或已下线')
  }
}

const onUnitChosen = (val) => {
  currentUnit.value = val
  loadSolicit(val)
}

const changeUnit = () => {
  currentUnit.value = ''
  pendingUnit.value = ''
  submitted.value = false
  submittedFeedback.value = null
}

const openPdfWindow = () => {
  window.open(pdfEmbedUrl.value, '_blank')
}

const downloadPdf = () => {
  window.open(`/api/public/solicits/${solicit.value.id}/file?type=pdf&mode=download`, '_blank')
}

const downloadWord = () => {
  window.open(`/api/public/solicits/${solicit.value.id}/file?type=word&mode=download`, '_blank')
}

// 标红修改稿附件
const beforeUploadDoc = (file) => {
  if (!validateUploadFileSecrecy(file, { isPublic: true })) {
    return false
  }
  const isLt20M = file.size / 1024 / 1024 < 20
  if (!isLt20M) {
    ElMessage.error('附件大小不能超过 20MB')
    return false
  }
  return true
}

const handleUploadDocSuccess = (res) => {
  form.value.attachment_path = res.file_path
  form.value.attachment_name = res.file_name
  ElMessage.success('修改稿附件上传成功')
}

const removeModifyDoc = () => {
  form.value.attachment_path = ''
  form.value.attachment_name = ''
}

// 盖章红头回函
const beforeUploadReply = (file) => {
  if (!validateUploadFileSecrecy(file, { isPublic: true })) {
    return false
  }
  const isLt20M = file.size / 1024 / 1024 < 20
  if (!isLt20M) {
    ElMessage.error('盖章公函大小不能超过 20MB')
    return false
  }
  return true
}

const handleUploadReplySuccess = (res) => {
  form.value.reply_doc_path = res.file_path
  form.value.reply_doc_name = res.file_name
  ElMessage.success('盖章红头回函上传成功')
}

const removeReplyDoc = () => {
  form.value.reply_doc_path = ''
  form.value.reply_doc_name = ''
}

const handleUploadError = (err, file) => {
  if (handleSecrecyUploadError(err, file, { isPublic: true })) return
  ElMessage.error('上传失败，请检查文件大小或网络')
}

const handleSubmitFeedback = async () => {
  if (!currentUnit.value) {
    ElMessage.warning('请选择所属单位')
    return
  }
  if (form.value.has_opinion === 1 && !form.value.opinion_detail.trim()) {
    ElMessage.warning('请填写具体修改建议说明')
    return
  }
  if (!form.value.reply_doc_path) {
    ElMessage.warning('请上传经单位主要领导审签并加盖公章的正式回函')
    return
  }
  if (!form.value.contact_name.trim()) {
    ElMessage.warning('请填写经办人姓名')
    return
  }
  if (!form.value.contact_phone.trim() || form.value.contact_phone.trim().length !== 11) {
    ElMessage.warning('请填写11位有效联系手机号')
    return
  }

  try {
    await ElMessageBox.confirm(
      `确认正式提交【${currentUnit.value}】对本次征求意见的审签盖章回函吗？\n\n【防恶意修改提示】\n公函提交后系统将即时锁定归档，公开端无法自行撤回或修改；如后续确需变更，须联系宣传部管理员在后台重置解锁。`,
      '提交确认与锁定提示',
      { 
        confirmButtonText: '确认提交并锁定', 
        cancelButtonText: '核对修改', 
        type: 'warning' 
      }
    )
  } catch {
    return
  }

  submitting.value = true
  try {
    const payload = {
      unit: currentUnit.value,
      has_opinion: form.value.has_opinion,
      opinion_detail: form.value.opinion_detail,
      reply_doc_path: form.value.reply_doc_path,
      reply_doc_name: form.value.reply_doc_name,
      attachment_path: form.value.attachment_path,
      attachment_name: form.value.attachment_name,
      contact_name: form.value.contact_name,
      contact_phone: form.value.contact_phone
    }
    const res = await request.post(`/public/solicits/${solicit.value.id}/feedback`, payload)
    ElMessage.success('回函与意见已成功提交并归档')
    submittedFeedback.value = {
      ...payload,
      created_at: res.time
    }
    submitted.value = true
    window.scrollTo({ top: 400, behavior: 'smooth' })
  } catch (e) {
    console.error(e)
    ElMessage.error(e.response?.data?.error || '提交失败，请重试')
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  loadSolicit()
})
</script>

<style scoped>
/* 延续全站顶级红金政务天际线 */
.gov-reg-wrapper {
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

/* 顶部天际线背景区 - 与会议填报页 100% 对齐 */
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
  max-width: 720px;
  width: 100%;
  margin: -70px auto 40px;
  padding: 0 16px;
  box-sizing: border-box;
  position: relative;
  z-index: 3;
  flex: 1;
}

.dossier-card {
  background: #ffffff;
  border-radius: 14px;
  box-shadow: 0 12px 36px rgba(0, 0, 0, 0.08), 0 2px 8px rgba(0,0,0,0.03);
  padding: 32px;
  position: relative;
  border: 1px solid rgba(220, 223, 230, 0.6);
}

.card-gold-crest-bar {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 4px;
  background: linear-gradient(90deg, #9e1b1e 0%, #f5d499 50%, #9e1b1e 100%);
  border-radius: 14px 14px 0 0;
}

.status-ribbon-row {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
  margin-bottom: 20px;
}

.status-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 600;
}

.status-pill.is-active {
  background: #e8f5e9;
  color: #2e7d32;
  border: 1px solid #c8e6c9;
}

.status-pill.is-expired {
  background: #f5f5f5;
  color: #757575;
  border: 1px solid #e0e0e0;
}

.pulse-dot {
  width: 8px;
  height: 8px;
  background-color: #2e7d32;
  border-radius: 50%;
  animation: pulse 1.8s infinite;
}

@keyframes pulse {
  0% { transform: scale(0.9); opacity: 0.8; }
  50% { transform: scale(1.3); opacity: 1; }
  100% { transform: scale(0.9); opacity: 0.8; }
}

.doc-no-pill, .countdown-pill {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 4px 12px;
  border-radius: 20px;
  font-size: 12px;
}

.doc-no-pill {
  background: #edf2f7;
  color: #4a5568;
}

.countdown-pill {
  background: #fff8e1;
  color: #b78103;
  border: 1px solid #ffe082;
  font-weight: 600;
}

.dossier-title {
  font-size: 24px;
  color: #1a1a1a;
  line-height: 1.45;
  margin-bottom: 24px;
  letter-spacing: 0.5px;
}

.meeting-dossier-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  background: #fcfcfd;
  border: 1px solid #edf0f5;
  border-radius: 10px;
  padding: 16px 20px;
  margin-bottom: 24px;
}

.item-full-width {
  grid-column: span 2;
}

.dossier-item {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.dossier-icon {
  width: 36px;
  height: 36px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  flex-shrink: 0;
}

.time-icon { background: #fee2e2; color: #b91c1c; }
.org-icon { background: #e0e7ff; color: #3730a3; }
.place-icon { background: #fef3c7; color: #92400e; }

.dossier-text {
  display: flex;
  flex-direction: column;
}

.dossier-label {
  font-size: 12px;
  color: #8c8c8c;
}

.dossier-val {
  font-size: 14px;
  font-weight: 600;
  color: #262626;
  margin-top: 2px;
  line-height: 1.5;
}

.expired-banner {
  display: flex;
  align-items: center;
  gap: 16px;
  background: #fff2f0;
  border: 1px solid #ffccc7;
  border-radius: 8px;
  padding: 16px 20px;
  margin-bottom: 24px;
  color: #cf1322;
}

.expired-banner h4 {
  margin: 0 0 4px 0;
  font-size: 15px;
}

.expired-banner p {
  margin: 0;
  font-size: 13px;
  opacity: 0.85;
}

/* 在线浏览 PDF 区域 */
.pdf-viewer-section {
  background: #fdfdfd;
  border: 1px solid #e9ecef;
  border-radius: 12px;
  padding: 16px;
  margin-bottom: 28px;
}

.section-lead-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
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
  color: #9e1b1e;
}

.lead-text {
  font-size: 16px;
  font-weight: 700;
  color: #24292e;
}

.file-name-tag {
  font-size: 12px;
  background: #f0f2f5;
  color: #555;
  padding: 2px 8px;
  border-radius: 4px;
}

.lead-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.pdf-embed-wrapper {
  width: 100%;
  border-radius: 8px;
  overflow: hidden;
  border: 1px solid #dcdfe6;
  background: #ffffff;
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.06);
}

.pdf-iframe-player {
  width: 100%;
  height: 580px;
  border: none;
  display: block;
  background: #ffffff;
}

.pdf-mobile-fallback {
  background: #fff;
  padding: 12px;
  text-align: center;
  font-size: 13px;
  color: #666;
  border-top: 1px solid #eee;
}

.flow-divider {
  display: flex;
  align-items: center;
  margin: 32px 0 24px 0;
}

.divider-line {
  flex: 1;
  height: 1px;
  background: linear-gradient(90deg, transparent, #e0e0e0, transparent);
}

.divider-text {
  padding: 0 16px;
  font-size: 16px;
  font-weight: 700;
  color: #9e1b1e;
  letter-spacing: 2px;
}

/* 单位选择卡片 */
.unit-select-card {
  background: #f8fafc;
  border: 1.5px dashed #cbd5e1;
  border-radius: 12px;
  padding: 24px;
  text-align: center;
}

.select-lead {
  margin-bottom: 16px;
}

.lead-badge {
  display: inline-block;
  background: #9e1b1e;
  color: #fff;
  font-size: 11px;
  font-weight: 700;
  padding: 2px 8px;
  border-radius: 10px;
  margin-bottom: 6px;
}

.lead-title {
  font-size: 18px;
  font-weight: 700;
  color: #1e293b;
}

.lead-desc {
  font-size: 13px;
  color: #64748b;
  margin-top: 4px;
}

.unit-picker-box {
  max-width: 480px;
  margin: 0 auto 12px auto;
}

.custom-unit-select {
  width: 100%;
}

.select-foot-tips {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  font-size: 12px;
  color: #64748b;
}

/* 选定单位授牌栏 */
.unit-honor-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: linear-gradient(135deg, #fdfbfb 0%, #f7f3f3 100%);
  border: 1px solid #eddada;
  border-radius: 10px;
  padding: 14px 20px;
  margin-bottom: 24px;
}

.honor-tag {
  font-size: 11px;
  color: #9e1b1e;
  background: #fbebeb;
  padding: 2px 6px;
  border-radius: 4px;
  font-weight: 600;
}

.honor-name {
  font-size: 19px;
  color: #262626;
  margin: 4px 0 0 0;
}

/* 回执单盖章样式 */
.receipt-dossier {
  background: #fffdf9;
  border: 1.5px solid #faeccd;
  border-radius: 12px;
  padding: 24px;
  position: relative;
  overflow: hidden;
  margin-bottom: 24px;
  box-shadow: 0 4px 16px rgba(220, 160, 50, 0.08);
}

.receipt-stamp {
  position: absolute;
  top: 16px;
  right: 20px;
  pointer-events: none;
}

.stamp-circle {
  width: 100px;
  height: 100px;
  border: 3px solid rgba(200, 30, 30, 0.7);
  border-radius: 50%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: rgba(200, 30, 30, 0.75);
  font-weight: 700;
  transform: rotate(-15deg);
  letter-spacing: 1px;
  background: radial-gradient(circle, rgba(255,255,255,0.8) 0%, transparent 80%);
}

.stamp-circle span {
  font-size: 13px;
}

.stamp-circle small {
  font-size: 9px;
  letter-spacing: 0.5px;
}

.receipt-top {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.receipt-badge {
  background: #9e1b1e;
  color: #fff;
  font-size: 11px;
  font-weight: 700;
  padding: 2px 8px;
  border-radius: 4px;
}

.receipt-badge-locked {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: #fff7e6;
  border: 1px solid #ffd591;
  color: #d46b08;
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 4px;
}

.receipt-time {
  font-size: 12px;
  color: #8c8c8c;
}

.receipt-status-title {
  font-size: 18px;
  color: #1a1a1a;
  margin: 0 0 14px 0;
}

.receipt-detail-grid {
  display: flex;
  flex-direction: column;
  gap: 10px;
  background: #ffffff;
  border: 1px solid #faeccd;
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 16px;
}

.detail-row {
  display: flex;
  align-items: flex-start;
  font-size: 14px;
  line-height: 1.6;
}

.detail-label {
  width: 90px;
  flex-shrink: 0;
  color: #8c8c8c;
  font-weight: 500;
}

.detail-val {
  color: #262626;
  font-weight: 500;
  display: flex;
  align-items: center;
  gap: 6px;
}

.detail-val.has-opinion {
  color: #d46b08;
}

.detail-val.no-opinion {
  color: #389e0d;
}

.detail-val.file-tag {
  color: #1d39c4;
  background: #f0f5ff;
  border: 1px solid #d6e4ff;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 13px;
  display: inline-flex;
  align-items: center;
  word-break: break-all;
}

.detail-box {
  flex: 1;
  background: #fcfcfc;
  border: 1px solid #f0f0f0;
  border-radius: 6px;
  padding: 8px 12px;
  font-size: 13px;
  color: #333;
  line-height: 1.6;
  white-space: pre-wrap;
}

.receipt-lock-notice {
  background: #fcf8ee;
  border: 1px solid #faecd8;
  border-radius: 8px;
  padding: 14px 16px;
  margin-top: 14px;
}

.lock-notice-header {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #b32400;
  font-weight: 600;
  font-size: 13px;
  margin-bottom: 6px;
}

.lock-icon {
  font-size: 16px;
  color: #d4380d;
}

.lock-title {
  letter-spacing: 0.5px;
}

.lock-desc {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: #614700;
}

.lock-desc strong {
  color: #9e1b1e;
}

.receipt-quick-actions {
  display: flex;
  gap: 12px;
  margin-top: 16px;
  flex-wrap: wrap;
}

/* 填报表单 */
.feedback-form-box {
  background: #ffffff;
}

.form-lead-alert {
  display: flex;
  align-items: center;
  gap: 8px;
  background: #e6f7ff;
  border: 1px solid #91d5ff;
  border-radius: 6px;
  padding: 10px 14px;
  font-size: 13px;
  color: #1890ff;
  margin-bottom: 20px;
}

.switch-container {
  margin-bottom: 20px;
}

.switch-lead {
  font-size: 15px;
  font-weight: 700;
  color: #24292e;
  margin-bottom: 10px;
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
  padding: 12px 16px;
  font-size: 14px;
  font-weight: 600;
}

.radio-label-inner {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
}

.opinion-input-card {
  background: #fcfcfc;
  border: 1px solid #eaedf1;
  border-radius: 10px;
  padding: 16px;
  margin-bottom: 24px;
}

.input-card-header {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 12px;
}

.header-mark {
  width: 4px;
  height: 16px;
  background: #9e1b1e;
  border-radius: 2px;
}

.input-card-header h4 {
  margin: 0;
  font-size: 15px;
  color: #24292e;
}

.custom-opinion-textarea {
  margin-bottom: 12px;
}

.modify-doc-uploader {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: #fff;
  border: 1px dashed #dcdfe6;
  border-radius: 6px;
  padding: 10px 14px;
  flex-wrap: wrap;
  gap: 10px;
}

.doc-upload-label {
  display: flex;
  flex-direction: column;
  font-size: 13px;
  color: #333;
}

.doc-upload-label small {
  color: #888;
  font-size: 11px;
}

.uploaded-doc-badge {
  display: flex;
  align-items: center;
  gap: 8px;
  background: #f0f9eb;
  border: 1px solid #e1f3d8;
  border-radius: 4px;
  padding: 4px 10px;
  font-size: 12px;
  color: #67c23a;
}

/* 盖章红头回函上传卡片 */
.reply-upload-card {
  background: #fffaf9;
  border: 1.5px solid #f6d1cc;
  border-radius: 12px;
  padding: 22px;
  margin-bottom: 24px;
}

.reply-card-header {
  margin-bottom: 16px;
}

.header-badge-tag {
  display: inline-block;
  background: #9e1b1e;
  color: #fff;
  font-size: 11px;
  font-weight: 700;
  padding: 2px 8px;
  border-radius: 4px;
  margin-bottom: 6px;
}

.reply-card-header h4 {
  font-size: 17px;
  color: #9e1b1e;
  margin: 0 0 6px 0;
}

.reply-card-sub {
  font-size: 13px;
  color: #666;
  margin: 0;
  line-height: 1.5;
}

.uploaded-reply-box {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: #ffffff;
  border: 1.5px solid #67c23a;
  border-radius: 8px;
  padding: 14px 18px;
}

.reply-file-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

.reply-file-icon {
  font-size: 28px;
  color: #67c23a;
}

.reply-file-text {
  display: flex;
  flex-direction: column;
}

.reply-name {
  font-size: 14px;
  font-weight: 600;
  color: #24292e;
}

.reply-status-ok {
  font-size: 12px;
  color: #67c23a;
}

.reply-upload-dropzone :deep(.el-upload-dragger) {
  padding: 24px;
  border-radius: 8px;
  background: #fff;
  border: 1.5px dashed #f1a89f;
}

.contact-card {
  background: #fbfbfc;
  border: 1px solid #eaedf1;
  border-radius: 10px;
  padding: 18px 20px 8px 20px;
  margin-bottom: 28px;
}

.contact-header {
  font-size: 15px;
  font-weight: 700;
  color: #24292e;
  margin-bottom: 14px;
}

.gov-input :deep(.el-input__inner) {
  font-size: 16px !important; /* 锁定 16px 防 iOS Safari 放大 */
}

.submit-action-wrap {
  text-align: center;
  padding: 12px 0;
}

.gov-submit-btn {
  width: 100%;
  max-width: 420px;
  height: 50px;
  font-size: 17px;
  font-weight: 700;
  border-radius: 25px;
  background: linear-gradient(135deg, #9e1b1e 0%, #b22222 100%);
  border: none;
  box-shadow: 0 6px 18px rgba(158, 27, 30, 0.3);
  transition: transform 0.2s, box-shadow 0.2s;
}

.gov-submit-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 8px 22px rgba(158, 27, 30, 0.4);
}

.submit-tip-text {
  font-size: 12px;
  color: #8c8c8c;
  margin-top: 10px;
}

/* 页脚 */
.gov-footer {
  text-align: center;
  padding: 30px 20px 40px;
  color: #8c8c8c;
  font-size: 12px;
  line-height: 1.8;
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
  background-color: #9e1b1e;
  border-radius: 50%;
  opacity: 0.6;
}

.footer-org {
  color: #4b5563;
  font-weight: 600;
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

/* 响应式适配 */
@media (max-width: 768px) {
  .skyline-bg {
    height: 190px;
  }
  .brand-sup {
    font-size: 16px;
  }
  .main-container {
    margin-top: -65px;
  }
  .dossier-card {
    padding: 22px 16px;
  }
  .dossier-title {
    font-size: 20px;
  }
  .meeting-dossier-grid {
    grid-template-columns: 1fr;
  }
  .item-full-width {
    grid-column: span 1;
  }
  .pdf-iframe-player {
    height: 380px;
  }
  .unit-honor-bar {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }
  .change-unit-btn {
    align-self: flex-end;
  }
}

/* 高亮加粗保密防线警示条 */
.secrecy-public-banner {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  background: #fef2f2;
  border: 1.5px solid #f87171;
  border-radius: 6px;
  padding: 12px 16px;
  margin-bottom: 16px;
}

.sec-warn-icon {
  font-size: 22px;
  color: #dc2626;
  margin-top: 2px;
  flex-shrink: 0;
}

.sec-warn-text {
  font-size: 13.5px;
  color: #991b1b;
  line-height: 1.6;
}

.sec-warn-text strong {
  color: #b91c1c;
  font-weight: 700;
}
</style>
