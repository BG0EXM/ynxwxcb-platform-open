import { ref, onMounted, onBeforeUnmount } from 'vue'

export function useMobile(breakpoint = 768) {
  const isMobile = ref(typeof window !== 'undefined' ? window.innerWidth <= breakpoint : false)

  const onResize = () => {
    if (typeof window !== 'undefined') {
      isMobile.value = window.innerWidth <= breakpoint
    }
  }

  onMounted(() => {
    onResize()
    window.addEventListener('resize', onResize)
  })

  onBeforeUnmount(() => {
    window.removeEventListener('resize', onResize)
  })

  return { isMobile }
}
