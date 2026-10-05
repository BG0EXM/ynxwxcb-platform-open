<template>
  <el-dialog
    v-model="visible"
    :show-close="false"
    :close-on-click-modal="true"
    :close-on-press-escape="true"
    width="680px"
    top="10vh"
    class="global-search-dialog"
    append-to-body
    transition="el-zoom-in-top"
    @opened="onDialogOpened"
  >
    <div class="search-modal-container">
      <!-- 顶部搜索输入栏 -->
      <div class="search-input-wrapper">
        <el-icon class="search-icon"><Search /></el-icon>
        <input
          ref="inputRef"
          v-model="keyword"
          type="text"
          class="search-native-input font-sans"
          placeholder="搜索收文、人员、会议、资料或功能页面..."
          @input="handleInput"
          @keydown="handleKeyDown"
        />
        <div class="search-right-badges">
          <span v-if="loading" class="search-loading-icon">
            <el-icon class="is-loading"><Loading /></el-icon>
          </span>
          <button v-else-if="keyword" class="clear-btn" @click="clearKeyword" title="清空输入">
            <el-icon><CircleClose /></el-icon>
          </button>
          <div class="shortcut-tag">
            <kbd>{{ isMac ? '⌘' : 'Ctrl' }}</kbd>
            <kbd>K</kbd>
          </div>
        </div>
      </div>

      <!-- 检索内容结果区 -->
      <div ref="listContainerRef" class="search-results-area">
        <!-- 1. 加载中状态 -->
        <div v-if="loading && flatItems.length === 0" class="search-status-box">
          <el-icon class="is-loading"><Loading /></el-icon>
          <span>正在跨模块检索公文、通讯录、资料...</span>
        </div>

        <!-- 2. 无匹配结果 -->
        <div v-else-if="keyword.trim() && flatItems.length === 0" class="search-status-box empty">
          <el-icon class="empty-icon"><DocumentDelete /></el-icon>
          <div class="empty-text">未检索到与“{{ keyword }}”相关的信息</div>
          <div class="empty-hint">可尝试搜索：文号、标题、干部姓名、科室、电话或菜单名称</div>
        </div>

        <!-- 3. 有搜索结果 或 空词展示快捷推荐 -->
        <div v-else class="groups-container">
          <!-- 3.1 页面快捷导航 -->
          <div v-if="navResults.length" class="result-group">
            <div class="group-title">
              <el-icon class="group-icon"><Compass /></el-icon>
              <span>快捷导航与指令</span>
              <span class="group-count">{{ navResults.length }}</span>
            </div>
            <div class="group-items">
              <div
                v-for="item in navResults"
                :key="'nav-' + item.path"
                :class="['result-item', 'nav-item', { 'is-active': activeFlatIndex === item._flatIndex }]"
                @click="selectItem(item)"
                @mouseenter="activeFlatIndex = item._flatIndex"
              >
                <div class="item-icon-box nav-icon">
                  <el-icon><component :is="item.icon || 'FolderOpened'" /></el-icon>
                </div>
                <div class="item-main">
                  <div class="item-title font-sans">
                    <span v-html="highlight(item.title)"></span>
                    <span class="item-badge nav-badge">{{ item.sub_title || '页面跳转' }}</span>
                  </div>
                  <div class="item-desc">{{ item.path }}</div>
                </div>
                <div class="item-action">
                  <span class="enter-hint">跳转 <el-icon><Right /></el-icon></span>
                </div>
              </div>
            </div>
          </div>

          <!-- 3.2 干部通讯录 -->
          <div v-if="contactResults.length" class="result-group">
            <div class="group-title">
              <el-icon class="group-icon"><Phone /></el-icon>
              <span>干部通讯录</span>
              <span class="group-count">{{ contactResults.length }}</span>
            </div>
            <div class="group-items">
              <div
                v-for="item in contactResults"
                :key="'contact-' + item.id"
                :class="['result-item', 'contact-item', { 'is-active': activeFlatIndex === item._flatIndex }]"
                @click="selectItem(item)"
                @mouseenter="activeFlatIndex = item._flatIndex"
              >
                <div class="item-icon-box contact-icon">
                  <span class="avatar-initial">{{ (item.name || '').slice(0, 1) }}</span>
                </div>
                <div class="item-main">
                  <div class="item-title font-sans">
                    <span class="person-name" v-html="highlight(item.name)"></span>
                    <span v-if="item.department" class="item-badge dept-badge">{{ item.department }}</span>
                    <span v-if="item.position" class="item-badge position-badge">{{ item.position }}</span>
                  </div>
                  <div class="item-desc phone-desc">
                    <el-icon class="phone-icon"><PhoneFilled /></el-icon>
                    <span class="phone-num font-mono" v-html="highlight(item.phone)"></span>
                  </div>
                </div>
                <div class="item-action contact-actions" @click.stop>
                  <a
                    v-if="item.phone"
                    :href="'tel:' + item.phone"
                    class="action-btn call-btn"
                    title="一键拨号"
                    @click.stop
                  >
                    <el-icon><Phone /></el-icon>
                    <span>拨号</span>
                  </a>
                  <button
                    v-if="item.phone"
                    class="action-btn copy-btn"
                    title="复制手机号"
                    @click.stop="copyPhone(item.phone)"
                  >
                    <el-icon><CopyDocument /></el-icon>
                    <span>复制</span>
                  </button>
                </div>
              </div>
            </div>
          </div>

          <!-- 3.3 收文公文 -->
          <div v-if="incomingResults.length" class="result-group">
            <div class="group-title">
              <el-icon class="group-icon"><Tickets /></el-icon>
              <span>收文公文</span>
              <span class="group-count">{{ incomingResults.length }}</span>
            </div>
            <div class="group-items">
              <div
                v-for="item in incomingResults"
                :key="'incoming-' + item.id"
                :class="['result-item', 'doc-item', { 'is-active': activeFlatIndex === item._flatIndex }]"
                @click="selectItem(item)"
                @mouseenter="activeFlatIndex = item._flatIndex"
              >
                <div class="item-icon-box doc-icon">
                  <el-icon><Document /></el-icon>
                </div>
                <div class="item-main">
                  <div class="item-title font-sans" v-html="highlight(item.title)"></div>
                  <div class="item-desc">
                    <span v-if="item.source_unit" class="doc-from">来文：{{ item.source_unit }}</span>
                    <span v-if="item.doc_no" class="doc-no font-mono">{{ item.doc_no }}</span>
                  </div>
                </div>
                <div class="item-action">
                  <span class="enter-hint">详情 <el-icon><Right /></el-icon></span>
                </div>
              </div>
            </div>
          </div>

          <!-- 3.4 公共资料 -->
          <div v-if="studyResults.length" class="result-group">
            <div class="group-title">
              <el-icon class="group-icon"><Reading /></el-icon>
              <span>公共资料</span>
              <span class="group-count">{{ studyResults.length }}</span>
            </div>
            <div class="group-items">
              <div
                v-for="item in studyResults"
                :key="'study-' + item.id"
                :class="['result-item', 'study-item', { 'is-active': activeFlatIndex === item._flatIndex }]"
                @click="selectItem(item)"
                @mouseenter="activeFlatIndex = item._flatIndex"
              >
                <div class="item-icon-box study-icon">
                  <el-icon><Reading /></el-icon>
                </div>
                <div class="item-main">
                  <div class="item-title font-sans" v-html="highlight(item.title)"></div>
                  <div class="item-desc">
                    <span v-if="item.category" class="study-cat">{{ item.category }}</span>
                    <span class="study-type">学习文件与资料</span>
                  </div>
                </div>
                <div class="item-action">
                  <span class="enter-hint">查看 <el-icon><Right /></el-icon></span>
                </div>
              </div>
            </div>
          </div>

          <!-- 3.5 重要会议 -->
          <div v-if="meetingResults.length" class="result-group">
            <div class="group-title">
              <el-icon class="group-icon"><OfficeBuilding /></el-icon>
              <span>重要会议</span>
              <span class="group-count">{{ meetingResults.length }}</span>
            </div>
            <div class="group-items">
              <div
                v-for="item in meetingResults"
                :key="'meeting-' + item.id"
                :class="['result-item', 'meeting-item', { 'is-active': activeFlatIndex === item._flatIndex }]"
                @click="selectItem(item)"
                @mouseenter="activeFlatIndex = item._flatIndex"
              >
                <div class="item-icon-box meeting-icon">
                  <el-icon><Calendar /></el-icon>
                </div>
                <div class="item-main">
                  <div class="item-title font-sans" v-html="highlight(item.title)"></div>
                  <div class="item-desc">
                    <span v-if="item.time" class="meeting-time">{{ item.time }}</span>
                    <span v-if="item.location" class="meeting-loc">地点：{{ item.location }}</span>
                  </div>
                </div>
                <div class="item-action">
                  <span class="enter-hint">会务 <el-icon><Right /></el-icon></span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 底部快捷操作提示栏 -->
      <div class="search-footer-bar">
        <div class="footer-tips">
          <span class="tip-item"><kbd>↑</kbd><kbd>↓</kbd> 选择</span>
          <span class="tip-item"><kbd>↵</kbd> 确认</span>
          <span class="tip-item"><kbd>ESC</kbd> 关闭</span>
        </div>
        <div class="footer-platform-info font-serif">
          <span>伊宁县委宣传部 · 快捷指挥台</span>
        </div>
      </div>
    </div>
  </el-dialog>
