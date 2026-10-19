import request from './request'

// 工作台统计数据：布局（待办角标）与工作台页面同时加载时只发一次请求，2 秒内复用同一结果
let pending = null
let pendingAt = 0

export function fetchDashboardStats({ force = false } = {}) {
  const now = Date.now()
  if (!force && pending && now - pendingAt < 2000) {
    return pending
  }
  pendingAt = now
  pending = request.get('/dashboard-stats').catch((e) => {
    // 失败结果不缓存，下次重新请求
    pending = null
    throw e
  })
  return pending
}
