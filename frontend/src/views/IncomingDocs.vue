<template>
  <div class="incoming-docs-page">
    <!-- 页头 -->
    <page-header 
      title="收文管理" 
      subtitle="严格规范登记、拟办批示、传阅承办与清退归档全流程公文流转"
      :tag="total ? `共 ${total} 份公文` : ''"
    >
      <template #actions>
        <el-button v-if="authStore.hasPerm('incoming.manage')" type="primary" :icon="'Plus'" @click="openCreate">收文登记</el-button>
        <el-button v-if="authStore.hasPerm('incoming.export')" type="success" :icon="'Download'" @click="exportData">导出Excel</el-button>
        <el-button :icon="'Refresh'" circle @click="loadData" />
      </template>
    </page-header>

    <!-- 筛选工具栏 -->
    <el-card shadow="never" class="gov-card">
      <div class="filter-bar">
        <div class="filter-main-row">
          <el-input 
            v-model="query.keyword" 
            placeholder="搜索标题 / 来文单位 / 字号" 
            clearable 
            style="width: 280px"
            @keyup.enter="reloadFirstPage" 
            @clear="reloadFirstPage"
          >
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>

          <el-select 
            v-model="query.status" 
            placeholder="办理状态" 
            clearable 
            style="width: 130px" 
            @change="reloadFirstPage"
          >
            <el-option label="全部办理状态" value="" />
            <el-option v-for="(name, val) in statusNames" :key="val" :label="name" :value="Number(val)" />
          </el-select>

          <el-button 
            type="primary" 
            plain 
            @click="reloadFirstPage"
          >
            查询
          </el-button>

          <el-button 
            link 
            type="primary" 
            @click="showAdvanced = !showAdvanced"
            class="advanced-toggle-btn"
          >
            <span>{{ showAdvanced ? '收起高级筛选' : '高级筛选' }}</span>
            <el-icon class="el-icon--right">
              <ArrowUp v-if="showAdvanced" />
              <ArrowDown v-else />
            </el-icon>
          </el-button>

          <el-button link @click="resetQuery" v-if="hasActiveFilters">重置筛选</el-button>
        </div>

        <!-- 高级筛选折叠区 -->
        <transition name="fade">
          <div v-show="showAdvanced" class="advanced-filter-panel">
            <el-input 
              v-model="query.archive_box_no" 
              placeholder="归档盒号/卷宗号" 
              clearable 
              style="width: 160px"
              @keyup.enter="reloadFirstPage" 
              @clear="reloadFirstPage"
            />

            <el-select v-model="query.returned" placeholder="是否已退" clearable style="width: 120px" @change="reloadFirstPage">
              <el-option label="已退" :value="1" />
              <el-option label="未退" :value="0" />
            </el-select>

            <el-select v-model="query.need_return" placeholder="是否需退回" clearable style="width: 130px" @change="reloadFirstPage">
              <el-option label="需要退回" :value="1" />
              <el-option label="不需要退回" :value="0" />
            </el-select>

            <el-date-picker 
              v-model="dateRange" 
              type="daterange" 
              value-format="YYYY-MM-DD"
              range-separator="至" 
              start-placeholder="收文起始" 
              end-placeholder="收文结束"
              style="width: 250px"
              @change="reloadFirstPage" 
            />
          </div>
        </transition>
      </div>

      <!-- 表格数据区 -->
      <el-table size="large" 
        :data="list" 
        v-loading="loading" 
        empty-text="暂无收文记录" 
        @row-click="openDetail"
        class="mt-16 custom-doc-table"
      >
        <template #empty>
          <empty-state description="暂无符合条件的收文记录" />
        </template>

        <el-table-column prop="receive_no" label="收文编号" width="125" />
        <el-table-column prop="received_date" label="收文日期" width="105" />
        <el-table-column prop="from_unit" label="来文单位" width="140" show-overflow-tooltip />
        <el-table-column prop="from_doc_no" label="来文字号" width="140" show-overflow-tooltip />
        <el-table-column prop="doc_no" label="文件编号" width="120" show-overflow-tooltip />
        <el-table-column prop="title" label="文件标题" min-width="260" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="table-doc-title">{{ row.title }}</span>
          </template>
        </el-table-column>

        <el-table-column prop="status" label="办理状态" width="105">
          <template #default="{ row }">
            <el-tag 
              :class="['status-tag', 'status-doc-' + (row.status || 1)]" 
              effect="plain"
            >
              {{ statusNames[row.status] || '待登记' }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column prop="assigned_department" label="承办科室" width="120" show-overflow-tooltip>
          <template #default="{ row }">
            <span>{{ row.assigned_department || '—' }}</span>
          </template>
        </el-table-column>

        <el-table-column prop="archive_box_no" label="归档盒号" width="125" show-overflow-tooltip>
          <template #default="{ row }">
            <span v-if="row.archive_box_no" class="box-no-badge">
              <el-icon style="vertical-align: -1px; margin-right: 3px;"><FolderOpened /></el-icon>
              {{ row.archive_box_no }}
            </span>
            <span v-else style="color: var(--yx-text-4); font-size: 12px;">—</span>
          </template>
        </el-table-column>

        <el-table-column prop="secret_level" label="密级" width="85">
          <template #default="{ row }">
            <el-tag size="small" :type="secretTag(row.secret_level)" effect="plain">{{ row.secret_level }}</el-tag>
          </template>
        </el-table-column>

        <el-table-column label="清退状态" width="85">
          <template #default="{ row }">
            <span v-if="row.returned === 1" style="color:var(--el-color-success);font-size:12px;">已退回</span>
            <span v-else-if="row.need_return === 1" style="color:var(--el-color-danger);font-size:12px;font-weight:500;">需清退</span>
            <span v-else style="color:var(--yx-text-4);font-size:12px;">无需退</span>
          </template>
        </el-table-column>

        <el-table-column label="操作" width="215" fixed="right" align="right">
          <template #default="{ row }">
            <div class="row-action-wrap" @click.stop>
              <el-button link type="primary" @click="openDetail(row)">办理</el-button>
              
              <el-button 
                v-if="authStore.hasPerm('incoming.manage')" 
                link 
                type="primary" 
                @click="openStatusDialog(row)"
              >
                办理/归档
              </el-button>
              
              <el-dropdown @command="(cmd) => handleRowCommand(cmd, row)" trigger="click">
                <el-button link type="primary" class="more-link">
                  <span>更多</span>
                  <el-icon class="el-icon--right"><ArrowDown /></el-icon>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item 
                      v-if="authStore.hasPerm('incoming.manage')" 
                      command="status" 
                      icon="Promotion"
                    >
                      办理/归档
                    </el-dropdown-item>
                    <el-dropdown-item command="print" icon="Printer">打印呈批单</el-dropdown-item>
                    <el-dropdown-item command="label" icon="PriceTag">打印文件标签</el-dropdown-item>
                    <el-dropdown-item command="card" icon="Tickets">打印传阅卡</el-dropdown-item>
                    <el-dropdown-item 
                      v-if="authStore.hasPerm('incoming.manage')" 
                      command="edit" 
                      icon="Edit" 
                      divided
                    >
                      编辑公文
                    </el-dropdown-item>
                    <el-dropdown-item 
                      v-if="authStore.isAdmin" 
                      command="delete" 
                      icon="Delete" 
                      class="text-danger-item"
                    >
                      删除公文
                    </el-dropdown-item>
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
          @current-change="onPageChange" 
        />
      </div>
    </el-card>

    <!-- 1. 收文登记/编辑：右侧滑出抽屉（Drawer） -->
    <el-drawer 
      v-model="drawerVisible" 
      :title="editId ? '编辑收文公文' : '新公文收文登记'" 
      size="660px"
      destroy-on-close
    >
      <el-form :model="form" label-width="95px" size="default">
        <!-- 分组 1：文件基本信息 -->
        <div class="form-section">
          <div class="form-section-title">公文基本信息</div>
          <el-row :gutter="16">
            <el-col :xs="24" :sm="12">
              <el-form-item label="收文编号">
                <el-input v-model="form.receive_no" placeholder="留空自动生成" />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12">
              <el-form-item label="收文日期">
                <el-date-picker 
                  v-model="form.received_date" 
                  type="date" 
                  value-format="YYYY-MM-DD"
                  style="width: 100%" 
                  placeholder="选择收文日期" 
                />
              </el-form-item>
            </el-col>
          </el-row>

          <el-form-item label="文件标题" required>
            <el-input v-model="form.title" placeholder="请输入完整文件标题" />
          </el-form-item>

          <el-row :gutter="16">
            <el-col :xs="24" :sm="12">
              <el-form-item label="来文单位" required>
                <el-input v-model="form.from_unit" placeholder="如：伊犁州党委宣传部" />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12">
              <el-form-item label="来文字号">
                <el-input v-model="form.from_doc_no" placeholder="如：州宣发〔2026〕5号" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="16">
            <el-col :xs="24" :sm="12">
              <el-form-item label="内部编号">
                <el-input v-model="form.doc_no" placeholder="如：XCB-2026-001" />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12">
              <el-form-item label="印制份数">
                <el-input-number v-model="form.copies" :min="1" :max="999" style="width: 100%" />
              </el-form-item>
            </el-col>
          </el-row>
        </div>

        <!-- 分组 2：密级与清退管理 -->
        <div class="form-section">
          <div class="form-section-title">密级与清退要求</div>
          <el-row :gutter="16">
            <el-col :xs="24" :sm="12">
              <el-form-item label="公文密级">
                <el-select v-model="form.secret_level" placeholder="请选择密级" style="width: 100%">
                  <el-option v-for="s in secretLevels" :key="s" :label="s" :value="s" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12">
              <el-form-item label="紧急程度">
                <el-select v-model="form.urgency" style="width: 100%">
                  <el-option label="一般" value="一般" />
                  <el-option label="平件" value="平件" />
                  <el-option label="急件" value="急件" />
                  <el-option label="特急" value="特急" />
                </el-select>
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="16">
            <el-col :xs="24" :sm="12">
              <el-form-item label="是否需退回">
                <el-radio-group v-model="form.need_return">
                  <el-radio :label="1">需要退回</el-radio>
                  <el-radio :label="0">不需退回</el-radio>
                </el-radio-group>
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12">
              <el-form-item label="清退状态">
                <el-radio-group v-model="form.returned" :disabled="form.need_return !== 1">
                  <el-radio :label="1">已清退</el-radio>
                  <el-radio :label="0">未退</el-radio>
                </el-radio-group>
              </el-form-item>
            </el-col>
          </el-row>

          <el-form-item label="退回日期" v-if="form.returned === 1">
            <el-date-picker 
              v-model="form.return_date" 
              type="date" 
              value-format="YYYY-MM-DD"
              style="width: 100%" 
              placeholder="请选择清退归档日期" 
            />
          </el-form-item>
        </div>

        <!-- 分组 3：拟办意见与领导批示 -->
        <div class="form-section">
          <div class="form-section-title">批示、承办与归档</div>
          <el-row :gutter="16">
            <el-col :xs="24" :sm="12">
              <el-form-item label="办理状态">
                <el-select v-model="form.status" style="width: 100%">
                  <el-option v-for="(name, val) in statusNames" :key="val" :label="name" :value="Number(val)" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12">
              <el-form-item label="承办科室">
                <el-select
                  v-model="form.assigned_department"
                  placeholder="请选择或输入承办科室"
                  filterable
                  allow-create
                  clearable
                  style="width: 100%"
                >
                  <el-option
                    v-for="dept in departments"
                    :key="dept.id"
                    :label="dept.name"
                    :value="dept.name"
                  />
                </el-select>
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="16">
            <el-col :xs="24" :sm="14">
              <el-form-item label="归档盒号">
                <el-input v-model="form.archive_box_no" placeholder="如：2026-宣-01盒" />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="10">
              <el-form-item label="归档年度">
                <el-input v-model="form.archive_year" placeholder="如：2026" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-form-item label="流转备注">
            <el-input v-model="form.handling_remarks" placeholder="录入批示或办理流转备注说明" />
          </el-form-item>

          <el-form-item label="拟办意见">
            <el-input 
              v-model="form.suggest" 
              type="textarea" 
              :rows="2" 
              placeholder="办公室拟办意见（如：拟请部长阅示，转办公室抓好落实）" 
            />
          </el-form-item>

          <el-form-item label="领导批示">
            <el-input 
              v-model="form.leader_comment" 
              type="textarea" 
              :rows="3" 
              placeholder="部领导或分管领导批示内容" 
            />
          </el-form-item>

          <el-form-item label="办理情况">
            <el-input 
              v-model="form.processing" 
              type="textarea" 
              :rows="2" 
              placeholder="责任科室办理成效与落实情况" 
            />
          </el-form-item>
        </div>
      </el-form>

      <template #footer>
        <div class="drawer-footer-wrap">
          <el-button @click="drawerVisible = false">取消</el-button>
          <el-button type="primary" @click="save">保存公文</el-button>
        </div>
      </template>
    </el-drawer>

    <!-- 2. 收文详情与传阅：右侧滑出抽屉（Drawer） -->
    <el-drawer 
      v-model="detailVisible" 
      :title="`收文详情 - ${detail.title || ''}`" 
      size="760px"
      destroy-on-close
    >
      <div class="detail-drawer-body">
        <el-tabs v-model="detailTab">
          <el-tab-pane label="基础信息" name="info">
            <el-descriptions :column="isMobile ? 1 : 2" border>
              <el-descriptions-item label="收文编号">{{ detail.receive_no || '—' }}</el-descriptions-item>
              <el-descriptions-item label="收文日期">{{ detail.received_date || '—' }}</el-descriptions-item>
              <el-descriptions-item label="来文单位">{{ detail.from_unit || '—' }}</el-descriptions-item>
              <el-descriptions-item label="来文字号">{{ detail.from_doc_no || '—' }}</el-descriptions-item>
              <el-descriptions-item label="文件编号">{{ detail.doc_no || '—' }}</el-descriptions-item>
              <el-descriptions-item label="密级">{{ detail.secret_level }}</el-descriptions-item>
              <el-descriptions-item label="紧急程度">{{ detail.urgency }}</el-descriptions-item>
              <el-descriptions-item label="办理状态">
                <el-tag :class="['status-tag', 'status-doc-' + (detail.status || 1)]" effect="plain">
                  {{ statusNames[detail.status] || '待登记' }}
                </el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="承办科室">{{ detail.assigned_department || '—' }}</el-descriptions-item>
              <el-descriptions-item label="归档盒号">{{ detail.archive_box_no || '—' }}</el-descriptions-item>
              <el-descriptions-item label="归档年度">{{ detail.archive_year || '—' }}</el-descriptions-item>
              <el-descriptions-item label="流转备注" :span="2">{{ detail.handling_remarks || '—' }}</el-descriptions-item>
              <el-descriptions-item label="份数">{{ detail.copies }}</el-descriptions-item>
              <el-descriptions-item label="需退回">{{ detail.need_return === 1 ? '需要' : '不需要' }}</el-descriptions-item>
              <el-descriptions-item label="是否已退">{{ detail.returned === 1 ? '已退' : '未退' }}</el-descriptions-item>
              <el-descriptions-item label="退回日期">{{ detail.return_date || '—' }}</el-descriptions-item>
              <el-descriptions-item label="登记人">{{ detail.registrar_name }}</el-descriptions-item>
              <el-descriptions-item label="拟办意见" :span="2">{{ detail.suggest || '—' }}</el-descriptions-item>
              <el-descriptions-item label="领导批示" :span="2">{{ detail.leader_comment || '—' }}</el-descriptions-item>
              <el-descriptions-item label="办理情况" :span="2">{{ detail.processing || '—' }}</el-descriptions-item>
            </el-descriptions>

            <div class="mt-20 print-btn-group">
              <el-button 
                v-if="authStore.hasPerm('incoming.manage')" 
                type="success" 
                :icon="'Promotion'" 
                @click="openStatusDialog(detail)"
              >
                办理/归档
              </el-button>
              <el-button type="primary" :icon="'Printer'" @click="openPrint(detail)">打印呈批单</el-button>
              <el-button type="warning" :icon="'Tickets'" @click="openPrintCard(detail)">打印传阅登记卡</el-button>
              <el-button type="info" :icon="'PriceTag'" @click="openLabel(detail)">打印文件标签</el-button>
            </div>
          </el-tab-pane>

          <el-tab-pane v-if="authStore.hasPerm('incoming.circulation')" label="传阅登记" name="circ">
            <div class="circ-toolbar">
              <el-select v-model="circUser" filterable placeholder="选择传阅人" style="width: 260px">
                <el-option v-for="a in assignees" :key="a.id" :label="a.real_name + (a.department ? ' (' + a.department + ')' : '')" :value="a.id" />
              </el-select>
              <el-button type="primary" :icon="'Plus'" @click="addCirc">添加传阅人</el-button>
            </div>

            <el-table size="large" :data="detail.circulations || []" class="mt-12">
              <el-table-column prop="order_no" label="序号" width="60" />
              <el-table-column prop="user_name" label="传阅人" width="120" />
              <el-table-column label="传阅日期" width="150">
                <template #default="{ row }">
                  <el-date-picker 
                    v-model="row.read_date" 
                    type="date" 
                    value-format="YYYY-MM-DD" 
                    size="small"
                    placeholder="选择日期" 
                    @change="updateCirc(row)" 
                  />
                </template>
              </el-table-column>
              <el-table-column label="签字">
                <template #default="{ row }">
                  <el-input v-model="row.signature" size="small" placeholder="签字确认" @change="updateCirc(row)" />
                </template>
              </el-table-column>
              <el-table-column label="操作" width="70" align="right">
                <template #default="{ row }">
                  <el-button link type="danger" size="small" @click="removeCirc(row)">移除</el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-tab-pane>
        </el-tabs>
      </div>

      <template #footer>
        <div class="drawer-footer-wrap">
          <el-button @click="detailVisible = false">关闭</el-button>
        </div>
      </template>
    </el-drawer>

    <!-- 3. 快捷办理状态与归档弹窗 -->
    <el-dialog 
      v-model="statusDialogVisible" 
      title="公文办理与归档" 
      :width="isMobile ? '92%' : '480px'"
      destroy-on-close
    >
      <el-form :model="statusForm" label-width="95px" size="default">
        <el-form-item label="公文标题">
          <div class="status-dialog-doc-title">{{ currentStatusDoc.title }}</div>
        </el-form-item>

        <el-form-item label="收文编号" v-if="currentStatusDoc.receive_no">
          <span style="font-family: monospace;">{{ currentStatusDoc.receive_no }}</span>
        </el-form-item>

        <el-form-item label="当前状态">
          <el-tag :class="['status-tag', 'status-doc-' + (currentStatusDoc.status || 1)]" effect="plain">
            {{ statusNames[currentStatusDoc.status] || '待登记' }}
          </el-tag>
        </el-form-item>

        <el-form-item label="办理状态" required>
          <el-select v-model="statusForm.status" style="width: 100%" placeholder="请选择目标办理状态">
            <el-option v-for="(name, val) in statusNames" :key="val" :label="name" :value="Number(val)" />
          </el-select>
        </el-form-item>

        <el-form-item label="承办科室">
          <el-select
            v-model="statusForm.assigned_department"
            placeholder="请选择或输入承办科室"
            filterable
            allow-create
            clearable
            style="width: 100%"
          >
            <el-option
              v-for="dept in departments"
              :key="dept.id"
              :label="dept.name"
              :value="dept.name"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="流转备注">
          <el-input type="textarea" v-model="statusForm.handling_remarks" rows="3" placeholder="录入领导批示意见或办理流转说明..." />
        </el-form-item>

        <el-form-item label="归档盒号">
          <el-input 
            v-model="statusForm.archive_box_no" 
            placeholder="如：2026-宣-01盒" 
            clearable
          />
          <div class="form-item-tip">用于实体卷宗盒归档定位与调阅，办结归档时建议填写入盒</div>
        </el-form-item>

        <el-form-item label="归档年度">
          <el-input 
            v-model="statusForm.archive_year" 
            placeholder="如：2026" 
            style="width: 160px" 
            clearable
          />
        </el-form-item>
      </el-form>

      <template #footer>
        <div class="drawer-footer-wrap">
          <el-button @click="statusDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="statusSubmitting" @click="submitStatusChange">确认更新</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import request, { exportFile } from '../utils/request'
import { useAuthStore } from '../store/auth'
import PageHeader from '../components/PageHeader.vue'
import StatusDot from '../components/StatusDot.vue'
import EmptyState from '../components/EmptyState.vue'
import { useMobile } from '../utils/useMobile'

const { isMobile } = useMobile()
const route = useRoute()
const authStore = useAuthStore()

const list = ref([])
const loading = ref(false)
const assignees = ref([])
const departments = ref([])
const page = ref(1)
const pageSize = 20
const total = ref(0)
const query = reactive({ 
  keyword: '', 
  status: '', 
  archive_box_no: '', 
  returned: '', 
  need_return: '' 
})
const dateRange = ref([])
const showAdvanced = ref(false)

const statusNames = { 1: '待登记', 2: '拟办中', 3: '待批示', 4: '办理中', 5: '已办结' }
const secretLevels = ['非密', '内部', '秘密★长期', '秘密★10年', '秘密★5年', '秘密★1年', '秘密', '秘密★1个月', '秘密★3个月', '秘密★6个月', '机密★长期', '机密', '机密★20年', '机密★10年', '机密★5年', '机密★3年', '机密★1年']
const secretTag = (s) => (s && s.startsWith('机密') ? 'danger' : s && s.startsWith('秘密') ? 'warning' : s === '内部' ? 'info' : 'info')

const drawerVisible = ref(false)
const detailVisible = ref(false)
const detailTab = ref('info')
const editId = ref(0)
const form = reactive({
  receive_no: '', received_date: '', from_unit: '', from_doc_no: '', doc_no: '', title: '',
  copies: 1, secret_level: '非密', urgency: '一般', suggest: '', leader_comment: '', processing: '',
  return_date: '', returned: 0, need_return: 0, status: 1,
  archive_box_no: '', archive_year: '', assigned_department: '', handling_remarks: ''
})
const detail = ref({})
const circUser = ref(null)

// 快捷办理状态与归档弹窗
const statusDialogVisible = ref(false)
const statusSubmitting = ref(false)
const currentStatusDoc = ref({})
const statusForm = reactive({
  status: 1,
  archive_box_no: '',
  archive_year: '',
  assigned_department: '',
  handling_remarks: ''
})

const openStatusDialog = (row) => {
  currentStatusDoc.value = row
  statusForm.status = row.status || 1
  statusForm.archive_box_no = row.archive_box_no || ''
  statusForm.archive_year = row.archive_year || (row.received_date ? row.received_date.slice(0, 4) : new Date().getFullYear().toString())
  statusForm.assigned_department = row.assigned_department || ''
  statusForm.handling_remarks = row.handling_remarks || ''
  statusDialogVisible.value = true
}

const submitStatusChange = async () => {
  if (!statusForm.status) {
    return ElMessage.warning('请选择办理状态')
  }
  statusSubmitting.value = true
  try {
    await request.post(`/incoming-docs/${currentStatusDoc.value.id}/status`, {
      status: statusForm.status,
      archive_box_no: statusForm.archive_box_no,
      archive_year: statusForm.archive_year,
      assigned_department: statusForm.assigned_department,
      handling_remarks: statusForm.handling_remarks
    })
    ElMessage.success('公文办理状态已更新')
    statusDialogVisible.value = false
    loadData()
    if (detailVisible.value && detail.value.id === currentStatusDoc.value.id) {
      openDetail(currentStatusDoc.value)
    }
    emitIncomingChanged()
  } catch (e) { console.error(e) } finally {
    statusSubmitting.value = false
  }
}

const hasActiveFilters = computed(() => {
  return query.keyword || query.status !== '' || query.archive_box_no || query.returned !== '' || query.need_return !== '' || (dateRange.value && dateRange.value.length > 0)
})

const resetQuery = () => {
  query.keyword = ''
  query.status = ''
  query.archive_box_no = ''
  query.returned = ''
  query.need_return = ''
  dateRange.value = []
  reloadFirstPage()
}

const loadData = async () => {
  loading.value = true
  try {
    const params = { page: page.value, page_size: pageSize, ...query }
    if (dateRange.value && dateRange.value.length === 2) {
      params.start_date = dateRange.value[0]
      params.end_date = dateRange.value[1]
    }
    const res = await request.get('/incoming-docs', { params })
    list.value = res.list || []
    total.value = res.total || 0
  } catch (e) { console.error(e) } finally {
    loading.value = false
  }
}

const reloadFirstPage = () => {
  page.value = 1
  loadData()
}

const onPageChange = (p) => {
  page.value = p
  loadData()
}

const loadAssignees = async () => {
  try {
    const res = await request.get('/assignees')
    assignees.value = res.list || []
  } catch (e) { console.error(e) }
}

const loadDepartments = async () => {
  try {
    const res = await request.get('/departments')
    departments.value = res.list || []
  } catch (e) { console.error(e) }
}

const openCreate = () => {
  editId.value = 0
  const nowYear = new Date().getFullYear().toString()
  Object.assign(form, {
    receive_no: '', received_date: new Date().toISOString().slice(0, 10), from_unit: '',
    from_doc_no: '', doc_no: '', title: '',
    copies: 1, secret_level: '非密', urgency: '一般', suggest: '', leader_comment: '', processing: '',
    return_date: '', returned: 0, need_return: 0, status: 1,
    archive_box_no: '', archive_year: nowYear, assigned_department: '', handling_remarks: ''
  })
  drawerVisible.value = true
}

const openEdit = (row) => {
  editId.value = row.id
  Object.assign(form, {
    receive_no: row.receive_no, received_date: row.received_date, from_unit: row.from_unit,
    from_doc_no: row.from_doc_no, doc_no: row.doc_no, title: row.title, copies: row.copies,
    secret_level: row.secret_level, urgency: row.urgency, suggest: row.suggest,
    leader_comment: row.leader_comment, processing: row.processing,
    return_date: row.return_date, returned: row.returned, need_return: row.need_return,
    status: row.status || 1,
    archive_box_no: row.archive_box_no || '',
    archive_year: row.archive_year || (row.received_date ? row.received_date.slice(0, 4) : ''),
    assigned_department: row.assigned_department || '',
    handling_remarks: row.handling_remarks || ''
  })
  drawerVisible.value = true
}

const save = async () => {
  if (!form.title) return ElMessage.warning('请输入文件标题')
  if (!form.from_unit) return ElMessage.warning('请输入来文单位')
  try {
    if (editId.value) {
      await request.put('/incoming-docs', { ...form, id: editId.value })
      ElMessage.success('更新成功')
    } else {
      await request.post('/incoming-docs', form)
      ElMessage.success('登记成功')
    }
    drawerVisible.value = false
    loadData()
    emitIncomingChanged()
  } catch (e) { console.error(e) }
}

const emitIncomingChanged = () => {
  window.dispatchEvent(new Event('incoming-changed'))
}

const openDetail = async (row) => {
  try {
    const res = await request.get(`/incoming-docs/${row.id}`)
    detail.value = res
    detailTab.value = 'info'
    detailVisible.value = true
  } catch (e) { console.error(e) }
}

const openPrint = (row) => {
  sessionStorage.setItem('printDoc', JSON.stringify(row))
  const url = `/incoming/print/${row.id}`
  window.open(url, '_blank')
}

const openPrintCard = (row) => {
  sessionStorage.setItem('printCard', JSON.stringify(row))
  const url = `/incoming/print-card/${row.id}`
  window.open(url, '_blank')
}

const openLabel = (row) => {
  sessionStorage.setItem('printLabel', JSON.stringify(row))
  const url = `/incoming/label/${row.id}`
  window.open(url, '_blank')
}

const handleRowCommand = (cmd, row) => {
  if (cmd === 'status') openStatusDialog(row)
  else if (cmd === 'print') openPrint(row)
  else if (cmd === 'label') openLabel(row)
  else if (cmd === 'card') openPrintCard(row)
  else if (cmd === 'edit') openEdit(row)
  else if (cmd === 'delete') remove(row)
}

const remove = async (row) => {
  try {
    await ElMessageBox.confirm(`确认删除收文「${row.title}」？此操作不可恢复。`, '高危操作确认', {
      type: 'warning',
      confirmButtonText: '确认删除',
      confirmButtonClass: 'el-button--danger',
      cancelButtonText: '取消'
    })
    await request.delete(`/incoming-docs/${row.id}`)
    ElMessage.success('已删除')
    loadData()
    emitIncomingChanged()
  } catch (e) { console.error(e) }
}

const addCirc = async () => {
  if (!circUser.value) return ElMessage.warning('请选择传阅人')
  try {
    await request.post('/incoming-circulations', { incoming_id: detail.value.id, user_id: circUser.value })
    circUser.value = null
    openDetail(detail.value)
  } catch (e) { console.error(e) }
}

const updateCirc = async (row) => {
  try {
    await request.put('/incoming-circulations', { id: row.id, read_date: row.read_date, signature: row.signature })
  } catch (e) { console.error(e) }
}

const removeCirc = async (row) => {
  try {
    await request.delete(`/incoming-circulations/${row.id}`)
    openDetail(detail.value)
  } catch (e) { console.error(e) }
}

const exportData = async () => {
  try {
    const params = { ...query }
    if (dateRange.value && dateRange.value.length === 2) {
      params.start_date = dateRange.value[0]
      params.end_date = dateRange.value[1]
    }
    await exportFile('/export/incoming-docs', params, '收文登记台账.xlsx')
  } catch (e) { console.error(e) }
}

onMounted(() => {
  loadData()
  loadAssignees()
  loadDepartments()
  if (route.query.id) {
    openDetail({ id: route.query.id })
  }
})

watch(() => route.query.id, (newId) => {
  if (newId) {
    openDetail({ id: newId })
  }
})
</script>

<style scoped>
.filter-bar {
  display: flex;
  flex-direction: column;
}

.filter-main-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.advanced-toggle-btn {
  font-size: var(--yx-fs-sm);
  color: var(--yx-text-2);
}

.custom-doc-table :deep(.el-table__row) {
  cursor: pointer;
}

.table-doc-title {
  color: var(--yx-text-1);
  font-weight: 500;
  transition: color 0.2s ease;
}

.table-doc-title:hover {
  color: var(--yx-ink-2);
}

.row-action-wrap {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  white-space: nowrap;
}

.more-link {
  font-size: 13px;
  color: var(--yx-text-3);
  white-space: nowrap;
  flex-shrink: 0;
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

.print-btn-group {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.circ-toolbar {
  display: flex;
  gap: 12px;
  align-items: center;
  margin-bottom: 12px;
}

.mt-16 {
  margin-top: 16px;
}
.mt-12 {
  margin-top: 12px;
}
.mt-20 {
  margin-top: 20px;
}

.status-tag {
  font-weight: 500;
  border-radius: 4px;
}
.status-tag.status-doc-1 {
  color: #595959;
  background: #f5f5f5;
  border-color: #d9d9d9;
}
.status-tag.status-doc-2 {
  color: #d46b08;
  background: #fff7e6;
  border-color: #ffd591;
}
.status-tag.status-doc-3 {
  color: #096dd9;
  background: #e6f7ff;
  border-color: #91d5ff;
}
.status-tag.status-doc-4 {
  color: #722ed1;
  background: #f9f0ff;
  border-color: #d3adf7;
}
.status-tag.status-doc-5 {
  color: #389e0d;
  background: #f6ffed;
  border-color: #b7eb8f;
}

.box-no-badge {
  display: inline-flex;
  align-items: center;
  padding: 1px 6px;
  background-color: var(--yx-ink-bg-light, #f0f4f9);
  color: var(--yx-ink-2, #1d39c4);
  border: 1px solid rgba(29, 57, 196, 0.15);
  border-radius: 4px;
  font-size: 12px;
  font-family: monospace;
}

.form-item-tip {
  font-size: 12px;
  color: var(--yx-text-4, #8c8c8c);
  margin-top: 4px;
  line-height: 1.4;
}

.status-dialog-doc-title {
  font-weight: 500;
  color: var(--yx-text-1);
  line-height: 1.5;
  word-break: break-all;
}
</style>
