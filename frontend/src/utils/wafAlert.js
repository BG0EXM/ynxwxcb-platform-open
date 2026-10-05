import { createVNode, render } from 'vue'
import WafAlertComponent from '../components/WafAlert.vue'

let vnode = null
let container = null

export const showWafAlert = () => {
  if (vnode) return // Already showing

  container = document.createElement('div')
  document.body.appendChild(container)

  vnode = createVNode(WafAlertComponent)
  render(vnode, container)

  // Give it a small tick to mount, then open
  setTimeout(() => {
    if (vnode.component && vnode.component.exposed) {
      vnode.component.exposed.open(() => {
        // cleanup after close
        setTimeout(() => {
          if (container) {
            render(null, container)
            if (document.body.contains(container)) {
              document.body.removeChild(container)
            }
            container = null
            vnode = null
          }
        }, 300) // wait for transition
      })
    }
  }, 10)
}
