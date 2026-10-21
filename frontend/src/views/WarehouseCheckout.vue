<template>
  <div class="wh-wrapper">
    <!-- 顶部政务天际线（与会务、征求意见、材料下发公开页一致） -->
    <div class="skyline-bg">
      <div class="skyline-glow"></div>
      <svg class="tianshan-peaks" viewBox="0 0 1440 220" fill="none" preserveAspectRatio="none">
        <path d="M0,220 L0,110 L120,60 L240,130 L380,40 L520,120 L660,20 L800,105 L960,35 L1120,115 L1280,55 L1440,95 L1440,220 Z"
              fill="url(#wh-peak-1)" opacity="0.14" />
        <path d="M0,220 L0,140 L160,85 L320,155 L460,70 L620,145 L760,50 L900,130 L1080,65 L1240,140 L1440,110 L1440,220 Z"
              fill="url(#wh-peak-2)" opacity="0.22" />
        <defs>
          <linearGradient id="wh-peak-1" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stop-color="#f5d499" />
            <stop offset="100%" stop-color="#9e1b1e" stop-opacity="0" />
          </linearGradient>
          <linearGradient id="wh-peak-2" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stop-color="#ffffff" />
            <stop offset="100%" stop-color="#24406e" stop-opacity="0" />
          </linearGradient>
        </defs>
      </svg>
      <div class="org-brand-bar">
        <img src="../assets/danghui.png" alt="党徽" class="brand-crest" />
        <div class="brand-titles">
          <div class="brand-sup font-serif">中共伊宁县委宣传部</div>
          <div class="brand-sub">库房物资扫码登记</div>
        </div>
      </div>
    </div>

    <div class="main-container" v-loading="loading">
      <!-- 链接失效 -->
      <div v-if="loadError" class="wh-card">
        <el-result icon="warning" title="二维码已失效" sub-title="请联系县委宣传部办公室获取最新二维码。" />
      </div>

      <!-- 暂停扫码 -->
      <div v-else-if="paused" class="wh-card">
        <el-result icon="info" title="扫码领用暂停中" :sub-title="notice || '如需领用物资，请联系办公室。'" />
      </div>

      <!-- 领用成功凭证 -->
      <div v-else-if="slip" class="wh-card">
        <div class="done-head">
          <el-icon class="done-icon"><CircleCheckFilled /></el-icon>
          <div class="done-title font-serif">登记成功</div>
          <div class="done-sub">请按登记数量取用，可截图留存</div>
        </div>
        <div class="slip-box">
          <div class="slip-row"><span>领用单号</span><b class="tabular-nums">{{ slip.slip_no }}</b></div>
          <div class="slip-row"><span>领用人</span><b>{{ slip.person_name }}（{{ slip.department }}）</b></div>
          <div class="slip-row"><span>登记时间</span><b>{{ formatTime(slip.created_at) }}</b></div>
          <div class="slip-row" v-if="slip.purpose"><span>用途</span><b>{{ slip.purpose }}</b></div>
          <div class="slip-lines">
            <div class="slip-line" v-for="(l, i) in slip.lines" :key="i">
              <span class="line-name">{{ l.name }}<small v-if="l.spec">（{{ l.spec }}）</small></span>
              <el-tag v-if="l.kind === 'borrow'" size="small" type="warning" effect="plain">借用·用后归还</el-tag>
              <span class="line-qty tabular-nums">{{ l.qty }}{{ l.unit }}</span>
            </div>
          </div>
        </div>
        <el-button type="primary" size="large" class="full-btn" @click="restart">继续登记</el-button>
      </div>

      <template v-else-if="loaded">
        <div class="wh-notice" v-if="notice">
          <el-icon><InfoFilled /></el-icon><span>{{ notice }}</span>
        </div>

        <!-- 第一步：领用人 -->
        <div class="wh-card">
          <div class="card-title"><span class="step-no">1</span>领用人</div>
          <div v-if="person" class="person-chosen">
            <div>
              <div class="person-name">{{ person.name }}</div>
              <div class="person-dept">{{ person.department }}<span v-if="person.external">（名单外人员）</span></div>
            </div>
            <el-button link type="primary" @click="changePerson">更换</el-button>
          </div>
          <template v-else>
            <div class="field-label">所在科室</div>
            <el-select v-model="pickDept" placeholder="请选择科室" size="large" class="full" @change="pickUser = null">
              <el-option v-for="d in departments" :key="d.department" :label="d.department" :value="d.department" />
              <el-option label="其他人员（名单里找不到自己）" :value="EXTERNAL" />
            </el-select>

            <template v-if="pickDept && pickDept !== EXTERNAL">
              <div class="field-label">姓名</div>
              <div class="people-grid">
                <button v-for="p in deptPeople" :key="p.id" type="button" class="person-chip"
                        :class="{ active: pickUser === p.id }" @click="pickUser = p.id">{{ p.name }}</button>
              </div>
            </template>

            <el-form v-if="pickDept === EXTERNAL" ref="extFormRef" :model="ext" :rules="extRules" label-position="top" class="ext-form">
              <el-form-item label="姓名" prop="name"><el-input v-model="ext.name" maxlength="20" size="large" /></el-form-item>
              <el-form-item label="单位" prop="unit"><el-input v-model="ext.unit" maxlength="50" size="large" placeholder="如：县融媒体中心" /></el-form-item>
              <el-form-item label="联系电话" prop="phone"><el-input v-model="ext.phone" maxlength="20" size="large" inputmode="tel" /></el-form-item>
            </el-form>

            <el-button type="primary" size="large" class="full-btn" :disabled="!canConfirmPerson" @click="confirmPerson">确定</el-button>
          </template>
        </div>

        <!-- 第二步：领用 / 归还 -->
        <div class="wh-card" v-if="person">
          <div class="seg">
            <button type="button" :class="{ active: tab === 'take' }" @click="tab = 'take'">领用 / 借用</button>
            <button type="button" :class="{ active: tab === 'return' }" @click="switchToReturn">
              归还<span v-if="borrows.length" class="seg-badge">{{ borrows.length }}</span>
            </button>
          </div>

          <template v-if="tab === 'take'">
            <el-input v-model="keyword" placeholder="搜索物资名称" clearable size="large" class="search">
              <template #prefix><el-icon><Search /></el-icon></template>
            </el-input>
            <div class="cat-row" v-if="categories.length > 1">
              <button type="button" class="cat-chip" :class="{ active: !category }" @click="category = ''">全部</button>
              <button type="button" v-for="c in categories" :key="c" class="cat-chip" :class="{ active: category === c }" @click="category = c">{{ c }}</button>
            </div>

            <EmptyState v-if="!filteredItems.length" description="没有找到物资，换个关键词试试，或联系办公室" />
            <div v-else class="item-list">
              <div v-for="it in filteredItems" :key="it.id" class="item-row" :class="{ disabled: it.stock <= 0 }">
                <div class="item-info">
                  <div class="item-name">
                    {{ it.name }}
                    <el-tag v-if="it.kind === 'returnable'" size="small" type="warning" effect="plain">需归还</el-tag>
                  </div>
                  <div class="item-meta">
                    <span v-if="it.spec">{{ it.spec }} · </span>
                    <span v-if="it.stock > 0">库存 {{ it.stock }}{{ it.unit }}</span>
                    <span v-else class="out">已领完，请联系办公室</span>
                  </div>
                </div>
                <div class="stepper" v-if="it.stock > 0">
                  <button type="button" class="step-btn" :disabled="!cart[it.id]" @click="dec(it)" aria-label="减少">
                    <el-icon><Minus /></el-icon>
                  </button>
                  <span class="step-val tabular-nums">{{ cart[it.id] || 0 }}</span>
                  <button type="button" class="step-btn plus" :disabled="(cart[it.id] || 0) >= it.stock" @click="inc(it)" aria-label="增加">
                    <el-icon><Plus /></el-icon>
                  </button>
                </div>
              </div>
            </div>

            <div class="field-label">用途（选填）</div>
            <el-input v-model="purpose" maxlength="100" size="large" placeholder="如：迎检布展、会议使用" />
          </template>

          <template v-else>
            <div v-loading="borrowLoading">
              <EmptyState v-if="!borrows.length" description="没有待归还的物品" />
              <div v-else class="item-list">
                <div v-for="b in borrows" :key="b.id" class="item-row borrow-row" @click="toggleReturn(b.id)">
                  <el-checkbox :model-value="returnPick.includes(b.id)" size="large" class="no-pointer" />
                  <div class="item-info">
                    <div class="item-name">{{ b.item_name }}<small v-if="b.spec">（{{ b.spec }}）</small></div>
                    <div class="item-meta">未还 {{ b.outstanding }}{{ b.unit }} · 借出 {{ b.days || 0 }} 天</div>
                  </div>
                </div>
              </div>
            </div>
          </template>
        </div>

        <!-- 底部提交栏 -->
        <div class="submit-bar" v-if="person">
          <template v-if="tab === 'take'">
            <div class="bar-sum">已选 <b>{{ cartLines.length }}</b> 样，共 <b>{{ cartTotal }}</b> 件</div>
            <el-button type="primary" size="large" :disabled="!cartLines.length" :loading="submitting" @click="submitTake">提交登记</el-button>
          </template>
          <template v-else>
            <div class="bar-sum">已选 <b>{{ returnPick.length }}</b> 件物品</div>
            <el-button type="primary" size="large" :disabled="!returnPick.length" :loading="submitting" @click="submitReturn">确认归还</el-button>
          </template>
        </div>
      </template>

      <footer class="gov-footer">
        <div class="footer-crest-row">
          <span class="crest-dot"></span>
          <span class="footer-org font-serif">中共伊宁县委宣传部</span>
          <span class="crest-dot"></span>
        </div>
        <div class="footer-system">部务工作平台 · 库房物资扫码登记</div>
        <div class="footer-beian">
          <a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener">[滇ICP备XXXXXXXX号-X]</a>
          <span class="sep">|</span>
          <a href="https://beian.mps.gov.cn/#/query/webSearch" target="_blank" rel="noopener">[滇公网安备XXXXXXXXXXXXXXXX号]</a>
        </div>
      </footer>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import { CircleCheckFilled, InfoFilled, Search, Minus, Plus } from '@element-plus/icons-vue'
