<template>
  <el-container class="layout">
    <!-- 侧边栏 -->
    <el-aside 
      :width="isMobile ? '260px' : (collapsed ? '68px' : '230px')"
      class="sidebar" 
      :class="{ 'sidebar-collapsed': collapsed && !isMobile, 'sidebar-mobile-open': mobileOpen }"
    >
      <!-- 品牌区域 -->
      <div class="logo" :class="{ 'logo-collapsed': collapsed && !isMobile }">
        <div class="logo-badge">
          <img src="../assets/danghui.png" alt="党徽" class="danghui-img" />
        </div>
        <div v-show="!collapsed || isMobile" class="logo-text">
          <h2 class="logo-title font-serif">伊宁县委宣传部</h2>
          <p class="logo-sub font-serif">部务工作平台</p>
        </div>
        <div v-if="isMobile" class="mobile-sidebar-close" @click="closeMobile">
          <el-icon><Close /></el-icon>
        </div>
      </div>

      <!-- 侧边菜单 -->
      <el-menu 
        :default-active="$route.path" 
        :collapse="collapsed && !isMobile"
        :collapse-transition="false"
        router 
        background-color="transparent"
        text-color="rgba(255, 255, 255, 0.72)" 
        active-text-color="#ffffff" 
        class="sidebar-menu"
      >
        <!-- 工作台置顶 -->
        <el-menu-item :index="dashboardMenu.path" @click="closeMobile">
          <el-icon><component :is="dashboardMenu.icon" /></el-icon>
          <template #title><span>{{ dashboardMenu.title }}</span></template>
        </el-menu-item>

        <!-- 分组菜单 -->
        <template v-for="group in menuGroups" :key="group.title">
          <el-sub-menu :index="group.title" popper-class="sidebar-menu-popper">
            <template #title>
              <el-icon><component :is="group.icon" /></el-icon>
              <span>{{ group.title }}</span>
            </template>
            <el-menu-item 
              v-for="item in group.items" 
              :key="item.path" 
              :index="item.path" 
              @click="closeMobile"
            >
              <el-icon><component :is="item.icon" /></el-icon>
              <template #title><span>{{ item.title }}</span></template>
            </el-menu-item>
          </el-sub-menu>
        </template>
      </el-menu>

      <!-- 底部折叠切换按钮（桌面端） -->
      <div v-if="!isMobile" class="sidebar-collapse-bar" @click="toggleCollapse">
        <el-icon class="collapse-icon">
          <Fold v-if="!collapsed" />
          <Expand v-else />
        </el-icon>
        <span v-show="!collapsed" class="collapse-text">收起侧栏</span>
      </div>
    </el-aside>

    <!-- 移动端遮罩 -->
    <div v-if="mobileOpen" class="sidebar-mask" @click="closeMobile"></div>

    <el-container class="main-container">
      <!-- 顶栏 -->
      <el-header class="header">
        <div class="header-left">
          <!-- 移动端汉堡菜单 -->
          <el-button 
            v-if="isMobile"
            class="mobile-toggle-btn" 
            :icon="'Expand'" 
            @click="toggleMobile" 
            circle 
            size="small" 
          />
          <!-- 桌面端折叠切换小按钮 -->
          <button 
            v-else
            class="desktop-toggle-btn" 
            @click="toggleCollapse" 
            :title="collapsed ? '展开菜单' : '收起菜单'"
          >
            <el-icon :size="18">
              <Fold v-if="!collapsed" />
              <Expand v-else />
            </el-icon>
          </button>

          <!-- 移动端当前页面标题 -->
          <span v-if="isMobile" class="mobile-page-title font-serif">{{ currentTitle || '工作台' }}</span>

          <!-- 桌面端面包屑 -->
          <el-breadcrumb v-else separator="/" class="header-breadcrumb">
            <el-breadcrumb-item :to="{ path: '/dashboard' }">首页</el-breadcrumb-item>
            <el-breadcrumb-item v-if="currentGroupTitle">{{ currentGroupTitle }}</el-breadcrumb-item>
            <el-breadcrumb-item v-if="currentTitle && currentTitle !== '工作台'">
              {{ currentTitle }}
            </el-breadcrumb-item>
          </el-breadcrumb>
        </div>

        <div class="header-right">
          <!-- 全局搜索控制台触发按钮 -->
          <div class="header-search-btn" @click="openSearchModal" title="全局搜索 (⌘K / Ctrl+K)">
            <el-icon class="search-btn-icon"><Search /></el-icon>
            <span class="search-btn-text hidden-xs">搜索公文、人员、资料...</span>
            <div class="search-btn-shortcut hidden-xs">
              <kbd>{{ isMac ? '⌘' : 'Ctrl' }}</kbd>
              <kbd>K</kbd>
            </div>
          </div>

          <!-- 今日日期 -->
          <div class="header-date hidden-xs">
            <el-icon class="date-icon"><Calendar /></el-icon>
            <span>{{ todayStr }}</span>
          </div>

          <!-- 待办消息铃铛 -->
          <el-tooltip content="待办收文提醒" placement="bottom">
            <div class="header-badge-wrap" @click="$router.push('/incoming')">
              <el-badge :value="unread" :hidden="!unread" :max="99" class="unread-badge">
                <el-icon class="header-icon"><Bell /></el-icon>
              </el-badge>
            </div>
          </el-tooltip>

          <!-- 用户信息下拉 -->
          <el-dropdown @command="handleCommand" trigger="click">
            <div class="user-info">
              <el-avatar :size="32" class="user-avatar">{{ avatarText }}</el-avatar>
              <div class="user-meta hidden-xs">
                <span class="user-name">{{ authStore.user?.real_name || '用户' }}</span>
                <span class="user-role">{{ roleText }}</span>
              </div>
              <el-icon class="dropdown-arrow"><ArrowDown /></el-icon>
            </div>
            <template #dropdown>
              <el-dropdown-menu class="user-dropdown-menu">
                <el-dropdown-item command="profile">
                  <el-icon><User /></el-icon>个人中心
                </el-dropdown-item>
                <el-dropdown-item command="logout" divided>
                  <el-icon><SwitchButton /></el-icon>退出登录
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <!-- 内容主区 -->
      <el-main class="main">
        <router-view />
      </el-main>
    </el-container>

    <!-- 全局快捷速查与指令面板 -->
    <GlobalSearchModal ref="searchModalRef" />
  </el-container>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '../store/auth'
