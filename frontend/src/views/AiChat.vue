<template>
  <div class="ai-chat-page">
    <page-header title="AI 写作助手" subtitle="选择场景后描述需求，AI 帮您起草通知、总结、信息稿等材料；按实际用量从个人额度扣费">
      <template #actions>
        <el-tooltip content="查看本月用量与每次扣费明细" placement="top">
          <el-button :icon="'Tickets'" @click="openUsage">用量明细</el-button>
        </el-tooltip>
        <el-tooltip content="清空当前对话，重新开始" placement="top">
          <el-button :icon="'Delete'" :disabled="!messages.length || generating" @click="clearChat">清空对话</el-button>
        </el-tooltip>
      </template>
    </page-header>

    <el-alert v-if="loaded && !aiState.enabled" type="info" :closable="false" show-icon title="管理员尚未开启 AI 写作助手，暂时无法使用。" class="mb" />

    <template v-else>
      <el-alert type="warning" :closable="false" show-icon class="mb">
        <template #title>
          <b>严禁输入国家秘密、工作秘密及内部资料。</b>
          <span v-if="aiState.provider === 'remote'">当前使用网络接口（{{ aiState.model }}），输入内容将发送至外部服务商。</span>
          <span v-else>当前使用本地模型，内容不出本机。</span>
          AI 生成内容可能有误，使用前须人工核对。
        </template>
      </el-alert>

      <div class="balance-bar">
        <template v-if="aiState.free">
          <el-tag type="success" effect="plain">本地模型 · 不扣费</el-tag>
        </template>
        <template v-else>
          <span>当前余额 <b class="balance" :class="{ 'is-low': aiState.low }">¥{{ formatYuan(aiState.balance, 2) }}</b></span>
          <el-tag v-if="aiState.low" type="danger" effect="plain" size="small">额度偏低，请联系管理员充值</el-tag>
          <span class="bar-sub">当前为{{ aiState.peakNow ? '高峰' : '空闲' }}时段{{ aiState.peakNow ? '' : '，价格更低' }}</span>
        </template>
      </div>

      <div class="chat-layout">
        <el-card shadow="never" class="scene-card" v-if="!isMobile">
          <div class="scene-title">选择场景</div>
          <div v-for="g in sceneGroups" :key="g.category" class="scene-group">
            <div class="scene-cat">{{ g.category }}</div>
            <el-tooltip v-for="p in g.items" :key="p.id" :content="p.hint || p.name" placement="right" :show-after="300">
              <div class="scene-item" :class="{ active: sceneId === p.id }" @click="pickScene(p)">{{ p.name }}</div>
            </el-tooltip>
          </div>
        </el-card>

        <el-card shadow="never" class="main-card">
          <el-select v-if="isMobile" v-model="sceneId" style="width:100%;margin-bottom:10px" @change="onSceneChange">
            <el-option-group v-for="g in sceneGroups" :key="g.category" :label="g.category">
              <el-option v-for="p in g.items" :key="p.id" :label="p.name" :value="p.id" />
            </el-option-group>
          </el-select>

          <div ref="listRef" class="msg-list">
            <div v-if="!messages.length" class="empty-tip">
              <div class="empty-title">{{ currentScene?.name || '自由提问' }}</div>
              <div>{{ currentScene?.hint || '任何办公写作问题都可以问' }}</div>
              <div class="empty-sub">在下方输入需求，按 Ctrl + Enter 发送</div>
            </div>
            <div v-for="(m, i) in messages" :key="i" class="msg" :class="m.role">
              <div class="bubble">
                <div class="text">{{ m.content }}<span v-if="generating && i === messages.length - 1 && m.role === 'assistant'" class="cursor">▍</span></div>
                <div v-if="m.role === 'assistant' && m.content && !(generating && i === messages.length - 1)" class="bubble-tools">
                  <el-tooltip content="复制这段内容" placement="top">
                    <el-button link type="primary" size="small" :icon="'CopyDocument'" @click="copy(m.content)">复制</el-button>
                  </el-tooltip>
                  <span v-if="m.cost !== undefined" class="cost">{{ aiState.free ? '不扣费' : '扣费 ¥' + formatYuan(m.cost) }}</span>
                </div>
              </div>
            </div>
          </div>

          <div v-if="errorMsg" class="err">{{ errorMsg }}</div>

          <div class="input-row">
            <el-input v-model="input" type="textarea" :autosize="{ minRows: 2, maxRows: 8 }" :maxlength="aiState.maxInput" show-word-limit
              :placeholder="currentScene?.placeholder || '请输入您的需求，或粘贴需要处理的文字'" :disabled="!aiState.enabled"
              @keydown.ctrl.enter.prevent="send" @keydown.meta.enter.prevent="send" />
            <div class="btn-col">
              <el-button v-if="generating" type="danger" plain @click="stop">停止</el-button>
              <el-tooltip v-else content="发送给 AI，将按用量扣费（Ctrl + Enter）" placement="top">
                <el-button type="primary" :disabled="!input.trim() || !aiState.enabled" @click="send">发送</el-button>
              </el-tooltip>
            </div>
          </div>
        </el-card>
      </div>
    </template>

    <el-drawer v-model="usageVisible" title="我的用量明细" :size="isMobile ? '100%' : '720px'">
      <div class="usage-sum" v-if="usage.summary">
        <div><span>当前余额</span><b>¥{{ formatYuan(usage.summary.balance, 2) }}</b></div>
        <div><span>本月花费</span><b>¥{{ formatYuan(usage.summary.month_cost, 4) }}</b></div>
        <div><span>本月调用</span><b>{{ usage.summary.month_calls }} 次</b></div>
        <div><span>累计充值</span><b>¥{{ formatYuan(usage.summary.total_topup, 2) }}</b></div>
      </div>
      <el-table :data="usage.list" v-loading="usageLoading" size="small" stripe>
        <el-table-column prop="time" label="时间" width="150" />
        <el-table-column prop="scene" label="场景" min-width="110" show-overflow-tooltip />
        <el-table-column label="用量(token)" width="110">
          <template #default="{ row }">{{ row.prompt_tokens + row.completion_tokens }}<el-tag v-if="row.estimated" size="small" type="info" style="margin-left:4px">估</el-tag></template>
        </el-table-column>
        <el-table-column label="扣费" width="90" align="right">
          <template #default="{ row }">¥{{ formatYuan(row.cost) }}</template>
        </el-table-column>
      </el-table>
      <el-pagination v-if="usage.total > usage.page_size" class="pager" small layout="prev, pager, next" :total="usage.total"
        :page-size="usage.page_size" :current-page="usage.page" @current-change="loadUsage" />
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '../utils/request'
import PageHeader from '../components/PageHeader.vue'
import { useMobile } from '../utils/useMobile'
import { aiState, loadAiState, streamChat, formatYuan } from '../utils/ai'

