<template>
  <div class="ai-admin-page">
    <page-header title="AI 助手管理" subtitle="设置模型接口与计费单价，给用户记账充值，维护提示词场景。仅系统管理员可见" />

    <el-tabs v-model="tab" @tab-change="onTab">
      <!-- ============ 模型与计费 ============ -->
      <el-tab-pane label="模型与计费" name="settings">
        <el-card shadow="never" v-loading="loading">
          <el-form :model="form" :label-width="isMobile ? 'auto' : '150px'" :label-position="isMobile ? 'top' : 'right'" class="set-form">
            <el-form-item label="启用 AI 助手">
              <el-tooltip content="关闭后所有人都不能使用，业务页上的 AI 按钮也会隐藏" placement="top">
                <el-switch v-model="form.enabled" />
              </el-tooltip>
            </el-form-item>
            <el-form-item label="模型类型">
              <el-radio-group v-model="form.provider">
                <el-radio-button value="remote">网络接口（DeepSeek 等）</el-radio-button>
                <el-radio-button value="local">本地模型（Ollama / vLLM 等）</el-radio-button>
              </el-radio-group>
              <div class="tip" v-if="form.provider === 'remote'">网络接口需要服务器能访问外网；用户输入的内容会发送给服务商，界面上已向用户明确提示。</div>
              <div class="tip" v-else>本地模型的内容不出本机/内网，通常不收费（单价填 0 即不扣费）。</div>
            </el-form-item>
            <el-form-item label="快速填入">
              <el-button size="small" @click="preset('flash')">DeepSeek Flash</el-button>
              <el-button size="small" @click="preset('pro')">DeepSeek Pro</el-button>
              <el-button size="small" @click="preset('ollama')">本地 Ollama</el-button>
              <span class="tip inline">官方价格 2026-10-23 核对，服务商调价后请在下方手动修改</span>
            </el-form-item>
            <el-form-item label="接口地址">
              <el-input v-model="form.base_url" placeholder="如 https://api.deepseek.com 或 http://127.0.0.1:11434/v1" />
              <div class="tip">填到 /v1 之前即可，系统自动补上 /chat/completions</div>
            </el-form-item>
            <el-form-item label="模型名称">
              <el-input v-model="form.model" placeholder="如 deepseek-flash、qwen2.5:7b" />
            </el-form-item>
            <el-form-item label="API 密钥">
              <el-input v-model="form.api_key" type="password" show-password autocomplete="new-password"
                :placeholder="hasKey ? '已保存：' + keyMasked + '（不修改请留空）' : '本地模型可不填'" />
              <el-checkbox v-if="hasKey" v-model="form.clear_key" style="margin-top:6px">清除已保存的密钥</el-checkbox>
              <div class="tip">密钥加密保存在数据库中，保存后界面只显示末四位</div>
            </el-form-item>

            <el-divider content-position="left">计费单价（元 / 百万 token）</el-divider>
            <el-form-item label="输入（缓存命中）"><el-input-number v-model="form.price_in_hit" :min="0" :max="10000" :precision="4" :step="0.01" controls-position="right" /></el-form-item>
            <el-form-item label="输入（缓存未命中）"><el-input-number v-model="form.price_in_miss" :min="0" :max="10000" :precision="4" :step="0.1" controls-position="right" /></el-form-item>
            <el-form-item label="输出"><el-input-number v-model="form.price_out" :min="0" :max="10000" :precision="4" :step="0.1" controls-position="right" /></el-form-item>
            <el-form-item label="空闲时段折扣系数">
              <el-input-number v-model="form.offpeak_ratio" :min="0.01" :max="1" :precision="2" :step="0.1" controls-position="right" />
              <div class="tip">上面三项填“高峰价”。DeepSeek 的空闲时段（北京时间周一至周五 9–12 点、14–18 点以外，含周末）价格为高峰价的一半，填 0.5；不打折填 1。法定节假日工作日按高峰价计，对用户偏保守。三项单价都为 0 时不扣费。</div>
            </el-form-item>

            <el-divider content-position="left">用量控制</el-divider>
            <el-form-item label="单次最大输出"><el-input-number v-model="form.max_output_tokens" :min="64" :max="32768" :step="256" controls-position="right" /> <span class="unit">token</span></el-form-item>
            <el-form-item label="单次最大输入"><el-input-number v-model="form.max_input_chars" :min="500" :max="100000" :step="1000" controls-position="right" /> <span class="unit">字</span></el-form-item>
            <el-form-item label="低额度提醒线"><el-input-number v-model="form.warn_balance" :min="0" :max="10000" :precision="2" controls-position="right" /> <span class="unit">元，低于此值用户界面显示提醒</span></el-form-item>
            <div class="tip tip-block">每次调用前按“最坏情况”预留费用，余额不足即停止服务并提示用户联系管理员，不会出现明显超支。</div>

            <el-form-item>
              <el-button type="primary" :loading="saving" @click="saveSettings">保存设置</el-button>
              <el-tooltip content="用已保存的设置发一条极短的测试对话，不计入任何人的费用" placement="top">
                <el-button :loading="testing" @click="testConn">测试连接</el-button>
              </el-tooltip>
              <span v-if="testResult" class="test-result" :class="{ ok: testResult.ok }">{{ testResult.text }}</span>
            </el-form-item>
          </el-form>
        </el-card>
      </el-tab-pane>

      <!-- ============ 用户额度 ============ -->
      <el-tab-pane label="用户额度" name="accounts">
        <StatStrip :items="accountStats" />
        <el-card shadow="never">
          <div class="toolbar">
            <el-input v-model="acc.keyword" placeholder="按姓名/账号搜索" clearable style="width:220px" @keyup.enter="loadAccounts(1)" @clear="loadAccounts(1)" />
            <el-button :icon="'Search'" @click="loadAccounts(1)">查询</el-button>
          </div>
          <el-table :data="acc.list" v-loading="acc.loading" stripe>
            <el-table-column prop="real_name" label="姓名" width="110" />
            <el-table-column prop="department" label="科室" min-width="120" show-overflow-tooltip />
            <el-table-column label="余额（元）" width="120" align="right">
              <template #default="{ row }"><b :class="{ neg: row.balance <= 0 }">{{ fmt(row.balance, 2) }}</b></template>
            </el-table-column>
            <el-table-column label="本月花费" width="110" align="right"><template #default="{ row }">{{ fmt(row.month_spent) }}</template></el-table-column>
            <el-table-column label="累计充值" width="110" align="right"><template #default="{ row }">{{ fmt(row.total_topup, 2) }}</template></el-table-column>
            <el-table-column label="累计花费" width="110" align="right"><template #default="{ row }">{{ fmt(row.total_spent) }}</template></el-table-column>
            <el-table-column prop="total_tokens" label="累计 token" width="110" align="right" />
            <el-table-column label="操作" width="170" fixed="right">
              <template #default="{ row }">
                <el-tooltip content="用户线下微信转账后，在这里给他加款" placement="top">
                  <el-button link type="primary" @click="openTopup(row)">充值记账</el-button>
                </el-tooltip>
                <el-button link type="primary" @click="viewLedger(row)">流水</el-button>
              </template>
            </el-table-column>
          </el-table>
          <el-pagination class="pager" background layout="total, prev, pager, next" :total="acc.total" :page-size="acc.page_size" :current-page="acc.page" @current-change="loadAccounts" />
        </el-card>
      </el-tab-pane>

      <!-- ============ 加款流水 ============ -->
      <el-tab-pane label="加款流水" name="ledger">
        <el-card shadow="never">
          <div class="toolbar" v-if="led.user_id">
            <el-tag closable @close="led.user_id = 0; loadLedger(1)">仅看：{{ led.user_name }}</el-tag>
          </div>
          <el-table :data="led.list" v-loading="led.loading" stripe>
            <el-table-column prop="time" label="时间" width="160" />
            <el-table-column prop="real_name" label="用户" width="110" />
            <el-table-column label="类型" width="100">
              <template #default="{ row }"><el-tag :type="row.kind === 'topup' ? 'success' : 'warning'" effect="plain" size="small">{{ row.kind === 'topup' ? '充值' : '扣减更正' }}</el-tag></template>
            </el-table-column>
            <el-table-column label="金额（元）" width="110" align="right"><template #default="{ row }">{{ row.amount > 0 ? '+' : '' }}{{ fmt(row.amount, 2) }}</template></el-table-column>
            <el-table-column label="记账后余额" width="120" align="right"><template #default="{ row }">{{ fmt(row.balance_after, 2) }}</template></el-table-column>
            <el-table-column prop="operator" label="经办人" width="100" />
            <el-table-column prop="remark" label="备注" min-width="160" show-overflow-tooltip />
          </el-table>
          <el-pagination class="pager" background layout="total, prev, pager, next" :total="led.total" :page-size="led.page_size" :current-page="led.page" @current-change="loadLedger" />
        </el-card>
      </el-tab-pane>

      <!-- ============ 用量明细 ============ -->
      <el-tab-pane label="用量明细" name="usage">
        <el-card shadow="never">
          <div class="toolbar">
            <el-date-picker v-model="use.month" type="month" value-format="YYYY-MM" placeholder="按月筛选" clearable style="width:150px" @change="loadUsage(1)" />
            <span class="sum" v-if="use.summary">共 {{ use.summary.calls }} 次，{{ use.summary.tokens }} token，花费 ¥{{ fmt(use.summary.cost) }}</span>
          </div>
          <el-table :data="use.list" v-loading="use.loading" stripe>
            <el-table-column prop="time" label="时间" width="160" />
            <el-table-column prop="real_name" label="用户" width="100" />
            <el-table-column prop="scene" label="场景" min-width="120" show-overflow-tooltip />
            <el-table-column prop="model" label="模型" width="140" show-overflow-tooltip />
            <el-table-column label="输入" width="90" align="right"><template #default="{ row }">{{ row.prompt_tokens }}</template></el-table-column>
            <el-table-column label="输出" width="90" align="right"><template #default="{ row }">{{ row.completion_tokens }}</template></el-table-column>
            <el-table-column label="扣费（元）" width="110" align="right"><template #default="{ row }">{{ fmt(row.cost) }}</template></el-table-column>
            <el-table-column label="备注" width="130">
              <template #default="{ row }">
                <el-tag v-if="row.estimated" size="small" type="info">估算用量</el-tag>
                <el-tag v-if="row.status === 'partial'" size="small" type="warning">中途中断</el-tag>
              </template>
            </el-table-column>
          </el-table>
          <el-pagination class="pager" background layout="total, prev, pager, next" :total="use.total" :page-size="use.page_size" :current-page="use.page" @current-change="loadUsage" />
        </el-card>
      </el-tab-pane>

      <!-- ============ 提示词库 ============ -->
      <el-tab-pane label="提示词库" name="prompts">
        <el-card shadow="never">
          <div class="toolbar">
            <el-button type="primary" :icon="'Plus'" @click="openPrompt()">新增场景</el-button>
            <span class="tip inline">“业务嵌入”类场景给各业务页的 AI 按钮使用，不出现在对话页；系统预置场景可修改、可停用，不可删除</span>
          </div>
          <el-table :data="prompts" v-loading="pLoading" stripe>
            <el-table-column prop="name" label="场景名称" width="170" />
            <el-table-column prop="category" label="分类" width="100" />
            <el-table-column prop="hint" label="说明" min-width="200" show-overflow-tooltip />
            <el-table-column label="状态" width="80"><template #default="{ row }"><el-tag :type="row.status ? 'success' : 'info'" size="small" effect="plain">{{ row.status ? '启用' : '停用' }}</el-tag></template></el-table-column>
            <el-table-column label="操作" width="140" fixed="right">
              <template #default="{ row }">
                <el-button link type="primary" @click="openPrompt(row)">编辑</el-button>
                <el-button v-if="!row.builtin" link type="danger" @click="removePrompt(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="topupVisible" title="充值记账" :width="isMobile ? '92%' : '460px'" append-to-body>
      <el-form :model="topup" :label-width="isMobile ? 'auto' : '100px'" :label-position="isMobile ? 'top' : 'right'">
        <el-form-item label="用户">{{ topup.name }}（当前余额 ¥{{ fmt(topup.balance, 2) }}）</el-form-item>
        <el-form-item label="金额（元）">
          <el-input-number v-model="topup.amount" :precision="2" :step="10" :min="-100000" :max="100000" controls-position="right" style="width:200px" />
          <div class="tip">收到微信转账后填正数；填错需更正时填负数（扣减后余额不能为负）</div>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="topup.remark" maxlength="100" show-word-limit placeholder="如：10月23日微信转账 50 元" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="topupVisible = false">取消</el-button>
        <el-button type="primary" :loading="topupSaving" @click="submitTopup">确认记账</el-button>
      </template>
    </el-dialog>

    <el-drawer v-model="promptVisible" :title="pForm.id ? '编辑场景' : '新增场景'" :size="isMobile ? '100%' : '620px'" append-to-body>
      <el-form :model="pForm" :label-width="isMobile ? 'auto' : '90px'" :label-position="isMobile ? 'top' : 'right'">
        <el-form-item label="场景名称" required><el-input v-model="pForm.name" maxlength="30" /></el-form-item>
        <el-form-item label="分类"><el-input v-model="pForm.category" maxlength="20" placeholder="如：公文写作" /></el-form-item>
        <el-form-item label="使用说明"><el-input v-model="pForm.hint" maxlength="100" placeholder="显示在场景旁的悬浮提示" /></el-form-item>
        <el-form-item label="输入示例"><el-input v-model="pForm.placeholder" type="textarea" :rows="2" maxlength="200" /></el-form-item>
        <el-form-item label="系统提示词" required>
          <el-input v-model="pForm.system_prompt" type="textarea" :rows="10" maxlength="3000" show-word-limit />
          <div class="tip">写清“你是谁、要做什么、输出要求”。建议保留“不得虚构文号、数据、人名”的约束。</div>
        </el-form-item>
        <el-form-item label="排序"><el-input-number v-model="pForm.sort" :min="0" :max="9999" controls-position="right" /></el-form-item>
        <el-form-item label="状态"><el-switch v-model="pForm.status" :active-value="1" :inactive-value="0" active-text="启用" inactive-text="停用" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="promptVisible = false">取消</el-button>
        <el-button type="primary" :loading="pSaving" @click="savePrompt">保存</el-button>
      </template>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '../utils/request'
