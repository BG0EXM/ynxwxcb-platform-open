import { reactive } from 'vue'
import request from './request'

// 今日待办：工作台面板与顶栏铃铛共用一份数据。
// 2 秒内的重复请求复用同一结果；非强制刷新时 60 秒内不重复拉取（切换页面不必每次都查）。
export const todoState = reactive({
  total: 0,
  groups: [],
  loading: false,
  loaded: false
})

let pending = null
let lastAt = 0

export function loadTodos({ force = false } = {}) {
  const now = Date.now()
  if (pending && now - lastAt < 2000) return pending
  if (!force && todoState.loaded && now - lastAt < 60000) return Promise.resolve(todoState)
  lastAt = now
  todoState.loading = true
  pending = request.get('/dashboard/todos')
    .then((res) => {
      if (!res || typeof res !== 'object' || !Array.isArray(res.groups)) {
        throw new Error('待办数据格式异常')
      }
      todoState.total = res.total || 0
      todoState.groups = res.groups || []
      todoState.loaded = true
      return todoState
    })
    .catch((e) => {
      console.error(e)
      pending = null
      throw e
    })
    .finally(() => {
      todoState.loading = false
    })
  return pending
}
