import { defineStore } from 'pinia'
import request from '../utils/request'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('token') || '',
    user: JSON.parse(localStorage.getItem('user') || 'null'),
    mustChange: localStorage.getItem('must_change') === '1'
  }),
  getters: {
    isAdmin: (state) => state.user?.role_code === 'admin',
    isLeader: (state) => state.user?.role_code === 'leader',
    canReview: (state) => state.user?.role_code === 'admin' || state.user?.role_code === 'leader',
    isLoggedIn: (state) => !!state.token,
    permissions: (state) => state.user?.permissions || [],
    // 是否拥有某权限点（admin 始终通过）
    hasPerm: (state) => (code) => {
      if (state.user?.role_code === 'admin') return true
      return (state.user?.permissions || []).includes(code)
    }
  },
  actions: {
    // 刷新当前用户资料（含权限列表），用于旧会话补齐 permissions
    async refreshProfile() {
      try {
        const res = await request.get('/auth/profile')
        this.user = res
        localStorage.setItem('user', JSON.stringify(res))
      } catch (e) { console.error(e) }
    },
    async login(username, password) {
      const res = await request.post('/auth/login', { username, password })
      this.token = res.token
      this.user = res.user
      this.mustChange = !!res.must_change
      localStorage.setItem('token', res.token)
      localStorage.setItem('user', JSON.stringify(res.user))
      localStorage.setItem('must_change', res.must_change ? '1' : '0')
      return res
    },
    setMustChange(val) {
      this.mustChange = val
      localStorage.setItem('must_change', val ? '1' : '0')
    },
    logout() {
      const oldToken = this.token || localStorage.getItem('token')
      // 1. 立即同步清空本地响应式状态与持久化缓存，确保路由守卫立即判定为未登录
      this.token = ''
      this.user = null
      this.mustChange = false
      localStorage.removeItem('token')
      localStorage.removeItem('user')
      localStorage.removeItem('must_change')

      // 2. 异步通知后端递增令牌版本使服务端失效（后台执行，不阻塞界面跳转）
      if (oldToken) {
        request.post('/auth/logout', {}, {
          headers: { Authorization: `Bearer ${oldToken}` }
        }).catch(e => console.error(e))
      }
    }
  }
})
