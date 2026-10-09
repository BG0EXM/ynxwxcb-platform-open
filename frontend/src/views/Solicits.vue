<template>
  <div class="solicits-page">
    <!-- 页头 -->
    <page-header 
      title="征求意见" 
      subtitle="发布文件材料征集通知，生成各单位公开反馈链接，在线查阅底稿并收集盖章红头回函"
      :tag="total ? `共 ${total} 项任务` : ''"
    >
      <template #actions>
        <el-button type="primary" :icon="'Plus'" @click="openCreate">发布征求意见</el-button>
        <el-button :icon="'Refresh'" circle @click="loadData" />
      </template>
    </page-header>

    <el-card shadow="never" class="gov-card">
      <el-table :data="list" v-loading="loading" size="large">
        <template #empty>
          <empty-state description="暂无正在征集意见的文件或材料" />
        </template>

        <el-table-column prop="title" label="文件 / 材料名称" min-width="260" show-overflow-tooltip>
          <template #default="{ row }">
            <div class="solicit-title-wrap">
              <span class="solicit-title font-serif" @click="openDetail(row)">{{ row.title }}</span>
              <div class="solicit-meta-row" v-if="row.doc_no">
                <span class="doc-no-badge">{{ row.doc_no }}</span>
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column prop="deadline" label="征集截止时间" width="175">
          <template #default="{ row }">
            <div class="deadline-col">
              <span>{{ row.deadline }}</span>
              <el-tag size="small" :type="isExpired(row.deadline) ? 'info' : 'success'" effect="light">
                {{ isExpired(row.deadline) ? '已截止' : '征集中' }}
              </el-tag>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="反馈进度" width="180">
          <template #default="{ row }">
            <div class="progress-col">
              <div class="progress-label">
                <span>{{ row.feedback_count || 0 }} / {{ row.total_units || 0 }} 单位</span>
                <span class="progress-pct">{{ calcPct(row.feedback_count, row.total_units) }}%</span>
              </div>
              <el-progress 
                :percentage="calcPct(row.feedback_count, row.total_units)" 
                :stroke-width="6" 
                :show-text="false"
                :color="isExpired(row.deadline) ? '#909399' : '#b22222'"
              />
            </div>
          </template>
        </el-table-column>

        <el-table-column label="意见统计" width="140" align="center">
          <template #default="{ row }">
            <div class="opinion-stats-wrap">
              <el-tooltip content="提出修改意见的单位数" placement="top">
                <span class="stat-tag tag-opinion" v-if="row.opinion_count > 0">
                  有意见 {{ row.opinion_count }}
                </span>
                <span class="stat-tag tag-none" v-else>无意见</span>
              </el-tooltip>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="操作" width="240" fixed="right" align="right">
          <template #default="{ row }">
            <div class="row-action-wrap">
              <el-button link type="primary" @click="openDetail(row)">反馈台账</el-button>
              <el-button link type="primary" @click="copyLink(row)">复制链接</el-button>

              <el-dropdown @command="(cmd) => handleRowCommand(cmd, row)" trigger="click">
                <el-button link type="primary" class="more-link">
                  <span>更多</span>
                  <el-icon class="el-icon--right"><ArrowDown /></el-icon>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="export" icon="Download">导出汇总表 (Excel)</el-dropdown-item>
                    <el-dropdown-item command="zip" icon="FolderChecked">打包下载回函 (ZIP)</el-dropdown-item>
                    <el-dropdown-item command="edit" icon="Edit" divided>编辑任务</el-dropdown-item>
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

    <!-- 1. 录入/编辑征求意见：抽屉（Drawer） -->
    <el-drawer 
      v-model="dialogVisible" 
      :title="editId ? '编辑征求意见通知' : '发布征求意见文件'" 
      :size="isMobile ? '100%' : '660px'"
      destroy-on-close
    >
      <el-form :model="form" :rules="rules" ref="formRef" label-width="115px">
        <div class="form-section">
          <div class="form-section-title">文件基本信息</div>
          <el-form-item label="文件材料标题" prop="title" required>
            <el-input 
              v-model="form.title" 
              placeholder="如：关于征求《伊宁县新时代文明实践阵地建设实施方案（征求意见稿）》意见的函" 
            />
          </el-form-item>

          <el-row :gutter="16">
            <el-col :xs="24" :sm="12">
              <el-form-item label="函号 / 文号">
                <el-input v-model="form.doc_no" placeholder="如：伊县宣字〔2026〕15号" />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12">
              <el-form-item label="征集截止时间" prop="deadline" required>
                <el-date-picker 
                  v-model="form.deadline" 
                  type="datetime" 
                  value-format="YYYY-MM-DD HH:mm"
                  format="YYYY-MM-DD HH:mm"
                  placeholder="选择截止日期与时间"
                  style="width: 100%" 
                />
              </el-form-item>
            </el-col>
          </el-row>

          <el-form-item label="征集要求说明">
            <el-input 
              v-model="form.content" 
              type="textarea" 
              :rows="3" 
              placeholder="如：请各单位主要领导审签并加盖公章后，将红头回函扫描件于截止时间前上传反馈。逾期未反馈视为无意见。" 
            />
          </el-form-item>
        </div>

        <div class="form-section">
          <div class="form-section-title">上传征求底稿附件</div>
          <!-- 高亮加粗保密防线警示条 -->
          <div class="secrecy-warning-banner">
            <el-icon class="sec-warn-icon"><WarningFilled /></el-icon>
            <div class="sec-warn-text">
              <strong>【保密红线 · 严正警示】</strong>
              本系统为非涉密政务平台，<strong>严禁上传任何标注为国家秘密（秘密、机密、绝密）以及工作秘密、内部级的文件资料！</strong>
              系统已部署涉密特征智能监测，违规上传将触发<strong>安全熔断、阻断并强制锁定会话</strong>，违规记录记入安全审计日志。
            </div>
          </div>

          <el-form-item label="正文 PDF 底稿" required>
            <div class="upload-container">
              <div v-if="form.pdf_path" class="uploaded-card">
                <el-icon class="file-icon"><Document /></el-icon>
                <span class="file-name">{{ form.pdf_name || '正文底稿.pdf' }}</span>
                <el-button link type="primary" size="small" @click="previewPdf(form.pdf_path)">预览</el-button>
                <el-button link type="danger" size="small" @click="removePdf">重新上传</el-button>
              </div>
              <el-upload
                v-else
                class="upload-box"
                action="/api/uploads"
                :headers="uploadHeaders"
                :data="{ owner_type: 'solicit' }"
                :show-file-list="false"
                :before-upload="beforeUploadPdf"
                :on-success="handleUploadPdfSuccess"
                :on-error="handleUploadError"
                accept=".pdf"
              >
                <el-button type="primary" plain :icon="'Upload'">上传 PDF 格式正文文件</el-button>
                <div class="upload-tip">必填，支持各单位在线阅读查看，限 20MB 以内</div>
              </el-upload>
            </div>
          </el-form-item>

          <el-form-item label="配套 Word 稿件">
            <div class="upload-container">
              <div v-if="form.word_path" class="uploaded-card">
                <el-icon class="file-icon"><Document /></el-icon>
                <span class="file-name">{{ form.word_name || '配套修改稿.docx' }}</span>
                <el-button link type="danger" size="small" @click="removeWord">删除</el-button>
              </div>
              <el-upload
                v-else
                class="upload-box"
                action="/api/uploads"
                :headers="uploadHeaders"
                :data="{ owner_type: 'solicit' }"
                :show-file-list="false"
                :before-upload="beforeUploadWord"
                :on-success="handleUploadWordSuccess"
                :on-error="handleUploadError"
                accept=".doc,.docx"
              >
                <el-button plain :icon="'Document'">上传 Word 稿件（选填）</el-button>
                <div class="upload-tip">选填，便于各单位下载并在本地标红修改后回传</div>
              </el-upload>
            </div>
          </el-form-item>
        </div>

        <div class="form-section">
          <div class="form-section-title">征求单位范围</div>
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
            <div class="form-sub-tip">每行独立一个单位，公开填报页面将严格按此名单供各单位匹配选择。</div>
          </el-form-item>
        </div>
      </el-form>

      <template #footer>
        <div class="drawer-footer-wrap">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="saving" @click="save">发布通知</el-button>
        </div>
      </template>
    </el-drawer>

    <!-- 2. 反馈台账详情：抽屉（Drawer） -->
    <el-drawer 
      v-model="detailVisible" 
      :title="detailTitle" 
      :size="isMobile ? '100%' : '820px'"
      destroy-on-close
    >
      <div v-if="detailSolicit" class="detail-header-card">
        <div class="detail-stats-bar">
          <div class="d-stat-box">
            <div class="stat-number">{{ detailSolicit.feedback_count || 0 }}</div>
            <div class="stat-title">已反馈单位</div>
          </div>
          <div class="d-stat-box">
            <div class="stat-number text-warning">{{ detailSolicit.opinion_count || 0 }}</div>
            <div class="stat-title">提出修改意见</div>
          </div>
          <div class="d-stat-box">
            <div class="stat-number text-success">{{ (detailSolicit.feedback_count || 0) - (detailSolicit.opinion_count || 0) }}</div>
            <div class="stat-title">原则同意无意见</div>
          </div>
          <div class="d-stat-box">
            <div class="stat-number text-danger">{{ (detailSolicit.unconfirmed_units || []).length }}</div>
            <div class="stat-title">待催报单位</div>
          </div>
        </div>

        <div class="detail-quick-meta">
          <span><strong>截止时间：</strong>{{ detailSolicit.deadline }}</span>
          <span v-if="detailSolicit.doc_no"><strong>文号：</strong>{{ detailSolicit.doc_no }}</span>
        </div>
      </div>

      <el-tabs v-model="detailTab" class="mt-12">
        <!-- 标签 1：已反馈列表 -->
        <el-tab-pane :label="`已反馈单位 (${feedbacks.length})`" name="feedbacks">
          <el-table :data="feedbacks" size="large" class="mt-8">
            <template #empty><empty-state description="暂无单位提交反馈" /></template>
            <el-table-column prop="unit" label="单位名称" min-width="160" show-overflow-tooltip />
            <el-table-column label="意见状态" width="130">
              <template #default="{ row }">
                <el-tag :type="row.has_opinion === 1 ? 'warning' : 'success'" size="small">
                  {{ row.has_opinion === 1 ? '提出修改意见' : '原则同意无意见' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="盖章红头回函" width="140">
              <template #default="{ row }">
                <el-button 
                  v-if="row.reply_doc_path" 
                  link 
                  type="primary" 
                  size="small"
                  @click="openFile(row.reply_doc_path)"
                >
                  <el-icon><DocumentChecked /></el-icon> 查看回函
                </el-button>
                <span v-else class="text-muted">未上传</span>
              </template>
            </el-table-column>
            <el-table-column prop="contact_name" label="经办人" width="90" />
            <el-table-column prop="contact_phone" label="联系电话" width="125" />
            <el-table-column label="操作" width="140" align="center" fixed="right">
              <template #default="{ row }">
                <el-button 
                  v-if="row.has_opinion === 1" 
                  link 
                  type="primary" 
                  size="small" 
                  @click="viewOpinionDetail(row)"
                >
                  意见详情
                </el-button>
                <el-button link type="danger" size="small" @click="handleResetUnit(row)">重置状态</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <!-- 标签 2：未反馈催报 -->
        <el-tab-pane :label="`待催报单位 (${unconfirmedUnits.length})`" name="unconfirmed">
          <div class="reminder-header-bar">
            <el-alert 
              v-if="unconfirmedUnits.length" 
              type="warning" 
              :closable="false" 
              class="mb-12"
              :title="`尚有 ${unconfirmedUnits.length} 个单位未提交反馈。可点击右侧快捷按钮复制催报名单。`" 
            />
            <el-button 
              type="warning" 
              :icon="'DocumentCopy'" 
              @click="copyUrgeList"
              :disabled="!unconfirmedUnits.length"
            >
              一键复制微信催报名单
            </el-button>
          </div>

          <el-table :data="unconfirmedUnits.map(u => ({ unit: u }))" size="large">
            <template #empty><empty-state description="所有征求单位均已反馈完成！" /></template>
            <el-table-column prop="unit" label="待催报单位" min-width="220" />
            <el-table-column label="状态" width="120" align="center">
              <template #default>
                <el-tag type="info" size="small">待反馈</el-tag>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>
      </el-tabs>

      <template #footer>
        <div class="drawer-footer-wrap">
          <el-button type="success" :icon="'Download'" @click="exportFeedbacks">导出反馈汇总 (Excel)</el-button>
          <el-button type="primary" :icon="'FolderChecked'" @click="downloadZip">打包下载盖章回函 (ZIP)</el-button>
          <el-button @click="detailVisible = false">关闭</el-button>
        </div>
      </template>
    </el-drawer>

    <!-- 意见详情对话框 -->
    <el-dialog v-model="opinionDialogVisible" title="反馈意见具体内容" width="560px" destroy-on-close>
      <div class="opinion-dialog-body" v-if="activeFeedback">
        <div class="opinion-unit-header">
          <span class="unit-badge">{{ activeFeedback.unit }}</span>
          <span class="contact-info">经办人：{{ activeFeedback.contact_name }}（{{ activeFeedback.contact_phone }}）</span>
        </div>
        <div class="opinion-content-box">
          <div class="opinion-title">具体修改意见与建议：</div>
          <div class="opinion-text">{{ activeFeedback.opinion_detail || '未填写文字说明' }}</div>
        </div>
        <div class="attachment-bar" v-if="activeFeedback.attachment_path">
          <span>标红修改稿附件：</span>
          <el-button link type="primary" @click="openFile(activeFeedback.attachment_path)">
            {{ activeFeedback.attachment_name || '点击下载修改稿' }}
          </el-button>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
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
const saving = ref(false)

const dialogVisible = ref(false)
const editId = ref(0)
const formRef = ref(null)

const form = ref({
  title: '',
  doc_no: '',
  deadline: dayjs().add(3, 'day').format('YYYY-MM-DD 18:00'),
  content: '',
  pdf_path: '',
  pdf_name: '',
  word_path: '',
  word_name: '',
  units: ''
})

const rules = {
  title: [{ required: true, message: '请输入文件材料标题', trigger: 'blur' }],
  deadline: [{ required: true, message: '请选择征集截止时间', trigger: 'change' }],
  units: [{ required: true, message: '请填写征求单位范围', trigger: 'blur' }]
}

const detailVisible = ref(false)
const detailTitle = ref('')
const detailTab = ref('feedbacks')
const detailSolicit = ref(null)
const feedbacks = ref([])
const unconfirmedUnits = ref([])

const opinionDialogVisible = ref(false)
const activeFeedback = ref(null)

const uploadHeaders = {
  Authorization: `Bearer ${localStorage.getItem('token')}`
}

const loadData = async () => {
  loading.value = true
  try {
    const res = await request.get('/solicits', {
      params: { page: page.value, page_size: pageSize.value }
    })
    list.value = res.list || []
    total.value = res.total || 0
  } catch (e) {
    console.error(e)
    ElMessage.error('加载征求意见列表失败')
  } finally {
    loading.value = false
  }
}

const isExpired = (deadline) => {
  if (!deadline) return false
  return dayjs().isAfter(dayjs(deadline))
}

const calcPct = (done, all) => {
  if (!all || all <= 0) return 0
  const pct = Math.round(((done || 0) / all) * 100)
  return pct > 100 ? 100 : pct
}

const openCreate = () => {
  editId.value = 0
  form.value = {
    title: '',
    doc_no: '',
    deadline: dayjs().add(3, 'day').format('YYYY-MM-DD 18:00'),
    content: '请各单位主要领导审签并加盖公章后，将红头回函扫描件或清晰照片于截止时间前上传反馈。逾期未反馈视为无意见。',
    pdf_path: '',
    pdf_name: '',
    word_path: '',
    word_name: '',
    units: ''
  }
  dialogVisible.value = true
}

const beforeUploadPdf = (file) => {
  if (!validateUploadFileSecrecy(file, { isPublic: false })) {
    return false
  }
  const isPdf = file.type === 'application/pdf' || file.name.toLowerCase().endsWith('.pdf')
  if (!isPdf) {
    ElMessage.error('只能上传 PDF 格式文件')
    return false
  }
  const isLt20M = file.size / 1024 / 1024 < 20
  if (!isLt20M) {
    ElMessage.error('文件大小不能超过 20MB')
    return false
  }
  return true
}

const handleUploadPdfSuccess = (res) => {
  form.value.pdf_path = res.file_path
  form.value.pdf_name = res.file_name
  ElMessage.success('PDF 正文上传成功')
}

const beforeUploadWord = (file) => {
  if (!validateUploadFileSecrecy(file, { isPublic: false })) {
    return false
  }
  const isLt20M = file.size / 1024 / 1024 < 20
  if (!isLt20M) {
    ElMessage.error('文件大小不能超过 20MB')
    return false
  }
  return true
}

const handleUploadWordSuccess = (res) => {
  form.value.word_path = res.file_path
  form.value.word_name = res.file_name
  ElMessage.success('Word 稿件上传成功')
}

const handleUploadError = (err, file) => {
  if (handleSecrecyUploadError(err, file, { isPublic: false })) return
  ElMessage.error('上传失败，请检查文件大小或网络')
}

const removePdf = () => {
  form.value.pdf_path = ''
  form.value.pdf_name = ''
}

const removeWord = () => {
  form.value.word_path = ''
  form.value.word_name = ''
}

const previewPdf = (path) => {
  window.open(path, '_blank')
}

const openFile = (path) => {
  window.open(path, '_blank')
}

// 预设单位名单
const presets = {
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

const appendPreset = (type) => {
  const chosen = presets[type] || []
  const currentLines = form.value.units.split('\n').map(s => s.trim()).filter(Boolean)
  const combined = Array.from(new Set([...currentLines, ...chosen]))
  form.value.units = combined.join('\n')
  ElMessage.success(`已追加 ${chosen.length} 个单位`)
}

const save = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    if (!form.value.pdf_path) {
      ElMessage.warning('请上传正文 PDF 底稿')
      return
    }

    saving.value = true
    try {
      if (editId.value) {
        await request.put('/solicits', { ...form.value, id: editId.value })
        ElMessage.success('征求意见任务已更新')
      } else {
        await request.post('/solicits', form.value)
        ElMessage.success('征求意见任务发布成功')
      }
      dialogVisible.value = false
      loadData()
    } catch (e) {
      console.error(e)
      ElMessage.error(e.response?.data?.error || '保存失败')
    } finally {
      saving.value = false
    }
  })
}

