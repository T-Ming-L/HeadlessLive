<script setup>
import { api } from "../api";
import { state, ui } from "../store";
import { toast } from "../toast";
import { showMenu } from "../contextmenu";

async function activate(id) {
  try {
    await api.activateScene(id);
    state.currentSceneId = id;
  } catch (e) {
    toast(e.message, "error");
  }
}

async function addScene() {
  const name = prompt("场景名称:", `场景 ${state.scenes.length + 1}`);
  if (!name) return;
  try {
    const sc = await api.createScene({
      name,
      canvas_w: 1920,
      canvas_h: 1080,
      fps: 30,
      items: [],
    });
    state.scenes.push(sc);
    await api.activateScene(sc.id);
    state.currentSceneId = sc.id;
  } catch (e) {
    toast(e.message, "error");
  }
}

async function removeScene(id) {
  if (!confirm("删除该场景？")) return;
  try {
    await api.deleteScene(id);
    state.scenes = state.scenes.filter((s) => s.id !== id);
    if (state.currentSceneId === id)
      state.currentSceneId = state.scenes[0]?.id || "";
  } catch (e) {
    toast(e.message, "error");
  }
}

// ---- 右键菜单 ----
function onSceneCtx(e, s) {
  showMenu(e, [
    { label: "设为当前场景", onClick: () => activate(s.id) },
    { label: "重命名", onClick: () => renameScene(s) },
    { label: "删除场景", danger: true, onClick: () => removeScene(s.id) },
  ]);
}
async function renameScene(s) {
  const name = prompt("场景名称:", s.name);
  if (!name || name === s.name) return;
  s.name = name;
  try {
    await api.updateScene(s.id, s);
  } catch (e) {
    toast(e.message, "error");
  }
}

async function start(o) {
  try {
    // 同一采集设备不能并发采集，推流前先停预览
    if (state.previewRunning) {
      await api.previewStop();
      state.previewRunning = false;
    }
    const r = await api.startOutput(o.id);
    if (r && r.warning && r.warning.length) {
      toast(r.warning.join("；"), "error");
    }
  } catch (e) {
    toast(e.message, "error");
  }
}
async function stop(o) {
  try {
    await api.stopOutput(o.id);
  } catch (e) {
    toast(e.message, "error");
  }
}

function statusOf(o) {
  return state.outputStatus[o.id] || { state: "idle" };
}
function statusText(s) {
  if (s.state === "running") {
    let t = "推流中";
    if (s.uptime) t += ` ${s.uptime}`;
    if (s.fps) t += ` · ${s.fps}fps`;
    if (s.bitrate) t += ` · ${s.bitrate}`;
    return t;
  }
  if (s.state === "error") return "出错";
  return "空闲";
}
function typeLabel(t) {
  return { rtmp: "RTMP", record: "录制", srt: "SRT", ndi: "NDI" }[t] || t;
}
</script>

<template>
  <footer class="bottom-bar">
    <div class="scenes">
      <span class="label">场景</span>
      <div
        v-for="s in state.scenes"
        :key="s.id"
        class="scene-tab"
        :class="{ active: s.id === state.currentSceneId }"
        @click="activate(s.id)"
        @contextmenu.prevent.stop="onSceneCtx($event, s)"
      >
        {{ s.name }}
        <span class="tab-x" @click.stop="removeScene(s.id)" title="删除"
          >✕</span
        >
      </div>
      <button class="btn small" @click="addScene" title="新建场景">＋</button>
    </div>

    <div class="outputs">
      <div v-for="o in state.outputs" :key="o.id" class="output-item">
        <span class="o-name" :title="o.url || o.file_path || ''"
          >{{ typeLabel(o.type) }} · {{ o.name }}</span
        >
        <span class="o-status" :class="statusOf(o).state">{{
          statusText(statusOf(o))
        }}</span>
        <button
          v-if="statusOf(o).state !== 'running'"
          class="btn small"
          @click="start(o)"
        >
          ▶ 开始
        </button>
        <button v-else class="btn small danger" @click="stop(o)">■ 停止</button>
      </div>
      <span v-if="state.outputs.length === 0" class="no-out"
        >无输出（点右上「输出管理」添加）</span
      >
    </div>

    <div class="spacer"></div>
    <div class="meta">
      <span class="m-item"
        >👁 预览 {{ state.previewRunning ? "ON" : "OFF" }}</span
      >
      <span class="m-item" v-if="state.currentSceneId"
        >场景:
        {{
          state.scenes.find((s) => s.id === state.currentSceneId)?.name
        }}</span
      >
    </div>
  </footer>
</template>

<style scoped>
.bottom-bar {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 6px 10px;
  background: var(--bg-panel);
  border-top: 1px solid var(--border);
  flex-shrink: 0;
  min-height: 42px;
}
.scenes {
  display: flex;
  align-items: center;
  gap: 6px;
}
.label {
  font-size: 11px;
  color: var(--text-dim);
}
.scene-tab {
  padding: 4px 10px;
  border-radius: 3px;
  cursor: pointer;
  border: 1px solid var(--border);
  font-size: 12px;
  display: flex;
  align-items: center;
  gap: 6px;
}
.scene-tab:hover {
  background: var(--bg-hover);
}
.scene-tab.active {
  background: var(--accent);
  border-color: var(--accent);
  color: #fff;
}
.tab-x {
  color: inherit;
  opacity: 0.6;
  font-size: 10px;
}
.tab-x:hover {
  opacity: 1;
}

.outputs {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  overflow-x: auto;
}
.output-item {
  display: flex;
  align-items: center;
  gap: 6px;
  border: 1px solid var(--border);
  border-radius: 3px;
  padding: 3px 8px;
  background: var(--bg);
}
.o-name {
  font-size: 11px;
  white-space: nowrap;
}
.o-status {
  font-size: 11px;
  color: var(--text-dim);
  white-space: nowrap;
}
.o-status.running {
  color: var(--green);
}
.o-status.error {
  color: var(--red);
}
.no-out {
  font-size: 11px;
  color: var(--text-dim);
}
.spacer {
  flex: 0 0 8px;
}
.meta {
  display: flex;
  gap: 12px;
}
.m-item {
  font-size: 11px;
  color: var(--text-dim);
  white-space: nowrap;
}
</style>