const { isMobile } = useMobile()
const loaded = ref(false)
const messages = ref([])
const input = ref('')
const generating = ref(false)
const errorMsg = ref('')
const sceneId = ref(0)
const listRef = ref(null)
let controller = null

// 对话页只展示“公文写作/通用”类场景；“业务嵌入”类场景是业务页按钮专用
const chatPrompts = computed(() => aiState.prompts.filter(p => p.category !== '业务嵌入'))
const sceneGroups = computed(() => {
  const map = new Map()
  for (const p of chatPrompts.value) {
    const c = p.category || '其他'
    if (!map.has(c)) map.set(c, [])
    map.get(c).push(p)
  }
  return [...map.entries()].map(([category, items]) => ({ category, items }))
})
const currentScene = computed(() => chatPrompts.value.find(p => p.id === sceneId.value))

const scrollBottom = () => nextTick(() => {
  if (listRef.value) listRef.value.scrollTop = listRef.value.scrollHeight
})

const pickScene = (p) => {
  if (generating.value) return
  sceneId.value = p.id
  onSceneChange()
}
// 切换场景后，之前的对话用的是另一套提示词，直接开新对话更清爽
const onSceneChange = () => {
  messages.value = []
  errorMsg.value = ''
}

const send = async () => {
  const text = input.value.trim()
  if (!text || generating.value || !aiState.enabled) return
  errorMsg.value = ''
  messages.value.push({ role: 'user', content: text })
  input.value = ''
  const reply = { role: 'assistant', content: '' }
  messages.value.push(reply)
  const idx = messages.value.length - 1
  generating.value = true
  controller = new AbortController()
  scrollBottom()
  const history = messages.value.slice(0, idx).map(m => ({ role: m.role, content: m.content }))
  const r = await streamChat({
    sceneId: sceneId.value,
    messages: history,
    onDelta: (t) => { messages.value[idx].content += t; scrollBottom() },
    signal: controller.signal
  })
  generating.value = false
  controller = null
  if (r.done) messages.value[idx].cost = r.done.cost
  if (!messages.value[idx].content) {
    // 没有任何输出：去掉空气泡，并把问题放回输入框方便重发
    messages.value.splice(idx, 1)
    const last = messages.value.pop()
    if (last) input.value = last.content
  }
  if (r.error) {
    errorMsg.value = r.error
    if (r.code === 'AI_NO_BALANCE') loadAiState(true)
  }
  scrollBottom()
}

const stop = () => controller?.abort()

const clearChat = async () => {
  try {
    await ElMessageBox.confirm('清空后当前对话内容将无法恢复，确认清空？', '清空对话', { type: 'warning', confirmButtonText: '清空' })
  } catch (e) { return }
  messages.value = []
  errorMsg.value = ''
}

