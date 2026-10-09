<template>
  <transition name="secrecy-fade">
    <div v-if="visible" class="secrecy-overlay">
      <!-- 动态激光扫描线 -->
      <div class="secrecy-scanline"></div>
      
      <!-- 告警主体弹窗 -->
      <div class="secrecy-modal">
        <div class="secrecy-corner corner-tl"></div>
        <div class="secrecy-corner corner-tr"></div>
        <div class="secrecy-corner corner-bl"></div>
        <div class="secrecy-corner corner-br"></div>

        <!-- 顶部告警状态条 -->
        <div class="secrecy-header">
          <div class="header-left">
            <span class="alarm-pulse"></span>
            <span class="alarm-text blink-text">STATE SECRECY BREACH DETECTED // 涉密文件安全阻断</span>
          </div>
          <span class="alarm-code">ERR_CODE: SECRECY_BLOCK_403</span>
        </div>

        <!-- 告警正文 -->
        <div class="secrecy-body">
          <div class="secrecy-icon-wrap">
            <el-icon :size="44" color="#ff3b30"><WarningFilled /></el-icon>
          </div>

          <h2 class="secrecy-title">【发现保密违法行为】</h2>
          <div class="secrecy-sub-title">检测到违规上传国家秘密或内部级保密文件资料</div>

          <div class="violation-dossier-box">
            <div class="dossier-row" v-if="fileName">
              <span class="d-label">违规文件：</span>
              <span class="d-val text-red">《{{ fileName }}》</span>
            </div>
            <div class="dossier-row" v-if="matchedKeyword">
              <span class="d-label">命中密级特征：</span>
              <span class="d-badge text-gold">{{ matchedKeyword }}</span>
            </div>
            <div class="dossier-row">
              <span class="d-label">法律依据：</span>
              <span class="d-val">《中华人民共和国保守国家秘密法》第二十九条</span>
            </div>
            <div class="dossier-row clause-row">
              <span class="d-label">法定义务：</span>
              <span class="d-val text-yellow">“禁止未按照国家保密规定和标准采取有效保密措施，在互联网及其他公共信息网络或者有线和无线通信中传递国家秘密”</span>
            </div>
          </div>

          <p class="secrecy-warning-text">
            <strong>严正声明：</strong>本平台为非涉密政务协同办公系统，<strong>严禁任何单位和个人上传、存储、处理、传输标注有国家秘密（秘密、机密、绝密）、工作秘密及内部级的文件资料！</strong>
          </p>

          <div class="lock-status-banner">
            <el-icon class="lock-icon"><Lock /></el-icon>
            <div class="lock-text">
              <div class="lock-title">【安全熔断触发 · 会话已强制锁定】</div>
              <div>该阻断事件、操作记录及源 IP 已即时存证上报保密审计中心。</div>
              <div class="contact-highlight">⚠️ 请等待保密主管部门与你联系。</div>
              <div class="countdown-text">当前系统操作已被锁定<span v-if="countdown > 0">（将在 <strong>{{ countdown }}</strong> 秒后强制退出）</span>。</div>
            </div>
          </div>
        </div>

        <!-- 底部控制按钮 -->
        <div class="secrecy-footer">
          <button class="secrecy-action-btn" @click="handleForceExit">
            立即退出并清除会话锁定
          </button>
        </div>
      </div>
    </div>
  </transition>
</template>

<script setup>
import { ref, onUnmounted } from 'vue'
import { WarningFilled, Lock } from '@element-plus/icons-vue'

const visible = ref(false)
const fileName = ref('')
const matchedKeyword = ref('')
const isPublicPage = ref(false)
const countdown = ref(30)
let timer = null
let onLockCallback = null

const open = (options = {}, onLock) => {
  fileName.value = options.fileName || '未知文件'
  matchedKeyword.value = options.keyword || '国家秘密/内部级标识'
  isPublicPage.value = !!options.isPublic
  onLockCallback = onLock
  visible.value = true
  countdown.value = 30

  // 立即触发锁定清理逻辑
  if (onLockCallback) {
    onLockCallback()
  }

  // 30秒倒计时自动强制跳转/重载
  if (timer) clearInterval(timer)
  timer = setInterval(() => {
    countdown.value--
    if (countdown.value <= 0) {
      clearInterval(timer)
      handleForceExit()
    }
  }, 1000)
}

const handleForceExit = () => {
  if (timer) clearInterval(timer)
  visible.value = false

  // 清除敏感缓存并根据页面类型执行安全重定向
  if (isPublicPage.value) {
    // 公开端：强制刷新页面并锁定
    window.location.reload()
  } else {
    // 管理后台：强制清空 Token 并跳转登录页
    localStorage.removeItem('token')
    localStorage.removeItem('user')
    sessionStorage.clear()
    window.location.href = '/login'
  }
}

onUnmounted(() => {
  if (timer) clearInterval(timer)
})

defineExpose({ open, handleForceExit })
</script>

<style scoped>
.secrecy-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
  background: rgba(12, 0, 0, 0.94);
  z-index: 999999;
  display: flex;
  align-items: center;
  justify-content: center;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif;
  overflow: hidden;
  backdrop-filter: blur(8px);
  padding: 12px;
  box-sizing: border-box;
}

