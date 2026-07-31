// 轻量 toast 提示
export function toast(msg, type = '') {
  let wrap = document.querySelector('.toast-wrap')
  if (!wrap) {
    wrap = document.createElement('div')
    wrap.className = 'toast-wrap'
    document.body.appendChild(wrap)
  }
  const el = document.createElement('div')
  el.className = `toast ${type}`
  el.textContent = msg
  wrap.appendChild(el)
  setTimeout(() => el.remove(), 3500)
}
