<template>
  <div class="vehicles-page">
    <!-- 页头 -->
    <page-header 
      title="公车管理" 
      subtitle="公车使用无需繁琐审批，出车前在此报备即可，支持一键打印《派车单》与台账统计"
    >
      <template #actions>
        <el-button v-if="authStore.hasPerm('vehicle.apply')" type="primary" :icon="'Plus'" @click="openApplyCreate">用车报备</el-button>
        <el-button v-if="authStore.hasPerm('vehicle.export')" type="success" :icon="'Download'" @click="exportApplies">导出Excel</el-button>
      </template>
    </page-header>

    <!-- 车辆指标卡片 -->
    <el-row :gutter="16">
      <el-col :xs="12" :sm="8" :md="6" v-for="c in statCards" :key="c.label">
        <stat-card 
          :label="c.label" 
          :value="c.value" 
          :icon="c.icon" 
          :icon-color="c.color"
        />
      </el-col>
    </el-row>

    <!-- 核心标签页切换 -->
    <el-tabs v-model="activeTab" class="mt-16">
      <!-- 1. 用车报备流转（首选/常用） -->
      <el-tab-pane label="用车报备台账" name="apply">
        <el-card shadow="never">
          <div class="toolbar">
            <div class="filter-left">
              <el-date-picker 
                v-model="queryDate" 
                type="date" 
                value-format="YYYY-MM-DD" 
                placeholder="按用车日期筛选" 
                style="width: 170px" 
                @change="reloadAppliesFirstPage" 
              />
              <el-button 
                v-if="authStore.isAdmin" 
                :type="showAll ? 'primary' : 'info'" 
                plain 
                @click="toggleMine"
              >
                {{ showAll ? '只看我的' : '查看全部' }}
              </el-button>
            </div>
            <div>
              <el-button type="primary" :icon="'Plus'" @click="openApplyCreate">填写报备</el-button>
              <el-button :icon="'Refresh'" circle class="ml-8" @click="loadApplies" />
            </div>
          </div>

          <el-table :data="applies" v-loading="loading" class="mt-12">
            <template #empty>
              <empty-state description="暂无公车报备记录" />
            </template>

            <el-table-column prop="use_date" label="用车日期" width="115" />
            <el-table-column prop="vehicle_no" label="车牌号" width="120" />
            <el-table-column prop="user_name" label="用车人" width="95" />
            <el-table-column prop="driver_name" label="开车人" width="95" />
            <el-table-column prop="purpose" label="用车事由" min-width="150" show-overflow-tooltip />
            <el-table-column prop="destination" label="目的地" width="130" show-overflow-tooltip />
            <el-table-column prop="use_time" label="时段" width="95" />
            <el-table-column prop="passengers" label="乘车人数" width="90" align="center" />

            <el-table-column label="操作" width="160" fixed="right" align="right">
              <template #default="{ row }">
                <div class="row-action-wrap">
                  <el-button link type="primary" @click="openPrint(row)">打印派车单</el-button>
                  
                  <el-dropdown 
                    v-if="authStore.isAdmin || row.reporter_id === authStore.user?.id"
                    @command="(cmd) => handleApplyRowCommand(cmd, row)" 
                    trigger="click"
                  >
                    <el-button link type="primary" class="more-link">
                      <span>更多</span>
                      <el-icon class="el-icon--right"><ArrowDown /></el-icon>
                    </el-button>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item command="edit" icon="Edit">修改报备</el-dropdown-item>
                        <el-dropdown-item command="delete" icon="Delete" class="text-danger-item">删除记录</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
              </template>
            </el-table-column>
          </el-table>

          <div class="pagination-wrap" v-if="total > 0">
            <el-pagination 
              background 
              layout="total, prev, pager, next" 
              :total="total" 
              :page-size="pageSize" 
              :current-page="page" 
              @current-change="onAppliesPageChange" 
            />
          </div>
        </el-card>
      </el-tab-pane>

      <!-- 2. 车辆档案管理（管理员） -->
      <el-tab-pane label="车辆基础档案" name="vehicles" v-if="authStore.hasPerm('vehicle.manage')">
        <el-card shadow="never">
          <div class="toolbar">
            <div class="filter-left">
              <el-input 
                v-model="vQuery.keyword" 
                placeholder="搜索车牌 / 品牌 / 司机" 
                clearable 
                style="width: 240px" 
                @keyup.enter="loadVehicles" 
                @clear="loadVehicles" 
              >
                <template #prefix><el-icon><Search /></el-icon></template>
              </el-input>
              <el-select 
                v-model="vQuery.status" 
                placeholder="状态" 
                clearable 
                style="width: 120px" 
                @change="loadVehicles"
              >
                <el-option label="可用" :value="1" />
                <el-option label="维修中" :value="3" />
              </el-select>
            </div>
            <el-button type="primary" :icon="'Plus'" @click="openVehicleCreate">新增车辆档案</el-button>
          </div>

          <el-table :data="vehicles" v-loading="loading" class="mt-12">
            <template #empty>
              <empty-state description="暂无录入车辆档案" />
            </template>

            <el-table-column prop="status" label="状态" width="100">
              <template #default="{ row }">
                <status-dot 
                  :type="row.status === 1 ? 'success' : 'warning'" 
                  :text="vStatusNames[row.status] || '未知'"
                />
              </template>
            </el-table-column>

            <el-table-column prop="plate_no" label="车牌号" width="120" />
            <el-table-column prop="brand" label="品牌型号" width="150" />
            <el-table-column prop="seats" label="座位数" width="80" align="center" />
            <el-table-column prop="driver" label="责任司机" width="100" />
            <el-table-column prop="vin" label="车架号(VIN)" width="160" show-overflow-tooltip />
            <el-table-column prop="insurance_date" label="保险到期日" width="120" />

            <el-table-column label="操作" width="140" fixed="right" align="right">
              <template #default="{ row }">
                <el-button link type="primary" size="small" @click="openVehicleEdit(row)">编辑</el-button>
                <el-button link type="danger" size="small" @click="removeVehicle(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- 用车报备：右侧滑出抽屉（Drawer） -->
    <el-drawer 
      v-model="applyDialogVisible" 
      :title="applyEditId ? '修改用车报备' : '公务用车报备登记'" 
      size="560px"
      destroy-on-close
    >
      <el-form ref="applyFormRef" :model="applyForm" :rules="applyRules" label-width="90px">
        <div class="form-section">
          <div class="form-section-title">车辆与人员信息</div>
          <el-form-item label="出车车辆" prop="vehicle_id">
            <el-select v-model="applyForm.vehicle_id" placeholder="选择车辆" style="width:100%">
              <el-option 
                v-for="v in vehicles" 
                :key="v.id"
                :label="v.plate_no + ' ' + (v.brand || '') + '（' + vStatusNames[v.status] + '）'" 
                :value="v.id" 
              />
            </el-select>
          </el-form-item>

          <el-row :gutter="16">
            <el-col :xs="24" :sm="12">
              <el-form-item label="用车人" prop="user_name">
                <el-input v-model="applyForm.user_name" placeholder="用车人姓名" />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12">
              <el-form-item label="开车人">
                <el-input v-model="applyForm.driver_name" placeholder="默认专职司机" />
              </el-form-item>
            </el-col>
          </el-row>
        </div>

        <div class="form-section">
          <div class="form-section-title">出车计划与目的地</div>
          <el-form-item label="用车事由" prop="purpose">
            <el-input v-model="applyForm.purpose" placeholder="如：赴州委宣传部参加专题宣讲会" />
          </el-form-item>

          <el-form-item label="目的地" prop="destination">
            <el-input v-model="applyForm.destination" placeholder="如：伊犁州委宣传部会议室" />
          </el-form-item>

          <el-row :gutter="16">
            <el-col :xs="24" :sm="12">
              <el-form-item label="用车日期" prop="use_date">
                <el-date-picker 
                  v-model="applyForm.use_date" 
                  type="date" 
                  value-format="YYYY-MM-DD" 
                  style="width:100%" 
                />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12">
              <el-form-item label="乘车人数">
                <el-input-number v-model="applyForm.passengers" :min="1" :max="50" style="width:100%" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-form-item label="用车时间">
            <el-input v-model="applyForm.use_time" placeholder="如：09:30 - 17:30" />
          </el-form-item>
        </div>
      </el-form>

      <template #footer>
        <div class="drawer-footer-wrap">
          <el-button @click="applyDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="submitApply">确认报备并完成</el-button>
        </div>
      </template>
    </el-drawer>

    <!-- 车辆档案：右侧抽屉（Drawer） -->
    <el-drawer 
      v-model="vDialogVisible" 
      :title="vEditId ? '编辑车辆档案' : '新增公车档案'" 
      size="500px"
      destroy-on-close
    >
      <el-form ref="vFormRef" :model="vForm" :rules="vRules" label-width="95px">
        <div class="form-section">
          <div class="form-section-title">车辆基础信息</div>
          <el-form-item label="车牌号" prop="plate_no">
            <el-input v-model="vForm.plate_no" placeholder="如：新F12345" />
          </el-form-item>
          <el-form-item label="品牌型号">
            <el-input v-model="vForm.brand" placeholder="如：大众帕萨特" />
          </el-form-item>
          <el-form-item label="核定座位">
            <el-input-number v-model="vForm.seats" :min="1" :max="50" style="width:100%" />
          </el-form-item>
          <el-form-item label="责任司机">
            <el-input v-model="vForm.driver" placeholder="专职驾驶员" />
          </el-form-item>
        </div>

        <div class="form-section">
          <div class="form-section-title">技术与保险状态</div>
          <el-form-item label="车架号(VIN)">
            <el-input v-model="vForm.vin" placeholder="车辆车架号" />
          </el-form-item>
          <el-form-item label="发动机号">
            <el-input v-model="vForm.engine_no" placeholder="发动机编号" />
          </el-form-item>
          <el-form-item label="保险到期">
            <el-date-picker v-model="vForm.insurance_date" type="date" value-format="YYYY-MM-DD" style="width:100%" />
          </el-form-item>
          <el-form-item label="车辆状态">
            <el-select v-model="vForm.status" style="width:100%">
              <el-option label="可用" :value="1" />
              <el-option label="维修中" :value="3" />
            </el-select>
          </el-form-item>
        </div>
      </el-form>

      <template #footer>
        <div class="drawer-footer-wrap">
          <el-button @click="vDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="saveVehicle">保存车辆</el-button>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request, { exportFile } from '../utils/request'
