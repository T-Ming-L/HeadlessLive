import { reactive } from 'vue'

// 全局状态（无 Pinia，保持轻量）
export const state = reactive({
  sources: [],
  scenes: [],
  outputs: [],
  currentSceneId: '',
  outputStatus: {},        // outputID -> { state, fps, bitrate, uptime, ... }
  previewRunning: false,
  connected: false,        // WebSocket 连接状态
  logs: [],
  // 选中项
  selectedSourceId: null,   // 源面板选中（源 ID）
  selectedItem: null,       // 画布选中（SceneItem 对象引用，支持同一源多实例）
})

// UI 开关状态
export const ui = reactive({
  showAddSource: false,
  showOutputs: false,
  showLogs: false,
  showBili: false,
})

// ---- 便捷 getter ----

export function currentScene() {
  return state.scenes.find(s => s.id === state.currentSceneId) || state.scenes[0] || null
}

export function findSource(id) {
  return state.sources.find(s => s.id === id) || null
}

export function currentSceneItems() {
  const sc = currentScene()
  return sc ? sc.items : []
}

export function sourceTypeLabel(t) {
  const map = {
    video_device: '📹 采集卡',
    audio_device: '🎙 USB声卡',
    image: '🖼 图片',
    text: '📝 文字',
    media_file: '🎬 媒体文件',
    screen: '🖥 屏幕',
    rtmp_source: '📡 RTMP拉流',
    color: '🎨 纯色',
  }
  return map[t] || t
}