import request from '../utils/request'
import GlobalSearchModal from '../components/GlobalSearchModal.vue'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const searchModalRef = ref(null)
const isMac = ref(typeof navigator !== 'undefined' && /Mac|iPod|iPhone|iPad/.test(navigator.platform))

const openSearchModal = () => {
  searchModalRef.value?.open()
}

const collapsed = ref(false)
const mobileOpen = ref(false)
const isMobile = ref(false)

const checkMobile = () => {
  isMobile.value = window.innerWidth < 768
  if (!isMobile.value) {
    mobileOpen.value = false
  }
}
const toggleCollapse = () => {
  collapsed.value = !collapsed.value
}
const toggleMobile = () => {
  mobileOpen.value = !mobileOpen.value
}
const closeMobile = () => {
  mobileOpen.value = false
}

const dashboardMenu = { path: '/dashboard', title: '工作台', icon: 'Odometer' }

const menuGroups = computed(() => {
  const hp = (code) => authStore.hasPerm(code)
  const business = [
    hp('incoming.view') && { path: '/incoming', title: '收文管理', icon: 'FolderOpened' },
    hp('vehicle.view') && { path: '/vehicles', title: '公车管理', icon: 'Van' },
    hp('duty.view') && { path: '/duty', title: '值守排班', icon: 'AlarmClock' },
    hp('calendar.view') && { path: '/calendar', title: '工作日历', icon: 'Calendar' },
    hp('meeting.manage') && { path: '/meetings', title: '会务管理', icon: 'OfficeBuilding' },
    hp('solicit.manage') && { path: '/solicits', title: '征求意见', icon: 'EditPen' },
    hp('dispatch.manage') && { path: '/dispatches', title: '材料下发', icon: 'Promotion' },
    hp('contact.view') && { path: '/contacts', title: '通讯录', icon: 'Phone' }
  ].filter(Boolean)
  const attendance = [
    hp('attendance.view') && { path: '/attendance', title: '考勤点到', icon: 'CircleCheck' },
    hp('leave.view') && { path: '/leave', title: '请假管理', icon: 'Memo' },
    hp('overtime.view') && { path: '/overtime', title: '加班管理', icon: 'Clock' },
    hp('annualleave.view') && { path: '/annualleave', title: '年休假管理', icon: 'Sunny' }
  ].filter(Boolean)
  const materials = [
    hp('event.view') && { path: '/reports', title: '大事记', icon: 'Collection' },
    hp('weekly.view') && { path: '/weekly', title: '每周工作总结', icon: 'Document' },
    hp('study.view') && { path: '/study', title: '公共资料', icon: 'Reading' }
  ].filter(Boolean)
  const groups = [
    { title: '业务办理', icon: 'FolderOpened', items: business },
    { title: '考勤休假', icon: 'Timer', items: attendance },
    { title: '材料报送', icon: 'Tickets', items: materials }
  ].filter(g => g.items.length > 0)
  const sys = [
    hp('standing.manage') && { path: '/standing', title: '常委管理', icon: 'UserFilled' },
    hp('user.manage') && { path: '/users', title: '用户管理', icon: 'User' },
    hp('user.manage') && { path: '/permissions', title: '权限管理', icon: 'Lock' },
    hp('oplog.view') && { path: '/operation-logs', title: '操作日志', icon: 'Files' }
  ].filter(Boolean)
  if (sys.length) {
    groups.push({ title: '系统管理', icon: 'Setting', items: sys })
  }
  return groups
})