import PageHeader from '../components/PageHeader.vue'
import StatStrip from '../components/StatStrip.vue'
import { useMobile } from '../utils/useMobile'
import { formatYuan, loadAiState } from '../utils/ai'

const { isMobile } = useMobile()
const tab = ref('settings')
const fmt = (v, d = 4) => formatYuan(v, d)

// ---------- 模型与计费 ----------
const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const testResult = ref(null)
const hasKey = ref(false)
const keyMasked = ref('')
const form = reactive({
  enabled: false, provider: 'remote', base_url: '', model: '', api_key: '', clear_key: false,
  price_in_hit: 0, price_in_miss: 0, price_out: 0, offpeak_ratio: 1,
  max_output_tokens: 2048, max_input_chars: 12000, warn_balance: 2
})

const PRESETS = {
  flash: { provider: 'remote', base_url: 'https://api.deepseek.com', model: 'deepseek-flash', price_in_hit: 0.04, price_in_miss: 2, price_out: 8, offpeak_ratio: 0.5 },
  pro: { provider: 'remote', base_url: 'https://api.deepseek.com', model: 'deepseek-v4-pro', price_in_hit: 0.3, price_in_miss: 9, price_out: 27, offpeak_ratio: 0.5 },
  ollama: { provider: 'local', base_url: 'http://127.0.0.1:11434/v1', model: 'qwen2.5:7b', price_in_hit: 0, price_in_miss: 0, price_out: 0, offpeak_ratio: 1 }
}
const preset = (k) => {
  Object.assign(form, PRESETS[k])
  ElMessage.info('已填入，请确认后点“保存设置”')
}