.secrecy-scanline {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 12px;
  background: linear-gradient(180deg, rgba(255, 59, 48, 0) 0%, rgba(255, 59, 48, 0.6) 50%, rgba(255, 59, 48, 0) 100%);
  box-shadow: 0 0 30px rgba(255, 59, 48, 0.9);
  animation: scan 2.6s infinite linear;
  pointer-events: none;
}

@keyframes scan {
  0% { top: -20px; }
  100% { top: 100%; }
}

.secrecy-modal {
  width: 92%;
  max-width: 600px;
  max-height: 90vh;
  overflow-y: auto;
  overflow-x: hidden;
  background: rgba(26, 4, 4, 0.96);
  border: 1.5px solid #ff3b30;
  box-shadow: 0 0 45px rgba(255, 59, 48, 0.45), inset 0 0 25px rgba(255, 59, 48, 0.25);
  border-radius: 8px;
  padding: 20px 24px 18px;
  position: relative;
  text-align: center;
  box-sizing: border-box;
  scrollbar-width: thin;
  scrollbar-color: #ff3b30 rgba(20, 0, 0, 0.5);
}

/* 自定义科技感纤细滚动条 */
.secrecy-modal::-webkit-scrollbar {
  width: 5px;
}
.secrecy-modal::-webkit-scrollbar-track {
  background: rgba(20, 0, 0, 0.5);
  border-radius: 3px;
}
.secrecy-modal::-webkit-scrollbar-thumb {
  background: #b91c1c;
  border-radius: 3px;
}
.secrecy-modal::-webkit-scrollbar-thumb:hover {
  background: #ff3b30;
}

/* 四角高科技激光角标 */
.secrecy-corner {
  position: absolute;
  width: 16px;
  height: 16px;
  border-color: #ff3b30;
  border-style: solid;
  pointer-events: none;
}
.corner-tl { top: -2px; left: -2px; border-width: 3px 0 0 3px; }
.corner-tr { top: -2px; right: -2px; border-width: 3px 3px 0 0; }
.corner-bl { bottom: -2px; left: -2px; border-width: 0 0 3px 3px; }
.corner-br { bottom: -2px; right: -2px; border-width: 0 3px 3px 0; }

.secrecy-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  color: #ff4d4f;
  font-size: 11px;
  border-bottom: 1px dashed rgba(255, 77, 79, 0.6);
  padding-bottom: 8px;
  margin-bottom: 10px;
  letter-spacing: 1px;
  font-family: 'Courier New', Courier, monospace;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 8px;
  font-family: 'Courier New', Courier, monospace;
  font-size: 11px;
  letter-spacing: 1px;
}

.alarm-text {
  font-family: 'Courier New', Courier, monospace;
  font-size: 11px;
  letter-spacing: 1px;
  color: #ff4d4f;
}

.alarm-pulse {
  width: 8px;
  height: 8px;
  background-color: #ff3b30;
  border-radius: 50%;
  box-shadow: 0 0 8px #ff3b30;
  animation: pulse-red 1s infinite;
}

@keyframes pulse-red {
  0% { transform: scale(0.9); opacity: 0.8; }
  50% { transform: scale(1.3); opacity: 1; }
  100% { transform: scale(0.9); opacity: 0.8; }
}

.alarm-code {
  font-family: 'Courier New', Courier, monospace;
  color: #ff4d4f;
  font-size: 11px;
  font-weight: bold;
  letter-spacing: 1px;
}

.blink-text {
  animation: blink 1.2s infinite;
}

@keyframes blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}

.secrecy-icon-wrap {
  margin: 2px 0 6px;
  filter: drop-shadow(0 0 14px rgba(255, 59, 48, 0.8));
  animation: shake 2.5s infinite ease-in-out;
}

@keyframes shake {
  0%, 100% { transform: translateY(0); }
  5% { transform: translateY(-3px); }
  10% { transform: translateY(3px); }
  15% { transform: translateY(-2px); }
  20% { transform: translateY(0); }
}

.secrecy-title {
  color: #ff3b30;
  font-size: 21px;
  font-weight: 800;
  margin: 0 0 4px;
  letter-spacing: 1.5px;
  text-shadow: 0 0 10px rgba(255, 59, 48, 0.6);
}

.secrecy-sub-title {
  color: #fca5a5;
  font-size: 13px;
  margin-bottom: 10px;
  letter-spacing: 0.5px;
}