const currentTitle = computed(() => route.meta.title || '')

const currentGroupTitle = computed(() => {
  const path = route.path
  for (const group of menuGroups.value) {
    if (group.items.some(item => item.path === path)) {
      return group.title
    }
  }
  return ''
})

const todayStr = computed(() => {
  const now = new Date()
  return now.toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    weekday: 'long'
  })
})

const unread = ref(0)

const loadUnread = async () => {
  try {
    const res = await request.get('/dashboard-stats')
    unread.value = res.pending_incoming || 0
  } catch (e) { console.error(e) }
}

const refreshUnread = () => {
  loadUnread()
}

let watermarkObserver = null

const initWatermark = () => {
  const userStr = localStorage.getItem('user')
  let real_name = '用户'
  let phone = ''
  if (userStr) {
    try {
      const user = JSON.parse(userStr)
      if (user.real_name) real_name = user.real_name
      if (user.phone) phone = String(user.phone).slice(-4)
    } catch (e) {
      console.error('解析水印用户信息失败:', e)
    }
  }
  
  const text1 = '内部系统 严禁外传'
  const text2 = `${real_name} ${phone}`
  const now = new Date()
  const text3 = `${now.getFullYear()}-${String(now.getMonth()+1).padStart(2,'0')}-${String(now.getDate()).padStart(2,'0')}`

  const canvas = document.createElement('canvas')
  canvas.width = 320
  canvas.height = 220
  const ctx = canvas.getContext('2d')
  ctx.translate(160, 110)
  ctx.rotate(-20 * Math.PI / 180)
  ctx.fillStyle = 'rgba(150, 150, 150, 0.15)'
  ctx.font = '16px "Microsoft YaHei"'
  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'
  ctx.fillText(text1, 0, -25)
  ctx.fillText(text2, 0, 0)
  ctx.fillText(text3, 0, 25)

  const bgUrl = canvas.toDataURL('image/png')
  
  const createDiv = () => {
    let div = document.getElementById('sys-watermark')
    if (!div) {
      div = document.createElement('div')
      div.id = 'sys-watermark'
      div.style.position = 'fixed'
      div.style.top = '0'
      div.style.left = '0'
      div.style.width = '100vw'
      div.style.height = '100vh'
      div.style.pointerEvents = 'none'
      div.style.zIndex = '99999'
      document.body.appendChild(div)
    }
    div.style.backgroundImage = `url(${bgUrl})`
    div.style.display = 'block'
    div.style.opacity = '1'
    div.style.visibility = 'visible'
    return div
  }

  let watermarkDiv = createDiv()

  watermarkObserver = new MutationObserver((mutations) => {
    let shouldRestore = false
    for (const m of mutations) {
      if (m.type === 'childList') {
        m.removedNodes.forEach(node => {
          if (node === watermarkDiv || node.id === 'sys-watermark') {
            shouldRestore = true
          }
        })
      } else if (m.type === 'attributes' && m.target === watermarkDiv) {
        if (
          watermarkDiv.style.display === 'none' ||
          watermarkDiv.style.opacity === '0' ||
          watermarkDiv.style.visibility === 'hidden' ||
          watermarkDiv.style.zIndex !== '99999'
        ) {
          shouldRestore = true
        }
      }
    }
    if (shouldRestore) {
      if (watermarkObserver) watermarkObserver.disconnect()
      const oldDiv = document.getElementById('sys-watermark')
      if (oldDiv) oldDiv.remove()
      watermarkDiv = createDiv()
      if (watermarkObserver) watermarkObserver.observe(document.body, { childList: true, subtree: true, attributes: true })
    }
  })
  
  watermarkObserver.observe(document.body, { childList: true, subtree: true, attributes: true })
}

