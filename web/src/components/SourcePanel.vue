<script setup>
import { api } from "../api";
import { state, ui, sourceTypeLabel } from "../store";
import { toast } from "../toast";
import { showMenu } from "../contextmenu";

function select(src) {
  state.selectedSourceId = src.id;
  state.selectedItem = null;
}

async function toggle(src) {
  src.enabled = !src.enabled;
  try {
    await api.updateSource(src.id, src);
  } catch (e) {
    src.enabled = !src.enabled;
    toast(e.message, "error");
  }
}

async function remove(src) {
  if (!confirm(`删除源「${src.name}」？将从所有场景移除`)) return;
  try {
    await api.deleteSource(src.id);
    state.sources = state.sources.filter((s) => s.id !== src.id);
    if (state.selectedSourceId === src.id) state.selectedSourceId = null;
    if (state.selectedItem && state.selectedItem.source_id === src.id)
      state.selectedItem = null;
    toast("已删除", "ok");
  } catch (e) {
    toast(e.message, "error");
  }
}

function dragStart(e, src) {
  e.dataTransfer.setData("text/source-id", src.id);
  e.dataTransfer.effectAllowed = "copy";
}

// ---- 右键菜单 ----
function onSourceCtx(e, src) {
  showMenu(e, [
    { label: "编辑属性", onClick: () => select(src) },
    { label: "加入当前场景", onClick: () => addToScene(src) },
    { label: "重命名", onClick: () => rename(src) },
    { label: "删除源", danger: true, onClick: () => remove(src) },
  ]);
}
function addToScene(src) {
  if (src.type === "audio_device") {
    toast("音频源无需加入画布（全局混音）");
    return;
  }
  const sc =
    state.scenes.find((s) => s.id === state.currentSceneId) || state.scenes[0];
  if (!sc) {
    toast("还没有场景", "error");
    return;
  }
  const maxZ = sc.items.reduce((m, i) => Math.max(m, i.z_index || 0), -1);
  const w = Math.min(640, sc.canvas_w);
  const h = Math.min(360, sc.canvas_h);
  const item = {
    source_id: src.id,
    x: Math.round((sc.canvas_w - w) / 2),
    y: Math.round((sc.canvas_h - h) / 2),
    width: w,
    height: h,
    opacity: 1,
    z_index: maxZ + 1,
    visible: true,
  };
  sc.items.push(item);
  state.selectedItem = item;
  state.selectedSourceId = null;
  api.updateScene(sc.id, sc).catch((e) => toast(e.message, "error"));
  toast(`已加入场景：${src.name}`, "ok");
}
function rename(src) {
  const name = prompt("重命名源:", src.name);
  if (!name || name === src.name) return;
  src.name = name;
  api.updateSource(src.id, src).catch((e) => toast(e.message, "error"));
}
</script>

<template>
  <aside class="source-panel panel">
    <div class="panel-title">
      <span>源（{{ state.sources.length }}）</span>
      <button class="btn small" @click="ui.showAddSource = true">
        ＋ 添加
      </button>
    </div>
    <div class="src-list">
      <div
        v-for="s in state.sources"
        :key="s.id"
        class="src-item"
        :class="{
          active: state.selectedSourceId === s.id,
          disabled: !s.enabled,
        }"
        draggable="true"
        @dragstart="dragStart($event, s)"
        @click="select(s)"
        @contextmenu.prevent.stop="onSourceCtx($event, s)"
      >
        <span class="icon">{{ sourceTypeLabel(s.type).split(" ")[0] }}</span>
        <div class="info">
          <div class="name">{{ s.name }}</div>
          <div class="type">{{ sourceTypeLabel(s.type) }}</div>
        </div>
        <button
          class="icon-btn"
          :class="{ off: !s.enabled }"
          @click.stop="toggle(s)"
          :title="s.enabled ? '停用' : '启用'"
        >
          {{ s.enabled ? "👁" : "🚫" }}
        </button>
        <button class="icon-btn del" @click.stop="remove(s)" title="删除">
          ✕
        </button>
      </div>
      <div v-if="state.sources.length === 0" class="empty">
        暂无源<br />点「＋ 添加」创建
      </div>
    </div>
    <div class="hint">💡 拖拽源到画布加入场景</div>
  </aside>
</template>

<style scoped>
.source-panel {
  width: 230px;
  flex-shrink: 0;
  margin: 8px;
  display: flex;
  flex-direction: column;
}
.src-list {
  flex: 1;
  overflow-y: auto;
  padding: 6px;
}
.src-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border-radius: 3px;
  cursor: pointer;
  border: 1px solid transparent;
  user-select: none;
}
.src-item:hover {
  background: var(--bg-hover);
}
.src-item.active {
  background: var(--bg-active);
  border-color: var(--accent);
}
.src-item.disabled .info {
  opacity: 0.5;
}
.src-item .icon {
  font-size: 15px;
  width: 20px;
  text-align: center;
}
.src-item .info {
  flex: 1;
  min-width: 0;
}
.src-item .name {
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.src-item .type {
  font-size: 10px;
  color: var(--text-dim);
}
.icon-btn {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 12px;
  padding: 2px;
}
.icon-btn.off {
  opacity: 0.4;
}
.icon-btn.del {
  color: var(--text-dim);
}
.icon-btn.del:hover {
  color: var(--red);
}
.empty {
  color: var(--text-dim);
  text-align: center;
  padding: 24px 0;
  line-height: 1.8;
}
.hint {
  padding: 8px 10px;
  font-size: 10px;
  color: var(--text-dim);
  border-top: 1px solid var(--border);
}
</style>
