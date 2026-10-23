<template>
  <template v-if="visible">
    <el-tooltip :content="tip || scene?.hint || 'AI 辅助写作，按用量从个人额度扣费，结果仅供参考'" placement="top" :show-after="300">
      <el-button :type="type" :link="link" :size="size" :icon="'MagicStick'" @click="open">{{ label }}</el-button>
    </el-tooltip>

    <el-dialog v-model="show" :title="scene?.name || 'AI 辅助写作'" :width="isMobile ? '100%' : '720px'" :fullscreen="isMobile"
      append-to-body destroy-on-close :close-on-click-modal="false" top="4vh" class="ai-assist-dialog" @closed="onClosed">
      <el-alert type="warning" :closable="false" show-icon class="ai-warn">
        <template #title>
          <b>严禁输入国家秘密、工作秘密及内部资料。</b>
          <span v-if="aiState.provider === 'remote'">当前使用网络接口，输入内容将发送至外部服务商。</span>
        </template>
      </el-alert>

      <div class="ai-meta">
        <span v-if="aiState.free">本地模型 · 不扣费</span>
        <span v-else>
          余额 <b :class="{ 'is-low': aiState.low }">¥{{ formatYuan(aiState.balance, 2) }}</b>
          <span class="ai-sub">（按实际用量扣费，一般一次几分钱）</span>
        </span>
        <span v-if="aiState.low && !aiState.free" class="ai-low">额度偏低，请联系管理员充值</span>
      </div>

      <div class="ai-label">原始内容 <span class="ai-sub">可直接修改；{{ scene?.hint }}</span></div>
      <el-input v-model="input" type="textarea" :rows="5" :maxlength="aiState.maxInput" show-word-limit
        :placeholder="scene?.placeholder || '请输入或粘贴需要处理的内容'" />
      <div class="ai-label">补充要求 <span class="ai-sub">选填，如“字数控制在300字内”“语气更正式”</span></div>
      <el-input v-model="extra" maxlength="200" placeholder="选填" />

      <div class="ai-label">
        AI 生成结果
        <span class="ai-sub">AI 生成，须人工核对后再使用</span>
      </div>
      <el-input v-model="result" type="textarea" :rows="7" :placeholder="generating ? 'AI 正在撰写……' : '点击“生成”后在这里显示，可直接修改'" />
      <div v-if="info" class="ai-info">{{ info }}</div>
      <div v-if="errorMsg" class="ai-error">{{ errorMsg }}</div>

      <template #footer>
        <el-button @click="show = false">关闭</el-button>
        <el-button v-if="generating" type="danger" plain @click="stop">停止</el-button>
        <el-button v-else type="primary" :disabled="!input.trim()" @click="generate">{{ result ? '重新生成' : '生成' }}</el-button>
        <el-button v-if="result && !generating" :icon="'CopyDocument'" @click="copyResult">复制</el-button>
        <el-button v-if="result && !generating" type="success" @click="apply">{{ applyText }}</el-button>
      </template>
    </el-dialog>
  </template>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '../store/auth'
import { useMobile } from '../utils/useMobile'
import { aiState, loadAiState, findScene, streamChat, formatYuan } from '../utils/ai'

// scene：提示词场景标识（如 meeting_notice）；text：初始内容（字符串或返回字符串的函数）；
// 不传 apply 监听时，不显示“采用到页面”按钮
const props = defineProps({
  scene: { type: String, required: true },
  text: { type: [String, Function], default: '' },
  label: { type: String, default: 'AI 润色' },
  tip: { type: String, default: '' },
  type: { type: String, default: 'default' },
  size: { type: String, default: 'default' },
  link: { type: Boolean, default: false },
  applyText: { type: String, default: '采用到页面' }
})
const emit = defineEmits(['apply'])

const authStore = useAuthStore()
const { isMobile } = useMobile()
const show = ref(false)
const input = ref('')
const extra = ref('')
const result = ref('')
const info = ref('')
const errorMsg = ref('')
const generating = ref(false)
let controller = null

const scene = computed(() => findScene(props.scene))
const visible = computed(() => authStore.hasPerm('ai.use') && aiState.enabled && !!scene.value)

onMounted(() => {
  if (authStore.hasPerm('ai.use')) loadAiState()
})
onBeforeUnmount(() => controller?.abort())

const open = async () => {
  await loadAiState(true)
  if (!aiState.enabled) return ElMessage.warning('管理员已关闭 AI 写作助手')
  input.value = (typeof props.text === 'function' ? props.text() : props.text) || ''
  extra.value = ''
  result.value = ''
  info.value = ''
  errorMsg.value = ''
  show.value = true
}

const generate = async () => {
  if (generating.value || !scene.value) return
  generating.value = true
  result.value = ''
  info.value = ''
  errorMsg.value = ''
  controller = new AbortController()
  const content = extra.value.trim() ? `${input.value.trim()}\n\n【补充要求】${extra.value.trim()}` : input.value.trim()
  const r = await streamChat({
    sceneId: scene.value.id,
    messages: [{ role: 'user', content }],
    onDelta: (t) => { result.value += t },
    signal: controller.signal
  })
  generating.value = false
  controller = null
  if (r.aborted) return
  if (r.error) errorMsg.value = r.error
  if (r.done) {
    const d = r.done
    const cost = aiState.free ? '本地模型不扣费' : `本次扣费 ¥${formatYuan(d.cost)}，余额 ¥${formatYuan(d.balance, 2)}`
    info.value = `${cost}${d.truncated ? '；内容较长已被截断，可缩短原文后重试' : ''}`
  }
}

const stop = () => controller?.abort()
const onClosed = () => controller?.abort()

const copyResult = () => {
  navigator.clipboard.writeText(result.value).then(() => ElMessage.success('已复制')).catch(() => ElMessage.info('请手动选中复制'))
}

const apply = () => {
  emit('apply', result.value)
  show.value = false
}
</script>

<style>
/* 对话框内容较多时在正文区内滚动，保证底部按钮始终可见（对话框挂在 body 下，需非 scoped 样式） */
.ai-assist-dialog .el-dialog__body { max-height: 64vh; overflow-y: auto; padding-top: 10px; }
</style>
<style scoped>
.ai-warn { margin-bottom: 10px; }
.ai-meta { display: flex; flex-wrap: wrap; gap: 12px; align-items: center; margin-bottom: 10px; color: var(--yx-text-2); font-size: var(--yx-fs-sm); }
.ai-meta .is-low { color: var(--yx-brand); }
.ai-low { color: var(--yx-brand); }
.ai-label { margin: 12px 0 6px; font-size: var(--yx-fs-sm); color: var(--yx-text-1); font-weight: 600; }
.ai-sub { color: var(--yx-text-3); font-weight: 400; font-size: var(--yx-fs-xs); margin-left: 4px; }
.ai-info { margin-top: 8px; color: var(--yx-text-3); font-size: var(--yx-fs-xs); }
.ai-error { margin-top: 8px; color: var(--yx-brand); font-size: var(--yx-fs-sm); }
@media (max-width: 767px) {
  :deep(.el-textarea__inner), :deep(.el-input__inner) { font-size: 16px; }
}
</style>
