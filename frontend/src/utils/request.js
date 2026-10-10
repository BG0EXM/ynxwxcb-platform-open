import axios from 'axios'
import { ElMessage } from 'element-plus'
import { showWafAlert } from './wafAlert'
import { showSecrecyAlert } from './secrecyGuard'
import router from '../router'
import { useAuthStore } from '../store/auth'

const request = axios.create({
  baseURL: '/api',
  timeout: 30000
})

let isRedirectingToLogin = false

const isPublicPage = (path = '') => {
  return ['/meeting', '/solicit', '/dispatch'].some(prefix => path.startsWith(prefix))
}

const handleUnauthorized = (error) => {
  const currentPath = router.currentRoute.value?.path || ''
  if (isPublicPage(currentPath)) {
    // 公开填报页面（以 /meeting, /solicit, /dispatch 等开头），严禁跳转 /login，仅返回 Promise.reject
    return
  }

  if (!isRedirectingToLogin) {
    isRedirectingToLogin = true
    try {
      const authStore = useAuthStore()
      authStore.logout()
    } catch (_) {
      localStorage.removeItem('token')
      localStorage.removeItem('user')
      localStorage.removeItem('must_change')
    }
    ElMessage.error('登录已过期，请重新登录')
    if (currentPath !== '/login') {
      router.push('/login').finally(() => {
        setTimeout(() => {
          isRedirectingToLogin = false
        }, 1000)
      })
    } else {
      setTimeout(() => {
        isRedirectingToLogin = false
      }, 1000)
    }
  }
}

async function parseBlobResponseData(resOrError) {
  const data = resOrError?.data || resOrError?.response?.data
  if (data instanceof Blob) {
    try {
      const text = await data.text()
      return JSON.parse(text)
    } catch (_) {
      return null
    }
  }
  return typeof data === 'object' ? data : null
}

