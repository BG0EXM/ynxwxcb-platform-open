<template>
  <div class="dispatches-page">
    <!-- 页头 -->
    <page-header 
      title="材料下发" 
      subtitle="向下辖乡镇场与部门单位下发官方文件与学习材料，各单位轻量查收签收，实时统计查收进度"
      :tag="total ? `共 ${total} 项材料` : ''"
    >
      <template #actions>
        <el-button type="primary" :icon="'Plus'" @click="openCreate">发布下发材料</el-button>
        <el-button :icon="'Refresh'" circle @click="loadData" />
      </template>
    </page-header>

    <el-card shadow="never" class="gov-card">
      <el-table :data="list" v-loading="loading" size="large">
        <template #empty>
          <empty-state description="暂无下发的材料或文件" />
        </template>

        <el-table-column prop="title" label="材料 / 文件名称" min-width="260" show-overflow-tooltip>
          <template #default="{ row }">
            <div class="dispatch-title-wrap">
              <span class="dispatch-title font-serif" @click="openDetail(row)">{{ row.title }}</span>
              <div class="dispatch-meta-row" v-if="row.doc_no">
                <span class="doc-no-badge">{{ row.doc_no }}</span>
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column prop="deadline" label="查收时限" width="175">
          <template #default="{ row }">
            <div class="deadline-col" v-if="row.deadline">
              <span>{{ row.deadline }}</span>
              <el-tag size="small" :type="isExpired(row.deadline) ? 'info' : 'success'" effect="light">
                {{ isExpired(row.deadline) ? '已截止' : '下发中' }}
              </el-tag>
            </div>
            <div class="deadline-col" v-else>
              <el-tag size="small" type="info" effect="plain">长期有效</el-tag>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="查收进度" width="180">
          <template #default="{ row }">
            <div class="progress-col">
              <div class="progress-label">
                <span>{{ row.receipt_count || 0 }} / {{ row.total_units || 0 }} 单位</span>
                <span class="progress-pct">{{ calcPct(row.receipt_count, row.total_units) }}%</span>
              </div>
              <el-progress 
                :percentage="calcPct(row.receipt_count, row.total_units)" 
                :stroke-width="6" 
                :show-text="false"
                :color="isExpired(row.deadline) ? '#909399' : '#b22222'"
              />
            </div>
          </template>
        </el-table-column>

        <el-table-column prop="created_at" label="发布时间" width="160">
          <template #default="{ row }">
            <span class="text-muted">{{ formatDate(row.created_at) }}</span>
          </template>
        </el-table-column>

        <el-table-column label="操作" width="260" fixed="right" align="right">
          <template #default="{ row }">
            <div class="row-action-wrap">
              <el-button link type="primary" @click="openDetail(row)">查收台账</el-button>
              <el-button link type="primary" @click="notifyDispatch(row)">一键通知材料</el-button>

              <el-dropdown @command="(cmd) => handleRowCommand(cmd, row)" trigger="click">
                <el-button link type="primary" class="more-link">
                  <span>更多</span>
                  <el-icon class="el-icon--right"><ArrowDown /></el-icon>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="copy_url" icon="Link">复制纯链接</el-dropdown-item>
                    <el-dropdown-item command="export" icon="Download">导出查收台账 (Excel)</el-dropdown-item>
                    <el-dropdown-item command="edit" icon="Edit" divided>编辑材料</el-dropdown-item>
                    <el-dropdown-item command="delete" icon="Delete" class="text-danger-item">删除任务</el-dropdown-item>
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

    <!-- 1. 发布/编辑材料：抽屉（Drawer） -->
    <el-drawer 
      v-model="dialogVisible" 
      :title="editId ? '编辑下发材料通知' : '发布材料下发通知'" 
      :size="isMobile ? '100%' : '660px'"
      destroy-on-close
    >
      <el-form :model="form" :rules="rules" ref="formRef" label-width="115px">
        <div class="form-section">
          <div class="form-section-title">文件基本信息</div>
          <el-form-item label="材料文件标题" prop="title" required>
            <el-input 
              v-model="form.title" 
              placeholder="如：关于认真组织开展2026年第二季度宣传思想文化理论学习的通知" 
            />
          </el-form-item>

          <el-row :gutter="16">
            <el-col :xs="24" :sm="12">
              <el-form-item label="发文字号">
                <el-input v-model="form.doc_no" placeholder="如：伊县宣发〔2026〕18号" />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12">
              <el-form-item label="查收截止时间">
                <el-date-picker 
                  v-model="form.deadline" 
                  type="datetime" 
                  value-format="YYYY-MM-DD HH:mm"
                  format="YYYY-MM-DD HH:mm"
                  placeholder="选填，如需催促查收请设置"
                  style="width: 100%" 
                />
              </el-form-item>
            </el-col>
          </el-row>

          <el-form-item label="学习贯彻说明">
            <el-input 
              v-model="form.content" 
              type="textarea" 
              :rows="3" 
              placeholder="如：请各单位认真领会文件精神，结合工作实际抓好贯彻落实。本文件查收即视为签收，无需报送回函。" 
            />
          </el-form-item>
        </div>

        <div class="form-section">
          <div class="form-section-title">上传材料文件与附件</div>
          <!-- 高亮加粗保密防线警示条 -->
          <div class="secrecy-warning-banner">
            <el-icon class="sec-warn-icon"><WarningFilled /></el-icon>
            <div class="sec-warn-text">
              <strong>【保密红线 · 严正警示】</strong>
              本系统为非涉密政务平台，<strong>严禁下发或上传任何标注为国家秘密（秘密、机密、绝密）以及工作秘密、内部级的文件资料！</strong>
              系统已部署涉密特征实时监测，违规上传将触发<strong>安全熔断、阻断并强制锁定会话</strong>，违规操作记录上报安全审计中心。
            </div>
          </div>

          <el-form-item label="正文 PDF 文件" required>
            <div class="upload-container">
              <div v-if="form.pdf_path" class="uploaded-card">
                <el-icon class="file-icon"><Document /></el-icon>
                <span class="file-name">{{ form.pdf_name || '正文文件.pdf' }}</span>
                <el-button link type="primary" size="small" @click="previewPdf(form.pdf_path)">预览</el-button>
                <el-button link type="danger" size="small" @click="removePdf">重新上传</el-button>
              </div>
              <el-upload
                v-else
                class="upload-box"
                action="/api/uploads"
                :headers="uploadHeaders"
                :data="{ owner_type: 'dispatch' }"
                :show-file-list="false"
                :before-upload="beforeUploadPdf"
                :on-success="handleUploadPdfSuccess"
                :on-error="handleUploadError"
                accept=".pdf"
              >
                <el-button type="primary" plain :icon="'Upload'">上传 PDF 格式正文文件</el-button>
                <div class="upload-tip">必填，供各单位在线查阅与下载，限 20MB 以内</div>
              </el-upload>
            </div>
          </el-form-item>

          <el-form-item label="配套附件">
            <div class="upload-container">
              <div v-if="form.attachment_path" class="uploaded-card">
                <el-icon class="file-icon"><Document /></el-icon>
                <span class="file-name">{{ form.attachment_name || '配套附件' }}</span>
                <el-button link type="danger" size="small" @click="removeAttachment">删除</el-button>
              </div>
              <el-upload
                v-else
                class="upload-box"
                action="/api/uploads"
                :headers="uploadHeaders"
                :data="{ owner_type: 'dispatch' }"
                :show-file-list="false"
                :before-upload="beforeUploadAttachment"
                :on-success="handleUploadAttachmentSuccess"
                :on-error="handleUploadError"
                accept=".doc,.docx,.xls,.xlsx,.zip,.rar"
              >
                <el-button plain :icon="'Document'">上传配套附件（选填）</el-button>
                <div class="upload-tip">选填，支持 Word、Excel、压缩包等配套学习材料</div>
              </el-upload>
            </div>
          </el-form-item>
        </div>

        <div class="form-section">
          <div class="form-section-title">下发单位范围</div>
          <div class="preset-btn-bar">
            <span class="preset-label">快捷填充：</span>
            <el-button size="small" round @click="appendPreset('towns')">全县 16 个乡镇</el-button>
            <el-button size="small" round @click="appendPreset('county_orgs')">所有县直机关</el-button>
          </div>
          <el-form-item label="单位名单范围" prop="units" required>
            <el-input 
              v-model="form.units" 
              type="textarea" 
              :rows="6"
              placeholder="每行填写一个单位" 
            />
            <div class="form-sub-tip">每行独立一个单位，公开查收页面将严格按此名单供各单位匹配签收。</div>
          </el-form-item>
        </div>
      </el-form>

      <template #footer>
        <div class="drawer-footer-wrap">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="saving" @click="save">发布下发</el-button>
        </div>
      </template>
    </el-drawer>

    <!-- 2. 查收台账详情：抽屉（Drawer） -->
    <el-drawer 
      v-model="detailVisible" 
      :title="detailTitle" 
      :size="isMobile ? '100%' : '820px'"
      destroy-on-close
    >
      <div v-if="detailDispatch" class="detail-header-card">
        <!-- 四大指标看板卡片 -->
        <div class="detail-stats-bar">
          <div class="d-stat-box">
            <div class="stat-number text-success">{{ detailReceipts.length }}</div>
            <div class="stat-title">已查收单位</div>
          </div>
          <div class="d-stat-box">
            <div class="stat-number text-danger">{{ detailUnreceipted.length }}</div>
            <div class="stat-title">待催查收单位</div>
          </div>
          <div class="d-stat-box">
            <div class="stat-number">{{ detailTotalUnits }}</div>
            <div class="stat-title">总下发单位</div>
          </div>
          <div class="d-stat-box">
            <div class="stat-number text-primary">{{ calcPct(detailReceipts.length, detailTotalUnits) }}%</div>
            <div class="stat-title">查收覆盖率</div>
          </div>
        </div>

        <!-- 综合查收进度条 -->
        <div class="drawer-progress-box">
          <div class="progress-label">
            <span>综合查收进度：{{ detailReceipts.length }} / {{ detailTotalUnits }} 单位</span>
            <span class="progress-pct">{{ calcPct(detailReceipts.length, detailTotalUnits) }}%</span>
          </div>
          <el-progress 
            :percentage="calcPct(detailReceipts.length, detailTotalUnits)" 
            :stroke-width="7" 
            :show-text="false"
            :color="isExpired(detailDispatch.deadline) ? '#909399' : '#b22222'"
          />
        </div>

        <div class="detail-quick-meta">
          <span v-if="detailDispatch.doc_no"><strong>发文字号：</strong>{{ detailDispatch.doc_no }}</span>
          <span v-if="detailDispatch.deadline"><strong>查收截止：</strong>{{ detailDispatch.deadline }}</span>
          <span><strong>正文文件：</strong>{{ detailDispatch.pdf_name }}</span>
        </div>
      </div>

      <el-tabs v-model="detailTab" class="mt-12">
        <!-- 标签页 1：已查收单位名单 -->
        <el-tab-pane :label="`已查收单位 (${detailReceipts.length})`" name="receipted">
          <el-table :data="detailReceipts" size="large" class="mt-8">
            <template #empty><empty-state description="暂无单位查收" /></template>
            <el-table-column prop="unit" label="查收单位" min-width="160" show-overflow-tooltip />
            <el-table-column prop="receiver_name" label="签收经办人" width="110" />
            <el-table-column prop="receiver_phone" label="联系电话" width="130" />
            <el-table-column prop="received_at" label="查收时间" width="160">
              <template #default="{ row }">
                <span class="text-muted">{{ formatDate(row.received_at) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="110" align="center" fixed="right">
              <template #default="{ row }">
                <el-button link type="danger" size="small" @click="resetUnitReceipt(row.unit)">
                  重置状态
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <!-- 标签页 2：待催查收单位名单 -->
        <el-tab-pane :label="`待催查收 (${detailUnreceipted.length})`" name="unreceipted">
          <div class="urge-action-bar">
            <el-alert 
              v-if="detailUnreceipted.length" 
              type="warning" 
              :closable="false" 
              class="mb-12"
              :title="`尚有 ${detailUnreceipted.length} 个单位未查收本材料。可点击右侧快捷按钮一键复制微信催查收名单。`" 
            />
            <el-button 
              type="warning" 
              :icon="CopyDocument" 
              @click="copyUrgeList"
              :disabled="!detailUnreceipted.length"
            >
              一键复制微信催查收名单
            </el-button>
          </div>

          <el-table :data="detailUnreceipted.map(u => ({ unit: u }))" size="large">
            <template #empty><empty-state description="所有下发单位均已全部查收！" /></template>
            <el-table-column prop="unit" label="待查收单位" min-width="220" />
            <el-table-column label="状态" width="110" align="center">
              <template #default>
                <el-tag type="info" size="small">未查收</el-tag>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>
      </el-tabs>

      <template #footer>
        <div class="drawer-footer-wrap">
          <el-button type="success" :icon="'Download'" @click="exportReceipts(detailDispatch)">导出查收台账 (Excel)</el-button>
          <el-button @click="detailVisible = false">关闭</el-button>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { CopyDocument, ArrowDown } from '@element-plus/icons-vue'
import request, { exportFile } from '../utils/request'
import dayjs from 'dayjs'
import PageHeader from '../components/PageHeader.vue'
import EmptyState from '../components/EmptyState.vue'
import { useMobile } from '../utils/useMobile'
import { validateUploadFileSecrecy, handleSecrecyUploadError } from '../utils/secrecyGuard'

const { isMobile } = useMobile()

const list = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(10)
const loading = ref(false)

const dialogVisible = ref(false)
const editId = ref(0)
const saving = ref(false)
const formRef = ref(null)

const form = ref({
  title: '',
  doc_no: '',
  deadline: '',
  content: '',
  pdf_path: '',
  pdf_name: '',
  attachment_path: '',
  attachment_name: '',
  units: ''
})

const rules = {
  title: [{ required: true, message: '请填写材料标题', trigger: 'blur' }],
  units: [{ required: true, message: '请指定下发单位范围', trigger: 'blur' }]
}

// 查收台账抽屉
const detailVisible = ref(false)
const detailTitle = ref('')
const detailTab = ref('receipted')
const detailDispatch = ref(null)
const detailReceipts = ref([])
const detailUnreceipted = ref([])
const detailTotalUnits = ref(0)

const uploadHeaders = computed(() => {
  const token = localStorage.getItem('token') || ''
  return token ? { Authorization: `Bearer ${token}` } : {}
})

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

const isExpired = (deadline) => {
  if (!deadline) return false
  return dayjs().isAfter(dayjs(deadline))
}

const calcPct = (count, total) => {
  if (!total || total <= 0) return 0
  return Math.min(100, Math.round(((count || 0) / total) * 100))
}

const formatDate = (val) => {
  if (!val) return ''
  return dayjs(val).format('YYYY-MM-DD HH:mm')
}

const loadData = async () => {
  loading.value = true
  try {
    const res = await request.get('/dispatches', {
      params: { page: page.value, page_size: pageSize.value }
    })
    list.value = res.list || []
    total.value = res.total || 0
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

const handlePageChange = (p) => {
  page.value = p
  loadData()
}

const handleSizeChange = (s) => {
  pageSize.value = s
  page.value = 1
  loadData()
}

const openCreate = () => {
  editId.value = 0
  form.value = {
    title: '',
    doc_no: '',
    deadline: '',
    content: '',
    pdf_path: '',
    pdf_name: '',
    attachment_path: '',
    attachment_name: '',
    units: ''
  }
  dialogVisible.value = true
}

const openEdit = (row) => {
  editId.value = row.id
  form.value = {
    title: row.title,
    doc_no: row.doc_no || '',
    deadline: row.deadline || '',
    content: row.content || '',
    pdf_path: row.pdf_path,
    pdf_name: row.pdf_name,
    attachment_path: row.attachment_path || '',
    attachment_name: row.attachment_name || '',
    units: row.units || ''
  }
  dialogVisible.value = true
}

const beforeUploadPdf = (file) => {
  if (!validateUploadFileSecrecy(file, { isPublic: false })) {
    return false
  }
  const isPdf = file.type === 'application/pdf' || file.name.toLowerCase().endsWith('.pdf')
  if (!isPdf) {
    ElMessage.error('仅支持上传 PDF 格式正文文件')
    return false
  }
  const isLt20M = file.size / 1024 / 1024 < 20
  if (!isLt20M) {
    ElMessage.error('文件大小不得超过 20MB')
    return false
  }
  return true
}

const handleUploadPdfSuccess = (res) => {
  form.value.pdf_path = res.file_path || res.url
  form.value.pdf_name = res.file_name || '正文文件.pdf'
  ElMessage.success('正文 PDF 上传成功')
}

const beforeUploadAttachment = (file) => {
  if (!validateUploadFileSecrecy(file, { isPublic: false })) {
    return false
  }
  const isLt30M = file.size / 1024 / 1024 < 30
  if (!isLt30M) {
    ElMessage.error('附件大小不得超过 30MB')
    return false
  }
  return true
}

const handleUploadAttachmentSuccess = (res) => {
  form.value.attachment_path = res.file_path || res.url
  form.value.attachment_name = res.file_name || '配套附件'
  ElMessage.success('配套附件上传成功')
}

const handleUploadError = (err, file) => {
  if (handleSecrecyUploadError(err, file, { isPublic: false })) return
  ElMessage.error('上传失败，请检查文件大小或网络后重试')
}

const removePdf = () => {
  form.value.pdf_path = ''
  form.value.pdf_name = ''
}

const removeAttachment = () => {
  form.value.attachment_path = ''
  form.value.attachment_name = ''
}

const previewPdf = (path) => {
  window.open(path, '_blank')
}

const save = async () => {
  if (!form.value.title) return ElMessage.warning('请填写材料标题')
  if (!form.value.pdf_path) return ElMessage.warning('请上传正文 PDF 文件')
  if (!form.value.units) return ElMessage.warning('请指定下发单位范围')

  saving.value = true
  try {
    if (editId.value) {
      await request.put('/dispatches', { ...form.value, id: editId.value })
      ElMessage.success('更新材料成功')
    } else {
      await request.post('/dispatches', form.value)
      ElMessage.success('材料下发通知发布成功')
    }
    dialogVisible.value = false
    loadData()
  } catch (e) {
    console.error(e)
  } finally {
    saving.value = false
  }
}

const removeDispatch = (row) => {
  ElMessageBox.confirm(`确定要彻底删除下发材料《${row.title}》吗？其所有查收记录也将一并清除。`, '警告', {
    type: 'warning',
    confirmButtonText: '确定删除',
    cancelButtonText: '取消'
  }).then(async () => {
    try {
      await request.delete(`/dispatches/${row.id}`)
      ElMessage.success('材料已删除')
      loadData()
    } catch (e) {
      console.error(e)
    }
  }).catch(() => {})
}

const copyLink = (row) => {
  const url = `${window.location.origin}/dispatch/${row.id}`
  navigator.clipboard.writeText(url).then(() => {
    ElMessage.success('材料查收纯链接已复制到剪贴板')
  }).catch(() => {
    ElMessage.info(`查收链接：${url}`)
  })
}

// 一键通知材料：生成微信工作群规范格式
const notifyDispatch = (row) => {
  const url = `${window.location.origin}/dispatch/${row.id}`
  const docNoText = row.doc_no ? `（${row.doc_no}）` : ''
  const deadlineText = row.deadline ? `请于 ${row.deadline} 前` : '请'

  let text = `【公文材料下发通知】\n`
  text += `各乡镇场、部门单位：\n`
  text += `现将《${row.title}》${docNoText}正式下发给各单位。请各单位认真组织学习领会，抓好贯彻落实。\n\n`
  if (row.content) {
    text += `【工作说明】${row.content}\n`
  }
  text += `【查收提示】${deadlineText}点击下方专属链接查收文件，本文件在线确认查收即可，无需报送回执公函：\n`
  text += `${url}\n\n`
  text += `中共伊宁县委宣传部`

  navigator.clipboard.writeText(text).then(() => {
    ElMessage.success('已复制公文材料下发通知文案，可直接发送微信工作群')
  }).catch(() => {
    ElMessage.info(`通知文案已生成：\n${text}`)
  })
}

const handleRowCommand = (cmd, row) => {
  if (cmd === 'copy_url') copyLink(row)
  else if (cmd === 'export') exportReceipts(row)
  else if (cmd === 'edit') openEdit(row)
  else if (cmd === 'delete') removeDispatch(row)
}

const openDetail = async (row) => {
  try {
    detailDispatch.value = row
    detailTitle.value = `查收台账 - ${row.title}`
    const res = await request.get(`/dispatches/${row.id}`)
    detailReceipts.value = res.receipts || []
    detailUnreceipted.value = res.unreceipted_units || []
    detailTotalUnits.value = res.total_units || 0
    detailTab.value = 'receipted'
    detailVisible.value = true
  } catch (e) {
    console.error(e)
  }
}

const resetUnitReceipt = (unit) => {
  ElMessageBox.confirm(`确定要重置【${unit}】的查收状态吗？重置后该单位需重新进行查收登记。`, '重置查收', {
    type: 'warning',
    confirmButtonText: '确定重置',
    cancelButtonText: '取消'
  }).then(async () => {
    try {
      await request.post(`/dispatches/${detailDispatch.value.id}/reset-unit`, { unit })
      ElMessage.success(`已重置【${unit}】查收状态`)
      openDetail(detailDispatch.value)
      loadData()
    } catch (e) {
      console.error(e)
    }
  }).catch(() => {})
}

// 一键复制微信催查收名单
const copyUrgeList = () => {
  if (!detailUnreceipted.value.length) return
  const d = detailDispatch.value
  const url = `${window.location.origin}/dispatch/${d.id}`
  const deadlineText = d.deadline ? `查收截止时间：${d.deadline}\n` : ''
  const unitList = detailUnreceipted.value.map((u, i) => `${i + 1}. ${u}`).join('\n')

  let text = `【材料查收温馨提醒】\n`
  text += `《${d.title}》已正式下发，经后台系统核对，截至目前以下单位尚未完成在线查收：\n\n`
  text += `${unitList}\n\n`
  if (deadlineText) text += `${deadlineText}`
  text += `请上述单位经办同志尽快点击链接查收（在线点击确认即可，无需回函）：\n`
  text += `${url}\n\n`
  text += `中共伊宁县委宣传部`

  navigator.clipboard.writeText(text).then(() => {
    ElMessage.success('催查收名单已复制到剪贴板，可直接发送微信工作群')
  }).catch(() => {
    ElMessage.info(`催查收名单：\n${text}`)
  })
}

const exportReceipts = (row) => {
  exportFile(`/export/dispatches/${row.id}/receipts`, `材料查收台账-${row.title}.xlsx`)
}

onMounted(() => {
  loadData()
})
</script>

<style scoped>
.dispatches-page {
  padding-bottom: 24px;
}

.gov-card {
  border-radius: 8px;
  border: 1px solid #e2e8f0;
}

.dispatch-title-wrap {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.dispatch-title {
  font-size: 15px;
  font-weight: 600;
  color: #1a202c;
  cursor: pointer;
  transition: color 0.2s;
}

.dispatch-title:hover {
  color: #b22222;
}

.dispatch-meta-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.doc-no-badge {
  display: inline-block;
  padding: 1px 6px;
  background: #f1f5f9;
  color: #475569;
  font-size: 11px;
  border-radius: 4px;
  font-family: monospace;
}

.deadline-col {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 13px;
}

.progress-col {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.progress-label {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: #64748b;
}

.progress-pct {
  font-weight: 600;
  color: #b22222;
}

.row-action-wrap {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  white-space: nowrap;
}

.more-link {
  display: inline-flex;
  align-items: center;
}

.text-danger-item {
  color: #f56c6c !important;
}

.form-section {
  background: #f8fafc;
  border-radius: 6px;
  padding: 16px;
  margin-bottom: 16px;
  border: 1px solid #edf2f7;
}

.form-section-title {
  font-weight: 600;
  font-size: 14px;
  color: #334155;
  margin-bottom: 14px;
  border-left: 3px solid #b22222;
  padding-left: 8px;
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

.form-sub-tip {
  font-size: 12px;
  color: #94a3b8;
  margin-top: 4px;
}

.upload-container {
  width: 100%;
}

.uploaded-card {
  display: flex;
  align-items: center;
  gap: 8px;
  background: #fff;
  border: 1px solid #e2e8f0;
  padding: 8px 12px;
  border-radius: 6px;
}

.file-icon {
  font-size: 18px;
  color: #b22222;
}

.file-name {
  font-size: 13px;
  color: #334155;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.upload-tip {
  font-size: 12px;
  color: #94a3b8;
  margin-top: 6px;
}

.drawer-footer-wrap {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

/* 查收台账头部看板 */
.detail-header-card {
  background: #fff8f8;
  border: 1px solid #fecaca;
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 16px;
}

.detail-stats-bar {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-bottom: 16px;
}

.d-stat-box {
  background: #fff;
  border-radius: 6px;
  padding: 12px 8px;
  text-align: center;
  border: 1px solid #f3e8e8;
  box-shadow: 0 1px 3px rgba(0,0,0,0.03);
}

.stat-number {
  font-size: 22px;
  font-weight: 700;
  color: #1e293b;
  line-height: 1.2;
}

.stat-title {
  font-size: 12px;
  color: #64748b;
  margin-top: 4px;
}

.drawer-progress-box {
  background: #fff;
  padding: 12px 16px;
  border-radius: 6px;
  border: 1px solid #f3e8e8;
  margin-bottom: 12px;
}

.detail-quick-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  font-size: 12px;
  color: #64748b;
  padding-top: 6px;
}

.urge-action-bar {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 12px;
}

.text-muted {
  color: #64748b;
  font-size: 13px;
}

.text-success { color: #16a34a !important; }
.text-danger { color: #dc2626 !important; }
.text-primary { color: #b22222 !important; }

@media (max-width: 640px) {
  .detail-stats-bar {
    grid-template-columns: repeat(2, 1fr);
  }
}

/* 高亮加粗保密防线警示条 */
.secrecy-warning-banner {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  background: #fef2f2;
  border: 1.5px solid #f87171;
  border-radius: 6px;
  padding: 10px 14px;
  margin-bottom: 14px;
}

.sec-warn-icon {
  font-size: 20px;
  color: #dc2626;
  margin-top: 2px;
  flex-shrink: 0;
}

.sec-warn-text {
  font-size: 13px;
  color: #991b1b;
  line-height: 1.6;
}

.sec-warn-text strong {
  color: #b91c1c;
  font-weight: 700;
}
</style>