const loadSettings = async () => {
  loading.value = true
  try {
    const s = await request.get('/ai/admin/settings')
    Object.assign(form, {
      enabled: s.enabled, provider: s.provider, base_url: s.base_url, model: s.model, api_key: '', clear_key: false,
      price_in_hit: s.price_in_hit, price_in_miss: s.price_in_miss, price_out: s.price_out, offpeak_ratio: s.offpeak_ratio || 1,
      max_output_tokens: s.max_output_tokens, max_input_chars: s.max_input_chars, warn_balance: s.warn_balance
    })
    hasKey.value = s.has_key
    keyMasked.value = s.key_masked
  } catch (e) { console.error(e) } finally { loading.value = false }
}

const saveSettings = async () => {
  if (saving.value) return
  saving.value = true
  try {
    await request.put('/ai/admin/settings', { ...form })
    ElMessage.success('已保存')
    testResult.value = null
    await loadSettings()
    loadAiState(true)
  } catch (e) { console.error(e) } finally { saving.value = false }
}

const testConn = async () => {
  testing.value = true
  testResult.value = null
  try {
    const r = await request.post('/ai/admin/test', {})
    testResult.value = r.ok
      ? { ok: true, text: `连接成功，耗时 ${r.ms} 毫秒，模型回复：${r.reply || '（空）'}` }
      : { ok: false, text: r.message }
  } catch (e) { console.error(e) } finally { testing.value = false }
}

