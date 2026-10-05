<template>
  <transition name="waf-fade">
    <div v-if="visible" class="waf-overlay">
      <div class="waf-scanline"></div>
      <div class="waf-modal">
        <div class="waf-header">
          <span class="blink-text">SYSTEM BREACH ATTEMPT DETECTED</span>
          <span class="waf-id">ERR_CODE: WAF_BLOCK_403</span>
        </div>
        <div class="waf-body">
          <div class="waf-icon">
            <el-icon :size="64" color="#ff4d4f"><WarningFilled /></el-icon>
          </div>
          <h2 class="waf-title">【系统安全警告】</h2>
          <p class="waf-desc">
            检测到非法注入或恶意探测行为！<br>
            您的非法请求已被政务安全防火墙强制熔断。<br>
            <strong class="highlight">该操作记录及您的源 IP 地址已上报至安全审计中心！</strong>
          </p>
        </div>
        <div class="waf-footer">
          <button class="waf-btn" @click="close">立即停止非法操作并返回</button>
        </div>
      </div>
    </div>
  </transition>
</template>

<script setup>
import { ref } from 'vue'
import { WarningFilled } from '@element-plus/icons-vue'

const visible = ref(false)
let onCloseCallback = null

const open = (onClose) => {
  visible.value = true
  onCloseCallback = onClose
}

const close = () => {
  visible.value = false
  if (onCloseCallback) {
    onCloseCallback()
  }
}

defineExpose({ open, close })
</script>

<style scoped>
.waf-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
  background: rgba(10, 0, 0, 0.9);
  z-index: 99999;
  display: flex;
  align-items: center;
  justify-content: center;
  font-family: 'Courier New', Courier, monospace;
  overflow: hidden;
}

.waf-scanline {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 10px;
  background: rgba(255, 0, 0, 0.3);
  box-shadow: 0 0 20px rgba(255, 0, 0, 0.8);
  animation: scan 3s infinite linear;
  pointer-events: none;
}

@keyframes scan {
  0% { top: -10px; }
  100% { top: 100%; }
}

.waf-modal {
  width: 600px;
  background: rgba(20, 0, 0, 0.8);
  border: 1px solid #ff4d4f;
  box-shadow: 0 0 30px rgba(255, 77, 79, 0.4), inset 0 0 20px rgba(255, 77, 79, 0.2);
  padding: 30px;
  position: relative;
  text-align: center;
}

.waf-modal::before, .waf-modal::after {
  content: '';
  position: absolute;
  width: 20px;
  height: 20px;
  border: 2px solid #ff4d4f;
}
.waf-modal::before { top: -2px; left: -2px; border-right: none; border-bottom: none; }
.waf-modal::after { bottom: -2px; right: -2px; border-left: none; border-top: none; }

.waf-header {
  display: flex;
  justify-content: space-between;
  color: #ff4d4f;
  font-size: 12px;
  border-bottom: 1px dashed #ff4d4f;
  padding-bottom: 10px;
  margin-bottom: 20px;
  letter-spacing: 1px;
}

.blink-text {
  animation: blink 1s infinite;
}

@keyframes blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0; }
}

.waf-icon {
  margin: 20px 0;
  filter: drop-shadow(0 0 10px #ff4d4f);
}

.waf-title {
  color: #ff4d4f;
  font-size: 24px;
  margin-bottom: 15px;
  font-weight: bold;
  letter-spacing: 2px;
}

.waf-desc {
  color: #fff;
  font-size: 16px;
  line-height: 1.8;
  margin-bottom: 30px;
}

.highlight {
  color: #ff4d4f;
  text-shadow: 0 0 5px #ff4d4f;
}

.waf-btn {
  background: transparent;
  color: #ff4d4f;
  border: 1px solid #ff4d4f;
  padding: 12px 30px;
  font-size: 16px;
  cursor: pointer;
  transition: all 0.3s;
  text-transform: uppercase;
  letter-spacing: 1px;
  font-family: inherit;
  font-weight: bold;
}

.waf-btn:hover {
  background: #ff4d4f;
  color: #000;
  box-shadow: 0 0 15px #ff4d4f;
}

.waf-fade-enter-active,
.waf-fade-leave-active {
  transition: opacity 0.3s ease;
}

.waf-fade-enter-from,
.waf-fade-leave-to {
  opacity: 0;
}
</style>
