<template>
  <div class="study-page">
    <page-header 
      title="公共资料" 
      subtitle="集中共享宣传理论文件、政策法规、学习材料与部务工作参考资料"
    >
      <template #actions>
        <el-button v-if="authStore.hasPerm('study.publish')" type="primary" :icon="'Plus'" @click="openCreate">发布资料</el-button>
        <el-button v-if="authStore.hasPerm('study.category')" type="warning" :icon="'Setting'" @click="openCategory">分类管理</el-button>
        <el-button :icon="'Refresh'" circle @click="loadData" />
      </template>
    </page-header>

    <el-card shadow="never" class="gov-card">
      <div class="toolbar">
        <div>
          <el-input v-model="keyword" placeholder="搜索公共资料" clearable style="width: 220px"
            @keyup.enter="onSearch" @clear="onSearch">
            <template #append><el-button :icon="'Search'" @click="onSearch" /></template>
          </el-input>
          <el-select v-model="category" placeholder="分类" clearable style="width: 140px" class="ml-8" @change="onSearch">
            <el-option v-for="c in categories" :key="c.code" :label="c.name" :value="c.code" />
          </el-select>
        </div>
      </div>

      <el-table :data="list" stripe v-loading="loading" empty-text="暂无资料" size="large">
        <el-table-column prop="title" label="标题" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">
            <a class="title-link" @click="openDetail(row)">{{ row.title }}</a>
          </template>
        </el-table-column>
        <el-table-column label="分类" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="categoryTag(row.category)">{{ categoryName(row.category) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="publisher" label="发布人" width="100" />
        <el-table-column prop="read_count" label="阅读数" width="80" />
        <el-table-column prop="created_at" label="发布时间" width="160">
          <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click="openDetail(row)">阅读</el-button>
            <el-button v-if="authStore.hasPerm('study.delete')" link type="danger" size="small" @click="removeMaterial(row)">删除</el-button>
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

    <!-- 发布资料 -->
    <el-drawer v-model="dialogVisible" title="发布公共资料" size="640px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="标题" required>
          <el-input v-model="form.title" placeholder="请输入标题" />
        </el-form-item>
        <el-form-item label="分类">
          <el-select v-model="form.category" style="width: 100%">
            <el-option v-for="c in categories" :key="c.code" :label="c.name" :value="c.code" />
          </el-select>
        </el-form-item>
        <el-form-item label="内容" required>
          <el-input v-model="form.content" type="textarea" :rows="8" placeholder="公共资料内容" />
        </el-form-item>
        <el-form-item label="附件">
          <el-upload :http-request="uploadFile" :file-list="fileList">
            <el-button :icon="'Upload'">选择文件</el-button>
            <template #tip>
              <div class="el-upload__tip">支持 doc/docx/pdf/xlsx/图片等</div>
            </template>
          </el-upload>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="save">发布</el-button>
      </template>
    </el-drawer>

    <!-- 分类管理 -->
    <el-drawer v-model="catVisible" title="分类管理" size="540px">
      <div class="dept-add-bar">
        <el-input v-model="newCatName" placeholder="新分类名称" clearable style="width: 180px" @keyup.enter="addCategory" />
        <el-input v-model="newCatCode" placeholder="英文标识" clearable style="width: 130px" @keyup.enter="addCategory" />
        <el-button type="primary" :icon="'Plus'" @click="addCategory">添加</el-button>
      </div>
      <el-table :data="categories" stripe empty-text="暂无分类" size="large">
        <el-table-column prop="name" label="名称" min-width="120" />
        <el-table-column prop="code" label="标识" width="110" />
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click="editCategory(row)">改名</el-button>
            <el-button link type="danger" size="small" @click="removeCategory(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-drawer>

    <!-- 详情 -->
    <el-drawer v-model="detailVisible" :title="detail.title" size="640px">
      <div class="meta">
        <el-tag size="small">{{ categoryName(detail.category) }}</el-tag>
        <span>发布人：{{ detail.publisher }}</span>
        <span>发布时间：{{ formatDate(detail.created_at) }}</span>
      </div>
      <el-divider />
      <div class="content">{{ detail.content }}</div>
      <el-divider v-if="detail.attachments && detail.attachments.length" />
      <div v-if="detail.attachments && detail.attachments.length" class="attachments">
        <h4>附件</h4>
        <div v-for="a in detail.attachments" :key="a.id" class="attach-item">
          <el-icon><Paperclip /></el-icon>
          <a href="javascript:;" @click="downloadAtt(a)">{{ a.file_name }}</a>
        </div>
      </div>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, reactive, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import request, { downloadFile } from '../utils/request'
import dayjs from 'dayjs'
import { useAuthStore } from '../store/auth'
import PageHeader from '../components/PageHeader.vue'
import { useMobile } from '../utils/useMobile'

const { isMobile } = useMobile()
const route = useRoute()
const authStore = useAuthStore()

const list = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(10)
const loading = ref(false)
const keyword = ref('')
const category = ref('')
const dialogVisible = ref(false)
const detailVisible = ref(false)
const catVisible = ref(false)
const categories = ref([])
const newCatName = ref('')
const newCatCode = ref('')
const editCatId = ref(0)
const form = reactive({ title: '', content: '', category: '' })
const detail = ref({})
const fileList = ref([])
const uploadedAttachments = ref([])

const categoryName = (c) => {
  const found = categories.value.find(x => x.code === c)
  return found ? found.name : c
}
const categoryTag = (c) => {
  const idx = categories.value.findIndex(x => x.code === c)
  const tags = ['primary', 'success', 'warning', 'info']
  return tags[idx % tags.length] || 'info'
}
const formatDate = (d) => d ? dayjs(d).format('YYYY-MM-DD HH:mm') : '—'

const loadCategories = async () => {
  try {
    const res = await request.get('/study-categories')
    categories.value = res.list || []
  } catch (e) { console.error(e) }
}

const loadData = async () => {
  loading.value = true
  try {
    const res = await request.get('/study-materials', { params: { keyword: keyword.value, category: category.value, page: page.value, page_size: pageSize.value } })
    list.value = res.list || []
    total.value = res.total || 0
  } catch (e) { console.error(e) } finally {
    loading.value = false
  }
}

// 分类管理
const openCategory = () => {
const handlePageChange = (val) => {
  page.value = val
  loadData()
}

const handleSizeChange = (val) => {
  pageSize.value = val
  page.value = 1
  loadData()
}

const onSearch = () => {
  page.value = 1
  loadData()
}
  editCatId.value = 0
  newCatName.value = ''
  newCatCode.value = ''
  loadCategories()
  catVisible.value = true
}

const addCategory = async () => {
  if (!newCatName.value.trim() || !newCatCode.value.trim()) return ElMessage.warning('请填写名称和标识')
  try {
    if (editCatId.value) {
      await request.put('/study-categories', { id: editCatId.value, name: newCatName.value.trim() })
      ElMessage.success('分类已改名')
    } else {
      await request.post('/study-categories', { name: newCatName.value.trim(), code: newCatCode.value.trim() })
      ElMessage.success('分类已添加')
    }
    newCatName.value = ''
    newCatCode.value = ''
    editCatId.value = 0
    loadCategories()
  } catch (e) { console.error(e) }
}

const editCategory = (row) => {
  editCatId.value = row.id
  newCatName.value = row.name
  newCatCode.value = row.code
}

const removeCategory = async (row) => {
  try {
    await ElMessageBox.confirm(`确认删除分类「${row.name}」？`, '删除确认', { type: 'warning', confirmButtonText: '删除' })
  } catch (e) { console.error(e); return }
  try {
    await request.delete(`/study-categories/${row.id}`)
    ElMessage.success('分类已删除')
    loadCategories()
    loadData()
  } catch (e) { console.error(e) }
}

// 删除资料
const removeMaterial = async (row) => {
  try {
    await ElMessageBox.confirm(`确认删除资料「${row.title}」？`, '删除确认', { type: 'warning', confirmButtonText: '删除' })
  } catch (e) { console.error(e); return }
  try {
    await request.delete(`/study-materials/${row.id}`)
    ElMessage.success('删除成功')
    loadData()
  } catch (e) { console.error(e) }
}

const uploadFile = async (options) => {
  const fd = new FormData()
  fd.append('file', options.file)
  fd.append('owner_type', 'study')
  fd.append('owner_id', '0')
  try {
    const res = await request.post('/uploads', fd)
    uploadedAttachments.value.push({ id: res.id, name: res.file_name })
    ElMessage.success(`附件「${res.file_name}」上传成功`)
  } catch (e) {
    options.onError(e)
  }
}

// 打开发布框：重置表单与附件，避免残留上次内容
const openCreate = () => {
  Object.assign(form, { title: '', content: '', category: 'theory' })
  fileList.value = []
  uploadedAttachments.value = []
  dialogVisible.value = true
}

const save = async () => {
  if (!form.title) return ElMessage.warning('请输入标题')
  if (!form.content) return ElMessage.warning('请输入内容')
  try {
    const res = await request.post('/study-materials', form)
    const matId = res.id
    for (const att of uploadedAttachments.value) {
      try {
        await request.put('/uploads/link', { id: att.id, owner_id: matId })
      } catch (e) { console.error(e) }
    }
    ElMessage.success('发布成功')
    dialogVisible.value = false
    Object.assign(form, { title: '', content: '', category: 'theory' })
    fileList.value = []
    uploadedAttachments.value = []
    loadData()
  } catch (e) { console.error(e) }
}

const openDetail = async (row) => {
  try {
    const res = await request.get(`/study-materials/${row.id}`)
    detail.value = res
    detailVisible.value = true
  } catch (e) { console.error(e) }
}

// 附件下载（带 token，避免直接 <a> 链接 401）
const downloadAtt = (a) => {
  downloadFile(`/uploads/${a.id}`, a.file_name || '附件')
}

onMounted(() => {
  loadData()
  loadCategories()
  if (route.query.id) {
    openDetail({ id: route.query.id })
  }
})

watch(() => route.query.id, (newId) => {
  if (newId) {
    openDetail({ id: newId })
  }
})
</script>

<style scoped>
.toolbar {
  display: flex;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 16px;
}
.dept-add-bar {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
  align-items: center;
}
.ml-8 { margin-left: 8px; }
.title-link {
  color: var(--el-text-color-primary);
  cursor: pointer;
}
.title-link:hover {
  color: var(--el-color-primary);
}
.meta {
  display: flex;
  gap: 16px;
  align-items: center;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.content {
  white-space: pre-wrap;
  line-height: 1.8;
}
.attachments h4 {
  margin-bottom: 8px;
  color: var(--el-text-color-primary);
}
.attach-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
  color: var(--el-color-primary);
}
.attach-item a {
  color: var(--el-color-primary);
  text-decoration: none;
}
.attach-item a:hover {
  text-decoration: underline;
}
</style>