// ---------- 用户额度 ----------
const acc = reactive({ list: [], total: 0, page: 1, page_size: 20, loading: false, keyword: '', summary: null })
const accountStats = computed(() => [
  { label: '全员剩余额度（元）', value: fmt(acc.summary?.balance || 0, 2) },
  { label: '累计充值（元）', value: fmt(acc.summary?.total_topup || 0, 2) },
  { label: '累计花费（元）', value: fmt(acc.summary?.total_spent || 0, 2) }
])
const loadAccounts = async (page = 1) => {
  acc.loading = true
  try {
    const r = await request.get('/ai/admin/accounts', { params: { page, keyword: acc.keyword || undefined } })
    Object.assign(acc, { list: r.list, total: r.total, page: r.page, page_size: r.page_size, summary: r.summary })
  } catch (e) { console.error(e) } finally { acc.loading = false }
}

const topupVisible = ref(false)
const topupSaving = ref(false)
const topup = reactive({ user_id: 0, name: '', balance: 0, amount: 50, remark: '' })
const openTopup = (row) => {
  Object.assign(topup, { user_id: row.user_id, name: row.real_name, balance: row.balance, amount: 50, remark: '' })
  topupVisible.value = true
}
const submitTopup = async () => {
  if (!topup.amount) return ElMessage.warning('请填写金额')
  if (topupSaving.value) return
  topupSaving.value = true
  try {
    const r = await request.post('/ai/admin/topup', { user_id: topup.user_id, amount: topup.amount, remark: topup.remark })
    ElMessage.success(`已记账，${topup.name} 当前余额 ¥${fmt(r.balance, 2)}`)
    topupVisible.value = false
    loadAccounts(acc.page)
  } catch (e) { console.error(e) } finally { topupSaving.value = false }
}