import dayjs from 'dayjs'
import request from '../utils/request'
import EmptyState from '../components/EmptyState.vue'

const EXTERNAL = '__external__'
const STORE_KEY = 'wh_person'

const route = useRoute()
const token = route.params.token
const base = `/public/warehouse/${token}`

const loading = ref(true)
const loaded = ref(false)
const loadError = ref(false)
const paused = ref(false)
const notice = ref('')
const departments = ref([])
const items = ref([])

const person = ref(null)
const pickDept = ref('')
const pickUser = ref(null)
const extFormRef = ref(null)
const ext = reactive({ name: '', unit: '', phone: '' })
const extRules = {
  name: [{ required: true, message: '请填写姓名', trigger: 'blur' }],
  unit: [{ required: true, message: '请填写单位', trigger: 'blur' }],
  phone: [
    { required: true, message: '请填写联系电话', trigger: 'blur' },
    { pattern: /^(1\d{10}|0\d{2,3}-?\d{7,8}(-\d{1,6})?)$/, message: '电话格式不正确', trigger: 'blur' }
  ]
}

const tab = ref('take')
const keyword = ref('')
const category = ref('')
const cart = reactive({})
const purpose = ref('')
const submitting = ref(false)
const slip = ref(null)
let submitId = ''

const borrows = ref([])
const borrowLoading = ref(false)
const returnPick = ref([])

