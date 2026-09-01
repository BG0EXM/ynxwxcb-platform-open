<template>
  <div class="login-page">
    <div class="login-box">
      <div class="login-header">
        <img src="../assets/danghui.png" alt="党徽" class="danghui-img" />
        <h1>伊宁县委宣传部</h1>
        <p>部务工作平台</p>
      </div>
      <el-form :model="form" :rules="rules" ref="formRef" size="large">
        <el-form-item prop="username">
          <el-input v-model="form.username" placeholder="请输入用户名" :prefix-icon="'User'" />
        </el-form-item>
        <el-form-item prop="password">
          <el-input v-model="form.password" type="password" placeholder="请输入密码"
            :prefix-icon="'Lock'" show-password @keyup.enter="handleLogin" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" class="login-btn" :loading="loading" @click="handleLogin">
            登 录
          </el-button>
        </el-form-item>
      </el-form>
      <p class="login-tip">伊宁县委宣传部部务工作平台 V1.3.9</p>
    </div>
    <div class="login-footer">
      <a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener">ICP备案号占位</a>
      <span class="footer-sep">|</span>
      <a href="http://www.beian.gov.cn/portal/registerSystemInfo?beian.miit.gov.cn" target="_blank" rel="noopener">
        公网安备号占位
      </a>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '../store/auth'

const router = useRouter()
const authStore = useAuthStore()

const formRef = ref()
const loading = ref(false)
const form = reactive({ username: '', password: '' })
const rules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
}

const handleLogin = async () => {
  await formRef.value.validate()
  loading.value = true
  try {
    const res = await authStore.login(form.username, form.password)
    if (res.must_change) {
      ElMessage.warning('您正在使用默认密码，请先修改密码')
      router.push('/profile')
    } else {
      ElMessage.success('登录成功')
      router.push('/dashboard')
    }
  } catch (e) {
    // 错误已由拦截器提示
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: url('../assets/login-bg.jpg');
  background-size: cover;
  background-position: center;
  position: relative;
}
.login-page::before {
  display: none;
}
.login-box {
  width: 400px;
  padding: 44px 40px;
  background: rgba(255, 255, 255, 0.92);
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: blur(14px);
  border-radius: 16px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.35);
  border: 1px solid rgba(255, 255, 255, 0.4);
  position: relative;
  z-index: 1;
}
.login-header {
  text-align: center;
  margin-bottom: 32px;
}
.danghui-img {
  width: 68px;
  height: 68px;
  object-fit: contain;
  margin-bottom: 12px;
}
.login-header h1 {
  font-size: 24px;
  color: var(--yx-primary);
  margin-bottom: 6px;
  letter-spacing: 2px;
}
.login-header p {
  color: #606266;
  font-size: 14px;
  letter-spacing: 6px;
}
.login-btn {
  width: 100%;
  background: linear-gradient(135deg, #e0354f, #a90d26) !important;
  border: none !important;
  border-radius: 8px !important;
  height: 44px;
  font-size: 16px;
  letter-spacing: 6px;
  box-shadow: 0 6px 18px rgba(200, 16, 46, 0.3);
}
.login-btn:hover {
  box-shadow: 0 8px 24px rgba(200, 16, 46, 0.4);
  transform: translateY(-1px);
}
.login-tip {
  text-align: center;
  color: #c0c4cc;
  font-size: 12px;
  margin-top: 12px;
}
@media (max-width: 480px) {
  .login-box {
    width: 90%;
    padding: 36px 24px;
  }
  .login-header h1 {
    font-size: 20px;
  }
}
@media (max-width: 767px) {
  .login-footer {
    position: static;
    padding-top: 16px;
    color: rgba(255,255,255,0.9);
  }
  .login-footer a {
    color: rgba(255,255,255,0.9);
  }
  .login-page {
    min-height: 100%;
    padding: 20px 12px;
    align-items: flex-start;
    justify-content: center;
  }
  .login-box {
    margin-top: 8vh;
  }
}
.login-footer {
  position: fixed;
  bottom: 16px;
  left: 50%;
  transform: translateX(-50%);
  text-align: center;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.95);
  background: rgba(0, 0, 0, 0.55);
  padding: 6px 16px;
  border-radius: 6px;
  white-space: nowrap;
}
.login-footer a {
  color: rgba(255, 255, 255, 0.92);
  text-decoration: none;
}
.login-footer a:hover {
  text-decoration: underline;
}
.footer-sep {
  margin: 0 8px;
  color: rgba(255, 255, 255, 0.7);
}
</style>
