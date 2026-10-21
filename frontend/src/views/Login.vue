<template>
  <div class="login-page">
    <!-- 左侧：品牌形象区（深红至墨蓝渐变 + 天山线描 + 大号宋体标题） -->
    <section class="brand-panel" aria-label="伊宁县委宣传部部务工作平台">
      <svg class="brand-skyline" viewBox="0 0 1200 260" preserveAspectRatio="none" aria-hidden="true">
        <path class="ridge-far" d="M0 260 L0 170 L90 132 L160 150 L250 96 L330 140 L420 110 L500 150 L590 70 L680 128 L770 100 L860 146 L950 88 L1040 136 L1120 118 L1200 140 L1200 260 Z" />
        <path class="ridge-near" d="M0 260 L0 210 L120 186 L220 204 L320 172 L430 200 L540 178 L650 206 L760 182 L880 208 L990 186 L1100 204 L1200 192 L1200 260 Z" />
      </svg>
      <div class="brand-glow" aria-hidden="true"></div>

      <header class="brand-top">
        <img src="../assets/danghui.png" alt="党徽" class="brand-badge" />
        <span class="brand-org">中共伊宁县委宣传部</span>
      </header>

      <div class="brand-main">
        <h1 class="brand-title">部务工作平台</h1>
        <div class="brand-rule"></div>
        <p class="brand-slogan">举旗帜 · 聚民心 · 育新人 · 兴文化 · 展形象</p>
        <p class="brand-desc">收文办理 · 征求意见 · 材料下发 · 会务协同 · 考勤休假</p>
      </div>

      <footer class="brand-bottom">非涉密政务协同系统 · 严禁存储、处理、传输国家秘密</footer>
    </section>

    <!-- 右侧：整高白色登录面板 -->
    <section class="form-panel">
      <div class="form-wrap">
        <h2 class="form-title">欢迎登录</h2>
        <p class="form-subtitle">请使用部务平台账号登录</p>

        <el-form :model="form" :rules="rules" ref="formRef" size="large" class="login-form" @submit.prevent>
          <el-form-item prop="username">
            <el-input
              v-model="form.username"
              placeholder="用户名"
              :prefix-icon="'User'"
              autocomplete="username"
              clearable
            />
          </el-form-item>

          <el-form-item prop="password">
            <el-input
              v-model="form.password"
              type="password"
              placeholder="密码"
              :prefix-icon="'Lock'"
              autocomplete="current-password"
              show-password
              @keyup.enter="handleLogin"
            />
          </el-form-item>

          <el-button type="primary" class="login-btn" :loading="loading" @click="handleLogin">登 录</el-button>
        </el-form>
      </div>

      <!-- 底部政务合规备案信息 -->
      <footer class="login-footer">
        <div class="footer-links">
          <a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener">[滇ICP备XXXXXXXX号-X]</a>
          <span class="footer-sep">|</span>
          <a href="https://beian.mps.gov.cn/#/query/webSearch" target="_blank" rel="noopener">[滇公网安备XXXXXXXXXXXXXXXX号]</a>
        </div>
        <div class="footer-meta">伊宁县委宣传部部务工作平台 V1.8.1 · 本站支持 IPv6</div>
      </footer>
    </section>
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
  // 回车键与按钮重复触发时，登录请求进行中不再重复提交
  if (loading.value) return
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
/* 登录页标题专用粗宋：思源宋体 Heavy（Noto Serif CJK SC Black，SIL OFL 1.1，许可见 assets/fonts/OFL-LICENSE.txt），
   只裁剪了登录页标题用到的字（约 14KB，随前端打包、完全离线）。修改标题文字后需重新裁剪：
   pyftsubset NotoSerifCJKsc-Black.otf --text="标题用到的全部汉字" --flavor=woff2 --layout-features='*' --no-hinting
     --output-file=src/assets/fonts/yx-serif-sc-900-login.woff2 */
@font-face {
  font-family: "YX Serif SC Heavy";
  src: url("../assets/fonts/yx-serif-sc-900-login.woff2") format("woff2");
  font-weight: 900;
  font-style: normal;
  font-display: swap;
}

.login-page {
  min-height: 100vh;
  display: grid;
  grid-template-columns: minmax(0, 1.45fr) minmax(400px, 1fr);
  background: var(--yx-surface);
}