</template>

<script setup>
import { ref, computed, nextTick, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import request from '../utils/request'

const router = useRouter()

const visible = ref(false)
const keyword = ref('')
const loading = ref(false)
const inputRef = ref(null)
const listContainerRef = ref(null)
const activeFlatIndex = ref(0)
let debounceTimer = null

const isMac = ref(typeof navigator !== 'undefined' && /Mac|iPod|iPhone|iPad/.test(navigator.platform))

// 结果分类
const navResults = ref([])
const contactResults = ref([])
const incomingResults = ref([])
const studyResults = ref([])
const meetingResults = ref([])

// 默认常用功能快捷导航（空搜索时展示）
const defaultQuickNavs = [
  { type: 'nav', title: '收文管理', path: '/incoming', icon: 'FolderOpened', sub_title: '公文呈批与办理' },
  { type: 'nav', title: '通讯录', path: '/contacts', icon: 'Phone', sub_title: '干部联系电话速查' },
  { type: 'nav', title: '考勤点到', path: '/attendance', icon: 'CircleCheck', sub_title: '每日点到与出勤统计' },
  { type: 'nav', title: '值守排班', path: '/duty', icon: 'AlarmClock', sub_title: '值班安排与大网格' },
  { type: 'nav', title: '工作日历', path: '/calendar', icon: 'Calendar', sub_title: '部务日程与活动备忘' },
  { type: 'nav', title: '公车管理', path: '/vehicles', icon: 'Van', sub_title: '派车申请与用车台账' },
  { type: 'nav', title: '会务管理', path: '/meetings', icon: 'OfficeBuilding', sub_title: '重要会议报名与签到' },
  { type: 'nav', title: '公共资料', path: '/study', icon: 'Reading', sub_title: '理论学习与业务资料' }
]

// 扁平化所有可见项，便于上下键盘导航
const flatItems = computed(() => {
  const list = []
  let index = 0

  const appendList = (items) => {
    for (const item of items) {
      item._flatIndex = index++
      list.push(item)
    }
  }

  appendList(navResults.value)
  appendList(contactResults.value)
  appendList(incomingResults.value)
  appendList(studyResults.value)
  appendList(meetingResults.value)

  return list
})

// 聚焦输入框
const onDialogOpened = () => {
  nextTick(() => {
    inputRef.value?.focus()
  })
}

// 打开弹窗
const open = () => {
  visible.value = true
  activeFlatIndex.value = 0
  if (!keyword.value.trim()) {
    initDefaultResults()
  }
  nextTick(() => {
    inputRef.value?.focus()
  })
}

// 关闭弹窗
const close = () => {
  visible.value = false
}

// 清空输入框
const clearKeyword = () => {
  keyword.value = ''
  initDefaultResults()
  nextTick(() => {
    inputRef.value?.focus()
  })
}

// 初始化默认推荐
const initDefaultResults = () => {
  navResults.value = defaultQuickNavs.map(item => ({ ...item }))
  contactResults.value = []
  incomingResults.value = []
  studyResults.value = []
  meetingResults.value = []
  activeFlatIndex.value = 0
}

// 监听输入，300ms 政务防抖
const handleInput = () => {
  clearTimeout(debounceTimer)
  const query = keyword.value.trim()
  if (!query) {
    loading.value = false
    initDefaultResults()
    return
  }

  loading.value = true
  debounceTimer = setTimeout(() => {
    executeSearch(query)
  }, 300)
}

// 执行联合检索
const executeSearch = async (query) => {
  try {
    const res = await request.get('/global-search', { params: { q: query } })
    const items = Array.isArray(res) ? res : (res?.list || [])

    navResults.value = items.filter(i => i.type === 'nav')
    contactResults.value = items.filter(i => i.type === 'contact')
    incomingResults.value = items.filter(i => i.type === 'incoming')
    studyResults.value = items.filter(i => i.type === 'study')
    meetingResults.value = items.filter(i => i.type === 'meeting')

    activeFlatIndex.value = 0
    scrollToActive()
  } catch (err) {
    console.error('全局速查请求失败', err)
  } finally {
    loading.value = false
  }
}

// 键盘按键事件（↑、↓、↵、Esc）
const handleKeyDown = (e) => {
  const total = flatItems.value.length
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    if (total > 0) {
      activeFlatIndex.value = (activeFlatIndex.value + 1) % total
      scrollToActive()
    }
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    if (total > 0) {
      activeFlatIndex.value = (activeFlatIndex.value - 1 + total) % total
      scrollToActive()
    }
  } else if (e.key === 'Enter') {
    e.preventDefault()
    if (total > 0 && activeFlatIndex.value >= 0 && activeFlatIndex.value < total) {
      selectItem(flatItems.value[activeFlatIndex.value])
    }
  } else if (e.key === 'Escape') {
    close()
  }
}

