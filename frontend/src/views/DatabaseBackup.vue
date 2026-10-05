<template>
  <div class="database-backup-page">
    <page-header 
      title="数据库备份" 
      subtitle="系统数据库在线快照管理与离线备份文件下载（仅超级管理员）"
    >
      <template #actions>
        <el-button 
          type="primary" 
          :icon="'CirclePlus'" 
          :loading="creating" 
          @click="handleCreateBackup"
        >
          立即创建备份
        </el-button>
        <el-button 
          :icon="'Refresh'" 
          :loading="loading" 
          @click="loadData"
        >
          刷新列表
        </el-button>
      </template>
    </page-header>

    <el-card shadow="never" class="backup-card gov-card">
      <!-- 政务级安全提示栏 -->
      <el-alert
        type="warning"
        show-icon
        :closable="false"
        class="security-alert"
      >
        <template #title>
          <span class="security-title">政务数据安全保密须知</span>
        </template>
        <template #default>
          <div class="security-desc">
            建议定期在重要业务操作前创建快照，并下载至离线涉密介质保存；备份文件包含部务平台全量业务数据，请严格遵守保密纪律。
          </div>
        </template>
      </el-alert>

      <!-- 概览信息栏 -->
      <div class="table-toolbar">
        <div class="toolbar-stats">
          <span>共 <strong class="stats-count">{{ backups.length }}</strong> 份历史快照备份</span>
        </div>
      </div>

      <!-- 备份记录表格 -->
      <el-table 
        :data="backups" 
        stripe 
        v-loading="loading" 
        class="backup-table"
        size="large"
      >
        <!-- 备份文件名称（带图标） -->
        <el-table-column prop="filename" label="备份文件名称" min-width="280">
          <template #default="{ row }">
            <div class="file-name-cell">
              <el-icon class="file-icon"><Coin /></el-icon>
              <span class="file-name-text">{{ row.filename }}</span>
            </div>
          </template>
        </el-table-column>

        <!-- 文件大小 -->
        <el-table-column prop="size_text" label="文件大小" width="140">
          <template #default="{ row }">
            <el-tag size="small" type="info" class="size-tag">{{ row.size_text }}</el-tag>
          </template>
        </el-table-column>

        <!-- 创建时间 -->
        <el-table-column prop="created_at" label="创建时间" width="190">
          <template #default="{ row }">
            <span class="time-text">{{ row.created_at }}</span>
          </template>
        </el-table-column>

        <!-- 安全类型 -->
        <el-table-column label="安全类型" width="220">
          <template #default>
            <el-tag size="small" type="success" effect="plain" class="security-tag">
              SQLite 在线一致性快照
            </el-tag>
          </template>
        </el-table-column>

        <!-- 操作列 -->
        <el-table-column label="操作" width="170" fixed="right">
          <template #default="{ row }">
            <el-button 
              link 
              type="primary" 
              :icon="'Download'" 
              :loading="downloadingFile === row.filename"
              @click="handleDownload(row)"
            >
              下载备份到本地
            </el-button>
          </template>
        </el-table-column>

        <!-- 空状态展示 -->
        <template #empty>
          <empty-state description="暂无备份记录，请点击右上角“立即创建备份”生成新快照" />
        </template>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request, { downloadFile } from '../utils/request'
import PageHeader from '../components/PageHeader.vue'
import EmptyState from '../components/EmptyState.vue'

const backups = ref([])
const loading = ref(false)
const creating = ref(false)
const downloadingFile = ref('')

// 获取备份列表
const loadData = async () => {
  loading.value = true
  try {
    const res = await request.get('/system/backups')
    backups.value = res.list || []
  } catch (e) {
    console.error(e)
    // request.js 会自动弹出错误提示
  } finally {
    loading.value = false
  }
}

// 立即创建备份
const handleCreateBackup = () => {
  ElMessageBox.confirm(
    '确认立即创建系统数据库备份快照？系统将使用在线一致性事务机制生成全量数据副本，过程通常在数秒内完成。',
    '创建数据库备份',
    {
      confirmButtonText: '确定创建',
      cancelButtonText: '取消',
      type: 'warning'
    }
  ).then(async () => {
    creating.value = true
    try {
      const res = await request.post('/system/backups')
      ElMessage.success(res.message ? `${res.message}：${res.filename}` : `备份成功：${res.filename}`)
      await loadData()
    } catch (e) {
      console.error(e)
      // request.js 会自动拦截并提示错误
    } finally {
      creating.value = false
    }
  }).catch(() => {
    // 用户取消创建
  })
}

// 下载备份到本地
const handleDownload = async (row) => {
  if (downloadingFile.value === row.filename) return
  downloadingFile.value = row.filename
  try {
    await downloadFile(
      `/system/backups/download?filename=${encodeURIComponent(row.filename)}`,
      row.filename
    )
    ElMessage.success(`开始下载备份文件「${row.filename}」`)
  } catch (e) {
    console.error(e)
    ElMessage.error('下载备份文件失败，请检查网络或权限')
  } finally {
    downloadingFile.value = ''
  }
}

onMounted(() => {
  loadData()
})
</script>

<style scoped>
.database-backup-page {
  display: flex;
  flex-direction: column;
}

.backup-card {
  border-radius: var(--yx-radius);
  border: 1px solid rgba(220, 223, 229, 0.6);
  background: var(--yx-surface);
  box-shadow: 0 1px 3px rgba(19, 26, 38, 0.04);
}

/* 政务级安全警示栏 */
.security-alert {
  margin-bottom: 20px;
  background: #fdfaf5;
  border: 1px solid #ebd8be;
  border-radius: var(--yx-radius-sm);
  padding: 12px 16px;
}

.security-alert :deep(.el-alert__icon) {
  color: var(--yx-gold);
  font-size: 18px;
}

.security-title {
  font-weight: 600;
  font-size: 14px;
  color: var(--yx-text-1);
}

.security-desc {
  margin-top: 4px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--yx-text-2);
}

/* 概览工具栏 */
.table-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}

.toolbar-stats {
  font-size: var(--yx-fs-sm);
  color: var(--yx-text-3);
}

.stats-count {
  color: var(--yx-brand);
  font-weight: 600;
  margin: 0 2px;
}

/* 文件名称单元格 */
.file-name-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.file-icon {
  color: var(--yx-gold);
  font-size: 16px;
  flex-shrink: 0;
}

.file-name-text {
  font-family: var(--yx-font-mono, monospace);
  font-size: 13.5px;
  font-weight: 500;
  color: var(--yx-text-1);
  word-break: break-all;
}

.size-tag {
  font-family: var(--yx-font-mono, monospace);
  font-weight: 500;
}

.time-text {
  font-family: var(--yx-font-mono, monospace);
  font-size: 13px;
  color: var(--yx-text-2);
}

.security-tag {
  border-color: rgba(103, 194, 58, 0.35);
  font-weight: 500;
}
</style>
