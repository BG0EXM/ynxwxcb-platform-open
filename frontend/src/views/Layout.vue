<template>
  <el-container class="layout">
    <el-aside width="220px" class="sidebar" :class="{ 'sidebar-collapsed': collapsed, 'sidebar-mobile-open': mobileOpen }">
      <div class="logo">
        <div class="logo-badge">
          <img src="../assets/danghui.png" alt="党徽" class="danghui-img" />
        </div>
        <div class="logo-text">
          <h2>伊宁县委宣传部</h2>
          <p>部务工作平台</p>
        </div>
      </div>
      <el-menu :default-active="$route.path" router background-color="transparent"
        text-color="rgba(255,255,255,0.7)" active-text-color="#ffffff" class="sidebar-menu">
        <!-- 工作台单独置顶 -->
        <el-menu-item :index="dashboardMenu.path" @click="closeMobile">
          <el-icon><component :is="dashboardMenu.icon" /></el-icon>
          <span>{{ dashboardMenu.title }}</span>
        </el-menu-item>
        <!-- 分组菜单 -->
        <template v-for="group in menuGroups" :key="group.title">
          <el-sub-menu :index="group.title">
            <template #title>
              <el-icon><component :is="group.icon" /></el-icon>
              <span>{{ group.title }}</span>
            </template>
            <el-menu-item v-for="item in group.items" :key="item.path" :index="item.path" @click="closeMobile">
              <el-icon><component :is="item.icon" /></el-icon>
              <span>{{ item.title }}</span>
            </el-menu-item>
          </el-sub-menu>
        </template>
      </el-menu>
    </el-aside>
    <div v-if="mobileOpen" class="sidebar-mask" @click="closeMobile"></div>
    <el-container>
      <el-header class="header">
        <div class="header-left">
          <el-button class="menu-toggle" :icon="'Expand'" @click="toggleMobile" circle size="small" />
          <el-breadcrumb separator="/">
            <el-breadcrumb-item :to="{ path: '/dashboard' }">首页</el-breadcrumb-item>
            <el-breadcrumb-item>{{ currentTitle }}</el-breadcrumb-item>
          </el-breadcrumb>
        </div>
        <div class="header-right">
          <el-badge :value="unread" :hidden="!unread" class="unread-badge">
            <el-icon class="header-icon" @click="$router.push('/incoming')"><Bell /></el-icon>
          </el-badge>
          <el-dropdown @command="handleCommand">
            <span class="user-info">
              <el-avatar :size="30" class="user-avatar">{{ avatarText }}</el-avatar>
              <span class="user-name">{{ authStore.user?.real_name }}</span>
              <el-icon><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="profile">个人中心</el-dropdown-item>
                <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>
      <el-main class="main">
        <router-view />
      </el-main>
      <el-footer class="app-footer" height="40px">
        <a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener">ICP备案号占位</a>
        <span class="footer-sep">|</span>
        <a href="https://beian.mps.gov.cn/#/query/webSearch" target="_blank" rel="noopener">
          公网安备号占位
        </a>
      </el-footer>
    </el-container>
  </el-container>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../store/auth'
import request from '../utils/request'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const collapsed = ref(false)
const mobileOpen = ref(false)
const isMobile = ref(false)

const checkMobile = () => {
  isMobile.value = window.innerWidth < 768
  if (!isMobile.value) mobileOpen.value = false
}
const toggleMobile = () => {
  mobileOpen.value = !mobileOpen.value
}
const closeMobile = () => {
  mobileOpen.value = false
}

const dashboardMenu = { path: '/dashboard', title: '工作台', icon: 'Odometer' }

const menuGroups = computed(() => {
  const groups = [
    {
      title: '业务办理', icon: 'FolderOpened', items: [
        { path: '/incoming', title: '收文管理', icon: 'FolderOpened' },
        { path: '/vehicles', title: '公车管理', icon: 'Van' },
        { path: '/duty', title: '值守排班', icon: 'AlarmClock' },
        { path: '/calendar', title: '工作日历', icon: 'Calendar' },
        { path: '/meetings', title: '会务管理', icon: 'OfficeBuilding' },
        { path: '/contacts', title: '通讯录', icon: 'Phone' }
      ]
    },
    {
      title: '考勤休假', icon: 'AlarmClock', items: [
        { path: '/attendance', title: '考勤点到', icon: 'AlarmClock' },
        { path: '/leave', title: '请假管理', icon: 'Calendar' },
        { path: '/overtime', title: '加班管理', icon: 'Clock' },
        { path: '/annualleave', title: '年休假管理', icon: 'Sunny' }
      ]
    },
    {
      title: '材料报送', icon: 'Document', items: [
        { path: '/reports', title: '大事记', icon: 'Tickets' },
        { path: '/weekly', title: '每周工作总结', icon: 'Document' },
        { path: '/study', title: '公共资料', icon: 'Reading' }
      ]
    }
  ]
  if (authStore.isAdmin) {
    groups.push({
      title: '系统管理', icon: 'Setting', items: [
        { path: '/standing', title: '常委管理', icon: 'UserFilled' },
        { path: '/users', title: '用户管理', icon: 'User' }
      ]
    })
  }
  return groups
})

const currentTitle = computed(() => route.meta.title || '')

const unread = ref(0)

const loadUnread = async () => {
  try {
    const res = await request.get('/dashboard-stats')
    unread.value = res.pending_incoming || 0
  } catch (e) {}
}

const refreshUnread = () => {
  loadUnread()
}