request.interceptors.request.use(config => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

request.interceptors.response.use(
  response => response.data,
  error => {
    const status = error.response?.status
    const msg = error.response?.data?.error || error.message
    const code = error.response?.data?.code
    const secViolation = error.response?.data?.security_violation

    // 1. 保密防线拦截（涉密文件上传熔断）
    if (status === 403 && (secViolation === 'SECRECY_LEAK_PREVENTED' || code === 'SECRECY_BLOCK')) {
      showSecrecyAlert({
        fileName: error.response?.data?.filename || '上传文件',
        keyword: error.response?.data?.matched_rule || '国家秘密/内部级标识',
        isPublic: !localStorage.getItem('token')
      })
      return Promise.reject(error)
    }

    // 2. WAF 防火墙拦截
    if (status === 403 && code === 'WAF_BLOCK') {
      showWafAlert()
      return Promise.reject(error)
    }

    // 3. 401 认证失效处理
    if (status === 401) {
      handleUnauthorized(error)
      return Promise.reject(error)
    }

    ElMessage.error(msg || '请求失败')
    return Promise.reject(error)
  }
)

// 下载导出文件（Excel），触发浏览器下载
export async function exportFile(url, params = {}, defaultName = '导出.xlsx') {
  const token = localStorage.getItem('token')
  try {
    // 用全局 axios 获取 blob（不走 JSON 响应拦截器）
    const res = await axios.get(url, {
      baseURL: '/api',
      params,
      responseType: 'blob',
      headers: { Authorization: `Bearer ${token}` },
      timeout: 60000
    })
    // 检查是否为 JSON 错误响应（如 401 时后端返回 {"error":...}）
    if (res.data && res.data.type === 'application/json') {
      const err = await parseBlobResponseData(res)
      if (err) {
        if (err.security_violation === 'SECRECY_LEAK_PREVENTED' || err.code === 'SECRECY_BLOCK') {
          showSecrecyAlert({
            fileName: err.filename || defaultName,
            keyword: err.matched_rule || '国家秘密/内部级标识',
            isPublic: !localStorage.getItem('token')
          })
          return
        }
        ElMessage.error(err.error || '导出失败')
        return
      }
    }
    // 从 Content-Disposition 提取文件名
    let fileName = defaultName
    const cd = res.headers['content-disposition']
    if (cd) {
      const matchUtf8 = cd.match(/filename\*=UTF-8''([^;]+)/i)
      const matchNormal = cd.match(/filename=(?:"([^"]+)"|([^;\n]+))/i)
      if (matchUtf8 && matchUtf8[1]) {
        try {
          fileName = decodeURIComponent(matchUtf8[1])
        } catch (e) {
          fileName = defaultName
        }
      } else if (matchNormal) {
        fileName = (matchNormal[1] || matchNormal[2] || defaultName).trim()
      }
    }
    // 用 FileReader 兼容方式触发下载（更稳）
    const blob = new Blob([res.data], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' })
    const link = document.createElement('a')
    link.href = URL.createObjectURL(blob)
    link.download = fileName
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    setTimeout(() => URL.revokeObjectURL(link.href), 1000)
    ElMessage.success('导出成功')
  } catch (e) {
    let errData = null
    if (e.response?.data instanceof Blob) {
      errData = await parseBlobResponseData(e)
    } else if (e.response?.data && typeof e.response.data === 'object') {
      errData = e.response.data
    }

    const status = e.response?.status
    const secViolation = errData?.security_violation
    const code = errData?.code

    if (status === 403 && (secViolation === 'SECRECY_LEAK_PREVENTED' || code === 'SECRECY_BLOCK')) {
      showSecrecyAlert({
        fileName: errData?.filename || defaultName,
        keyword: errData?.matched_rule || '国家秘密/内部级标识',
        isPublic: !localStorage.getItem('token')
      })
      return
    }

    if (status === 403 && code === 'WAF_BLOCK') {
      showWafAlert()
      return
    }

    if (status === 401) {
      handleUnauthorized(e)
      return
    }

    const msg = errData?.error || e.message || '导出失败，请重试'
    ElMessage.error(msg)
  }
}

export default request

// 通用文件下载（带 token，用于附件等需要鉴权的文件）
export async function downloadFile(url, defaultName = '下载文件') {
  const token = localStorage.getItem('token')
  try {
    const res = await axios.get(url, {
      baseURL: '/api',
      responseType: 'blob',
      headers: { Authorization: `Bearer ${token}` },
      timeout: 60000
    })
    if (res.data && res.data.type === 'application/json') {
      const err = await parseBlobResponseData(res)
      if (err) {
        if (err.security_violation === 'SECRECY_LEAK_PREVENTED' || err.code === 'SECRECY_BLOCK') {
          showSecrecyAlert({
            fileName: err.filename || defaultName,
            keyword: err.matched_rule || '国家秘密/内部级标识',
            isPublic: !localStorage.getItem('token')
          })
          return
        }
        ElMessage.error(err.error || '下载失败')
        return
      }
    }
    let fileName = defaultName
    const cd = res.headers['content-disposition']
    if (cd) {
      const matchUtf8 = cd.match(/filename\*=UTF-8''([^;]+)/i)
      const matchNormal = cd.match(/filename=(?:"([^"]+)"|([^;\n]+))/i)
      if (matchUtf8 && matchUtf8[1]) {
        try {
          fileName = decodeURIComponent(matchUtf8[1])
        } catch (e) {
          fileName = defaultName
        }
      } else if (matchNormal) {
        fileName = (matchNormal[1] || matchNormal[2] || defaultName).trim()
      }
    }
    const blob = new Blob([res.data], { type: res.headers['content-type'] || 'application/octet-stream' })
    const link = document.createElement('a')
    link.href = URL.createObjectURL(blob)
    link.download = fileName
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    setTimeout(() => URL.revokeObjectURL(link.href), 1000)
  } catch (e) {
    let errData = null
    if (e.response?.data instanceof Blob) {
      errData = await parseBlobResponseData(e)
    } else if (e.response?.data && typeof e.response.data === 'object') {
      errData = e.response.data
    }

    const status = e.response?.status
    const secViolation = errData?.security_violation
    const code = errData?.code

    if (status === 403 && (secViolation === 'SECRECY_LEAK_PREVENTED' || code === 'SECRECY_BLOCK')) {
      showSecrecyAlert({
        fileName: errData?.filename || defaultName,
        keyword: errData?.matched_rule || '国家秘密/内部级标识',
        isPublic: !localStorage.getItem('token')
      })
      return
    }

    if (status === 403 && code === 'WAF_BLOCK') {
      showWafAlert()
      return
    }

    if (status === 401) {
      handleUnauthorized(e)
      return
    }

    const msg = errData?.error || e.message || '下载失败，请重试'
    ElMessage.error(msg)
  }
}