const openDetail = async (row) => {
  detailTitle.value = `反馈台账 — ${row.title}`
  detailTab.value = 'feedbacks'
  try {
    const res = await request.get(`/solicits/${row.id}`)
    detailSolicit.value = {
      ...res.solicit,
      unconfirmed_units: res.unconfirmed_units || []
    }
    feedbacks.value = res.feedbacks || []
    unconfirmedUnits.value = res.unconfirmed_units || []
    detailVisible.value = true
  } catch (e) {
    console.error(e)
    ElMessage.error('加载台账详情失败')
  }
}

const copyLink = (row) => {
  const url = `${window.location.origin}/solicit/${row.id}`
  navigator.clipboard.writeText(url)
    .then(() => ElMessage.success('公开征求意见填报链接已复制到剪贴板'))
    .catch(() => ElMessage.error('复制失败，请手动复制：' + url))
}

const handleRowCommand = async (cmd, row) => {
  if (cmd === 'export') {
    exportFile(`/export/solicits/${row.id}/feedbacks`, `征求意见反馈汇总-${row.title}.xlsx`)
  } else if (cmd === 'zip') {
    exportFile(`/solicits/${row.id}/download-replies`, `各单位盖章回函汇总-${row.title}.zip`)
  } else if (cmd === 'edit') {
    editId.value = row.id
    form.value = {
      title: row.title,
      doc_no: row.doc_no || '',
      deadline: row.deadline,
      content: row.content || '',
      pdf_path: row.pdf_path || '',
      pdf_name: row.pdf_name || '',
      word_path: row.word_path || '',
      word_name: row.word_name || '',
      units: row.units || ''
    }
    dialogVisible.value = true
  } else if (cmd === 'delete') {
    try {
      await ElMessageBox.confirm(`确定要彻底删除征求意见任务「${row.title}」吗？已收集的反馈记录将一并删除。`, '严正警告', {
        type: 'warning',
        confirmButtonText: '确定删除',
        cancelButtonText: '取消'
      })
      await request.delete(`/solicits/${row.id}`)
      ElMessage.success('已成功删除')
      loadData()
    } catch (e) {
      if (e !== 'cancel') {
        console.error(e)
        ElMessage.error('删除失败')
      }
    }
  }
}

