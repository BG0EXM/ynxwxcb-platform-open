<template>
  <router-view />
</template>

<script setup>
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

/* ===== 全局响应式适配 ===== */
/* 小屏：表格横向滚动、工具栏换行、对话框全宽 */
@media (max-width: 767px) {
  .el-dialog {
    width: 92% !important;
    margin: 8vh auto !important;
  }
  .el-message-box {
    width: 90% !important;
  }
  .el-table {
    display: block;
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
  }
  .el-table .el-table__body-wrapper,
  .el-table .el-table__header-wrapper {
    overflow: visible;
  }
  .el-main {
    padding: 10px !important;
  }
  .el-card {
    border-radius: 6px;
  }
  .el-pagination {
    justify-content: center;
    margin-top: 10px;
  }
  .el-form-item {
    display: block;
  }
  .el-form-item__label {
    float: none;
    text-align: left;
    padding: 0 0 4px;
    line-height: 1.5;
  }
  .el-form-item__content {
    margin-left: 0 !important;
  }
  .el-date-editor,
  .el-input,
  .el-select {
    max-width: 100%;
  }
  input, select, textarea, button {
    font-size: 16px !important;
  }
}
</style>