// 滚动激活项至可视区
const scrollToActive = () => {
  nextTick(() => {
    const container = listContainerRef.value
    if (!container) return
    const activeEl = container.querySelector('.result-item.is-active')
    if (activeEl) {
      activeEl.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
    }
  })
}

// 点击或回车选中项
const selectItem = (item) => {
  if (!item) return
  close()

  switch (item.type) {
    case 'nav':
      if (item.path) {
        router.push(item.path)
      }
      break
    case 'incoming':
      router.push({ path: '/incoming', query: { id: item.id } })
      break
    case 'contact':
      router.push({ path: '/contacts', query: { keyword: item.name } })
      break
    case 'study':
      router.push({ path: '/study', query: { id: item.id } })
      break
    case 'meeting':
      router.push({ path: '/meetings', query: { id: item.id } })
      break
    default:
      if (item.path) router.push(item.path)
      break
  }
}

// 复制联系方式
const copyPhone = async (phone) => {
  if (!phone) return
  try {
    if (navigator?.clipboard?.writeText) {
      await navigator.clipboard.writeText(phone)
    } else {
      const textarea = document.createElement('textarea')
      textarea.value = phone
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
    }
    ElMessage.success(`电话号码「${phone}」已复制`)
  } catch (err) {
    ElMessage.error('复制失败')
  }
}