onMounted(() => {
  authStore.refreshProfile()
  loadUnread()
  checkMobile()
  window.addEventListener('resize', checkMobile)
  window.addEventListener('incoming-changed', refreshUnread)
  setTimeout(() => initWatermark(), 500)
})

onUnmounted(() => {
  if (watermarkObserver) {
    watermarkObserver.disconnect()
    watermarkObserver = null
  }
  const oldDiv = document.getElementById('sys-watermark')
  if (oldDiv) {
    oldDiv.remove()
  }
  window.removeEventListener('incoming-changed', refreshUnread)
  window.removeEventListener('resize', checkMobile)
})

const avatarText = computed(() => {
  const name = authStore.user?.real_name || '用户'
  return name.charAt(name.length - 1)
})

const roleText = computed(() => {
  const role = authStore.user?.role_name || authStore.user?.role || '机关干部'
  return role
})

const handleCommand = (cmd) => {
  if (cmd === 'logout') {
    if (watermarkObserver) {
      watermarkObserver.disconnect()
      watermarkObserver = null
    }
    const oldDiv = document.getElementById('sys-watermark')
    if (oldDiv) {
      oldDiv.remove()
    }
    authStore.logout()
    ElMessage.success('已安全退出登录')
    router.replace('/login')
  } else if (cmd === 'profile') {
    router.push('/profile')
  }
}
</script>

<style scoped>
.layout {
  height: 100%;
}

.main-container {
  min-width: 0;
  display: flex;
  flex-direction: column;
}

/* ===== 侧边栏 ===== */
.sidebar {
  background: linear-gradient(180deg, var(--yx-ink) 0%, var(--yx-ink-dark) 100%);
  background-image: 
    linear-gradient(180deg, rgba(27, 36, 51, 0.95) 0%, rgba(19, 26, 38, 0.98) 100%),
    url("data:image/svg+xml,%3Csvg width='120' height='120' viewBox='0 0 120 120' xmlns='http://www.w3.org/2000/svg'%3E%3Cpath d='M0 60 Q 30 50, 60 60 T 120 60' fill='none' stroke='rgba(184,146,90,0.04)' stroke-width='1.5'/%3E%3Cpath d='M0 90 Q 30 80, 60 90 T 120 90' fill='none' stroke='rgba(184,146,90,0.03)' stroke-width='1'/%3E%3C/svg%3E");
  overflow-x: hidden;
  box-shadow: 2px 0 16px rgba(19, 26, 38, 0.14);
  position: relative;
  z-index: 100;
  transition: width 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  display: flex;
  flex-direction: column;
}

/* 顶部 Logo 区 */
.logo {
  height: 72px;
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 12px;
  color: #fff;
  border-bottom: 1px solid rgba(184, 146, 90, 0.24);
  padding: 0 16px;
  background: linear-gradient(180deg, rgba(184, 146, 90, 0.08), transparent);
  transition: all 0.25s ease;
  flex-shrink: 0;
}

.logo.logo-collapsed {
  justify-content: center;
  padding: 0;
}

.logo-badge {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

/* 党徽：完全沿用现有图片与金色光晕 */
.danghui-img {
  width: 38px;
  height: 38px;
  object-fit: contain;
  filter: drop-shadow(0 0 8px rgba(201, 160, 99, 0.55));
}

.logo-text {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
  overflow: hidden;
  white-space: nowrap;
}

.logo-title {
  font-size: 16px;
  color: #ffffff;
  margin: 0 0 3px 0;
  line-height: 1.25;
  letter-spacing: 2px;
  font-weight: 600;
  font-family: var(--yx-font-serif);
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.5);
}

.logo-sub {
  margin: 0;
  font-size: 11px;
  color: var(--yx-gold-light);
  letter-spacing: 3px;
  line-height: 1.2;
  font-weight: 500;
  font-family: var(--yx-font-serif);
}

/* 侧边菜单 */
.sidebar .el-menu {
  border-right: none;
  padding: 10px 8px;
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  transition: all 0.3s ease;
}

