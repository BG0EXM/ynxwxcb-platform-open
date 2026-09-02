<template>
  <div>
    <el-card shadow="never">
      <div class="toolbar">
        <div>
          <el-button type="primary" :icon="'Plus'" @click="openCreate">录入会议</el-button>
        </div>
      </div>

      <el-table :data="list" stripe v-loading="loading" empty-text="暂无会议">
        <el-table-column prop="title" label="会议标题" min-width="200" show-overflow-tooltip />
        <el-table-column prop="meeting_date" label="日期" width="110" />
        <el-table-column prop="meeting_time" label="时间" width="90" />
        <el-table-column prop="location" label="地点" width="130" show-overflow-tooltip />
        <el-table-column label="已报名" width="80">
          <template #default="{ row }">
            <el-tag size="small" type="success">{{ row.reg_count }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="不参加" width="80">
          <template #default="{ row }">
            <el-tag size="small" type="info">{{ row.not_attend }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="300" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click.stop="openEdit(row)">编辑</el-button>
            <el-button link type="success" size="small" @click.stop="copyLink(row)">复制链接</el-button>
            <el-button link type="info" size="small" @click.stop="openDetail(row)">报名情况</el-button>
            <el-button link type="warning" size="small" @click.stop="exportReg(row)">导出</el-button>
            <el-button link type="danger" size="small" @click.stop="removeMeeting(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 录入/编辑会议 -->
    <el-dialog v-model="dialogVisible" :title="editId ? '编辑会议' : '录入会议'" width="640px">
      <el-form :model="form" label-width="100px">
        <el-form-item label="会议标题" required>
          <el-input v-model="form.title" placeholder="如：全县宣传思想文化工作会议" />
        </el-form-item>
        <el-form-item label="会议日期" required>
          <el-date-picker v-model="form.meeting_date" type="date" value-format="YYYY-MM-DD" style="width:100%" />
        </el-form-item>
        <el-form-item label="会议时间">
          <el-time-picker v-model="form.meeting_time" value-format="HH:mm" format="HH:mm" placeholder="如 10:00" style="width:100%" />
        </el-form-item>
        <el-form-item label="会议地点">
          <el-input v-model="form.location" placeholder="如：县委三楼会议室" />
        </el-form-item>
        <el-form-item label="参会单位范围" required>
          <el-input v-model="form.units" type="textarea" :rows="6"
            placeholder="每行填写一个单位，如：&#10;县委办&#10;县政府办&#10;各乡镇党委&#10;宣传部各科室" />
          <div class="form-tip">每行一个单位，参会单位从这些中下拉选择</div>
        </el-form-item>
        <el-form-item label="参会人数">
          <el-radio-group v-model="form.unit_limit" @change="onLimitChange">
            <el-radio :label="1">每单位 1 人</el-radio>
            <el-radio :label="2">每单位 2 人</el-radio>
            <el-radio :label="3">每单位 3 人</el-radio>
            <el-radio :label="4">每单位 4 人</el-radio>
            <el-radio :label="5">每单位 5 人</el-radio>
            <el-radio :label="0">不限制</el-radio>
          </el-radio-group>
          <div class="form-tip">每个单位可报名的人数上限，实际可少于该人数；"不限制"表示不限人数。</div>
        </el-form-item>
        <el-form-item label="会议内容">
          <el-input v-model="form.content" type="textarea" :rows="4" placeholder="会议议程、要求等（选填）" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <!-- 报名情况 -->
    <el-dialog v-model="detailVisible" :title="detailTitle" width="800px">
      <el-tabs v-model="detailTab">
        <el-tab-pane label="参会人员" name="attend">
          <el-table :data="attendRegs" stripe size="small" empty-text="暂无参会报名">
            <el-table-column prop="unit" label="单位" width="160" show-overflow-tooltip />
            <el-table-column prop="attendee_name" label="姓名" width="90" />
            <el-table-column prop="attendee_title" label="职务" width="110" />
            <el-table-column prop="phone" label="电话" width="130" />
          </el-table>
        </el-tab-pane>
        <el-tab-pane :label="`不参加(${notAttendRegs.length})`" name="absent">
          <el-table :data="notAttendRegs" stripe size="small" empty-text="暂无请假不参加">
            <el-table-column prop="unit" label="单位" width="160" show-overflow-tooltip />
            <el-table-column prop="reason" label="不参加原因" min-width="200" show-overflow-tooltip />
          </el-table>
        </el-tab-pane>
      </el-tabs>
      <div class="detail-links">
        <el-button type="success" :icon="'Download'" @click="exportReg(detailMeeting)">导出签到单</el-button>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import request, { exportFile } from '../utils/request'
import dayjs from 'dayjs'

const list = ref([])
const loading = ref(false)
const dialogVisible = ref(false)
const editId = ref(0)
const form = ref({ title: '', meeting_date: dayjs().format('YYYY-MM-DD'), meeting_time: '', location: '', content: '', units: '', unit_limit: 1 })
const detailVisible = ref(false)
const detailTitle = ref('')
const detailTab = ref('attend')
const attendRegs = ref([])
const notAttendRegs = ref([])
const detailMeeting = ref(null)

const loadData = async () => {
  loading.value = true
  try {
    const res = await request.get('/meetings')
    list.value = res.list || []
  } catch (e) {
  } finally {
    loading.value = false
  }
}

const openCreate = () => {
  editId.value = 0
  form.value = { title: '', meeting_date: dayjs().format('YYYY-MM-DD'), meeting_time: '10:00', location: '', content: '', units: '', unit_limit: 1 }
  dialogVisible.value = true
}

const openEdit = (row) => {
  editId.value = row.id
  form.value = {
    title: row.title, meeting_date: row.meeting_date, meeting_time: row.meeting_time || '',
    location: row.location || '', content: row.content || '', units: row.units || '',
    unit_limit: row.unit_limit || 1
  }
  dialogVisible.value = true
}

const save = async () => {
  if (!form.value.title) return ElMessage.warning('请输入会议标题')
  if (!form.value.meeting_date) return ElMessage.warning('请选择会议日期')
  if (!form.value.units) return ElMessage.warning('请填写参会单位范围')
  try {
    if (editId.value) {
      await request.put('/meetings', { ...form.value, id: editId.value })
      ElMessage.success('更新成功')
    } else {
      await request.post('/meetings', form.value)
      ElMessage.success('创建成功，可复制报名链接发送给各单位')
    }
    dialogVisible.value = false
    loadData()
  } catch (e) {}
}

const openDetail = async (row) => {
  detailMeeting.value = row
  detailTitle.value = row.title
  detailTab.value = 'attend'
  try {
    const res = await request.get(`/meetings/${row.id}`)
    const regs = res.registrations || []
    attendRegs.value = regs.filter(r => r.not_attend !== 1)
    notAttendRegs.value = regs.filter(r => r.not_attend === 1)
  } catch (e) {}
  detailVisible.value = true
}

const copyLink = async (row) => {
  const base = window.location.origin
  const url = `${base}/meeting/${row.id}`
  try {
    await navigator.clipboard.writeText(url)
    ElMessage.success('报名链接已复制：' + url)
  } catch (e) {
    ElMessageBox.alert(`请手动复制报名链接：\n${url}`, '报名链接', { confirmButtonText: '知道了' })
  }
}

const exportReg = (row) => {
  exportFile(`/export/meetings/${row.id}/registration`)
}

const removeMeeting = async (row) => {
  try {
    await ElMessageBox.confirm(`确认删除会议「${row.title}」及其报名记录？`, '删除确认', { type: 'warning', confirmButtonText: '删除' })
  } catch (e) { return }
  try {
    await request.delete(`/meetings/${row.id}`)
    ElMessage.success('删除成功')
    loadData()
  } catch (e) {}
}

onMounted(loadData)
</script>

<style scoped>
.toolbar {
  display: flex;
  justify-content: space-between;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 8px;
}
.form-tip {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}
.detail-links {
  margin-top: 16px;
  display: flex;
  gap: 8px;
}
</style>
