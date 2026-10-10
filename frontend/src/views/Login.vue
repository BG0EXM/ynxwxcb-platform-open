<template>
  <div class="login-page">
    <!-- 背景天山金色微纹与暗色渐变 -->
    <div class="login-backdrop"></div>

    <!-- 登录主体：桌面端左右分栏大气卡片 -->
    <div class="login-container">
      <!-- 左侧：宣传部品牌形象展示区 -->
      <div class="login-brand-panel">
        <div class="brand-inner">
          <div class="brand-danghui-wrap">
            <img src="../assets/danghui.png" alt="党徽" class="brand-danghui-img" />
          </div>
          <h1 class="brand-org font-serif">伊宁县委宣传部</h1>
          <h2 class="brand-system font-serif">部务工作平台</h2>
          <div class="brand-divider"></div>
          <p class="brand-slogan">
            举旗帜 &nbsp;·&nbsp; 聚民心 &nbsp;·&nbsp; 育新人 &nbsp;·&nbsp; 兴文化 &nbsp;·&nbsp; 展形象
          </p>
          <div class="brand-tags">
            <span class="brand-tag-item">严谨规范</span>
            <span class="brand-tag-item">高效协同</span>
            <span class="brand-tag-item">安全可控</span>
          </div>
        </div>
      </div>

      <!-- 右侧：账号登录表单区 -->
      <div class="login-form-panel">
        <div class="form-header">
          <h3 class="form-title font-serif">用户登录</h3>
          <p class="form-subtitle">请输入部务平台账号与密码开展工作</p>
        </div>

        <el-form :model="form" :rules="rules" ref="formRef" size="large" class="login-form">
          <el-form-item prop="username">
            <el-input 
              v-model="form.username" 
              placeholder="请输入用户名" 
              :prefix-icon="'User'" 
              clearable 
            />
          </el-form-item>

          <el-form-item prop="password">
            <el-input 
              v-model="form.password" 
              type="password" 
              placeholder="请输入密码"
              :prefix-icon="'Lock'" 
              show-password 
              @keyup.enter="handleLogin" 
            />
          </el-form-item>

          <el-form-item class="mt-8">
            <el-button 
              type="primary" 
              class="login-btn font-serif" 
              :loading="loading" 
              @click="handleLogin"
            >
              登 &nbsp; 录
            </el-button>
          </el-form-item>
        </el-form>

        <div class="form-footer-tip">
          <span>伊宁县委宣传部部务工作平台 V1.7.1</span>
        </div>
      </div>
    </div>

    <!-- 底部政务合规备案信息 -->
    <footer class="login-footer">
      <a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener">[滇ICP备XXXXXXXX号-X]</a>
      <span class="footer-sep">|</span>
      <a href="https://beian.mps.gov.cn/#/query/webSearch" target="_blank" rel="noopener">
        [滇公网安备XXXXXXXXXXXXXXXX号]
      </a>
      <span class="footer-sep">|</span>
      <span class="ipv6-tip">本站支持IPv6</span>
    </footer>
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
      ElMessage.warning('您正在使用初始密码，请先修改密码')
      router.push('/profile')
    } else {
      ElMessage.success('登录成功，欢迎进入工作台')
      router.push('/dashboard')
    }
  } catch (e) {
    console.error(e)
    // 错误已由请求拦截器统一提示
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: url('../assets/login-bg.jpg') no-repeat center center;
  background-size: cover;
  position: relative;
  padding: 30px 16px 80px 16px;
  overflow-y: auto;
}

/* 深邃墨色滤镜，压制杂乱背景，烘托红金质感 */
.login-backdrop {
  position: absolute;
  inset: 0;
  background: linear-gradient(135deg, rgba(19, 26, 38, 0.88) 0%, rgba(27, 36, 51, 0.82) 100%);
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
  z-index: 0;
}

/* 左右分栏整体大卡片 */
.login-container {
  position: relative;
  z-index: 1;
  display: flex;
  width: 920px;
  max-width: 100%;
  min-height: 520px;
  background: var(--yx-surface);
  border-radius: 16px;
  box-shadow: 0 24px 70px rgba(0, 0, 0, 0.45);
  border: 1px solid rgba(184, 146, 90, 0.35);
  overflow: hidden;
}

.login-container::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 4px;
  background: var(--yx-crest-gradient);
  z-index: 10;
}