const deptPeople = computed(() => departments.value.find(d => d.department === pickDept.value)?.people || [])
const canConfirmPerson = computed(() => {
  if (pickDept.value === EXTERNAL) return !!(ext.name && ext.unit && ext.phone)
  return !!pickUser.value
})
const categories = computed(() => [...new Set(items.value.map(i => i.category).filter(Boolean))])
const filteredItems = computed(() => {
  const kw = keyword.value.trim()
  return items.value.filter(i =>
    (!category.value || i.category === category.value) &&
    (!kw || i.name.includes(kw) || (i.spec || '').includes(kw)))
})
const cartLines = computed(() => items.value.filter(i => cart[i.id] > 0).map(i => ({ item: i, qty: cart[i.id] })))
const cartTotal = computed(() => cartLines.value.reduce((s, l) => s + l.qty, 0))

const formatTime = (v) => (v ? dayjs(v).format('YYYY年MM月DD日 HH:mm') : '')

const newSubmitId = () => {
  const r = () => Math.random().toString(36).slice(2, 10)
  return `${Date.now().toString(36)}-${r()}${r()}`
}

// 领用人身份参数（名单内只传 ID，姓名科室由服务端核定）
const personPayload = () => (person.value.external
  ? { external: true, name: person.value.name, unit: person.value.department, phone: person.value.phone }
  : { user_id: person.value.user_id })

