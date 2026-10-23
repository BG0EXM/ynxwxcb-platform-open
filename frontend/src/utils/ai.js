// AI 写作助手公共逻辑：状态缓存、提示词场景、流式对话
import { reactive } from 'vue'
import request, { handleUnauthorized } from './request'
import { showSecrecyAlert } from './secrecyGuard'
import { showWafAlert } from './wafAlert'

// 状态在各页面间共享：业务页的 AI 按钮据此决定是否显示；余额在每次对话结束后随流返回，即时更新
export const aiState = reactive({
  loaded: false,
  enabled: false,
  provider: 'remote',
  providerLabel: '',
  model: '',
  free: false,
  balance: 0,
  low: false,
  warn: 2,
  maxInput: 12000,
  peakNow: false,
  priceOut: 0,
  prompts: []
})

let loading = null

export const loadAiState = (force = false) => {
  if (aiState.loaded && !force) return Promise.resolve(aiState)
  if (loading) return loading
  loading = (async () => {
    try {
      const [st, ps] = await Promise.all([request.get('/ai/status'), request.get('/ai/prompts')])
      aiState.enabled = !!st.enabled
      aiState.provider = st.provider
      aiState.providerLabel = st.provider_label
      aiState.model = st.model
      aiState.free = !!st.free
      aiState.balance = st.balance
      aiState.low = !!st.low
      aiState.warn = st.warn
      aiState.maxInput = st.max_input
      aiState.peakNow = !!st.peak_now
      aiState.priceOut = st.price_out
      aiState.prompts = ps || []
      aiState.loaded = true
    } catch (e) {
      console.error(e)
    } finally {
      loading = null
    }
    return aiState
  })()
  return loading
}

export const findScene = (key) => aiState.prompts.find(p => p.scene_key === key)

export const formatYuan = (v, digits = 4) => {
  const n = Number(v || 0)
  if (n === 0) return '0'
  if (Math.abs(n) >= 1) return n.toFixed(2)
  return n.toFixed(digits).replace(/0+$/, '').replace(/\.$/, '')
}

/**
 * 流式对话
 * @param {{sceneId:number, messages:{role:string,content:string}[], onDelta:(t:string)=>void, signal?:AbortSignal}} opts
 * @returns {Promise<{ok:boolean, done?:object, error?:string, code?:string}>}
 */
export async function streamChat({ sceneId = 0, messages, onDelta, signal }) {
  let resp
  try {
    resp = await fetch('/api/ai/chat', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${localStorage.getItem('token') || ''}`
      },
      body: JSON.stringify({ scene_id: sceneId, messages }),
      signal
    })
  } catch (e) {
    if (e.name === 'AbortError') return { ok: false, aborted: true }
    console.error(e)
    return { ok: false, error: '网络异常，请检查连接后重试' }
  }

  if (!resp.ok) {
    let data = {}
    try { data = await resp.json() } catch (e) { console.error(e) }
    if (resp.status === 401) {
      handleUnauthorized({ response: { data } })
      return { ok: false, error: data.error || '登录已过期' }
    }
    if (resp.status === 403 && data.security_violation === 'SECRECY_LEAK_PREVENTED') {
      showSecrecyAlert({
        fileName: data.filename || 'AI 写作助手输入内容',
        keyword: data.matched_rule ? `${data.matched_rule}${data.detail ? '（' + data.detail + '）' : ''}` : '国家秘密/内部级标识',
        isPublic: false
      })
      return { ok: false, error: '内容含涉密标识，已阻断', code: 'SECRECY' }
    }
    if (resp.status === 403 && data.code === 'WAF_BLOCK') {
      showWafAlert()
      return { ok: false, error: '请求已被拦截', code: 'WAF_BLOCK' }
    }
    return { ok: false, error: data.error || `请求失败（${resp.status}）`, code: data.code }
  }

  const reader = resp.body.getReader()
  const decoder = new TextDecoder('utf-8')
  let buf = ''
  let done = null
  let error = ''
  try {
    for (;;) {
      const { value, done: finished } = await reader.read()
      if (finished) break
      buf += decoder.decode(value, { stream: true })
      let idx
      while ((idx = buf.indexOf('\n\n')) >= 0) {
        const block = buf.slice(0, idx).trim()
        buf = buf.slice(idx + 2)
        if (!block.startsWith('data:')) continue
        let ev
        try { ev = JSON.parse(block.slice(5).trim()) } catch (e) { console.error(e); continue }
        if (ev.delta) onDelta(ev.delta)
        if (ev.error) error = ev.error
        if (ev.done) done = ev
      }
    }
  } catch (e) {
    if (e.name === 'AbortError') {
      // 用户主动停止：服务端会按已生成内容结算，这里刷新余额
      loadAiState(true)
      return { ok: false, aborted: true }
    }
    console.error(e)
    error = error || '连接中断，已生成的内容已保留'
  }
  if (done) {
    aiState.balance = done.balance
    aiState.low = !!done.low
  } else {
    loadAiState(true)
  }
  return { ok: !error, done, error }
}