onMounted(() => {
  loadUnread()
  checkMobile()
  window.addEventListener('resize', checkMobile)
  // 事件驱动刷新：收文操作后触发
  window.addEventListener('incoming-changed', refreshUnread)
})
onUnmounted(() => {
  window.removeEventListener('incoming-changed', refreshUnread)
  window.removeEventListener('resize', checkMobile)
})

const avatarText = computed(() => {
  const name = authStore.user?.real_name || '用户'
  return name.charAt(name.length - 1)
})

const handleCommand = (cmd) => {
  if (cmd === 'logout') {
    authStore.logout()
    router.push('/login')
  } else if (cmd === 'profile') {
    router.push('/profile')
  }
}
</script>

<style scoped>
.layout {
  height: 100%;
}
.sidebar {
  background: linear-gradient(180deg, #1a2535 0%, #141c28 100%);
  overflow-x: hidden;
  box-shadow: 4px 0 24px rgba(20, 28, 40, 0.15);
  position: relative;
  z-index: 100;
}
.logo {
  height: 72px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  color: #fff;
  border-bottom: 1px solid rgba(201, 160, 99, 0.25);
  padding: 0 8px;
  background: linear-gradient(180deg, rgba(201,160,99,0.08), transparent);
}
.logo-badge {
  flex-shrink: 0;
  display: flex;
  align-items: center;
}
.danghui-img {
  width: 38px;
  height: 38px;
  object-fit: contain;
  filter: drop-shadow(0 0 6px rgba(201, 160, 99, 0.5));
}
.logo-text {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
}
.logo h2 {
  font-size: 15px;
  color: #fff;
  margin-bottom: 2px;
  line-height: 1.2;
  white-space: nowrap;
  letter-spacing: 1px;
}
.logo p {
  font-size: 10px;
  color: var(--yx-gold-light);
  letter-spacing: 3px;
  line-height: 1.2;
}
.sidebar .el-menu {
  border-right: none;
  padding: 10px 8px;
}
.sidebar-menu :deep(.el-menu-item) {
  height: 44px;
  line-height: 44px;
  border-radius: 8px;
  margin-bottom: 4px;
  color: rgba(255, 255, 255, 0.72);
  position: relative;
  transition: all 0.2s ease;
}
.sidebar-menu :deep(.el-menu-item:hover) {
  background: rgba(255, 255, 255, 0.08);
  color: #fff;
}
.sidebar-menu :deep(.el-menu-item.is-active) {
  background: linear-gradient(90deg, rgba(200, 16, 46, 0.85), rgba(200, 16, 46, 0.55));
  color: #fff !important;
  box-shadow: 0 4px 14px rgba(200, 16, 46, 0.35);
}
.sidebar-menu :deep(.el-menu-item.is-active)::before {
  content: '';
  position: absolute;
  left: -8px;
  top: 50%;
  transform: translateY(-50%);
  width: 4px;
  height: 24px;
  border-radius: 0 4px 4px 0;
  background: linear-gradient(180deg, var(--yx-gold-light), var(--yx-gold));
}
/* 分组子菜单：标题与子项在深色侧边栏下的配色 */
.sidebar-menu :deep(.el-sub-menu__title) {
  height: 44px;
  line-height: 44px;
  border-radius: 8px;
  margin-bottom: 4px;
  color: rgba(255, 255, 255, 0.6);
  font-size: 13px;
}
.sidebar-menu :deep(.el-sub-menu__title:hover) {
  background: rgba(255, 255, 255, 0.08);
  color: #fff;
}
.sidebar-menu :deep(.el-sub-menu .el-menu) {
  background: transparent;
}
.sidebar-menu :deep(.el-sub-menu .el-menu-item) {
  height: 40px;
  line-height: 40px;
  border-radius: 8px;
  padding-left: 52px !important;
  color: rgba(255, 255, 255, 0.62);
  font-size: 13px;
  min-width: 0;
}
.sidebar-menu :deep(.el-sub-menu .el-menu-item.is-active) {
  background: linear-gradient(90deg, rgba(200, 16, 46, 0.85), rgba(200, 16, 46, 0.55));
  color: #fff !important;
}
.header {
  background: #fff;
  border-bottom: 1px solid #e4e7ed;
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 64px;
}
.header-icon {
  font-size: 20px;
  cursor: pointer;
  color: #606266;
  margin-right: 20px;
}
.user-info {
  display: flex;
  align-items: center;
  cursor: pointer;
  gap: 8px;
}
.user-avatar {
  background: #c8102e;
}
.user-name {
  color: #303133;
  font-size: 14px;
}
.main {
  background: #f0f2f5;
  padding: 16px;
}
.unread-badge {
  margin-right: 8px;
}
.app-footer {
  background: #fff;
  border-top: 1px solid #e4e7ed;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  font-size: 12px;
  color: #909399;
}
.app-footer a {
  color: #909399;
  text-decoration: none;
}
.app-footer a:hover {
  color: #409eff;
}
.footer-sep {
  color: #c0c4cc;
}
.menu-toggle {
  display: none;
}
@media (max-width: 767px) {
  .menu-toggle {
    display: inline-flex;
    margin-right: 8px;
  }
  .layout {
    position: relative;
  }
  .sidebar {
    position: fixed;
    left: 0;
    top: 0;
    bottom: 0;
    z-index: 1001;
    transform: translateX(-100%);
    transition: transform 0.25s ease;
  }
  .sidebar-mobile-open {
    transform: translateX(0);
  }
  .sidebar-mask {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.4);
    z-index: 1000;
  }
  .header {
    padding: 0 10px;
  }
  .user-name {
    max-width: 80px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .logo-text h2 {
    font-size: 13px;
  }
}
</style>