const savePerson = () => {
  try { localStorage.setItem(STORE_KEY, JSON.stringify(person.value)) } catch (e) { console.error(e) }
}

// 恢复上次选择的领用人（名单内人员须仍在册）
const restorePerson = () => {
  try {
    const saved = JSON.parse(localStorage.getItem(STORE_KEY) || 'null')
    if (!saved) return
    if (saved.external) {
      if (saved.name && saved.department && saved.phone) person.value = saved
      return
    }
    for (const d of departments.value) {
      const p = d.people.find(x => x.id === saved.user_id)
      if (p) {
        person.value = { user_id: p.id, name: p.name, department: d.department }
        return
      }
    }
  } catch (e) {
    console.error(e)
  }
}

const loadData = async () => {
  loading.value = true
  try {
    const res = await request.get(base)
    paused.value = !!res.paused
    notice.value = res.notice || ''
    departments.value = res.departments || []
    items.value = res.items || []
    loaded.value = true
    if (!person.value) restorePerson()
    if (person.value) loadBorrows()
  } catch (e) {
    console.error(e)
    if (e.response?.status === 404) loadError.value = true
  } finally {
    loading.value = false
  }
}

const confirmPerson = async () => {
  if (pickDept.value === EXTERNAL) {
    try {
      await extFormRef.value.validate()
    } catch (e) {
      return
    }
    person.value = { external: true, name: ext.name.trim(), department: ext.unit.trim(), phone: ext.phone.trim() }
  } else {
    const p = deptPeople.value.find(x => x.id === pickUser.value)
    if (!p) return
    person.value = { user_id: p.id, name: p.name, department: pickDept.value }
  }
  savePerson()
  loadBorrows()
}

const changePerson = () => {
  person.value = null
  pickDept.value = ''
  pickUser.value = null
  borrows.value = []
  returnPick.value = []
  tab.value = 'take'
}

const inc = (it) => {
  const cur = cart[it.id] || 0
  if (cur < it.stock) cart[it.id] = cur + 1
}
const dec = (it) => {
  const cur = cart[it.id] || 0
  if (cur > 1) cart[it.id] = cur - 1
  else delete cart[it.id]
}