const viewOpinionDetail = (row) => {
  activeFeedback.value = row
  opinionDialogVisible.value = true
}

const handleResetUnit = async (row) => {
  try {
    await ElMessageBox.confirm(`确定要重置【${row.unit}】的反馈记录吗？重置后该单位可重新登录链接填报。`, '提示', {
      type: 'warning'
    })
    await request.post(`/solicits/${detailSolicit.value.id}/feedbacks/reset-unit`, { unit: row.unit })
    ElMessage.success(`已重置【${row.unit}】反馈状态`)
    openDetail(detailSolicit.value)
    loadData()
  } catch (e) {
    if (e !== 'cancel') {
      console.error(e)
      ElMessage.error('重置失败')
    }
  }
}

const copyUrgeList = () => {
  if (!unconfirmedUnits.value.length) return
  const nowStr = dayjs().format('MM月DD日 HH:mm')
  let text = `【宣传部催报提醒 · ${nowStr}】\n`
  text += `关于《${detailSolicit.value.title}》征求意见工作，截至目前，以下 ${unconfirmedUnits.value.length} 个单位尚未上传盖章红头回函反馈：\n\n`
  unconfirmedUnits.value.forEach((u, i) => {
    text += `${i + 1}. ${u}\n`
  })
  text += `\n请上述单位主要负责同志高度重视，尽快于截止时间（${detailSolicit.value.deadline}）前点击以下链接反馈：\n`
  text += `${window.location.origin}/solicit/${detailSolicit.value.id}`

  navigator.clipboard.writeText(text)
    .then(() => ElMessage.success('已复制微信催报名单至剪贴板，可直接发送微信群'))
    .catch(() => ElMessage.error('复制失败'))
}