// 高亮关键词
const highlight = (text) => {
  if (!text) return ''
  const q = keyword.value.trim()
  if (!q) return String(text)
  const escaped = q.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const reg = new RegExp(`(${escaped})`, 'gi')
  return String(text).replace(reg, '<mark class="highlight-kw">$1</mark>')
}

// 全局快捷键监听（Cmd+K / Ctrl+K）
const handleGlobalShortcut = (e) => {
  if ((e.metaKey || e.ctrlKey) && (e.key === 'k' || e.key === 'K')) {
    e.preventDefault()
    if (visible.value) {
      close()
    } else {
      open()
    }
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleGlobalShortcut)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleGlobalShortcut)
})

defineExpose({
  open,
  close
})
</script>

<style scoped>
/* 弹窗整体政务风容器 */
.search-modal-container {
  display: flex;
  flex-direction: column;
  background: #ffffff;
  border-radius: var(--yx-radius, 10px);
  overflow: hidden;
  box-shadow: 0 20px 48px rgba(19, 26, 38, 0.22), 0 2px 8px rgba(19, 26, 38, 0.08);
}

/* 顶部搜索输入框 */
.search-input-wrapper {
  display: flex;
  align-items: center;
  padding: 16px 20px;
  background: #ffffff;
  border-bottom: 1px solid rgba(220, 223, 229, 0.7);
  position: relative;
  gap: 12px;
}

