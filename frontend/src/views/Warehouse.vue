<template>
  <div class="warehouse-page">
    <page-header title="库房物资" subtitle="库房门口张贴二维码，干部扫码登记后取用，库存自动扣减；借用物品归还时再次扫码登记">
      <template #actions>
        <el-button type="primary" :icon="'Plus'" @click="openItemCreate">新增物资</el-button>
        <el-button :icon="'Picture'" @click="openQr">门口二维码</el-button>
        <el-button :icon="'EditPen'" @click="openCheckout">补录领用</el-button>
        <el-button :icon="'Refresh'" circle @click="reloadTab" />
      </template>
    </page-header>

    <StatStrip :items="statItems" />

    <el-tabs v-model="tab" @tab-change="reloadTab">
      <!-- 1. 物资台账 -->
      <el-tab-pane label="物资台账" name="items">
        <el-card shadow="never" class="gov-card">
          <div class="toolbar">
            <div class="filter-left">
              <el-input v-model="itemQuery.keyword" placeholder="名称 / 规格 / 位置" clearable class="w-200" @change="loadItems" />
              <el-select v-model="itemQuery.category" placeholder="全部分类" clearable class="w-140" @change="loadItems">
                <el-option v-for="c in categories" :key="c" :label="c" :value="c" />
              </el-select>
              <el-select v-model="itemQuery.status" class="w-120" @change="loadItems">
                <el-option label="启用中" value="1" />
                <el-option label="已停用" value="0" />
                <el-option label="全部" value="" />
              </el-select>
              <el-checkbox v-model="itemQuery.low" @change="loadItems">只看低于预警</el-checkbox>
            </div>
            <el-button type="success" :icon="'Download'" @click="exportItems">导出台账</el-button>
          </div>

          <MobileCardList v-if="isMobile" class="mt-12" :data="items" :loading="itemLoading" empty-text="还没有物资，点“新增物资”录入">
            <template #title="{ row }">{{ row.name }}<span class="muted" v-if="row.spec">（{{ row.spec }}）</span></template>
            <template #tag="{ row }">
              <el-tag v-if="row.status !== 1" size="small" type="info">已停用</el-tag>
              <el-tag v-else-if="isLow(row)" size="small" type="danger" effect="plain">低于预警</el-tag>
            </template>
            <template #meta="{ row }">
              <span :class="{ 'low-num': isLow(row) }">库存 {{ row.stock }}{{ row.unit }}</span>
              <span v-if="row.borrowed">借出 {{ row.borrowed }}{{ row.unit }}</span>
              <span>{{ row.kind === 'returnable' ? '可归还' : '耗材' }}</span>
              <span v-if="row.location">{{ row.location }}</span>
            </template>
            <template #actions="{ row }">
              <el-button link type="primary" @click="openStock(row, 'in')">入库</el-button>
              <el-button link type="primary" @click="openStock(row, 'adjust')">盘点</el-button>
              <el-button link type="primary" @click="openItemEdit(row)">编辑</el-button>
              <el-button link type="primary" @click="viewItemRecords(row)">流水</el-button>
            </template>
          </MobileCardList>
          <el-table v-else :data="items" v-loading="itemLoading" class="mt-12" :row-class-name="itemRowClass">
            <template #empty><empty-state description="还没有物资，点“新增物资”录入" /></template>
            <el-table-column label="物资名称" min-width="200">
              <template #default="{ row }">
                <span class="item-name">{{ row.name }}</span>
                <span class="muted" v-if="row.spec">（{{ row.spec }}）</span>
              </template>
            </el-table-column>
            <el-table-column prop="category" label="分类" width="110" />
            <el-table-column label="类型" width="90">
              <template #default="{ row }">
                <el-tag size="small" :type="row.kind === 'returnable' ? 'warning' : 'info'" effect="plain">{{ row.kind === 'returnable' ? '可归还' : '耗材' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="库存" width="110" align="right">
              <template #default="{ row }">
                <span class="tabular-nums" :class="{ 'low-num': isLow(row) }">{{ row.stock }}</span> <span class="muted">{{ row.unit }}</span>
              </template>
            </el-table-column>
            <el-table-column label="借出未还" width="96" align="right">
              <template #default="{ row }"><span class="tabular-nums" v-if="row.borrowed">{{ row.borrowed }}</span><span v-else class="muted">—</span></template>
            </el-table-column>
            <el-table-column label="预警" width="80" align="right">
              <template #default="{ row }"><span class="muted tabular-nums">{{ row.warn_qty || '—' }}</span></template>
            </el-table-column>
            <el-table-column prop="location" label="存放位置" min-width="120" show-overflow-tooltip />
            <el-table-column label="状态" width="80">
              <template #default="{ row }">
                <el-tag size="small" :type="row.status === 1 ? 'success' : 'info'" effect="light">{{ row.status === 1 ? '启用' : '停用' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="230" fixed="right" align="right">
              <template #default="{ row }">
                <el-button link type="primary" @click="openStock(row, 'in')">入库</el-button>
                <el-button link type="primary" @click="openStock(row, 'adjust')">盘点</el-button>
                <el-button link type="primary" @click="openItemEdit(row)">编辑</el-button>
                <el-dropdown trigger="click" @command="(cmd) => itemCommand(cmd, row)">
                  <el-button link type="primary" class="ml-8">更多<el-icon class="el-icon--right"><ArrowDown /></el-icon></el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item command="records">查看流水</el-dropdown-item>
                      <el-dropdown-item command="toggle">{{ row.status === 1 ? '停用' : '启用' }}</el-dropdown-item>
                      <el-dropdown-item command="delete" class="text-danger-item" divided>删除</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- 2. 出入库流水 -->
      <el-tab-pane label="领用记录" name="records">
        <el-card shadow="never" class="gov-card">
          <div class="toolbar">
            <div class="filter-left">
              <el-date-picker v-model="recQuery.month" type="month" value-format="YYYY-MM" placeholder="全部月份" class="w-140" @change="reloadRecords" />
              <el-select v-model="recQuery.kind" placeholder="全部类型" clearable class="w-120" @change="reloadRecords">
                <el-option v-for="(label, k) in KIND_LABEL" :key="k" :label="label" :value="k" />
              </el-select>
              <el-select v-model="recQuery.department" placeholder="全部科室" clearable filterable allow-create class="w-160" @change="reloadRecords">
                <el-option v-for="d in deptNames" :key="d" :label="d" :value="d" />
              </el-select>
              <el-input v-model="recQuery.keyword" placeholder="姓名 / 物资 / 单号" clearable class="w-180" @change="reloadRecords" />
              <el-tag v-if="recQuery.item_id" closable @close="clearItemFilter">只看：{{ recItemName }}</el-tag>
            </div>
            <el-button type="success" :icon="'Download'" @click="exportRecords">导出流水</el-button>
          </div>

          <MobileCardList v-if="isMobile" class="mt-12" :data="records" :loading="recLoading" empty-text="暂无记录">
            <template #title="{ row }">{{ row.item_name }} <span :class="qtyClass(row)">{{ signed(row.qty) }}{{ row.unit }}</span></template>
            <template #tag="{ row }">
              <el-tag size="small" :type="KIND_TAG[row.kind]" effect="plain">{{ KIND_LABEL[row.kind] }}</el-tag>
              <el-tag v-if="row.voided" size="small" type="info" class="ml-4">已作废</el-tag>
            </template>
            <template #meta="{ row }">
              <span>{{ formatTime(row.created_at) }}</span>
              <span v-if="row.person_name">{{ row.person_name }} · {{ row.department }}</span>
              <span v-else>{{ row.operator }}</span>
              <span v-if="row.purpose || row.remark">{{ row.purpose || row.remark }}</span>
            </template>
            <template #actions="{ row }">
              <el-button v-if="canVoid(row)" link type="danger" @click="voidSlip(row)">作废此单</el-button>
            </template>
          </MobileCardList>
          <el-table v-else :data="records" v-loading="recLoading" class="mt-12" :row-class-name="recRowClass">
            <template #empty><empty-state description="暂无出入库记录" /></template>
            <el-table-column label="时间" width="150">
              <template #default="{ row }"><span class="tabular-nums">{{ formatTime(row.created_at) }}</span></template>
            </el-table-column>
            <el-table-column label="类型" width="100">
              <template #default="{ row }">
                <el-tag size="small" :type="KIND_TAG[row.kind]" effect="plain">{{ KIND_LABEL[row.kind] }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="物资" min-width="160">
              <template #default="{ row }">{{ row.item_name }}<span class="muted" v-if="row.spec">（{{ row.spec }}）</span></template>
            </el-table-column>
            <el-table-column label="库存变动" width="100" align="right">
              <template #default="{ row }"><span class="tabular-nums" :class="qtyClass(row)">{{ signed(row.qty) }}</span> <span class="muted">{{ row.unit }}</span></template>
            </el-table-column>
            <el-table-column label="结余" width="76" align="right">
              <template #default="{ row }"><span class="tabular-nums muted">{{ row.stock_after }}</span></template>
            </el-table-column>
            <el-table-column label="领用人" min-width="150">
              <template #default="{ row }">
                <template v-if="row.person_name">
                  {{ row.person_name }}<span class="muted"> · {{ row.department }}</span>
                  <el-tag v-if="row.external" size="small" type="warning" effect="plain" class="ml-4">名单外</el-tag>
                  <div class="muted small" v-if="row.external && row.phone">{{ row.phone }}</div>
                </template>
                <span v-else class="muted">—</span>
              </template>
            </el-table-column>
            <el-table-column label="用途 / 备注" min-width="160" show-overflow-tooltip>
              <template #default="{ row }">{{ [row.purpose, row.remark].filter(Boolean).join('；') || '—' }}</template>
            </el-table-column>
            <el-table-column label="单号 / 经办" width="160">
              <template #default="{ row }">
                <div class="tabular-nums small" v-if="row.slip_no">{{ row.slip_no }}</div>
                <div class="muted small">{{ row.source === 'admin' ? '后台补录 · ' : '' }}{{ row.operator }}</div>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="96" fixed="right" align="right">
              <template #default="{ row }">
                <el-button v-if="canVoid(row)" link type="danger" @click="voidSlip(row)">作废此单</el-button>
                <el-tooltip v-else-if="row.voided" :content="row.void_reason || '已作废'" placement="left">
                  <el-tag size="small" type="info">已作废</el-tag>
                </el-tooltip>
              </template>
            </el-table-column>
          </el-table>

          <div class="pagination-wrap" v-if="recTotal > 0">
            <el-pagination v-model:current-page="recPage" v-model:page-size="recPageSize" :page-sizes="[20, 50, 100]"
              :layout="isMobile ? 'total, prev, pager, next' : 'total, sizes, prev, pager, next, jumper'" :total="recTotal"
              @size-change="reloadRecords" @current-change="loadRecords" />
          </div>
        </el-card>
      </el-tab-pane>

      <!-- 3. 借出未还 -->
      <el-tab-pane :label="summary.borrowed ? `借出未还（${summary.borrowed}）` : '借出未还'" name="borrows">
        <el-card shadow="never" class="gov-card">
          <MobileCardList v-if="isMobile" :data="borrows" :loading="borrowLoading" empty-text="没有借出未还的物品">
            <template #title="{ row }">{{ row.item_name }} × {{ row.outstanding }}{{ row.unit }}</template>
            <template #tag="{ row }"><el-tag size="small" :type="row.days >= 30 ? 'danger' : 'warning'" effect="plain">已借 {{ row.days || 0 }} 天</el-tag></template>
            <template #meta="{ row }">
              <span>{{ row.person_name }} · {{ row.department }}</span>
              <span>{{ formatTime(row.created_at) }}</span>
            </template>
            <template #actions="{ row }">
              <el-button link type="primary" @click="adminReturn(row)">登记归还</el-button>
            </template>
          </MobileCardList>
          <el-table v-else :data="borrows" v-loading="borrowLoading">
            <template #empty><empty-state description="没有借出未还的物品" /></template>
            <el-table-column label="物品" min-width="180">
              <template #default="{ row }">{{ row.item_name }}<span class="muted" v-if="row.spec">（{{ row.spec }}）</span></template>
            </el-table-column>
            <el-table-column label="未还 / 借出" width="110" align="right">
              <template #default="{ row }"><span class="tabular-nums">{{ row.outstanding }} / {{ row.borrowed }}</span> <span class="muted">{{ row.unit }}</span></template>
            </el-table-column>
            <el-table-column label="借用人" min-width="160">
              <template #default="{ row }">
                {{ row.person_name }}<span class="muted"> · {{ row.department }}</span>
                <el-tag v-if="row.external" size="small" type="warning" effect="plain" class="ml-4">名单外</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="借出时间" width="150">
              <template #default="{ row }"><span class="tabular-nums">{{ formatTime(row.created_at) }}</span></template>
            </el-table-column>
            <el-table-column label="已借天数" width="96" align="right">
              <template #default="{ row }"><span class="tabular-nums" :class="{ 'low-num': row.days >= 30 }">{{ row.days || 0 }} 天</span></template>
            </el-table-column>
            <el-table-column prop="slip_no" label="领用单号" width="140" />
            <el-table-column label="操作" width="100" fixed="right" align="right">
              <template #default="{ row }"><el-button link type="primary" @click="adminReturn(row)">登记归还</el-button></template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- 4. 统计 -->
      <el-tab-pane label="领用统计" name="stats">
        <el-card shadow="never" class="gov-card">
          <div class="toolbar">
            <div class="filter-left">
              <el-date-picker v-model="statRange" type="daterange" value-format="YYYY-MM-DD" range-separator="至"
                start-placeholder="开始日期" end-placeholder="结束日期" :clearable="false" @change="loadStats" />
              <el-button-group>
                <el-button @click="setStatRange('month')">本月</el-button>
                <el-button @click="setStatRange('quarter')">本季度</el-button>
                <el-button @click="setStatRange('year')">本年</el-button>
              </el-button-group>
            </div>
            <el-button type="success" :icon="'Download'" @click="exportStats">导出科室汇总</el-button>
          </div>
          <div class="muted small mt-12">统计口径：扫码与补录的领用、借用数量，已作废的领用单不计入。</div>
          <el-row :gutter="16" class="mt-12" v-loading="statLoading">
            <el-col :xs="24" :md="11">
              <div class="stat-title">按科室</div>
              <el-table :data="stats.by_department" size="small">
                <template #empty><empty-state description="该时段没有领用" /></template>
                <el-table-column prop="department" label="科室 / 单位" min-width="120" />
                <el-table-column prop="times" label="领用次数" width="86" align="right" />
                <el-table-column prop="out_qty" label="领用件数" width="86" align="right" />
                <el-table-column prop="borrow_qty" label="借用件数" width="86" align="right" />
              </el-table>
            </el-col>
            <el-col :xs="24" :md="13">
              <div class="stat-title">按物资</div>
              <el-table :data="stats.by_item" size="small">
                <template #empty><empty-state description="该时段没有领用" /></template>
                <el-table-column label="物资" min-width="140">
                  <template #default="{ row }">{{ row.item_name }}<span class="muted" v-if="row.spec">（{{ row.spec }}）</span></template>
                </el-table-column>
                <el-table-column label="领用" width="90" align="right">
                  <template #default="{ row }">{{ row.out_qty }} <span class="muted">{{ row.unit }}</span></template>
                </el-table-column>
                <el-table-column label="借用" width="90" align="right">
                  <template #default="{ row }">{{ row.borrow_qty }} <span class="muted">{{ row.unit }}</span></template>
                </el-table-column>
                <el-table-column prop="times" label="次数" width="70" align="right" />
              </el-table>
            </el-col>
          </el-row>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- 新增 / 编辑物资 -->
    <el-drawer v-model="itemVisible" :title="itemForm.id ? '编辑物资' : '新增物资'" :size="isMobile ? '100%' : '520px'" destroy-on-close>
      <el-form ref="itemFormRef" :model="itemForm" :rules="itemRules" label-width="96px">
        <el-form-item label="物资名称" prop="name"><el-input v-model="itemForm.name" maxlength="50" placeholder="如：A4 复印纸" /></el-form-item>
        <el-form-item label="规格型号"><el-input v-model="itemForm.spec" maxlength="50" placeholder="如：70g 500张/包（选填）" /></el-form-item>
        <el-form-item label="分类">
          <el-select v-model="itemForm.category" filterable allow-create clearable placeholder="选择或直接输入新分类" class="full">
            <el-option v-for="c in categoryPresets" :key="c" :label="c" :value="c" />
          </el-select>
        </el-form-item>
        <el-form-item label="计量单位" prop="unit"><el-input v-model="itemForm.unit" maxlength="10" placeholder="个 / 包 / 箱 / 支" class="w-140" /></el-form-item>
        <el-form-item label="物资类型" prop="kind">
          <el-radio-group v-model="itemForm.kind">
            <el-radio value="consumable">耗材（领了不还）</el-radio>
            <el-radio value="returnable">可归还（借用后归还）</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="!itemForm.id" label="初始库存"><el-input-number v-model="itemForm.stock" :min="0" :max="100000" /></el-form-item>
        <el-form-item label="预警数量">
          <el-input-number v-model="itemForm.warn_qty" :min="0" :max="100000" />
          <div class="form-tip">库存低于或等于此数时标红提醒补货，0 为不提醒</div>
        </el-form-item>
        <el-form-item label="存放位置"><el-input v-model="itemForm.location" maxlength="50" placeholder="如：2 号柜第 3 层（选填）" /></el-form-item>
        <el-form-item label="排序"><el-input-number v-model="itemForm.sort" :min="0" :max="9999" /><div class="form-tip">数字小的排在扫码页前面</div></el-form-item>
        <el-form-item v-if="itemForm.id" label="状态">
          <el-switch v-model="itemForm.status" :active-value="1" :inactive-value="0" active-text="启用" inactive-text="停用（扫码页不再显示）" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="itemVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveItem">保存</el-button>
      </template>
    </el-drawer>

    <!-- 入库 / 盘点 -->
    <el-dialog v-model="stockVisible" :title="stockMode === 'in' ? '入库' : '盘点调整'" :width="isMobile ? '92%' : '440px'" destroy-on-close>
      <div class="stock-head">
        <b>{{ stockItem?.name }}</b><span class="muted" v-if="stockItem?.spec">（{{ stockItem.spec }}）</span>
        <div class="muted small">系统库存 {{ stockItem?.stock }}{{ stockItem?.unit }}<span v-if="stockItem?.borrowed">，另有 {{ stockItem.borrowed }}{{ stockItem.unit }} 借出未还（不在库房）</span></div>
      </div>
      <el-form ref="stockFormRef" :model="stockForm" :rules="stockRules" label-width="96px" class="mt-12">
        <el-form-item v-if="stockMode === 'in'" label="入库数量" prop="qty">
          <el-input-number v-model="stockForm.qty" :min="1" :max="100000" /> <span class="muted ml-8">{{ stockItem?.unit }}</span>
        </el-form-item>
        <el-form-item v-else label="实点数量" prop="actual">
          <el-input-number v-model="stockForm.actual" :min="0" :max="1000000" /> <span class="muted ml-8">{{ stockItem?.unit }}</span>
        </el-form-item>
        <el-form-item :label="stockMode === 'in' ? '备注' : '调整原因'" prop="remark">
          <el-input v-model="stockForm.remark" maxlength="100" :placeholder="stockMode === 'in' ? '如：10 月采购（选填）' : '如：月底盘点、领用未登记'" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="stockVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveStock">确定</el-button>
      </template>
    </el-dialog>

    <!-- 门口二维码 -->
    <el-dialog v-model="qrVisible" title="库房门口二维码" :width="isMobile ? '96%' : '520px'">
      <div class="qr-wrap" v-loading="qrLoading">
        <img v-if="qrDataUrl" :src="qrDataUrl" alt="库房扫码领用二维码" class="qr-img" />
        <div class="qr-url">
          <el-input :model-value="publicUrl" readonly>
            <template #append><el-button :icon="'CopyDocument'" @click="copyUrl">复制</el-button></template>
          </el-input>
        </div>
        <div class="qr-actions">
          <el-button type="primary" :icon="'Printer'" :disabled="!qrDataUrl" @click="printPoster">打印张贴海报</el-button>
          <el-button type="danger" plain :icon="'Refresh'" @click="resetToken">重置二维码</el-button>
        </div>
        <div class="muted small">二维码照片外传或怀疑被滥用时点“重置二维码”，旧码立即失效，需重新打印张贴。</div>
      </div>
      <el-divider />
      <el-form label-width="96px">
        <el-form-item label="扫码领用">
          <el-switch v-model="qrSettings.open" active-text="开放" inactive-text="暂停（扫码后提示联系办公室）" />
        </el-form-item>
        <el-form-item label="页面提示语">
          <el-input v-model="qrSettings.notice" type="textarea" :rows="2" maxlength="200" show-word-limit placeholder="显示在扫码页顶部，如：大宗物资请提前联系办公室（选填）" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="qrVisible = false">关闭</el-button>
        <el-button type="primary" plain :loading="saving" @click="saveQrSettings">保存设置</el-button>
      </template>
    </el-dialog>

    <!-- 后台补录领用 -->
    <el-drawer v-model="coVisible" title="补录领用" :size="isMobile ? '100%' : '600px'" destroy-on-close>
      <el-alert type="info" :closable="false" show-icon class="mb-12" title="用于干部未扫码就取走物资、或手机不便时由办公室代为登记，提交后同样扣减库存。" />
      <el-form ref="coFormRef" :model="coForm" :rules="coRules" label-width="96px">
        <el-form-item label="领用人" prop="person">
          <el-cascader v-model="coForm.person" :options="personOptions" placeholder="选择科室与姓名" filterable class="full" />
        </el-form-item>
        <template v-if="coForm.person && coForm.person[0] === EXTERNAL">
          <el-form-item label="姓名" prop="name"><el-input v-model="coForm.name" maxlength="20" /></el-form-item>
          <el-form-item label="单位" prop="unit"><el-input v-model="coForm.unit" maxlength="50" /></el-form-item>
          <el-form-item label="联系电话" prop="phone"><el-input v-model="coForm.phone" maxlength="20" /></el-form-item>
        </template>
        <el-form-item label="物资" required>
          <div class="co-lines">
            <div class="co-line" v-for="(l, i) in coForm.lines" :key="i">
              <el-select v-model="l.item_id" filterable placeholder="选择物资" class="co-item">
                <el-option v-for="it in activeItems" :key="it.id" :value="it.id" :disabled="it.stock <= 0"
                  :label="`${it.name}${it.spec ? '（' + it.spec + '）' : ''} · 库存${it.stock}${it.unit}`" />
              </el-select>
              <el-input-number v-model="l.qty" :min="1" :max="999" class="co-qty" />
              <el-button :icon="'Delete'" circle text @click="coForm.lines.splice(i, 1)" :disabled="coForm.lines.length === 1" />
            </div>
            <el-button link type="primary" :icon="'Plus'" @click="coForm.lines.push({ item_id: null, qty: 1 })">再加一样</el-button>
          </div>
        </el-form-item>
        <el-form-item label="用途"><el-input v-model="coForm.purpose" maxlength="100" placeholder="选填" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="coVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveCheckout">提交登记</el-button>
      </template>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowDown } from '@element-plus/icons-vue'
import dayjs from 'dayjs'
import request, { exportFile } from '../utils/request'
import PageHeader from '../components/PageHeader.vue'
import StatStrip from '../components/StatStrip.vue'
import MobileCardList from '../components/MobileCardList.vue'
import EmptyState from '../components/EmptyState.vue'
import { useMobile } from '../utils/useMobile'

const { isMobile } = useMobile()
const EXTERNAL = '__external__'
const KIND_LABEL = { out: '领用', borrow: '借用', return: '归还', in: '入库', adjust: '盘点调整', void: '作废冲回' }
const KIND_TAG = { out: 'primary', borrow: 'warning', return: 'success', in: 'success', adjust: 'info', void: 'info' }
const PRESET_CATEGORIES = ['办公耗材', '纸张文具', '打印耗材', '电器电池', '茶水接待', '宣传物料', '摄录设备', '会务用品', '清洁用品']

const tab = ref('items')
const summary = ref({})
const statItems = computed(() => [
  { label: '物资品种', value: summary.value.items, unit: '种', onClick: () => showItems(false) },
  { label: '低于预警', value: summary.value.low, unit: '种', tone: summary.value.low ? 'danger' : '', onClick: () => showItems(true) },
  { label: '本月领用', value: summary.value.month_slips, unit: '单', tone: 'primary', onClick: showMonthRecords },
  { label: '借出未还', value: summary.value.borrowed, unit: '件', tone: summary.value.borrowed ? 'warning' : '', onClick: () => { tab.value = 'borrows'; loadBorrows() } }
])

const formatTime = (v) => (v ? dayjs(v).format('YYYY-MM-DD HH:mm') : '')
const signed = (n) => (n > 0 ? `+${n}` : `${n}`)
const qtyClass = (row) => (row.qty > 0 ? 'qty-in' : 'qty-out')
const isLow = (row) => row.status === 1 && row.warn_qty > 0 && row.stock <= row.warn_qty

// ---------- 物资台账 ----------
const items = ref([])
const categories = ref([])
const itemLoading = ref(false)
const itemQuery = reactive({ keyword: '', category: '', status: '1', low: false })
const itemRowClass = ({ row }) => (row.status !== 1 ? 'row-disabled' : '')
const activeItems = computed(() => items.value.filter(i => i.status === 1))
const categoryPresets = computed(() => [...new Set([...categories.value, ...PRESET_CATEGORIES])])

const loadItems = async () => {
  itemLoading.value = true
  try {
    const res = await request.get('/warehouse/items', {
      params: { keyword: itemQuery.keyword, category: itemQuery.category, status: itemQuery.status, low: itemQuery.low ? 1 : '' }
    })
    items.value = res.list || []
    categories.value = res.categories || []
    summary.value = res.summary || {}
  } catch (e) {
    console.error(e)
  } finally {
    itemLoading.value = false
  }
}

const showItems = (low) => {
  tab.value = 'items'
  itemQuery.low = low
  itemQuery.status = '1'
  loadItems()
}

const itemVisible = ref(false)
const itemFormRef = ref(null)
const saving = ref(false)
const blankItem = () => ({ id: 0, name: '', spec: '', category: '', unit: '个', kind: 'consumable', stock: 0, warn_qty: 0, location: '', sort: 0, status: 1 })
const itemForm = ref(blankItem())
const itemRules = {
  name: [{ required: true, message: '请填写物资名称', trigger: 'blur' }],
  unit: [{ required: true, message: '请填写计量单位', trigger: 'blur' }],
  kind: [{ required: true, message: '请选择物资类型', trigger: 'change' }]
}

const openItemCreate = () => {
  itemForm.value = blankItem()
  itemVisible.value = true
}
const openItemEdit = (row) => {
  itemForm.value = { ...row }
  itemVisible.value = true
}
const saveItem = async () => {
  try {
    await itemFormRef.value.validate()
  } catch (e) {
    return
  }
  saving.value = true
  try {
    if (itemForm.value.id) {
      await request.put('/warehouse/items', itemForm.value)
    } else {
      await request.post('/warehouse/items', itemForm.value)
    }
    ElMessage.success('已保存')
    itemVisible.value = false
    loadItems()
  } catch (e) {
    console.error(e)
  } finally {
    saving.value = false
  }
}

const itemCommand = async (cmd, row) => {
  if (cmd === 'records') return viewItemRecords(row)
  if (cmd === 'toggle') {
    try {
      await request.put('/warehouse/items', { ...row, status: row.status === 1 ? 0 : 1 })
      ElMessage.success(row.status === 1 ? '已停用，扫码页不再显示' : '已启用')
      loadItems()
    } catch (e) {
      console.error(e)
    }
    return
  }
  if (cmd === 'delete') {
    try {
      await ElMessageBox.confirm(`确定删除「${row.name}」？只有从未出入库的物资可以删除，否则请停用。`, '删除物资', { type: 'warning', confirmButtonText: '删除' })
    } catch (e) {
      return
    }
    try {
      await request.delete(`/warehouse/items/${row.id}`)
      ElMessage.success('已删除')
      loadItems()
    } catch (e) {
      console.error(e)
    }
  }
}

// ---------- 入库 / 盘点 ----------
const stockVisible = ref(false)
const stockMode = ref('in')
const stockItem = ref(null)
const stockFormRef = ref(null)
const stockForm = reactive({ qty: 1, actual: 0, remark: '' })
const stockRules = computed(() => (stockMode.value === 'adjust'
  ? { remark: [{ required: true, message: '请填写调整原因', trigger: 'blur' }] }
  : {}))

const openStock = (row, mode) => {
  stockMode.value = mode
  stockItem.value = row
  stockForm.qty = 1
  stockForm.actual = row.stock
  stockForm.remark = ''
  stockVisible.value = true
}
const saveStock = async () => {
  try {
    await stockFormRef.value.validate()
  } catch (e) {
    return
  }
  saving.value = true
  try {
    const url = `/warehouse/items/${stockItem.value.id}/${stockMode.value === 'in' ? 'stock-in' : 'adjust'}`
    const res = await request.post(url, { qty: stockForm.qty, actual: stockForm.actual, remark: stockForm.remark })
    ElMessage.success(`已保存，现有库存 ${res.stock}${stockItem.value.unit}`)
    stockVisible.value = false
    loadItems()
  } catch (e) {
    console.error(e)
  } finally {
    saving.value = false
  }
}

// ---------- 领用记录 ----------
const records = ref([])
const recLoading = ref(false)
const recTotal = ref(0)
const recPage = ref(1)
const recPageSize = ref(20)
const recQuery = reactive({ month: '', kind: '', department: '', keyword: '', item_id: 0 })
const recItemName = ref('')
const recRowClass = ({ row }) => (row.voided ? 'row-disabled' : '')
const canVoid = (row) => row.slip_id && !row.voided && (row.kind === 'out' || row.kind === 'borrow')

const loadRecords = async () => {
  recLoading.value = true
  try {
    const res = await request.get('/warehouse/records', {
      params: { ...recQuery, item_id: recQuery.item_id || '', page: recPage.value, page_size: recPageSize.value }
    })
    records.value = res.list || []
    recTotal.value = res.total || 0
    summary.value = res.summary || summary.value
  } catch (e) {
    console.error(e)
  } finally {
    recLoading.value = false
  }
}
const reloadRecords = () => {
  recPage.value = 1
  loadRecords()
}
const viewItemRecords = (row) => {
  recQuery.item_id = row.id
  recItemName.value = row.name
  recQuery.month = ''
  tab.value = 'records'
  reloadRecords()
}
const clearItemFilter = () => {
  recQuery.item_id = 0
  reloadRecords()
}
const showMonthRecords = () => {
  Object.assign(recQuery, { month: dayjs().format('YYYY-MM'), kind: '', department: '', keyword: '', item_id: 0 })
  tab.value = 'records'
  reloadRecords()
}

const voidSlip = async (row) => {
  let reason
  try {
    const res = await ElMessageBox.prompt(
      `作废领用单 ${row.slip_no}（${row.person_name}）后，该单所有物资未归还的部分加回库存。请填写作废原因：`,
      '作废领用单',
      { confirmButtonText: '确认作废', cancelButtonText: '取消', inputPlaceholder: '如：填错数量、重复登记', inputValidator: v => (v && v.trim() ? true : '请填写作废原因') }
    )
    reason = res.value.trim()
  } catch (e) {
    return
  }
  try {
    await request.post(`/warehouse/slips/${row.slip_id}/void`, { reason })
    ElMessage.success('已作废，库存已加回')
    loadRecords()
  } catch (e) {
    console.error(e)
  }
}

// ---------- 借出未还 ----------
const borrows = ref([])
const borrowLoading = ref(false)
const loadBorrows = async () => {
  borrowLoading.value = true
  try {
    const res = await request.get('/warehouse/borrows')
    borrows.value = res.list || []
    summary.value = res.summary || summary.value
  } catch (e) {
    console.error(e)
  } finally {
    borrowLoading.value = false
  }
}
const adminReturn = async (row) => {
  let qty = row.outstanding
  try {
    if (row.outstanding > 1) {
      const res = await ElMessageBox.prompt(`${row.person_name} 借用的「${row.item_name}」未还 ${row.outstanding}${row.unit}，本次归还数量：`, '登记归还', {
        inputValue: String(row.outstanding), confirmButtonText: '确认归还', cancelButtonText: '取消',
        inputValidator: v => (/^\d+$/.test(v) && +v >= 1 && +v <= row.outstanding ? true : `请输入 1 到 ${row.outstanding} 之间的整数`)
      })
      qty = +res.value
    } else {
      await ElMessageBox.confirm(`确认 ${row.person_name} 借用的「${row.item_name}」已归还入库？`, '登记归还', { confirmButtonText: '确认归还', cancelButtonText: '取消' })
    }
  } catch (e) {
    return
  }
  try {
    await request.post(`/warehouse/borrows/${row.id}/return`, { qty })
    ElMessage.success('已登记归还')
    loadBorrows()
  } catch (e) {
    console.error(e)
  }
}

// ---------- 统计 ----------
const stats = ref({ by_department: [], by_item: [] })
const statLoading = ref(false)
const statRange = ref([dayjs().startOf('month').format('YYYY-MM-DD'), dayjs().format('YYYY-MM-DD')])
const setStatRange = (unit) => {
  const now = dayjs()
  const start = unit === 'quarter'
    ? now.month(Math.floor(now.month() / 3) * 3).startOf('month')
    : now.startOf(unit)
  statRange.value = [start.format('YYYY-MM-DD'), now.format('YYYY-MM-DD')]
  loadStats()
}
const loadStats = async () => {
  if (!statRange.value?.length) return
  statLoading.value = true
  try {
    const res = await request.get('/warehouse/stats', { params: { start: statRange.value[0], end: statRange.value[1] } })
    stats.value = { by_department: res.by_department || [], by_item: res.by_item || [] }
  } catch (e) {
    console.error(e)
  } finally {
    statLoading.value = false
  }
}

// ---------- 导出 ----------
const exportItems = () => exportFile('/export/warehouse/items', {}, `库房库存台账${dayjs().format('YYYYMMDD')}.xlsx`)
const exportRecords = () => exportFile('/export/warehouse/records', { ...recQuery, item_id: recQuery.item_id || '' }, '库房出入库流水.xlsx')
const exportStats = () => exportFile('/export/warehouse/stats', { start: statRange.value[0], end: statRange.value[1] },
  `库房科室领用汇总${statRange.value[0]}至${statRange.value[1]}.xlsx`)

// ---------- 门口二维码 ----------
const qrVisible = ref(false)
const qrLoading = ref(false)
const qrDataUrl = ref('')
const qrToken = ref('')
const qrSettings = reactive({ open: true, notice: '' })
const publicUrl = computed(() => (qrToken.value ? `${window.location.origin}/warehouse/${qrToken.value}` : ''))

const renderQr = async () => {
  // 二维码库只在打开此弹窗时按需加载，不计入首屏体积
  const QRCode = (await import('qrcode')).default
  qrDataUrl.value = await QRCode.toDataURL(publicUrl.value, { width: 640, margin: 1, errorCorrectionLevel: 'M' })
}

const openQr = async () => {
  qrVisible.value = true
  qrLoading.value = true
  try {
    const res = await request.get('/warehouse/settings')
    qrToken.value = res.public_token
    qrSettings.open = !res.paused
    qrSettings.notice = res.notice || ''
    await renderQr()
  } catch (e) {
    console.error(e)
    ElMessage.error('二维码生成失败')
  } finally {
    qrLoading.value = false
  }
}

const copyUrl = async () => {
  try {
    await navigator.clipboard.writeText(publicUrl.value)
    ElMessage.success('链接已复制')
  } catch (e) {
    console.error(e)
    ElMessage.warning('复制失败，请手动选中链接复制')
  }
}

const resetToken = async () => {
  try {
    await ElMessageBox.confirm('重置后门口现有二维码立即失效，需要重新打印张贴。确定重置？', '重置二维码', { type: 'warning', confirmButtonText: '确定重置' })
  } catch (e) {
    return
  }
  try {
    const res = await request.post('/warehouse/settings/reset-token')
    qrToken.value = res.public_token
    await renderQr()
    ElMessage.success('已重置，请重新打印张贴')
  } catch (e) {
    console.error(e)
  }
}

const saveQrSettings = async () => {
  saving.value = true
  try {
    await request.put('/warehouse/settings', { paused: !qrSettings.open, notice: qrSettings.notice })
    ElMessage.success('已保存')
  } catch (e) {
    console.error(e)
  } finally {
    saving.value = false
  }
}

// 张贴海报：新窗口排版后调用打印（A4 竖版）
const printPoster = () => {
  const w = window.open('', '_blank')
  if (!w) {
    ElMessage.warning('浏览器拦截了弹出窗口，请允许后重试')
    return
  }
  w.document.write(`<!doctype html><html><head><meta charset="utf-8"><title>库房物资领用登记</title>
<style>
@page { size: A4 portrait; margin: 18mm; }
body { margin: 0; font-family: "Songti SC", "SimSun", "STSong", serif; color: #1f2329; text-align: center; }
.bar { height: 6px; background: linear-gradient(90deg, #9e1b1e, #b8925a, #24406e); margin-bottom: 18mm; }
h1 { font-size: 34pt; letter-spacing: 6pt; margin: 0 0 6mm; }
.sub { font-size: 16pt; color: #4e5969; margin-bottom: 12mm; font-family: -apple-system, "Microsoft YaHei", sans-serif; }
img { width: 95mm; height: 95mm; }
.steps { margin: 12mm auto 0; width: 150mm; text-align: left; font-size: 14pt; line-height: 2; font-family: -apple-system, "Microsoft YaHei", sans-serif; }
.steps b { color: #24406e; }
.foot { margin-top: 16mm; font-size: 14pt; color: #4e5969; }
</style></head><body>
<div class="bar"></div>
<h1>库房物资领用登记</h1>
<div class="sub">取用物资前，请用手机扫码登记</div>
<img src="${qrDataUrl.value}" alt="二维码">
<div class="steps">
<div><b>①</b> 选择所在科室和本人姓名（首次选择后自动记住）</div>
<div><b>②</b> 选择物资和数量，提交后按登记数量取用</div>
<div><b>③</b> 相机、话筒等借用物品归还时，再扫此码点“归还”</div>
</div>
<div class="foot">中共伊宁县委宣传部办公室</div>
<script>window.onload = function () { setTimeout(function () { window.print() }, 200) }<\/script>
</body></html>`)
  w.document.close()
}

// ---------- 补录领用 ----------
const coVisible = ref(false)
const coFormRef = ref(null)
const people = ref([])
const coForm = reactive({ person: null, name: '', unit: '', phone: '', purpose: '', lines: [{ item_id: null, qty: 1 }] })
const coRules = {
  person: [{ required: true, message: '请选择领用人', trigger: 'change' }],
  name: [{ required: true, message: '请填写姓名', trigger: 'blur' }],
  unit: [{ required: true, message: '请填写单位', trigger: 'blur' }],
  phone: [
    { required: true, message: '请填写联系电话', trigger: 'blur' },
    { pattern: /^(1\d{10}|0\d{2,3}-?\d{7,8}(-\d{1,6})?)$/, message: '电话格式不正确', trigger: 'blur' }
  ]
}
const deptNames = computed(() => people.value.map(d => d.department))
const personOptions = computed(() => [
  ...people.value.map(d => ({ value: d.department, label: d.department, children: d.people.map(p => ({ value: p.id, label: p.name })) })),
  { value: EXTERNAL, label: '其他人员（名单外）', children: [{ value: 0, label: '手动填写' }] }
])

const loadPeople = async () => {
  try {
    const res = await request.get('/warehouse/people')
    people.value = res.departments || []
  } catch (e) {
    console.error(e)
  }
}

const openCheckout = async () => {
  Object.assign(coForm, { person: null, name: '', unit: '', phone: '', purpose: '', lines: [{ item_id: null, qty: 1 }] })
  coVisible.value = true
  if (!people.value.length) loadPeople()
  if (!items.value.length || itemQuery.status !== '1' || itemQuery.low || itemQuery.keyword || itemQuery.category) {
    Object.assign(itemQuery, { keyword: '', category: '', status: '1', low: false })
    loadItems()
  }
}

const saveCheckout = async () => {
  try {
    await coFormRef.value.validate()
  } catch (e) {
    return
  }
  const lines = coForm.lines.filter(l => l.item_id && l.qty > 0)
  if (!lines.length) {
    ElMessage.warning('请至少选择一样物资')
    return
  }
  const external = coForm.person[0] === EXTERNAL
  const payload = external
    ? { external: true, name: coForm.name, unit: coForm.unit, phone: coForm.phone }
    : { user_id: coForm.person[1] }
  saving.value = true
  try {
    const res = await request.post('/warehouse/checkout', { ...payload, purpose: coForm.purpose, lines })
    ElMessage.success(`已登记，领用单号 ${res.slip?.slip_no || ''}`)
    coVisible.value = false
    reloadTab()
  } catch (e) {
    console.error(e)
  } finally {
    saving.value = false
  }
}

const reloadTab = () => {
  if (tab.value === 'items') loadItems()
  else if (tab.value === 'records') loadRecords()
  else if (tab.value === 'borrows') loadBorrows()
  else loadStats()
}

// 工作台待办跳转：?tab=borrows 借出未还，?tab=items&low=1 只看低库存
const route = useRoute()
const applyRouteQuery = () => {
  const t = route.query.tab
  if (['items', 'records', 'borrows', 'stats'].includes(t)) tab.value = t
  if (tab.value === 'items' && route.query.low === '1') {
    Object.assign(itemQuery, { keyword: '', category: '', status: '1', low: true })
  }
}
watch(() => route.fullPath, () => {
  applyRouteQuery()
  reloadTab()
})

onMounted(() => {
  applyRouteQuery()
  if (tab.value !== 'items') reloadTab()
  loadItems()
  loadPeople()
})
</script>

<style scoped>
.warehouse-page {
  padding-bottom: 24px;
}
.gov-card {
  border-radius: var(--yx-radius);
  border: 1px solid var(--yx-border-soft);
}
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
  flex-wrap: wrap;
  gap: 10px;
}
.w-120 { width: 120px; }
.w-140 { width: 140px; }
.w-160 { width: 160px; }
.w-180 { width: 180px; }
.w-200 { width: 200px; }
.full { width: 100%; }
.mt-12 { margin-top: 12px; }
.mb-12 { margin-bottom: 12px; }
.ml-4 { margin-left: 4px; }
.ml-8 { margin-left: 8px; }
.muted { color: var(--yx-text-3); }
.small { font-size: var(--yx-fs-xs); }
.item-name { font-weight: 500; color: var(--yx-text-1); }
.low-num { color: var(--yx-brand); font-weight: 600; }
.qty-in { color: var(--el-color-success); font-weight: 600; }
.qty-out { color: var(--yx-ink-2); font-weight: 600; }
.pagination-wrap {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}
:deep(.row-disabled) td {
  color: var(--yx-text-4);
}
.stat-title {
  font-weight: 600;
  color: var(--yx-text-1);
  margin: 8px 0;
}
.form-tip {
  width: 100%;
  font-size: var(--yx-fs-xs);
  color: var(--yx-text-3);
  line-height: 1.6;
}
.stock-head {
  background: var(--yx-surface-muted);
  border: 1px solid var(--yx-border-soft);
  border-radius: var(--yx-radius-sm);
  padding: 10px 12px;
}

.qr-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  min-height: 120px;
}
.qr-img {
  width: 240px;
  height: 240px;
  border: 1px solid var(--yx-border-soft);
  border-radius: var(--yx-radius-sm);
}
.qr-url { width: 100%; }
.qr-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  justify-content: center;
}

.co-lines {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-items: flex-start;
}
.co-line {
  display: flex;
  gap: 8px;
  width: 100%;
  align-items: center;
}
.co-item { flex: 1; min-width: 0; }
.co-qty { width: 120px; flex: none; }

@media (max-width: 768px) {
  .filter-left > * { width: 100% !important; }
  .toolbar > .el-button { width: 100%; }
}
</style>
