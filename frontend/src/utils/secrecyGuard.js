import { createVNode, render } from 'vue'
import SecrecyAlertComponent from '../components/SecrecyAlert.vue'

// 涉密特征检测规则库
const SECRECY_PATTERNS = [
  // 1. 国家秘密核心标识（绝密、机密、秘密、★）
  { regex: /绝密/i, level: '国家秘密 · 绝密级 (TOP SECRET)' },
  { regex: /机密/i, level: '国家秘密 · 机密级 (CONFIDENTIAL)' },
  { regex: /秘密/i, level: '国家秘密 · 秘密级 (SECRET)' },
  { regex: /[0-9一二三四五六七八九十]+年★/, level: '国家秘密保密期限标识 (★)' },
  { regex: /长期★/, level: '国家秘密长期保密标识 (★)' },
  { regex: /★[0-9一二三四五六七八九十]+年/, level: '国家秘密保密期限标识 (★)' },
  { regex: /密级[：:]/, level: '公文密级标识' },
  { regex: /保密期限[：:]/, level: '保密期限标识' },
  { regex: /定密依据/, level: '定密依据标识' },

  // 2. 工作秘密与内部级文件资料
  { regex: /工作秘密/i, level: '工作秘密 (WORK SECRET)' },
  { regex: /内部资料/i, level: '内部级文件资料 (INTERNAL ONLY)' },
  { regex: /内部文件/i, level: '内部级文件资料 (INTERNAL ONLY)' },
  { regex: /内部掌握/i, level: '内部级文件资料 (INTERNAL ONLY)' },
  { regex: /内部参考/i, level: '内部级文件资料 (INTERNAL ONLY)' },
  { regex: /内部使用/i, level: '内部级文件资料 (INTERNAL ONLY)' },
  { regex: /不得外传/i, level: '涉密/敏感管控 (RESTRICTED)' },
  { regex: /严禁外传/i, level: '涉密/敏感管控 (RESTRICTED)' },
  { regex: /非公开发布/i, level: '非公开控制文件' },
  { regex: /非公开/i, level: '非公开控制文件' },
  { regex: /涉密/i, level: '涉密标识文件 (CLASSIFIED)' }
]

/**
 * 校验文件名或文本是否包含涉密特征
 * @param {string} fileName 
 * @param {string} textSample 
 * @returns {{ violated: boolean, level: string, keyword: string }}
 */
export const checkSecrecyViolation = (fileName = '', textSample = '') => {
  const target = `${fileName} ${textSample}`.replace(/[\s\-_]/g, '')
  for (const item of SECRECY_PATTERNS) {
    if (item.regex.test(target)) {
      const match = target.match(item.regex)
      return {
        violated: true,
        level: item.level,
        keyword: match ? match[0] : item.level
      }
    }
  }
  return { violated: false, level: '', keyword: '' }
}

let vnode = null
let container = null

/**
 * 清理涉密警报 DOM 与 VNode 实例
 */
export const cleanupSecrecyAlert = () => {
  if (container) {
    render(null, container)
    if (document.body.contains(container)) {
      document.body.removeChild(container)
    }
    container = null
    vnode = null
  }
}

/**
 * 弹出全屏震撼保密警报并锁定当前会话
 * @param {Object} options { fileName, keyword, isPublic, onLock, onClose }
 */
export const showSecrecyAlert = (options = {}) => {
  if (vnode) return // 已有弹窗显示中

  container = document.createElement('div')
  document.body.appendChild(container)

  vnode = createVNode(SecrecyAlertComponent)
  render(vnode, container)

  setTimeout(() => {
    if (vnode && vnode.component && vnode.component.exposed) {
      vnode.component.exposed.open(
        options,
        () => {
          // 锁定会话核心逻辑
          if (options.isPublic) {
            // 公开端：清空表单并锁定会话状态
            console.warn('[SecrecyGuard] 公开端涉密阻断，锁定当前会话')
          } else {
            // 管理后台：直接清除认证凭证，使现有 JWT 立即失效
            console.warn('[SecrecyGuard] 管理后台涉密阻断，会话强制锁定并清空令牌')
            localStorage.removeItem('token')
            localStorage.removeItem('user')
          }
        },
        () => {
          // 警报关闭或退出时的清理逻辑
          cleanupSecrecyAlert()
          if (typeof options.onClose === 'function') {
            options.onClose()
          }
        }
      )
    }
  }, 10)
}

/**
 * 上传前涉密预检助手（用于 Element Plus before-upload）
 * @param {File} file 
 * @param {Object} options { isPublic: boolean }
 * @returns {boolean} true=通过，false=涉密拦截并报警锁定
 */
export const validateUploadFileSecrecy = (file, options = {}) => {
  if (!file) return true
  const check = checkSecrecyViolation(file.name || '')
  if (check.violated) {
    showSecrecyAlert({
      fileName: file.name,
      keyword: `${check.level} (命中关键字: ${check.keyword})`,
      isPublic: !!options.isPublic
    })
    return false
  }
  return true
}

/**
 * 集中处理上传失败事件（专门拦截后端 PDF/Word 内文涉密检测响应）
 * @param {Error|Object} err 
 * @param {File} file 
 * @param {Object} options { isPublic: boolean }
 * @returns {boolean} true=命中涉密阻断并已显示全屏雷达警报, false=普通错误
 */
export const handleSecrecyUploadError = (err, file, options = {}) => {
  let data = null
  try {
    if (err && err.message) {
      data = JSON.parse(err.message)
    }
  } catch (e) {
    // 忽略非 JSON
  }
  if (!data && err?.response?.data) {
    data = err.response.data
  }
  if (data?.security_violation === 'SECRECY_LEAK_PREVENTED' || data?.code === 'SECRECY_BLOCK' || (data?.error && data.error.includes('国家保密安全警报'))) {
    showSecrecyAlert({
      fileName: data.filename || file?.name || '涉密文件',
      keyword: data.matched_rule ? `${data.matched_rule} ${data.detail ? '(' + data.detail + ')' : ''}` : (data.error || '国家秘密/内部级标识'),
      isPublic: !!options.isPublic
    })
    return true
  }
  return false
}