const exportFeedbacks = () => {
  if (!detailSolicit.value) return
  exportFile(`/export/solicits/${detailSolicit.value.id}/feedbacks`, `征求意见反馈汇总-${detailSolicit.value.title}.xlsx`)
}

const downloadZip = () => {
  if (!detailSolicit.value) return
  exportFile(`/solicits/${detailSolicit.value.id}/download-replies`, `各单位盖章回函汇总-${detailSolicit.value.title}.zip`)
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

onMounted(() => {
  loadData()
})
</script>

<style scoped>
.solicits-page {
  padding-bottom: 24px;
}

.solicit-title-wrap {
  cursor: pointer;
}

.solicit-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--yx-text-1, #24292e);
  line-height: 1.5;
  transition: color 0.2s;
}

.solicit-title:hover {
  color: #b22222;
}

.solicit-meta-row {
  margin-top: 4px;
}

.doc-no-badge {
  font-size: 11px;
  color: #888;
  background: #f4f5f7;
  padding: 1px 6px;
  border-radius: 4px;
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
  gap: 4px;
}

.progress-label {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: #666;
}

.progress-pct {
  font-weight: 600;
  color: #b22222;
}

.opinion-stats-wrap {
  display: inline-flex;
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
  color: #999;
}

.row-action-wrap {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
}