const submitTake = async () => {
  const lines = cartLines.value
  if (!lines.length) return
  const fmt = (arr) => arr.map(l => `${l.item.name}×${l.qty}${l.item.unit}`).join('、')
  const take = lines.filter(l => l.item.kind !== 'returnable')
  const borrow = lines.filter(l => l.item.kind === 'returnable')
  const desc = [take.length && `领用 ${fmt(take)}`, borrow.length && `借用 ${fmt(borrow)}（用后请扫码归还）`].filter(Boolean).join('；')
  try {
    await ElMessageBox.confirm(`${person.value.name}，确认登记：${desc}？`, '请核对', {
      confirmButtonText: '确认提交', cancelButtonText: '再看看', type: 'info'
    })
  } catch (e) {
    return
  }
  if (!submitId) submitId = newSubmitId()
  submitting.value = true
  try {
    const res = await request.post(`${base}/checkout`, {
      ...personPayload(),
      purpose: purpose.value.trim(),
      submit_id: submitId,
      lines: lines.map(l => ({ item_id: l.item.id, qty: l.qty }))
    })
    slip.value = res.slip
    submitId = ''
    Object.keys(cart).forEach(k => delete cart[k])
    purpose.value = ''
    window.scrollTo({ top: 0, behavior: 'smooth' })
  } catch (e) {
    console.error(e)
    // 库存不足等业务错误：刷新库存，换一个提交号以便修改后重新提交
    if (e.response?.status && e.response.status < 500) {
      submitId = ''
      refreshItems()
    }
  } finally {
    submitting.value = false
  }
}

const refreshItems = async () => {
  try {
    const res = await request.get(base)
    items.value = res.items || []
    for (const id of Object.keys(cart)) {
      const it = items.value.find(i => String(i.id) === id)
      if (!it || it.stock <= 0) delete cart[id]
      else if (cart[id] > it.stock) cart[id] = it.stock
    }
  } catch (e) {
    console.error(e)
  }
}

const restart = async () => {
  slip.value = null
  tab.value = 'take'
  await refreshItems()
  loadBorrows()
}

const loadBorrows = async () => {
  if (!person.value) return
  borrowLoading.value = true
  try {
    const res = await request.post(`${base}/borrows`, personPayload())
    borrows.value = res.list || []
    returnPick.value = returnPick.value.filter(id => borrows.value.some(b => b.id === id))
  } catch (e) {
    console.error(e)
  } finally {
    borrowLoading.value = false
  }
}

const switchToReturn = () => {
  tab.value = 'return'
  loadBorrows()
}

const toggleReturn = (id) => {
  const i = returnPick.value.indexOf(id)
  if (i >= 0) returnPick.value.splice(i, 1)
  else returnPick.value.push(id)
}

const submitReturn = async () => {
  if (!returnPick.value.length) return
  const desc = borrows.value.filter(b => returnPick.value.includes(b.id)).map(b => `${b.item_name}×${b.outstanding}${b.unit}`).join('、')
  try {
    await ElMessageBox.confirm(`确认已将以下物品放回库房：${desc}？`, '登记归还', {
      confirmButtonText: '确认归还', cancelButtonText: '取消', type: 'info'
    })
  } catch (e) {
    return
  }
  submitting.value = true
  try {
    await request.post(`${base}/return`, { ...personPayload(), borrow_ids: returnPick.value })
    ElMessageBox.alert('归还登记成功，谢谢！', '已归还', { confirmButtonText: '好的', type: 'success' })
    returnPick.value = []
    loadBorrows()
    refreshItems()
  } catch (e) {
    console.error(e)
    loadBorrows()
  } finally {
    submitting.value = false
  }
}

onMounted(loadData)
</script>

<style scoped>
.wh-wrapper {
  min-height: 100vh;
  background: var(--yx-bg);
  display: flex;
  flex-direction: column;
  font-family: var(--yx-font-sans);
}

.skyline-bg {
  position: relative;
  background: var(--yx-skyline-gradient);
  padding: 28px 20px 96px;
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
  max-width: 640px;
  margin: 0 auto;
  display: flex;
  align-items: center;
  gap: 14px;
  z-index: 2;
}
.brand-crest {
  width: 48px;
  height: 48px;
  object-fit: contain;
  filter: drop-shadow(0 0 14px rgba(245, 212, 153, 0.65));
}
.brand-sup {
  font-size: 18px;
  font-weight: 600;
  letter-spacing: 1.2px;
  color: var(--yx-brand-bg);
}
.brand-sub {
  font-size: 13px;
  color: rgba(255, 255, 255, 0.78);
  margin-top: 4px;
  letter-spacing: 1px;
}

.main-container {
  max-width: 640px;
  width: 100%;
  margin: -64px auto 0;
  padding: 0 16px;
  position: relative;
  z-index: 3;
  flex: 1;
  min-height: 200px;
}

