import { state } from './store'

// 拉取全量状态并写入全局 state（启动/重连/关键操作后调用）
export async function refreshStatus() {
  const data = await request('GET', '/api/status')
  state.sources = data.sources || []
  state.scenes = data.scenes || []
  state.outputs = data.outputs || []
  state.currentSceneId = data.current_scene || (data.scenes && data.scenes[0]?.id) || ''
  state.outputStatus = data.output_status || {}
  state.previewRunning = data.preview_running || false
  // 场景对象被替换后，旧选中引用失效，清理避免画布无法选中/拖动
  if (
    !(data.scenes || []).some((s) =>
      (s.items || []).includes(state.selectedItem),
    )
  ) {
    state.selectedItem = null
  }
  return data
}

// REST API 封装
async function request(method, path, body) {
  const opts = { method }
  if (body !== undefined) {
    opts.headers = { 'Content-Type': 'application/json' }
    opts.body = JSON.stringify(body)
  }
  const res = await fetch(path, opts)
  let data = null
  try { data = await res.json() } catch (_) { /* 非 JSON */ }
  if (!res.ok) {
    throw new Error((data && data.error) || `请求失败 ${res.status}`)
  }
  return data
}

export const api = {
  get: (p) => request('GET', p),
  post: (p, b) => request('POST', p, b),
  put: (p, b) => request('PUT', p, b),
  del: (p) => request('DELETE', p),

  // 全量状态
  status: () => request('GET', '/api/status'),

  // 源
  createSource: (s) => request('POST', '/api/sources', s),
  updateSource: (id, s) => request('PUT', `/api/sources/${id}`, s),
  deleteSource: (id) => request('DELETE', `/api/sources/${id}`),
  probeSource: (id) => request('POST', `/api/sources/${id}/probe`),

  // 场景
  createScene: (s) => request('POST', '/api/scenes', s),
  updateScene: (id, s) => request('PUT', `/api/scenes/${id}`, s),
  deleteScene: (id) => request('DELETE', `/api/scenes/${id}`),
  activateScene: (id) => request('POST', `/api/scenes/${id}/activate`),

  // 输出
  createOutput: (o) => request('POST', '/api/outputs', o),
  updateOutput: (id, o) => request('PUT', `/api/outputs/${id}`, o),
  deleteOutput: (id) => request('DELETE', `/api/outputs/${id}`),
  startOutput: (id) => request('POST', `/api/outputs/${id}/start`),
  stopOutput: (id) => request('POST', `/api/outputs/${id}/stop`),

  // 预览
  previewStart: (sceneId, maxW) => request('POST', '/api/preview/start', { scene_id: sceneId, max_w: maxW }),
  previewStop: () => request('POST', '/api/preview/stop'),

  // 设备
  videoDevices: () => request('GET', '/api/devices/video'),
  audioDevices: () => request('GET', '/api/devices/audio'),
  controls: (dev) => request('GET', `/api/devices/${dev}/controls`),

  // 上传
  upload: async (url, file) => {
    const fd = new FormData()
    fd.append('file', file)
    const res = await fetch(url, { method: 'POST', body: fd })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '上传失败')
    return data
  },
}

// 上传辅助
export async function uploadImage(file) {
  return api.upload('/api/upload/image', file)
}
export async function uploadMedia(file) {
  return api.upload('/api/upload/media', file)
}