.more-link {
  display: inline-flex;
  align-items: center;
  gap: 2px;
}

.text-danger-item {
  color: #f56c6c !important;
}

.form-section {
  background: #fafbfc;
  border: 1px solid #ebedf0;
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 20px;
}

.form-section-title {
  font-size: 14px;
  font-weight: 600;
  color: #24292e;
  margin-bottom: 16px;
  border-left: 3px solid #b22222;
  padding-left: 8px;
}

.upload-container {
  width: 100%;
}

.uploaded-card {
  display: flex;
  align-items: center;
  background: #ffffff;
  border: 1px solid #dcdfe6;
  border-radius: 6px;
  padding: 8px 12px;
  gap: 8px;
}

.file-icon {
  font-size: 18px;
  color: #b22222;
}

.file-name {
  flex: 1;
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.upload-tip {
  font-size: 12px;
  color: #888;
  margin-top: 4px;
}

.preset-btn-bar {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.preset-label {
  font-size: 12px;
  color: #666;
}

.form-sub-tip {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}

.drawer-footer-wrap {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

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

.text-muted {
  color: #999;
  font-size: 12px;
}

.detail-quick-meta {
  display: flex;
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

.opinion-unit-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid #eee;
}

.unit-badge {
  font-size: 16px;
  font-weight: 600;
  color: #b22222;
}

.contact-info {
  font-size: 13px;
  color: #666;
}

.opinion-content-box {
  background: #fdf6ec;
  border: 1px solid #faecd8;
  border-radius: 6px;
  padding: 14px;
  margin-bottom: 16px;
}

.opinion-title {
  font-size: 13px;
  font-weight: 600;
  color: #e6a23c;
  margin-bottom: 8px;
}

.opinion-text {
  font-size: 14px;
  color: #303133;
  line-height: 1.6;
  white-space: pre-wrap;
}

.attachment-bar {
  font-size: 13px;
  color: #606266;
}

/* 高亮加粗保密警示条 */
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