import dayjs from 'dayjs'
import { useAuthStore } from '../store/auth'
import PageHeader from '../components/PageHeader.vue'
import StatCard from '../components/StatCard.vue'
import StatusDot from '../components/StatusDot.vue'
import EmptyState from '../components/EmptyState.vue'

const authStore = useAuthStore()
const activeTab = ref('apply')
const loading = ref(false)

// 车辆档案
const vehicles = ref([])
const vQuery = reactive({ keyword: '', status: '' })
const vDialogVisible = ref(false)
const vEditId = ref(0)
const vFormRef = ref(null)
const vForm = reactive({ plate_no: '', brand: '', seats: 5, driver: '', status: 1, vin: '', engine_no: '', insurance_date: '', inspect_date: '', register_date: '', purchase_at: '', note: '' })
const vRules = reactive({
  plate_no: [
    { required: true, message: '请输入车牌号', trigger: 'blur' },
    { max: 20, message: '最多 20 个字符', trigger: 'blur' }
  ]
})

// 报备记录
const applies = ref([])
const page = ref(1)
const pageSize = 20
const total = ref(0)
const queryDate = ref('')
const showAll = ref(false)
const applyDialogVisible = ref(false)
const applyEditId = ref(0)
const applyFormRef = ref(null)
const applyForm = reactive({ vehicle_id: null, user_name: '', driver_name: '', destination: '', purpose: '', use_date: dayjs().format('YYYY-MM-DD'), use_time: '09:00-18:00', passengers: 1 })
const applyRules = reactive({
  vehicle_id: [{ required: true, message: '请选择车辆', trigger: 'change' }],
  user_name: [
    { required: true, message: '请输入用车人', trigger: 'blur' },
    { max: 20, message: '最多 20 个字符', trigger: 'blur' }
  ],
  purpose: [
    { required: true, message: '请输入用车事由', trigger: 'blur' },
    { max: 200, message: '最多 200 个字符', trigger: 'blur' }
  ],
  destination: [
    { required: true, message: '请输入目的地', trigger: 'blur' },
    { max: 100, message: '最多 100 个字符', trigger: 'blur' }
  ],
  use_date: [{ required: true, message: '请选择用车日期', trigger: 'change' }]
})

