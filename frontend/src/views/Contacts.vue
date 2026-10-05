<template>
  <div class="contacts-page">
    <page-header 
      title="通讯录" 
      subtitle="部机关干部职工与宣传系统各单位联系人名册，方便日常工作联络对接"
    >
      <template #actions>
        <el-button type="primary" :icon="'Plus'" @click="openCreate">添加联系人</el-button>
        <el-button :icon="'Refresh'" circle @click="loadData" />
      </template>
    </page-header>

    <el-row :gutter="20">
      <el-col :span="5">
        <el-card shadow="never" class="gov-card" style="height: 100%; border: none;">
          <el-tree
            :data="treeData"
            :props="{ label: 'name', children: 'children' }"
            default-expand-all
            highlight-current
            @node-click="handleNodeClick"
          />
        </el-card>
      </el-col>
      <el-col :span="19">
        <el-card shadow="never" class="gov-card">
          <div class="toolbar">
            <div>
              <el-input v-model="keyword" placeholder="搜索姓名/职务/电话" clearable style="width: 240px"
                @keyup.enter="reloadFirstPage" @clear="reloadFirstPage">
                <template #append><el-button :icon="'Search'" @click="loadData" /></template>
              </el-input>
              <el-select v-model="department_id" placeholder="按部门" clearable style="width: 160px" class="ml-8" @change="reloadFirstPage">
                <el-option v-for="d in departments" :key="d.id" :label="d.name" :value="d.id" />
              </el-select>
            </div>
          </div>

          <el-table size="large" :data="list" stripe v-loading="loading" empty-text="暂无联系人">
            <el-table-column prop="name" label="姓名" width="130">
              <template #default="{ row }">
                <el-button
                  v-if="getContactUserId(row)"
                  link
                  type="primary"
                  class="font-medium"
                  @click="openProfile(row)"
                >
                  {{ row.name }}
                </el-button>
                <span v-else>{{ row.name }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="position" label="职务" width="150" />
            <el-table-column prop="department_name" label="部门" width="140" />
            <el-table-column prop="phone" label="电话" width="150" />
            <el-table-column label="操作" width="220" fixed="right">
              <template #default="{ row }">
                <el-button
                  v-if="getContactUserId(row)"
                  link
                  type="primary"
                  size="small"
                  @click="openProfile(row)"
                >出勤档案</el-button>
                <el-button link type="primary" size="small" @click="openEdit(row)">编辑</el-button>
                <el-button link type="danger" size="small" @click="remove(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
          <div class="pagination-wrap" v-if="total > 0">
            <el-pagination background layout="total, prev, pager, next" :total="total" :page-size="pageSize" :current-page="page" @current-change="onPageChange" />
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-drawer v-model="dialogVisible" :title="editId ? '编辑联系人' : '添加联系人'" size="540px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="姓名" required>
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item label="职务">
          <el-input v-model="form.position" />
        </el-form-item>
        <el-form-item label="部门">
          <el-select v-model="form.department_id" style="width: 100%">
            <el-option v-for="d in departments" :key="d.id" :label="d.name" :value="d.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="电话">
          <el-input v-model="form.phone" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-drawer>

    <!-- 干部出勤与休假个人全息档案抽屉 -->
    <attendance-profile-drawer
      v-model:visible="profileDrawerVisible"
      :user-id="currentProfileUserId"
      :user-name="currentProfileUserName"
    />
  </div>
</template>

<script setup>
import { ref, reactive, watch, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import request from '../utils/request'
import PageHeader from '../components/PageHeader.vue'
import AttendanceProfileDrawer from '../components/AttendanceProfileDrawer.vue'

const route = useRoute()
const list = ref([])
const loading = ref(false)
const page = ref(1)
const pageSize = 20
const total = ref(0)
const keyword = ref('')
const department_id = ref('')
const departments = ref([])
const dialogVisible = ref(false)
const editId = ref(0)
const form = reactive({ name: '', position: '', department_id: null, phone: '' })

// 干部出勤全息档案
const userMap = ref({})
const profileDrawerVisible = ref(false)
const currentProfileUserId = ref(null)
const currentProfileUserName = ref('')

const loadUserMapping = async () => {
  try {
    let res = null
    try {
      res = await request.get('/assignees')
    } catch {
      res = await request.get('/users')
    }
    const users = res?.list || []
    const map = {}
    users.forEach(u => {
      if (u.real_name && u.id) {
        map[u.real_name] = u.id
      }
    })
    userMap.value = map
  } catch (e) { console.error(e) }
}

const getContactUserId = (row) => {
  if (row.user_id) return row.user_id
  return userMap.value[row.name] || null
}

const openProfile = (row) => {
  const uid = getContactUserId(row)
  if (!uid) return
  currentProfileUserId.value = uid
  currentProfileUserName.value = row.name
  profileDrawerVisible.value = true
}

const treeData = computed(() => {
  return [{ id: '', name: '中共伊宁县委宣传部', children: departments.value || [] }]
})
const handleNodeClick = (data) => {
  department_id.value = data.id || ''
  reloadFirstPage()
}

const loadData = async () => {
  loading.value = true
  try {
    const res = await request.get('/contacts', { params: { keyword: keyword.value, department_id: department_id.value, page: page.value, page_size: pageSize } })
    list.value = res.list || []
    total.value = res.total || 0
  } catch (e) { console.error(e) } finally {
    loading.value = false
  }
}

const reloadFirstPage = () => { page.value = 1; loadData() }
const onPageChange = (p) => { page.value = p; loadData() }

const loadDepartments = async () => {
  try {
    const res = await request.get('/departments')
    departments.value = res.list || []
  } catch (e) { console.error(e) }
}

const openCreate = () => {
  editId.value = 0
  Object.assign(form, { name: '', position: '', department_id: null, phone: '' })
  dialogVisible.value = true
}

const openEdit = (row) => {
  editId.value = row.id
  Object.assign(form, { name: row.name, position: row.position, department_id: row.department_id, phone: row.phone })
  dialogVisible.value = true
}

const save = async () => {
  if (!form.name) return ElMessage.warning('请输入姓名')
  try {
    if (editId.value) {
      await request.put('/contacts', { ...form, id: editId.value })
      ElMessage.success('更新成功')
    } else {
      await request.post('/contacts', form)
      ElMessage.success('添加成功')
    }
    dialogVisible.value = false
    loadData()
  } catch (e) { console.error(e) }
}

const remove = async (row) => {
  try {
    await ElMessageBox.confirm(`确认删除联系人「${row.name}」？`, '提示', { type: 'warning' })
  } catch (e) { console.error(e); return }
  try {
    await request.delete(`/contacts/${row.id}`)
    ElMessage.success('删除成功')
    loadData()
  } catch (e) { console.error(e) }
}

onMounted(() => {
  if (route.query.keyword) {
    keyword.value = String(route.query.keyword)
  }
  loadData()
  loadDepartments()
  loadUserMapping()
})

watch(() => route.query.keyword, (newKw) => {
  if (newKw !== undefined) {
    keyword.value = String(newKw || '')
    page.value = 1
    loadData()
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
.ml-8 { margin-left: 8px; }
.pagination-wrap { margin-top: 14px; display: flex; justify-content: flex-end; }
</style>