.sidebar-menu :deep(.el-menu-item) {
  height: 44px;
  line-height: 44px;
  border-radius: 8px;
  margin-bottom: 4px;
  color: rgba(255, 255, 255, 0.72);
  font-size: var(--yx-fs-sm);
  position: relative;
  transition: all 0.3s ease;
}

.sidebar-menu :deep(.el-menu-item:hover) {
  background: rgba(255, 255, 255, 0.06);
  color: #ffffff;
}

/* 选中项：政务红变体 */
.sidebar-menu :deep(.el-menu-item.is-active) {
  background: rgba(158, 27, 30, 0.16) !important;
  color: #ffffff !important;
  font-weight: 500;
}

.sidebar-menu :deep(.el-menu-item.is-active)::before {
  content: '';
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 3.5px;
  height: 22px;
  border-radius: 0 3px 3px 0;
  background: linear-gradient(180deg, #e53935, #9e1b1e);
}

.sidebar-menu :deep(.el-sub-menu__title) {
  height: 44px;
  line-height: 44px;
  border-radius: 8px;
  margin-bottom: 4px;
  color: rgba(255, 255, 255, 0.65);
  font-size: var(--yx-fs-sm);
  transition: all 0.3s ease;
}

.sidebar-menu :deep(.el-sub-menu__title:hover) {
  background: rgba(255, 255, 255, 0.06);
  color: #ffffff;
}

.sidebar-menu :deep(.el-sub-menu .el-menu) {
  background: transparent;
}

.sidebar-menu :deep(.el-sub-menu .el-menu-item) {
  height: 40px;
  line-height: 40px;
  border-radius: 8px;
  padding-left: 50px !important;
  color: rgba(255, 255, 255, 0.68);
  font-size: var(--yx-fs-sm);
  min-width: 0;
}

.sidebar-menu :deep(.el-sub-menu .el-menu-item.is-active) {
  background: rgba(158, 27, 30, 0.16) !important;
  color: #ffffff !important;
}

/* 侧边栏折叠按钮栏（桌面端底部） */
.sidebar-collapse-bar {
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: rgba(255, 255, 255, 0.6);
  font-size: 13px;
  cursor: pointer;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(0, 0, 0, 0.12);
  transition: color 0.2s ease, background 0.2s ease;
  flex-shrink: 0;
}

.sidebar-collapse-bar:hover {
  color: #ffffff;
  background: rgba(255, 255, 255, 0.06);
}

/* ===== 顶栏 ===== */
.header {
  background: var(--yx-surface);
  border-bottom: 1px solid rgba(220, 223, 229, 0.7);
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 60px;
  padding: 0 24px;
  box-shadow: 0 1px 4px rgba(19, 26, 38, 0.04);
  position: relative;
  z-index: 10;
}

.header::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 2px;
  background: var(--yx-crest-gradient);
  opacity: 0.75;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 14px;
}

.desktop-toggle-btn {
  background: none;
  border: none;
  cursor: pointer;
  color: var(--yx-text-2);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 6px;
  border-radius: 6px;
  transition: all 0.2s ease;
}

.desktop-toggle-btn:hover {
  background: var(--el-fill-color-light);
  color: var(--yx-ink-2);
}

.header-breadcrumb :deep(.el-breadcrumb__inner) {
  color: var(--yx-text-3);
  font-size: var(--yx-fs-sm);
}

.header-breadcrumb :deep(.el-breadcrumb__item:last-child .el-breadcrumb__inner) {
  color: var(--yx-text-1);
  font-weight: 500;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 14px;
}

/* 顶栏全局搜索按钮 */
.header-search-btn {
  display: flex;
  align-items: center;
  gap: 8px;
  background: #f5f6f8;
  border: 1px solid #dcdfe5;
  border-radius: 20px;
  padding: 5px 12px;
  cursor: pointer;
  color: var(--yx-text-3);
  font-size: 13px;
  transition: all 0.2s ease;
  user-select: none;
}

.header-search-btn:hover {
  background: #ffffff;
  border-color: var(--yx-brand);
  color: var(--yx-text-1);
  box-shadow: 0 2px 8px rgba(158, 27, 30, 0.08);
}

.search-btn-icon {
  font-size: 14px;
  color: var(--yx-brand);
}

.search-btn-text {
  font-size: 13px;
  color: var(--yx-text-3);
}