const vStatusNames = { 1: '可用', 2: '使用中', 3: '维修中', 4: '报废' }

const statCards = computed(() => {
  const totalV = vehicles.value.length
  const availableV = vehicles.value.filter(v => v.status === 1).length
  const todayApplies = applies.value.filter(a => a.use_date === dayjs().format('YYYY-MM-DD')).length
  return [
    { label: '车辆总数', value: totalV, unit: '辆', icon: 'Van', color: 'var(--yx-ink-2)' },
    { label: '可用车辆', value: availableV, unit: '辆', icon: 'CircleCheck', color: 'var(--el-color-success)' },
    { label: '今日派车', value: todayApplies, unit: '趟', icon: 'Position', color: 'var(--yx-gold-dark)' }
  ]
})

const loadVehicles = async () => {
  try {
    const res = await request.get('/vehicles', { params: vQuery })
    vehicles.value = res.list || []
  } catch (e) { console.error(e) }
}

const openVehicleCreate = () => {
  vEditId.value = 0
  Object.assign(vForm, { plate_no: '', brand: '', seats: 5, driver: '', status: 1, vin: '', engine_no: '', insurance_date: '', inspect_date: '', register_date: '', purchase_at: '', note: '' })
  vDialogVisible.value = true
}

const openVehicleEdit = (row) => {
  vEditId.value = row.id
  Object.assign(vForm, {
    plate_no: row.plate_no, brand: row.brand, seats: row.seats, driver: row.driver,
    status: row.status, vin: row.vin, engine_no: row.engine_no,
    insurance_date: row.insurance_date, inspect_date: row.inspect_date,
    register_date: row.register_date, purchase_at: row.purchase_at, note: row.note
  })
  vDialogVisible.value = true
}