.wh-card {
  background: var(--yx-surface);
  border-radius: var(--yx-radius);
  box-shadow: var(--yx-shadow);
  padding: 20px 18px;
  margin-bottom: 14px;
}

.wh-notice {
  display: flex;
  gap: 8px;
  align-items: flex-start;
  background: var(--yx-surface);
  border-left: 3px solid var(--yx-gold);
  border-radius: var(--yx-radius-sm);
  padding: 10px 14px;
  margin-bottom: 14px;
  font-size: var(--yx-fs-sm);
  color: var(--yx-text-2);
  box-shadow: var(--yx-shadow);
}
.wh-notice .el-icon {
  color: var(--yx-gold-dark);
  margin-top: 3px;
}

.card-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: var(--yx-fs-md);
  font-weight: 600;
  color: var(--yx-text-1);
  margin-bottom: 12px;
}
.step-no {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: var(--yx-ink-2);
  color: #fff;
  font-size: 13px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.field-label {
  font-size: var(--yx-fs-sm);
  color: var(--yx-text-2);
  margin: 14px 0 8px;
}
.full {
  width: 100%;
}
.full-btn {
  width: 100%;
  margin-top: 18px;
}

.people-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(88px, 1fr));
  gap: 8px;
}
.person-chip {
  height: 44px;
  border: 1px solid var(--yx-border);
  border-radius: var(--yx-radius-sm);
  background: var(--yx-surface);
  font-size: 16px;
  color: var(--yx-text-1);
  cursor: pointer;
}
.person-chip.active {
  border-color: var(--yx-ink-2);
  background: var(--yx-ink-2);
  color: #fff;
  font-weight: 600;
}

.ext-form {
  margin-top: 12px;
}

.person-chosen {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: var(--yx-surface-muted);
  border: 1px solid var(--yx-border-soft);
  border-radius: var(--yx-radius-sm);
  padding: 12px 14px;
}
.person-name {
  font-size: 18px;
  font-weight: 600;
  color: var(--yx-text-1);
}
.person-dept {
  font-size: var(--yx-fs-sm);
  color: var(--yx-text-3);
  margin-top: 2px;
}

.seg {
  display: flex;
  background: var(--yx-ink-2-bg);
  border-radius: var(--yx-radius-sm);
  padding: 3px;
  margin-bottom: 14px;
}
.seg button {
  flex: 1;
  height: 40px;
  border: 0;
  border-radius: 5px;
  background: transparent;
  font-size: 15px;
  color: var(--yx-text-2);
  cursor: pointer;
}
.seg button.active {
  background: var(--yx-surface);
  color: var(--yx-ink-2);
  font-weight: 600;
  box-shadow: 0 1px 3px rgba(27, 36, 51, 0.12);
}
.seg-badge {
  display: inline-block;
  min-width: 18px;
  height: 18px;
  line-height: 18px;
  margin-left: 6px;
  padding: 0 5px;
  border-radius: 9px;
  background: var(--yx-gold);
  color: #fff;
  font-size: 12px;
}

.search {
  margin-bottom: 10px;
}
.cat-row {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  padding-bottom: 4px;
  margin-bottom: 6px;
  scrollbar-width: none;
}
.cat-row::-webkit-scrollbar {
  display: none;
}
.cat-chip {
  flex: none;
  height: 32px;
  padding: 0 14px;
  border: 1px solid var(--yx-border);
  border-radius: 16px;
  background: var(--yx-surface);
  font-size: 14px;
  color: var(--yx-text-2);
  cursor: pointer;
}
.cat-chip.active {
  border-color: var(--yx-ink-2);
  color: var(--yx-ink-2);
  background: var(--yx-ink-2-bg);
}