/* ===== 左侧：品牌形象区 ===== */
.login-brand-panel {
  flex: 1.15;
  background: linear-gradient(145deg, #131a26 0%, #1b2433 60%, #7a1417 100%);
  background-image: 
    linear-gradient(145deg, rgba(24, 34, 50, 0.94) 0%, rgba(15, 22, 34, 0.96) 100%),
    url("data:image/svg+xml,%3Csvg width='300' height='150' viewBox='0 0 300 150' fill='none' xmlns='http://www.w3.org/2000/svg'%3E%3Cpath d='M0 150 L100 60 L150 110 L210 30 L300 150' stroke='rgba(227, 207, 166, 0.08)' stroke-width='1.5'/%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: center bottom;
  padding: 48px 40px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  text-align: center;
  color: #ffffff;
  border-right: 1px solid rgba(184, 146, 90, 0.2);
  position: relative;
}

.brand-inner {
  max-width: 380px;
}

.brand-danghui-wrap {
  margin-bottom: 20px;
}

/* 党徽：沿用现有图片与金色光晕 */
.brand-danghui-img {
  width: 72px;
  height: 72px;
  object-fit: contain;
  filter: drop-shadow(0 0 12px rgba(201, 160, 99, 0.65));
}

.brand-org {
  margin: 0;
  font-size: 28px;
  font-weight: 700;
  color: #ffffff;
  letter-spacing: 3px;
  line-height: 1.25;
  font-family: var(--yx-font-serif);
  text-shadow: 0 2px 8px rgba(0, 0, 0, 0.45);
}

.brand-system {
  margin: 10px 0 0 0;
  font-size: 18px;
  color: var(--yx-gold-light);
  letter-spacing: 5px;
  font-weight: 500;
  font-family: var(--yx-font-serif);
}

.brand-divider {
  width: 60px;
  height: 2px;
  background: linear-gradient(90deg, transparent, var(--yx-gold), transparent);
  margin: 24px auto;
}

.brand-slogan {
  margin: 0;
  font-size: 13px;
  color: rgba(255, 255, 255, 0.78);
  line-height: 1.8;
  letter-spacing: 1px;
}

.brand-tags {
  margin-top: 28px;
  display: flex;
  justify-content: center;
  gap: 12px;
}

.brand-tag-item {
  font-size: 11px;
  color: var(--yx-gold-light);
  border: 1px solid rgba(184, 146, 90, 0.35);
  padding: 3px 10px;
  border-radius: 20px;
  background: rgba(184, 146, 90, 0.08);
  letter-spacing: 1px;
}

/* ===== 右侧：登录操作区 ===== */
.login-form-panel {
  flex: 1;
  padding: 56px 44px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  background: #ffffff;
}

.form-header {
  margin-bottom: 32px;
}

.form-title {
  margin: 0;
  font-size: 24px;
  font-weight: 600;
  color: var(--yx-text-1);
  letter-spacing: 1px;
}

.form-subtitle {
  margin: 8px 0 0 0;
  font-size: 13px;
  color: var(--yx-text-3);
}

.login-form :deep(.el-input__wrapper) {
  border-radius: 8px;
  padding: 4px 14px;
  box-shadow: 0 0 0 1px var(--el-border-color) inset;
}

.login-form :deep(.el-input__wrapper.is-focus) {
  box-shadow: 0 0 0 1px var(--yx-ink-2) inset, 0 0 0 3px rgba(36, 64, 110, 0.12) !important;
}

.login-form :deep(.el-input__inner) {
  height: 42px;
  font-size: 15px;
}

.mt-8 {
  margin-top: 8px;
}

/* 墨蓝主操作按钮，带鎏金阴影 */
.login-btn {
  width: 100%;
  height: 46px;
  font-size: 17px;
  font-weight: 600;
  letter-spacing: 6px;
  border-radius: 8px;
  background: linear-gradient(135deg, #1b2433 0%, #24406e 100%) !important;
  border: none !important;
  box-shadow: 0 6px 18px rgba(36, 64, 110, 0.28);
  transition: all 0.25s ease;
}

.login-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 8px 24px rgba(36, 64, 110, 0.4);
  background: linear-gradient(135deg, #24406e 0%, #2d518d 100%) !important;
}

.form-footer-tip {
  margin-top: 24px;
  text-align: center;
  font-size: 12px;
  color: var(--yx-text-4);
}

/* ===== 底部政务合规备案号 ===== */
.login-footer {
  position: absolute;
  bottom: 20px;
  left: 50%;
  transform: translateX(-50%);
  text-align: center;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.85);
  background: rgba(15, 22, 34, 0.65);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  padding: 8px 22px;
  border-radius: 20px;
  border: 1px solid rgba(255, 255, 255, 0.15);
  white-space: nowrap;
  z-index: 2;
}

.login-footer a {
  color: rgba(255, 255, 255, 0.85);
  text-decoration: none;
  transition: color 0.2s ease;
}

.login-footer a:hover {
  color: var(--yx-gold-light);
  text-decoration: underline;
}

.footer-sep {
  margin: 0 10px;
  color: rgba(255, 255, 255, 0.35);
}

.ipv6-tip {
  color: rgba(255, 255, 255, 0.85);
}

/* ===== 移动端响应式处理 ===== */
@media (max-width: 768px) {
  .login-container {
    flex-direction: column;
    width: 100%;
    max-width: 440px;
  }
  .login-brand-panel {
    padding: 32px 20px;
    border-right: none;
    border-bottom: 1px solid rgba(184, 146, 90, 0.2);
  }
  .brand-danghui-img {
    width: 54px;
    height: 54px;
  }
  .brand-org {
    font-size: 22px;
  }
  .brand-system {
    font-size: 16px;
  }
  .brand-tags {
    display: none;
  }
  .login-form-panel {
    padding: 32px 24px;
  }
  .login-footer {
    position: static;
    margin-top: 24px;
    transform: none;
    white-space: normal;
    display: inline-block;
  }
  .login-page {
    padding: 20px 12px;
    display: flex;
    flex-direction: column;
  }
}
</style>