const saveVehicle = async () => {
  if (!vFormRef.value) return
  await vFormRef.value.validate(async (valid) => {
    if (!valid) return
    try {
      if (vEditId.value) {
        await request.put('/vehicles', { ...vForm, id: vEditId.value })
        ElMessage.success('更新成功')
      } else {
        await request.post('/vehicles', vForm)
        ElMessage.success('新增成功')
      }
      vDialogVisible.value = false
      loadVehicles()
    } catch (e) { console.error(e) }
  })
}

const removeVehicle = async (row) => {
  try {
    await ElMessageBox.confirm(`确认删除车辆「${row.plate_no}」？此操作不可恢复。`, '高危操作确认', {
      type: 'warning',
      confirmButtonText: '确认删除',
      confirmButtonClass: 'el-button--danger'
    })
    await request.delete(`/vehicles/${row.id}`)
    ElMessage.success('已删除')
    loadVehicles()
  } catch (e) { console.error(e) }
}

const loadApplies = async () => {
  loading.value = true
  try {
    const params = { page: page.value, page_size: pageSize }
    if (queryDate.value) params.date = queryDate.value
    if (showAll.value && authStore.isAdmin) params.all = 1
    const res = await request.get('/vehicle-applies', { params })
    applies.value = res.list || []
    total.value = res.total || 0
  } catch (e) { console.error(e) } finally {
    loading.value = false
  }
}