/* ===== 左侧品牌区 ===== */
.brand-panel {
  position: relative;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding: 48px 64px 40px;
  color: #fff;
  background: linear-gradient(135deg, #8f1a1d 0%, #5c1013 52%, var(--yx-ink) 100%);
}
.brand-skyline {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  width: 100%;
  height: 38%;
  pointer-events: none;
}
.ridge-far {
  fill: var(--yx-gold-light);
  opacity: 0.07;
}
.ridge-near {
  fill: var(--yx-gold);
  opacity: 0.09;
}
.brand-glow {
  position: absolute;
  top: -20%;
  right: -10%;
  width: 70%;
  height: 80%;
  background: radial-gradient(circle, rgba(227, 207, 166, 0.16) 0%, rgba(227, 207, 166, 0) 70%);
  pointer-events: none;
}
.brand-top,
.brand-main,
.brand-bottom {
  position: relative;
  z-index: 1;
}
.brand-top {
  display: flex;
  align-items: center;
  gap: 14px;
}
.brand-badge {
  width: 44px;
  height: 44px;
  object-fit: contain;
  filter: drop-shadow(0 2px 8px rgba(0, 0, 0, 0.25));
}
.brand-org {
  font-family: "YX Serif SC Heavy", var(--yx-font-serif);
  font-weight: 900;
  font-size: 18px;
  letter-spacing: 4px;
  color: var(--yx-gold-light);
}
.brand-main {
  animation: rise 0.6s ease-out both;
}
.brand-title {
  margin: 0;
  font-family: "YX Serif SC Heavy", var(--yx-font-serif);
  font-size: 56px;
  font-weight: 900;
  letter-spacing: 12px;
  line-height: 1.2;
  text-shadow: 0 2px 16px rgba(0, 0, 0, 0.2);
}
.brand-rule {
  width: 72px;
  height: 2px;
  margin: 24px 0 20px;
  background: linear-gradient(90deg, var(--yx-gold-light), var(--yx-gold));
}
.brand-slogan {
  margin: 0;
  font-family: var(--yx-font-serif);
  font-size: 17px;
  letter-spacing: 6px;
  color: #f3e6cc;
}
.brand-desc {
  margin: 14px 0 0;
  font-size: 14px;
  letter-spacing: 2px;
  color: rgba(255, 255, 255, 0.62);
}
.brand-bottom {
  font-size: 13px;
  letter-spacing: 1px;
  color: rgba(255, 255, 255, 0.6);
}

/* ===== 右侧登录面板 ===== */
.form-panel {
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: 48px 56px 28px;
  background: var(--yx-surface);
}
.form-wrap {
  width: 100%;
  max-width: 380px;
  margin: auto;
  animation: rise 0.6s ease-out 0.08s both;
}
.form-title {
  margin: 0;
  font-family: "YX Serif SC Heavy", var(--yx-font-serif);
  font-size: 30px;
  font-weight: 900;
  letter-spacing: 4px;
  color: var(--yx-text-1);
}
.form-subtitle {
  margin: 10px 0 32px;
  font-size: 14px;
  color: var(--yx-text-3);
}
.login-form :deep(.el-form-item) {
  margin-bottom: 20px;
}
.login-form :deep(.el-input__wrapper) {
  height: 48px;
  padding: 0 14px;
  box-shadow: 0 0 0 1px var(--yx-border) inset;
}
.login-form :deep(.el-input__wrapper.is-focus) {
  box-shadow: 0 0 0 1px var(--yx-ink-2) inset, 0 0 0 3px var(--yx-ink-2-bg);
}
.login-form :deep(.el-input__inner) {
  font-size: 16px;
}
.login-btn {
  width: 100%;
  height: 48px;
  margin-top: 8px;
  font-size: 17px;
  letter-spacing: 12px;
  text-indent: 12px;
  font-family: "YX Serif SC Heavy", var(--yx-font-serif);
  font-weight: 900;
}
.login-footer {
  margin-top: 32px;
  text-align: center;
  font-size: 12px;
  color: var(--yx-text-4);
  line-height: 1.9;
}
.login-footer a {
  color: var(--yx-text-3);
  text-decoration: none;
  white-space: nowrap; /* 备案号整体换行，不从号码中间断开 */
}
.login-footer a:hover {
  color: var(--yx-ink-2);
}
/* 两个备案号用间距分隔并允许整体换行，窄屏时不会留下孤立的分隔符 */
.footer-links {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  column-gap: 18px;
}
.footer-sep {
  display: none;
}

@keyframes rise {
  from {
    opacity: 0;
    transform: translateY(12px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
@media (prefers-reduced-motion: reduce) {
  .brand-main,
  .form-wrap {
    animation: none;
  }
}

/* 中等宽度：缩小品牌区标题 */
@media (max-width: 1100px) {
  .brand-panel {
    padding: 40px 40px 32px;
  }
  .brand-title {
    font-size: 44px;
    letter-spacing: 8px;
  }
}

/* 手机端：品牌区收为顶部横幅，登录面板在下方 */
@media (max-width: 768px) {
  .login-page {
    grid-template-columns: 1fr;
    grid-template-rows: auto 1fr;
  }
  .brand-panel {
    padding: 28px 20px 30px;
    min-height: 220px;
    gap: 20px;
  }
  .brand-skyline {
    height: 46%;
  }
  .brand-badge {
    width: 34px;
    height: 34px;
  }
  .brand-org {
    font-size: 15px;
    letter-spacing: 2px;
  }
  .brand-title {
    font-size: 32px;
    letter-spacing: 6px;
  }
  .brand-rule {
    margin: 14px 0 12px;
  }
  .brand-slogan {
    font-size: 13px;
    letter-spacing: 2px;
  }
  .brand-desc,
  .brand-bottom {
    display: none;
  }
  .form-panel {
    padding: 28px 20px 20px;
  }
  .form-title {
    font-size: 24px;
  }
  .form-subtitle {
    margin-bottom: 22px;
  }
}
</style>
