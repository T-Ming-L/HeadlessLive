import { state } from './store'
import { refreshStatus } from './api'

let ws = null
let reconnectTimer = null

// WebSocket 客户端：日志 + 输出状态实时推送
export function connectWS() {
  if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) return
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  ws = new WebSocket(`${proto}://${location.host}/ws`)

  ws.onopen = () => {
    state.connected = true
    // 重连后同步一次全量数据
    refreshStatus().catch(() => {})
  }
  ws.onclose = () => {
    state.connected = false
    reconnectTimer = setTimeout(connectWS, 3000)
  }
  ws.onmessage = (ev) => {
    let msg
    try { msg = JSON.parse(ev.data) } catch (_) { return }
    handle(msg)
  }
}

function handle(msg) {
  switch (msg.type) {
    case 'log':
      pushLog(msg.data)
      break
    case 'output_log':
      pushLog(`[${msg.data.output_id}] ${msg.data.line}`)
      break
    case 'output_status':
      state.outputStatus[msg.data.output_id] = msg.data
      break
    case 'scene_changed':
      state.currentSceneId = msg.data.scene_id
      break
    case 'preview_stopped':
      state.previewRunning = false
      pushLog('预览已停止')
      break
  }
}

function pushLog(line) {
  state.logs.push(line)
  if (state.logs.length > 500) state.logs.shift()
}