const reloadAppliesFirstPage = () => {
  page.value = 1
  loadApplies()
}

const onAppliesPageChange = (p) => {
  page.value = p
  loadApplies()
}

const toggleMine = () => {
  showAll.value = !showAll.value
  reloadAppliesFirstPage()
}

const openApplyCreate = () => {
  applyEditId.value = 0
  const defVehicle = vehicles.value.find(v => v.status === 1)
  Object.assign(applyForm, {
    vehicle_id: defVehicle ? defVehicle.id : null,
    user_name: authStore.user?.real_name || '',
    driver_name: defVehicle ? defVehicle.driver : '',
    destination: '', purpose: '',
    use_date: dayjs().format('YYYY-MM-DD'),
    use_time: '09:00-18:00',
    passengers: 1
  })
  applyDialogVisible.value = true
}

const openApplyEdit = (row) => {
  applyEditId.value = row.id
  Object.assign(applyForm, {
    vehicle_id: row.vehicle_id, user_name: row.user_name, driver_name: row.driver_name,
    destination: row.destination, purpose: row.purpose, use_date: row.use_date,
    use_time: row.use_time, passengers: row.passengers
  })
  applyDialogVisible.value = true
}

const submitApply = async () => {
  if (!applyFormRef.value) return
  await applyFormRef.value.validate(async (valid) => {
    if (!valid) return
    try {
      if (applyEditId.value) {
        await request.put('/vehicle-applies', { ...applyForm, id: applyEditId.value })
        ElMessage.success('报备更新成功')
      } else {
        await request.post('/vehicle-applies', applyForm)
        ElMessage.success('报备成功')
      }
      applyDialogVisible.value = false
      loadApplies()
    } catch (e) { console.error(e) }
  })
}

const handleApplyRowCommand = (cmd, row) => {
  if (cmd === 'edit') openApplyEdit(row)
  else if (cmd === 'delete') removeApply(row)
}

const removeApply = async (row) => {
  try {
    await ElMessageBox.confirm(`确认删除该用车报备记录？`, '提示', {
      type: 'warning',
      confirmButtonText: '确认删除',
      confirmButtonClass: 'el-button--danger'
    })
    await request.delete(`/vehicle-applies/${row.id}`)
    ElMessage.success('已删除')
    loadApplies()
  } catch (e) { console.error(e) }
}

const openPrint = (row) => {
  sessionStorage.setItem('printVehicle', JSON.stringify(row))
  sessionStorage.setItem('printApply', JSON.stringify(row))
  const url = `/vehicle/print/${row.id}`
  window.open(url, '_blank')
}

const exportApplies = async () => {
  try {
    const params = {}
    if (queryDate.value) params.date = queryDate.value
    if (showAll.value && authStore.isAdmin) params.all = 1
    await exportFile('/export/vehicles', params, '公车使用台账.xlsx')
  } catch (e) { console.error(e) }
}

onMounted(() => {
  loadVehicles()
  loadApplies()
})
</script>

<style scoped>
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}

.filter-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.row-action-wrap {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

.more-link {
  font-size: 13px;
  color: var(--yx-text-3);
}

.more-link:hover {
  color: var(--yx-ink-2);
}

:deep(.text-danger-item) {
  color: var(--el-color-danger) !important;
}

.pagination-wrap {
  margin-top: 18px;
  display: flex;
  justify-content: flex-end;
}

.drawer-footer-wrap {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.mt-12 {
  margin-top: 12px;
}
.mt-16 {
  margin-top: 16px;
}
.ml-8 {
  margin-left: 8px;
}
</style>