.header-search-btn:hover .search-btn-text {
  color: var(--yx-text-2);
}

.search-btn-shortcut {
  display: flex;
  align-items: center;
  gap: 3px;
  margin-left: 6px;
}

.search-btn-shortcut kbd {
  display: inline-block;
  padding: 1px 5px;
  font-size: 10.5px;
  font-family: var(--yx-font-mono, monospace);
  color: var(--yx-text-3);
  background: #ffffff;
  border: 1px solid #dcdfe5;
  border-radius: 4px;
  box-shadow: 0 1px 0 rgba(0, 0, 0, 0.06);
}

.header-date {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12.5px;
  color: var(--yx-text-2);
  padding: 4px 12px;
  border-radius: 20px;
  background: #fdfaf5;
  border: 1px solid #ebd8be;
  font-weight: 500;
}

.date-icon {
  color: var(--yx-gold);
}

.header-badge-wrap {
  cursor: pointer;
  padding: 4px;
  border-radius: 6px;
  display: flex;
  align-items: center;
}

.header-icon {
  font-size: 19px;
  color: var(--yx-text-2);
  transition: color 0.2s ease;
}

.header-badge-wrap:hover .header-icon {
  color: var(--yx-ink-2);
}

.user-info {
  display: flex;
  align-items: center;
  cursor: pointer;
  gap: 8px;
  padding: 4px 8px;
  border-radius: 6px;
  transition: background 0.2s ease;
}

.user-info:hover {
  background: var(--el-fill-color-light);
}

.user-avatar {
  background: linear-gradient(135deg, var(--yx-ink-2), var(--yx-brand));
  font-size: 13px;
  font-weight: 600;
  color: #fff;
}

.user-meta {
  display: flex;
  flex-direction: column;
  line-height: 1.2;
}

.user-name {
  color: var(--yx-text-1);
  font-size: var(--yx-fs-sm);
  font-weight: 500;
}

.user-role {
  color: var(--yx-text-4);
  font-size: 11px;
}

.dropdown-arrow {
  font-size: 12px;
  color: var(--yx-text-4);
}

/* 主内容区 */
.main {
  background: var(--yx-bg);
  padding: 20px;
  overflow-y: auto;
  min-height: calc(100vh - 60px);
}

/* 响应式适配 */
@media (max-width: 767px) {
  .hidden-xs {
    display: none !important;
  }
  .header {
    padding: 0 10px;
    height: 56px;
  }
  .header-left {
    gap: 8px;
    overflow: hidden;
    flex: 1;
  }
  .mobile-toggle-btn {
    flex-shrink: 0;
  }
  .mobile-page-title {
    font-size: 16px;
    font-weight: 600;
    color: var(--yx-text-1);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    line-height: 1.2;
  }
  .header-right {
    gap: 8px;
    flex-shrink: 0;
  }
  .header-search-btn {
    width: 32px;
    height: 32px;
    padding: 0;
    justify-content: center;
    border-radius: 50%;
    border-color: rgba(220, 223, 229, 0.7);
  }
  .header-search-btn .search-btn-icon {
    font-size: 16px;
  }
  .header-badge-wrap {
    padding: 2px;
  }
  .header-icon {
    font-size: 18px;
  }
  .user-info {
    padding: 2px;
    gap: 2px;
  }
  .user-avatar {
    width: 28px !important;
    height: 28px !important;
    line-height: 28px !important;
    font-size: 12px !important;
  }
  .sidebar {
    width: 260px !important;
    position: fixed;
    left: 0;
    top: 0;
    bottom: 0;
    z-index: 2001;
    transform: translateX(-100%);
    transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);
    box-shadow: 4px 0 24px rgba(19, 26, 38, 0.4);
    -webkit-overflow-scrolling: touch;
  }
  .sidebar-mobile-open {
    transform: translateX(0);
  }
  .mobile-sidebar-close {
    margin-left: auto;
    font-size: 18px;
    color: rgba(255, 255, 255, 0.75);
    cursor: pointer;
    padding: 6px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 4px;
    transition: all 0.2s ease;
  }
  .mobile-sidebar-close:hover {
    color: #fff;
    background: rgba(255, 255, 255, 0.12);
  }
  .sidebar-mask {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    backdrop-filter: blur(2px);
    z-index: 2000;
  }
  .main {
    padding: 10px 8px;
    min-height: calc(100vh - 56px);
    overflow-x: hidden;
  }
}
</style>