// ---------- 流水 ----------
const led = reactive({ list: [], total: 0, page: 1, page_size: 20, loading: false, user_id: 0, user_name: '' })
const loadLedger = async (page = 1) => {
  led.loading = true
  try {
    const r = await request.get('/ai/admin/ledger', { params: { page, user_id: led.user_id || undefined } })
    Object.assign(led, { list: r.list, total: r.total, page: r.page, page_size: r.page_size })
  } catch (e) { console.error(e) } finally { led.loading = false }
}
const viewLedger = (row) => {
  led.user_id = row.user_id
  led.user_name = row.real_name
  tab.value = 'ledger'
  loadLedger(1)
}

// ---------- 用量 ----------
const use = reactive({ list: [], total: 0, page: 1, page_size: 20, loading: false, month: '', summary: null })
const loadUsage = async (page = 1) => {
  use.loading = true
  try {
    const r = await request.get('/ai/admin/usage', { params: { page, month: use.month || undefined } })
    Object.assign(use, { list: r.list, total: r.total, page: r.page, page_size: r.page_size, summary: r.summary })
  } catch (e) { console.error(e) } finally { use.loading = false }
}

// ---------- 提示词 ----------
const prompts = ref([])
const pLoading = ref(false)
const promptVisible = ref(false)
const pSaving = ref(false)
const emptyPrompt = () => ({ id: 0, name: '', category: '自定义', hint: '', placeholder: '', system_prompt: '', sort: 500, status: 1 })
const pForm = reactive(emptyPrompt())
const loadPrompts = async () => {
  pLoading.value = true
  try { prompts.value = await request.get('/ai/admin/prompts') } catch (e) { console.error(e) } finally { pLoading.value = false }
}
const openPrompt = (row) => {
  Object.assign(pForm, row ? { ...row } : emptyPrompt())
  promptVisible.value = true
}
const savePrompt = async () => {
  if (!pForm.name.trim() || !pForm.system_prompt.trim()) return ElMessage.warning('请填写场景名称和系统提示词')
  if (pSaving.value) return
  pSaving.value = true
  try {
    if (pForm.id) await request.put('/ai/admin/prompts', { ...pForm })
    else await request.post('/ai/admin/prompts', { ...pForm })
    ElMessage.success('已保存')
    promptVisible.value = false
    loadPrompts()
    loadAiState(true)
  } catch (e) { console.error(e) } finally { pSaving.value = false }
}
const removePrompt = async (row) => {
  try {
    await ElMessageBox.confirm(`确认删除场景「${row.name}」？`, '删除确认', { type: 'warning', confirmButtonText: '删除' })
  } catch (e) { return }
  try {
    await request.delete(`/ai/admin/prompts/${row.id}`)
    ElMessage.success('已删除')
    loadPrompts()
    loadAiState(true)
  } catch (e) { console.error(e) }
}

