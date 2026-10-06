<template>
  <el-config-provider :locale="locale">
    <router-view />
  </el-config-provider>
</template>

<script setup>
import { ref } from 'vue'
import zhCn from 'element-plus/es/locale/lang/zh-cn'

const locale = ref(zhCn)
</script>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}
html, body, #app {
  height: 100%;
  font-family: "Microsoft YaHei", "PingFang SC", "Helvetica Neue", Arial, sans-serif;
}

/* ===== 全局移动端与小屏响应式适配 ===== */
@media (max-width: 767px) {
  /* 弹窗与抽屉全宽适配 */
  .el-dialog {
    width: 94% !important;
    max-width: 94vw !important;
    margin: 4vh auto !important;
    border-radius: var(--yx-radius, 8px) !important;
  }
  .el-dialog__body {
    padding: 16px !important;
  }
  .el-message-box {
    width: 90% !important;
    max-width: 90vw !important;
  }

  .el-drawer,
  .el-drawer.rtl,
  .el-drawer.ltr {
    width: 100% !important;
    max-width: 100vw !important;
  }
  .el-drawer__header {
    padding: 14px 16px !important;
    margin-bottom: 0 !important;
  }
  .el-drawer__body {
    padding: 14px 16px !important;
    overflow-x: hidden;
  }
  .el-drawer__footer {
    padding: 12px 16px !important;
  }

  /* 抽屉与弹窗内的表单标签自动顶对齐，避免挤压输入框 */
  .el-drawer .el-form-item,
  .el-dialog .el-form-item {
    display: flex;
    flex-direction: column;
    align-items: stretch;
    margin-bottom: 14px;
  }
  .el-drawer .el-form-item__label,
  .el-dialog .el-form-item__label {
    float: none !important;
    text-align: left !important;
    padding: 0 0 4px 0 !important;
    line-height: 1.4 !important;
    width: auto !important;
  }
  .el-drawer .el-form-item__content,
  .el-dialog .el-form-item__content {
    margin-left: 0 !important;
    width: 100% !important;
  }

  /* 输入控件在移动端防 iOS 聚焦自动缩放 */
  input[type="text"],
  input[type="password"],
  input[type="number"],
  input[type="tel"],
  textarea,
  select {
    font-size: 16px;
  }

  /* 卡片与主体容器边距微调 */
  .el-main {
    padding: 10px 8px !important;
  }
  .el-card {
    border-radius: 8px;
    max-width: 100%;
  }
  .el-card__header {
    padding: 12px 14px !important;
  }
  .el-card__body {
    padding: 14px 12px !important;
    max-width: 100%;
    min-width: 0;
  }

  /* 表格容器弹性滚动 */
  .el-table {
    max-width: 100%;
  }
  .el-table__inner-wrapper {
    max-width: 100%;
  }

  /* 分页条移动端居中并折行 */
  .el-pagination {
    display: flex;
    flex-wrap: wrap;
    justify-content: center !important;
    gap: 6px;
    margin-top: 14px;
  }
  .el-pagination .el-pagination__sizes,
  .el-pagination .el-pagination__jump {
    display: none !important;
  }
}
</style>