.search-icon {
  font-size: 20px;
  color: var(--yx-brand, #9e1b1e);
  flex-shrink: 0;
}

.search-native-input {
  flex: 1;
  border: none;
  outline: none;
  font-size: 16px;
  color: var(--yx-text-1, #1f2329);
  background: transparent;
  padding: 0;
}

.search-native-input::placeholder {
  color: var(--yx-text-4, #8a94a0);
  font-size: 15px;
}

.search-right-badges {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.search-loading-icon {
  color: var(--yx-brand, #9e1b1e);
  font-size: 18px;
}

.clear-btn {
  background: none;
  border: none;
  color: var(--yx-text-4, #8a94a0);
  cursor: pointer;
  padding: 2px;
  display: flex;
  align-items: center;
  border-radius: 4px;
}

.clear-btn:hover {
  color: var(--yx-text-2, #4e5969);
}

.shortcut-tag {
  display: flex;
  align-items: center;
  gap: 4px;
}

.shortcut-tag kbd {
  display: inline-block;
  padding: 2px 6px;
  font-size: 11px;
  font-family: var(--yx-font-mono, monospace);
  color: var(--yx-text-3, #6b7785);
  background: #f2f3f5;
  border: 1px solid #dcdfe5;
  border-radius: 4px;
  box-shadow: 0 1px 0 rgba(0, 0, 0, 0.05);
}

/* 结果列表区 */
.search-results-area {
  max-height: 440px;
  min-height: 200px;
  overflow-y: auto;
  padding: 10px 14px;
  background: #fcfcfc;
}

/* 状态展示 */
.search-status-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 20px;
  gap: 12px;
  color: var(--yx-text-3, #6b7785);
  font-size: 14px;
}

.search-status-box.empty {
  color: var(--yx-text-2, #4e5969);
}

.empty-icon {
  font-size: 40px;
  color: #c0c4cc;
}

.empty-text {
  font-size: 15px;
  font-weight: 500;
  color: var(--yx-text-1, #1f2329);
}

.empty-hint {
  font-size: 13px;
  color: var(--yx-text-3, #6b7785);
}

/* 分组结构 */
.groups-container {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.result-group {
  display: flex;
  flex-direction: column;
}

.group-title {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 8px;
  font-size: 12px;
  font-weight: 600;
  color: var(--yx-text-3, #6b7785);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.group-icon {
  font-size: 13px;
  color: var(--yx-brand, #9e1b1e);
}

.group-count {
  margin-left: auto;
  font-size: 11px;
  color: var(--yx-text-4, #8a94a0);
  background: #ebedf0;
  padding: 1px 6px;
  border-radius: 10px;
}

.group-items {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

/* 单行条目 */
.result-item {
  display: flex;
  align-items: center;
  padding: 9px 12px;
  border-radius: 8px;
  background: #ffffff;
  border: 1px solid transparent;
  cursor: pointer;
  transition: all 0.15s ease;
  gap: 12px;
}

.result-item:hover,
.result-item.is-active {
  background: #f7f3f3;
  border-color: rgba(158, 27, 30, 0.2);
}

.result-item.is-active {
  box-shadow: 0 2px 8px rgba(158, 27, 30, 0.08);
}

/* 图标容器 */
.item-icon-box {
  width: 34px;
  height: 34px;
  border-radius: 7px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 17px;
  flex-shrink: 0;
}

.nav-icon {
  background: #f0f4fa;
  color: var(--yx-ink-2, #24406e);
}

.contact-icon {
  background: #fbf1e8;
  color: #d46b08;
  font-weight: bold;
}

.avatar-initial {
  font-size: 15px;
}

.doc-icon {
  background: #faecec;
  color: var(--yx-brand, #9e1b1e);
}

.study-icon {
  background: #eef7f2;
  color: #389e0d;
}

.meeting-icon {
  background: #f3f0fa;
  color: #722ed1;
}

/* 内容主体 */
.item-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.item-title {
  font-size: 14.5px;
  font-weight: 500;
  color: var(--yx-text-1, #1f2329);
  display: flex;
  align-items: center;
  gap: 8px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.person-name {
  font-weight: 600;
}

.item-desc {
  font-size: 12.5px;
  color: var(--yx-text-3, #6b7785);
  display: flex;
  align-items: center;
  gap: 10px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 标签样式 */
.item-badge {
  font-size: 11px;
  padding: 1px 6px;
  border-radius: 4px;
  font-weight: normal;
}

.nav-badge {
  background: #eef2f8;
  color: #24406e;
}

.dept-badge {
  background: #fdf6ec;
  color: #b88230;
  border: 1px solid #faecd8;
}

.position-badge {
  background: #f0f5ff;
  color: #2f54eb;
  border: 1px solid #d6e4ff;
}

.doc-from {
  color: var(--yx-text-2, #4e5969);
}

.doc-no {
  color: var(--yx-brand, #9e1b1e);
  background: #fff1f0;
  padding: 0 4px;
  border-radius: 3px;
}

.phone-desc {
  display: flex;
  align-items: center;
  gap: 4px;
}

.phone-icon {
  font-size: 12px;
  color: #d46b08;
}

.phone-num {
  font-weight: 500;
  letter-spacing: 0.5px;
}

.study-cat {
  background: #f6ffed;
  color: #389e0d;
  padding: 0 4px;
  border-radius: 3px;
}

.meeting-time {
  color: var(--yx-brand, #9e1b1e);
  font-weight: 500;
}

.meeting-loc {
  color: var(--yx-text-2, #4e5969);
}

/* 右侧操作按钮 */
.item-action {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 6px;
}

.enter-hint {
  font-size: 12px;
  color: var(--yx-text-4, #8a94a0);
  display: flex;
  align-items: center;
  gap: 4px;
  opacity: 0;
  transition: opacity 0.15s ease;
}

.result-item:hover .enter-hint,
.result-item.is-active .enter-hint {
  opacity: 1;
  color: var(--yx-brand, #9e1b1e);
}

.contact-actions {
  display: flex;
  gap: 6px;
}

.action-btn {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 3px 8px;
  border-radius: 4px;
  font-size: 12px;
  cursor: pointer;
  text-decoration: none;
  transition: all 0.15s ease;
}

.call-btn {
  background: #f6ffed;
  color: #389e0d;
  border: 1px solid #b7eb8f;
}

.call-btn:hover {
  background: #52c41a;
  color: #ffffff;
  border-color: #52c41a;
}

.copy-btn {
  background: #f0f5ff;
  color: #1d39c4;
  border: 1px solid #adc6ff;
}

.copy-btn:hover {
  background: #2f54eb;
  color: #ffffff;
  border-color: #2f54eb;
}

/* 底部操作提示栏 */
.search-footer-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 18px;
  background: #f5f6f8;
  border-top: 1px solid rgba(220, 223, 229, 0.7);
  font-size: 12px;
  color: var(--yx-text-3, #6b7785);
}

.footer-tips {
  display: flex;
  align-items: center;
  gap: 14px;
}

.tip-item {
  display: flex;
  align-items: center;
  gap: 4px;
}

.tip-item kbd {
  display: inline-block;
  padding: 1px 5px;
  font-size: 11px;
  font-family: var(--yx-font-mono, monospace);
  color: var(--yx-text-2, #4e5969);
  background: #ffffff;
  border: 1px solid #dcdfe5;
  border-radius: 3px;
  box-shadow: 0 1px 0 rgba(0, 0, 0, 0.08);
}

.footer-platform-info {
  font-size: 12px;
  color: var(--yx-gold, #b8925a);
}

:deep(.highlight-kw) {
  background: #fffb8f;
  color: #d4380d;
  padding: 0 2px;
  border-radius: 2px;
  font-weight: 600;
}
</style>

<style>
/* 覆盖弹窗样式为现代面板 */
.global-search-dialog.el-dialog {
  padding: 0 !important;
  border-radius: 10px !important;
  background: transparent !important;
  box-shadow: none !important;
  overflow: hidden !important;
}

.global-search-dialog .el-dialog__header {
  display: none !important;
}

.global-search-dialog .el-dialog__body {
  padding: 0 !important;
}
</style>