const onTab = (name) => {
  if (name === 'accounts') loadAccounts(1)
  else if (name === 'ledger') loadLedger(1)
  else if (name === 'usage') loadUsage(1)
  else if (name === 'prompts') loadPrompts()
}

onMounted(loadSettings)
</script>

<style scoped>
.set-form { max-width: 820px; }
.tip { color: var(--yx-text-3); font-size: var(--yx-fs-xs); line-height: 1.7; margin-top: 4px; width: 100%; }
.tip.inline { display: inline; margin-left: 10px; width: auto; }
.tip-block { margin-left: 150px; margin-bottom: 12px; }
.unit { color: var(--yx-text-3); margin-left: 8px; font-size: var(--yx-fs-sm); }
.test-result { margin-left: 12px; color: var(--yx-brand); font-size: var(--yx-fs-sm); }
.test-result.ok { color: var(--yx-ink-2); }
.toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; margin-bottom: 14px; }
.sum { color: var(--yx-text-2); font-size: var(--yx-fs-sm); }
.pager { margin-top: 14px; justify-content: flex-end; }
.neg { color: var(--yx-brand); }
@media (max-width: 767px) {
  .tip-block { margin-left: 0; }
  .pager { justify-content: center; }
  :deep(.el-input__inner), :deep(.el-textarea__inner) { font-size: 16px; }
}
</style>