const copy = (text) => {
  navigator.clipboard.writeText(text).then(() => ElMessage.success('已复制')).catch(() => ElMessage.info('请手动选中复制'))
}

// 用量明细
const usageVisible = ref(false)
const usageLoading = ref(false)
const usage = ref({ list: [], total: 0, page: 1, page_size: 20, summary: null })
const loadUsage = async (page = 1) => {
  usageLoading.value = true
  try {
    usage.value = await request.get('/ai/usage', { params: { page } })
  } catch (e) { console.error(e) } finally { usageLoading.value = false }
}
const openUsage = () => {
  usageVisible.value = true
  loadUsage(1)
}

onMounted(async () => {
  await loadAiState(true)
  loaded.value = true
  const first = chatPrompts.value.find(p => p.scene_key === 'chat_free') || chatPrompts.value[0]
  if (first) sceneId.value = first.id
})
onBeforeUnmount(() => controller?.abort())
</script>

<style scoped>
.mb { margin-bottom: 12px; }
.balance-bar { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; margin-bottom: 12px; color: var(--yx-text-2); font-size: var(--yx-fs-sm); }
.balance { font-size: var(--yx-fs-md); color: var(--yx-ink-2); }
.balance.is-low { color: var(--yx-brand); }
.bar-sub { color: var(--yx-text-3); font-size: var(--yx-fs-xs); }
.chat-layout { display: flex; gap: 16px; align-items: stretch; }
.scene-card { width: 210px; flex-shrink: 0; align-self: flex-start; }
.scene-title { font-weight: 600; margin-bottom: 8px; color: var(--yx-text-1); }
.scene-cat { font-size: var(--yx-fs-xs); color: var(--yx-text-3); margin: 10px 0 4px; }
.scene-item { padding: 8px 10px; border-radius: var(--yx-radius-sm); cursor: pointer; color: var(--yx-text-2); font-size: var(--yx-fs-sm); }
.scene-item:hover { background: var(--yx-ink-2-bg); }
.scene-item.active { background: var(--yx-ink-2-bg); color: var(--yx-ink-2); font-weight: 600; }
.main-card { flex: 1; min-width: 0; }
.msg-list { height: 52vh; min-height: 300px; overflow-y: auto; padding: 4px 2px 12px; }
.empty-tip { text-align: center; color: var(--yx-text-3); padding-top: 12vh; line-height: 2; }
.empty-title { font-size: var(--yx-fs-lg); color: var(--yx-text-1); font-family: var(--yx-font-serif, serif); }
.empty-sub { font-size: var(--yx-fs-xs); }
.msg { display: flex; margin-bottom: 14px; }
.msg.user { justify-content: flex-end; }
.bubble { max-width: 88%; padding: 10px 14px; border-radius: var(--yx-radius); line-height: 1.8; font-size: var(--yx-fs-sm); }
.msg.user .bubble { background: var(--yx-ink-2); color: #fff; }
.msg.assistant .bubble { background: var(--yx-surface-muted); border: 1px solid var(--yx-border-soft); color: var(--yx-text-1); }
.text { white-space: pre-wrap; word-break: break-word; }
.cursor { animation: blink 1s steps(2) infinite; }
@keyframes blink { 50% { opacity: 0; } }
.bubble-tools { margin-top: 6px; display: flex; align-items: center; gap: 12px; }
.cost { color: var(--yx-text-3); font-size: var(--yx-fs-xs); }
.err { color: var(--yx-brand); margin: 6px 0; font-size: var(--yx-fs-sm); }
.input-row { display: flex; gap: 10px; align-items: flex-end; margin-top: 8px; }
.input-row .el-textarea { flex: 1; }
.usage-sum { display: grid; grid-template-columns: repeat(4, 1fr); gap: 10px; margin-bottom: 14px; }
.usage-sum div { background: var(--yx-surface-muted); border: 1px solid var(--yx-border-soft); border-radius: var(--yx-radius-sm); padding: 8px 10px; display: flex; flex-direction: column; gap: 2px; }
.usage-sum span { font-size: var(--yx-fs-xs); color: var(--yx-text-3); }
.usage-sum b { color: var(--yx-text-1); }
.pager { margin-top: 12px; justify-content: center; }
@media (max-width: 767px) {
  /* 页头按钮在手机端固定在屏幕底部，给页面留出位置，避免盖住发送按钮 */
  .ai-chat-page { padding-bottom: 88px; }
  .chat-layout { display: block; }
  .msg-list { height: 48vh; }
  .bubble { max-width: 96%; }
  .input-row { flex-direction: column; align-items: stretch; }
  .btn-col :deep(.el-button) { width: 100%; height: 44px; }
  .usage-sum { grid-template-columns: repeat(2, 1fr); }
  :deep(.el-textarea__inner) { font-size: 16px; }
}
</style>