.item-list {
  border-top: 1px solid var(--yx-border-soft);
}
.item-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 2px;
  border-bottom: 1px solid var(--yx-border-soft);
}
.item-row.disabled .item-name {
  color: var(--yx-text-4);
}
.borrow-row {
  cursor: pointer;
}
.no-pointer {
  pointer-events: none;
}
.item-info {
  flex: 1;
  min-width: 0;
}
.item-name {
  font-size: 16px;
  color: var(--yx-text-1);
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.item-name small {
  color: var(--yx-text-3);
}
.item-meta {
  font-size: 13px;
  color: var(--yx-text-3);
  margin-top: 3px;
}
.item-meta .out {
  color: var(--yx-brand);
}

.stepper {
  display: flex;
  align-items: center;
  gap: 4px;
  flex: none;
}
.step-btn {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  border: 1px solid var(--yx-border);
  background: var(--yx-surface);
  color: var(--yx-text-2);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
}
.step-btn.plus {
  border-color: var(--yx-ink-2);
  background: var(--yx-ink-2);
  color: #fff;
}
.step-btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}
.step-val {
  min-width: 30px;
  text-align: center;
  font-size: 17px;
  font-weight: 600;
  color: var(--yx-text-1);
}

/* 底部固定提交栏 */
.submit-bar {
  position: sticky;
  bottom: 0;
  z-index: 5;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  background: var(--yx-surface);
  border-radius: var(--yx-radius) var(--yx-radius) 0 0;
  box-shadow: 0 -4px 16px rgba(27, 36, 51, 0.08);
  padding: 12px 16px calc(12px + env(safe-area-inset-bottom));
  margin: 0 -16px;
}
.bar-sum {
  font-size: var(--yx-fs-sm);
  color: var(--yx-text-2);
}
.bar-sum b {
  color: var(--yx-ink-2);
  font-size: 18px;
}
.submit-bar .el-button {
  min-width: 132px;
}
/* 未选物资/未选人时按钮明显置灰，避免误以为可以提交 */
.submit-bar .el-button.is-disabled,
.full-btn.is-disabled {
  opacity: 0.4;
}

/* 成功凭证 */
.done-head {
  text-align: center;
  padding: 6px 0 16px;
}
.done-icon {
  font-size: 52px;
  color: var(--el-color-success);
}
.done-title {
  font-size: 22px;
  color: var(--yx-text-1);
  margin-top: 6px;
}
.done-sub {
  font-size: 13px;
  color: var(--yx-text-3);
  margin-top: 4px;
}
.slip-box {
  border: 1px dashed var(--yx-border);
  border-radius: var(--yx-radius-sm);
  padding: 12px 14px;
  background: var(--yx-surface-muted);
}
.slip-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  font-size: var(--yx-fs-sm);
  padding: 4px 0;
}
.slip-row span {
  color: var(--yx-text-3);
  flex: none;
}
.slip-row b {
  color: var(--yx-text-1);
  font-weight: 500;
  text-align: right;
}
.slip-lines {
  border-top: 1px solid var(--yx-border-soft);
  margin-top: 8px;
  padding-top: 6px;
}
.slip-line {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
  font-size: 15px;
}
.line-name {
  flex: 1;
  color: var(--yx-text-1);
}
.line-name small {
  color: var(--yx-text-3);
}
.line-qty {
  font-weight: 600;
  color: var(--yx-ink-2);
}

/* 页脚 */
.gov-footer {
  text-align: center;
  padding: 28px 16px 24px;
  color: var(--yx-text-3);
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
  background: var(--yx-brand);
  border-radius: 50%;
}
.footer-org {
  font-size: 14px;
  font-weight: 600;
  color: var(--yx-text-2);
}
.footer-system,
.footer-beian {
  font-size: 12px;
  color: var(--yx-text-4);
  margin-bottom: 6px;
}
.footer-beian {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  flex-wrap: wrap;
}
.footer-beian a {
  color: var(--yx-text-4);
  text-decoration: none;
}
.sep {
  color: var(--yx-border);
}

/* 手机端输入框 16px，防止 iOS 聚焦放大 */
:deep(.el-input__inner),
:deep(.el-select__selected-item),
:deep(.el-select__placeholder) {
  font-size: 16px;
}
</style>