.violation-dossier-box {
  background: rgba(45, 10, 10, 0.85);
  border: 1px solid rgba(255, 59, 48, 0.4);
  border-radius: 6px;
  padding: 10px 14px;
  margin-bottom: 10px;
  text-align: left;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.dossier-row {
  display: flex;
  align-items: center;
  font-size: 13px;
}

.d-label {
  color: #cbd5e1;
  width: 95px;
  flex-shrink: 0;
}

.d-val {
  color: #f8fafc;
  word-break: break-all;
}

.text-red {
  color: #ff6b6b;
  font-weight: 700;
}

.text-gold {
  color: #fef08a;
}

.text-yellow {
  color: #fde047;
  font-weight: 600;
  line-height: 1.4;
  font-size: 12.5px;
}

.clause-row {
  margin-top: 2px;
  align-items: flex-start;
}

.d-badge {
  background: #7f1d1d;
  color: #fef08a;
  padding: 2px 8px;
  border-radius: 4px;
  font-weight: 700;
  border: 1px solid #b91c1c;
  font-size: 12px;
}

.secrecy-warning-text {
  color: #f1f5f9;
  font-size: 12px;
  line-height: 1.55;
  margin-bottom: 10px;
  text-align: left;
  background: rgba(0, 0, 0, 0.4);
  padding: 8px 12px;
  border-left: 3px solid #ff3b30;
  border-radius: 0 5px 5px 0;
}

.secrecy-warning-text strong {
  color: #ff8787;
}

.contact-highlight {
  color: #fde047;
  font-weight: 700;
  display: inline-block;
  margin: 3px 0;
  text-shadow: 0 0 8px rgba(253, 224, 71, 0.4);
}

.lock-status-banner {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  background: rgba(220, 38, 38, 0.22);
  border: 1px solid #ef4444;
  border-radius: 6px;
  padding: 10px 14px;
  text-align: left;
  margin-bottom: 14px;
}

.lock-icon {
  font-size: 22px;
  color: #ff3b30;
  flex-shrink: 0;
  margin-top: 2px;
}

.lock-text {
  font-size: 12px;
  color: #fecaca;
  line-height: 1.5;
}

.lock-title {
  color: #ffffff;
  font-weight: 700;
  margin-bottom: 2px;
}

.countdown-text {
  color: #fca5a5;
  margin-top: 2px;
}

.countdown-text strong {
  color: #fef08a;
  font-size: 13px;
}

.secrecy-footer {
  display: flex;
  justify-content: center;
}

.secrecy-action-btn {
  background: linear-gradient(135deg, #b91c1c 0%, #7f1d1d 100%);
  color: #ffffff;
  border: 1px solid #ef4444;
  padding: 9px 32px;
  font-size: 14px;
  font-weight: 700;
  border-radius: 6px;
  cursor: pointer;
  letter-spacing: 1px;
  box-shadow: 0 0 16px rgba(239, 68, 68, 0.4);
  transition: all 0.25s ease;
}

.secrecy-action-btn:hover {
  background: linear-gradient(135deg, #dc2626 0%, #991b1b 100%);
  box-shadow: 0 0 25px rgba(239, 68, 68, 0.8);
  transform: translateY(-1px);
}

.secrecy-fade-enter-active,
.secrecy-fade-leave-active {
  transition: opacity 0.3s ease;
}

.secrecy-fade-enter-from,
.secrecy-fade-leave-to {
  opacity: 0;
}

/* ================= 移动端响应式深度适配 ================= */
@media (max-width: 640px) {
  .secrecy-overlay {
    padding: 8px;
    align-items: center;
  }

  .secrecy-modal {
    width: 96%;
    max-width: 100%;
    padding: 12px 10px 12px;
    max-height: 94vh;
    border-radius: 6px;
  }

  .secrecy-corner {
    width: 12px;
    height: 12px;
  }

  .secrecy-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 3px;
    font-size: 10px;
    margin-bottom: 8px;
    padding-bottom: 5px;
  }

  .header-left {
    font-size: 10px;
    gap: 5px;
  }

  .alarm-text {
    font-size: 10px;
  }

  .alarm-code {
    font-size: 9.5px;
  }

  .secrecy-icon-wrap {
    margin: 0 0 4px;
  }

  .secrecy-title {
    font-size: 17px;
    letter-spacing: 0.5px;
    margin: 0 0 2px;
  }

  .secrecy-sub-title {
    font-size: 11.5px;
    margin-bottom: 8px;
  }

  .violation-dossier-box {
    padding: 8px 10px;
    margin-bottom: 8px;
    gap: 5px;
  }

  .dossier-row {
    font-size: 12px;
  }

  .d-label {
    width: 82px;
    font-size: 11.5px;
  }

  .clause-row {
    flex-direction: column;
    gap: 2px;
  }

  .clause-row .d-label {
    width: 100%;
  }

  .text-yellow {
    font-size: 11.5px;
    line-height: 1.35;
  }

  .d-badge {
    padding: 1px 6px;
    font-size: 11px;
  }

  .secrecy-warning-text {
    font-size: 11px;
    line-height: 1.45;
    padding: 6px 8px;
    margin-bottom: 8px;
  }

  .lock-status-banner {
    padding: 8px 10px;
    margin-bottom: 10px;
    gap: 8px;
  }

  .lock-icon {
    font-size: 18px;
  }

  .lock-text {
    font-size: 11px;
    line-height: 1.4;
  }

  .contact-highlight {
    font-size: 11.5px;
    margin: 2px 0;
  }

  .secrecy-action-btn {
    width: 100%;
    padding: 10px 0;
    font-size: 13.5px;
  }
}
</style>
