<template>
  <div>
    <el-card shadow="never" v-loading="loading">
      <div class="toolbar">
        <div>
          <div class="title">角色-功能权限</div>
          <div class="tip">勾选即授权；管理员（admin）始终拥有全部权限，不可取消。保存后立即生效。</div>
        </div>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </div>

      <el-table :data="permissions" border size="small" :span-method="spanMethod">
        <el-table-column prop="module" label="模块" width="110" />
        <el-table-column prop="name" label="功能" min-width="240" />
        <el-table-column v-for="r in roles" :key="r.code" :label="r.name" width="130" align="center">
          <template #default="{ row }">
            <el-checkbox v-if="r.code === 'admin'" :model-value="true" disabled />
            <el-checkbox v-else v-model="matrix[r.code][row.code]" />
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import request from '../utils/request'

const loading = ref(false)
const saving = ref(false)
const permissions = ref([])
const roles = ref([])
const matrix = reactive({})

// 相同模块的单元格合并
const spanMethod = ({ row, columnIndex }) => {
  if (columnIndex !== 0) return
  const rows = permissions.value
  const idx = rows.findIndex(r => r.code === row.code)
  const isFirst = idx === 0 || rows[idx - 1].module !== row.module
  if (!isFirst) return { rowspan: 0, colspan: 0 }
  let count = 0
  for (let i = idx; i < rows.length && rows[i].module === row.module; i++) count++
  return { rowspan: count, colspan: 1 }
}

const load = async () => {
  loading.value = true
  try {
    const res = await request.get('/permissions')
    permissions.value = (res.permissions || []).slice().sort((a, b) => (a.sort || 0) - (b.sort || 0))
    roles.value = res.roles || []
    const m = res.matrix || {}
    for (const r of roles.value) {
      matrix[r.code] = {}
      const codes = m[r.code] || []
      for (const p of permissions.value) {
        matrix[r.code][p.code] = r.code === 'admin' ? true : codes.includes(p.code)
      }
    }
  } catch (e) {
  } finally {
    loading.value = false
  }
}

const save = async () => {
  saving.value = true
  try {
    const payload = {
      roles: roles.value
        .filter(r => r.code !== 'admin')
        .map(r => ({ role_id: r.id, codes: Object.keys(matrix[r.code]).filter(c => matrix[r.code][c]) }))
    }
    await request.put('/permissions', payload)
    ElMessage.success('保存成功，已立即生效')
  } catch (e) {
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 16px;
}
.title { font-size: 16px; font-weight: 600; }
.tip { font-size: 12px; color: #909399; margin-top: 4px; }
</style>
