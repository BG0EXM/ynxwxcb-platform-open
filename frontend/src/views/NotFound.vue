<template>
  <div class="not-found">
    <div class="code">404</div>
    <div class="title">您访问的页面不存在</div>
    <div class="desc">链接可能有误或页面已调整，请返回工作台继续办理。</div>
    <el-button type="primary" @click="goHome">返回工作台</el-button>
  </div>
</template>

<script setup>
import { useRouter } from 'vue-router'

const router = useRouter()
const goHome = () => {
  router.replace(localStorage.getItem('token') ? '/dashboard' : '/login')
}
</script>

<style scoped>
.not-found {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 0 16px;
  text-align: center;
  background: #f4f5f8;
}
.code {
  font-size: 64px;
  font-weight: 700;
  color: #b91c1c;
  line-height: 1;
}
.title {
  font-size: 18px;
  color: #1f2937;
}
.desc {
  font-size: 14px;
  color: #6b7280;
  margin-bottom: 8px;
}
</style>
