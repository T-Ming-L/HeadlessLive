<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from "vue";
import { api } from "../api";
import {
  state,
  currentScene,
  currentSceneItems,
  findSource,
  sourceTypeLabel,
} from "../store";
import { toast } from "../toast";
import { showMenu } from "../contextmenu";

const SCALE_MAX_W = 960;

const sc = computed(() => currentScene());
const items = computed(() => currentSceneItems());

// 画布自适应
const boxRef = ref(null);
const availW = ref(960);
const dispW = computed(() => Math.min(availW.value, SCALE_MAX_W));
const scale = computed(() => dispW.value / (sc.value?.canvas_w || 1920));
const dispH = computed(() => (sc.value?.canvas_h || 1080) * scale.value);

function measure() {
  if (boxRef.value) availW.value = boxRef.value.clientWidth - 32;
}
let ro = null;
onMounted(() => {
  measure();
  ro = new ResizeObserver(measure);
  if (boxRef.value) ro.observe(boxRef.value);
});
onUnmounted(() => {
  if (ro) ro.disconnect();
  if (frameTimer) clearInterval(frameTimer);
});

// 预览单帧轮询（比 multipart/x-mixed-replace 兼容性更好，所有浏览器可显示）
const previewFrameTs = ref(0);
let frameTimer = null;
watch(
  () => state.previewRunning,
  (v) => {
    if (frameTimer) {
      clearInterval(frameTimer);
      frameTimer = null;
    }
    if (v) {
      previewFrameTs.value = Date.now();
      frameTimer = setInterval(() => {
        previewFrameTs.value = Date.now();
      }, 120);
    }
  },
);

// 选中
function selectItem(item) {
  state.selectedItem = item;
  state.selectedSourceId = null;
}

// 每项稳定唯一 key（旧配置可能 z_index 重复导致 v-for key 冲突、个别项无法拖动）
let uidSeq = 0;
function genUid() {
  uidSeq += 1;
  return "it_" + Date.now().toString(36) + "_" + uidSeq;
}
function itemKey(item) {
  if (!item.uid) item.uid = genUid();
  return item.uid;
}

// 拖拽移动场景项
let drag = null;
function beginDrag(e, item, targetEl) {
  drag = {
    item,
    startX: e.clientX,
    startY: e.clientY,
    origX: item.x || 0,
    origY: item.y || 0,
  };
  try {
    targetEl.setPointerCapture(e.pointerId);
  } catch (_) {}
  window.addEventListener("pointermove", onMove);
  window.addEventListener("pointerup", onUp);
  e.preventDefault();
}
function endDrag() {
  if (!drag) return;
  window.removeEventListener("pointermove", onMove);
  window.removeEventListener("pointerup", onUp);
  drag = null;
}

// 画布按下：命中检测选中最上层可见项；若当前选中项也在指针下方则优先拖它
// （被其它项遮挡的场景项选中后依然能直接拖动，OBS 风格，解决"源拖不动"）
function onCanvasDown(e) {
  if (e.button !== 0 || !sc.value) return;
  const r = e.currentTarget.getBoundingClientRect();
  const x = (e.clientX - r.left) / scale.value;
  const y = (e.clientY - r.top) / scale.value;
  const under = items.value
    .filter((i) => i.visible !== false)
    .filter(
      (i) =>
        x >= (i.x || 0) &&
        x <= (i.x || 0) + (i.width || 0) &&
        y >= (i.y || 0) &&
        y <= (i.y || 0) + (i.height || 0),
    )
    .sort((a, b) => (b.z_index || 0) - (a.z_index || 0));
  let target = under[0] || null;
  if (state.selectedItem && under.includes(state.selectedItem)) {
    target = state.selectedItem;
  }
  if (!target) {
    state.selectedItem = null;
    return;
  }
  selectItem(target);
  beginDrag(e, target, e.currentTarget);
}
function onMove(e) {
  if (!drag) return;
  // 场景被刷新/切换导致项被替换时中止拖拽
  if (!items.value.includes(drag.item)) {
    endDrag();
    return;
  }
  const dx = (e.clientX - drag.startX) / scale.value;
  const dy = (e.clientY - drag.startY) / scale.value;
  let nx = drag.origX + dx;
  let ny = drag.origY + dy;
  // 贴边吸附（按住 Ctrl 拖动解除，OBS 风格）
  if (!e.ctrlKey && sc.value) {
    const snapped = snapPos(
      nx,
      ny,
      drag.item,
      sc.value.canvas_w,
      sc.value.canvas_h,
    );
    nx = snapped.x;
    ny = snapped.y;
  }
  drag.item.x = Math.round(nx);
  drag.item.y = Math.round(ny);
}

// 贴边吸附：元素边缘/中心对齐画布或其它元素边缘/中心，阈值内自动对齐
const SNAP_THRESHOLD = 8; // 场景坐标像素
function snapPos(nx, ny, item, cw, ch) {
  const w = item.width || 0;
  const h = item.height || 0;

  const targetsX = [0, cw / 2, cw];
  const targetsY = [0, ch / 2, ch];
  for (const o of items.value) {
    if (o === item || !o.visible) continue;
    const ow = o.width || 0;
    const oh = o.height || 0;
    targetsX.push(o.x, o.x + ow / 2, o.x + ow);
    targetsY.push(o.y, o.y + oh / 2, o.y + oh);
  }

  // 横向：找最近的吸附目标（item 左/中/右 vs 所有目标）
  let bestDx = 0;
  let bestXDist = SNAP_THRESHOLD;
  for (const off of [0, w / 2, w]) {
    for (const t of targetsX) {
      const d = t - (nx + off);
      if (Math.abs(d) < bestXDist) {
        bestXDist = Math.abs(d);
        bestDx = d;
      }
    }
  }
  // 纵向
  let bestDy = 0;
  let bestYDist = SNAP_THRESHOLD;
  for (const off of [0, h / 2, h]) {
    for (const t of targetsY) {
      const d = t - (ny + off);
      if (Math.abs(d) < bestYDist) {
        bestYDist = Math.abs(d);
        bestDy = d;
      }
    }
  }
  return { x: nx + bestDx, y: ny + bestDy };
}
function onUp() {
  if (!drag) return;
  endDrag();
  saveScene();
}

// 缩放场景项（OBS 风格：8 方向手柄，Shift 保持宽高比）
const resizeDirs = ["nw", "n", "ne", "e", "se", "s", "sw", "w"];
const MIN_SIZE = 8;
let resize = null;
function onResizeDown(e, item, dir) {
  if (e.button !== 0) return;
  e.stopPropagation();
  e.preventDefault();
  selectItem(item);
  resize = {
    item,
    dir,
    startX: e.clientX,
    startY: e.clientY,
    origX: item.x || 0,
    origY: item.y || 0,
    origW: item.width || 0,
    origH: item.height || 0,
    aspect: item.height > 0 ? item.width / item.height : 1,
    shift: e.shiftKey,
  };
  window.addEventListener("pointermove", onResizeMove);
  window.addEventListener("pointerup", onResizeUp);
}
function onResizeMove(e) {
  if (!resize) return;
  // 场景被刷新/切换导致项被替换时中止缩放
  if (!items.value.includes(resize.item)) {
    window.removeEventListener("pointermove", onResizeMove);
    window.removeEventListener("pointerup", onResizeUp);
    resize = null;
    return;
  }
  const dx = (e.clientX - resize.startX) / scale.value;
  const dy = (e.clientY - resize.startY) / scale.value;
  const it = resize.item;
  const dir = resize.dir;

  // 按方向计算新宽高（东/南增加，西/北减少）
  let nw =
    resize.origW + (dir.includes("e") ? dx : 0) - (dir.includes("w") ? dx : 0);
  let nh =
    resize.origH + (dir.includes("s") ? dy : 0) - (dir.includes("n") ? dy : 0);

  // Shift 保持宽高比
  if (resize.shift) {
    const byW = nw / resize.aspect;
    const byH = nh * resize.aspect;
    if (Math.abs(nw - resize.origW) >= Math.abs(nh - resize.origH)) {
      nh = byW;
    } else {
      nw = byH;
    }
  }

  // 最小尺寸
  nw = Math.max(MIN_SIZE, Math.round(nw));
  nh = Math.max(MIN_SIZE, Math.round(nh));

  // 西/北方向缩放时锚点随动
  let nx = resize.origX;
  let ny = resize.origY;
  if (dir.includes("w")) nx = resize.origX + (resize.origW - nw);
  if (dir.includes("n")) ny = resize.origY + (resize.origH - nh);

  it.width = nw;
  it.height = nh;
  it.x = nx;
  it.y = ny;
}
function onResizeUp() {
  if (!resize) return;
  window.removeEventListener("pointermove", onResizeMove);
  window.removeEventListener("pointerup", onResizeUp);
  saveScene();
  resize = null;
}

// 保存场景（防抖）
let saveTimer = null;
function saveScene() {
  clearTimeout(saveTimer);
  saveTimer = setTimeout(async () => {
    if (!sc.value) return;
    try {
      await api.updateScene(sc.value.id, sc.value);
    } catch (e) {
      toast(e.message, "error");
    }
  }, 400);
}

// 修改场景帧率（需重启预览/推流生效）
function onFpsChange() {
  if (!sc.value) return;
  const f = Math.max(1, Math.min(120, Math.round(sc.value.fps || 30)));
  sc.value.fps = f;
  saveScene();
  if (
    state.previewRunning ||
    Object.values(state.outputStatus).some((s) => s.state === "running")
  ) {
    toast("帧率已保存，重启预览/推流后生效", "ok");
  }
}

// 预览
async function startPreview() {
  try {
    const r = await api.previewStart(sc.value?.id, SCALE_MAX_W);
    state.previewRunning = true;
    if (r && r.warning && r.warning.length) {
      toast(r.warning.join("；"), "error");
    }
  } catch (e) {
    toast(e.message, "error");
  }
}
async function stopPreview() {
  try {
    await api.previewStop();
    state.previewRunning = false;
  } catch (e) {
    toast(e.message, "error");
  }
}

// 源拖入场景
function maxZ() {
  return items.value.reduce((m, i) => Math.max(m, i.z_index || 0), -1);
}
function onDrop(e) {
  e.preventDefault();
  const sid = e.dataTransfer.getData("text/source-id");
  if (!sid || !sc.value) return;
  const src = findSource(sid);
  if (!src || src.type === "audio_device") {
    toast("音频源无需加入画布（全局混音）");
    return;
  }
  const rect = e.currentTarget.getBoundingClientRect();
  const x = (e.clientX - rect.left) / scale.value;
  const y = (e.clientY - rect.top) / scale.value;
  const w = Math.min(640, sc.value.canvas_w);
  const h = Math.min(360, sc.value.canvas_h);
  sc.value.items.push({
    uid: genUid(),
    source_id: sid,
    x: Math.round(Math.max(0, x - w / 2)),
    y: Math.round(Math.max(0, y - h / 2)),
    width: w,
    height: h,
    opacity: 1,
    z_index: maxZ() + 1,
    visible: true,
  });
  toast(`已加入场景：${src.name}`, "ok");
  saveScene();
}
function onDragOver(e) {
  e.preventDefault();
  e.dataTransfer.dropEffect = "copy";
}

// 场景项工具
function removeItem(item) {
  if (!sc.value) return;
  sc.value.items = sc.value.items.filter((i) => i !== item);
  if (state.selectedItem === item) state.selectedItem = null;
  saveScene();
}
function toggleVisible(item) {
  item.visible = !item.visible;
  saveScene();
}

// ---- 右键菜单 ----
// 场景项：层级 / 居中 / 显隐 / 复制 / 删除（OBS 风格）
function onItemCtx(e, item) {
  selectItem(item);
  showMenu(e, [
    { label: "上移一层", onClick: () => zMove(item, 1) },
    { label: "下移一层", onClick: () => zMove(item, -1) },
    { label: "置顶", onClick: () => zTop(item) },
    { label: "置底", onClick: () => zBottom(item) },
    { label: "水平居中", onClick: () => centerItem(item, "h") },
    { label: "垂直居中", onClick: () => centerItem(item, "v") },
    {
      label: item.visible === false ? "显示" : "隐藏",
      onClick: () => toggleVisible(item),
    },
    { label: "复制一份", onClick: () => duplicateItem(item) },
    { sep: true },
    { label: "从场景移除", danger: true, onClick: () => removeItem(item) },
  ]);
}
// 空白画布：预览 / 全部居中 / 清空
function onCanvasCtx(e) {
  showMenu(e, [
    {
      label: state.previewRunning ? "停止预览" : "启动预览",
      onClick: state.previewRunning ? stopPreview : startPreview,
    },
    { label: "居中所有项", onClick: centerAllItems },
    { label: "清空场景", danger: true, onClick: clearAllItems },
  ]);
}
function zMove(item, dir) {
  const list = items.value
    .slice()
    .sort((a, b) => (a.z_index || 0) - (b.z_index || 0));
  const idx = list.indexOf(item);
  const j = idx + dir;
  if (j < 0 || j >= list.length) return;
  const az = list[idx].z_index || 0;
  const bz = list[j].z_index || 0;
  list[idx].z_index = bz;
  list[j].z_index = az;
  saveScene();
}
function zTop(item) {
  item.z_index = maxZ() + 1;
  saveScene();
}
function zBottom(item) {
  const min = items.value.reduce(
    (m, i) => (i === item ? m : Math.min(m, i.z_index || 0)),
    Infinity,
  );
  item.z_index = (min === Infinity ? 0 : min) - 1;
  saveScene();
}
function centerItem(item, axis) {
  if (!sc.value) return;
  if (axis === "h") {
    item.x = Math.round((sc.value.canvas_w - (item.width || 0)) / 2);
  } else {
    item.y = Math.round((sc.value.canvas_h - (item.height || 0)) / 2);
  }
  saveScene();
}
function duplicateItem(item) {
  if (!sc.value) return;
  const copy = {
    ...item,
    uid: genUid(),
    x: (item.x || 0) + 24,
    y: (item.y || 0) + 24,
    z_index: maxZ() + 1,
  };
  sc.value.items.push(copy);
  selectItem(copy);
  saveScene();
  toast("已复制一份", "ok");
}
function centerAllItems() {
  if (!sc.value) return;
  for (const it of items.value) {
    it.x = Math.round((sc.value.canvas_w - (it.width || 0)) / 2);
    it.y = Math.round((sc.value.canvas_h - (it.height || 0)) / 2);
  }
  saveScene();
}
function clearAllItems() {
  if (!confirm("清空当前场景的所有场景项？")) return;
  sc.value.items = [];
  state.selectedItem = null;
  saveScene();
}
function itemStyle(item) {
  return {
    left: (item.x || 0) * scale.value + "px",
    top: (item.y || 0) * scale.value + "px",
    width: (item.width || 0) * scale.value + "px",
    height: (item.height || 0) * scale.value + "px",
    zIndex: item.z_index || 0,
  };
}
function itemLabel(item) {
  const s = findSource(item.source_id);
  return s ? s.name : item.source_id;
}
</script>

<template>
  <main class="canvas-wrap panel">
    <div class="canvas-head">
      <span class="scene-name">{{ sc?.name || "（无场景）" }}</span>
      <span class="res" v-if="sc"
        >{{ sc.canvas_w }}x{{ sc.canvas_h }} @
        <input
          class="fps-input"
          type="number"
          min="1"
          max="120"
          v-model.number="sc.fps"
          @change="onFpsChange"
          title="场景输出帧率，修改后需重启预览/推流生效"
        />
        fps</span
      >
      <div class="spacer"></div>
      <button
        v-if="!state.previewRunning"
        class="btn small"
        @click="startPreview"
      >
        ▶ 启动预览
      </button>
      <button v-else class="btn small danger" @click="stopPreview">
        ■ 停止预览
      </button>
    </div>

    <div class="canvas-box" ref="boxRef">
      <div
        class="canvas-inner"
        :style="{ width: dispW + 'px', height: dispH + 'px' }"
        @dragover="onDragOver"
        @drop="onDrop"
        @pointerdown="onCanvasDown"
        @contextmenu.prevent="onCanvasCtx"
      >
        <img
          v-if="state.previewRunning"
          :src="'/preview/frame?t=' + previewFrameTs"
          class="preview-img"
          alt="预览"
        />
        <div v-else class="preview-placeholder">
          <div class="ph-big">MJPEG 实时预览</div>
          <div class="ph-small">点击上方「▶ 启动预览」</div>
        </div>

        <div
          v-for="item in items"
          :key="itemKey(item)"
          class="scene-item"
          :class="{
            selected: state.selectedItem === item,
            hidden: !item.visible,
          }"
          :style="itemStyle(item)"
          @dblclick="removeItem(item)"
          @contextmenu.prevent.stop="onItemCtx($event, item)"
        >
          <span class="item-label">{{ itemLabel(item) }}</span>
          <span class="item-btns">
            <button
              class="mini"
              :title="item.visible ? '隐藏' : '显示'"
              @pointerdown.stop
              @click.stop="toggleVisible(item)"
            >
              {{ item.visible ? "👁" : "🚫" }}
            </button>
            <button
              class="mini del"
              title="从场景移除"
              @pointerdown.stop
              @click.stop="removeItem(item)"
            >
              ✕
            </button>
          </span>
          <!-- OBS 风格缩放手柄（仅选中项显示） -->
          <template v-if="state.selectedItem === item">
            <span
              v-for="d in resizeDirs"
              :key="d"
              class="rs-handle"
              :class="'rs-' + d"
              @pointerdown.stop="onResizeDown($event, item, d)"
            ></span>
          </template>
        </div>
      </div>
      <div v-if="items.length === 0" class="drop-tip">
        从左侧拖拽源到此处加入场景
      </div>
    </div>
  </main>
</template>

<style scoped>
.canvas-wrap {
  flex: 1;
  min-width: 0;
  margin: 8px 8px 8px 0;
  display: flex;
  flex-direction: column;
}
.canvas-head {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--border);
}
.scene-name {
  font-weight: 700;
}
.res {
  font-size: 11px;
  color: var(--text-dim);
}
.fps-input {
  width: 52px;
  margin: 0 2px;
  padding: 1px 4px;
  text-align: center;
  background: var(--bg-panel);
  border: 1px solid var(--border);
  color: inherit;
  border-radius: 3px;
  font-size: 11px;
}
.spacer {
  flex: 1;
}
.canvas-box {
  flex: 1;
  overflow: auto;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  background: repeating-conic-gradient(#202020 0% 25%, #1a1a1a 0% 50%) 0 0 /
    24px 24px;
}
.canvas-inner {
  position: relative;
  background: #111;
  border: 1px solid var(--border);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.5);
  flex-shrink: 0;
  margin: 16px;
}
.preview-img {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: fill;
  pointer-events: none;
}
.preview-placeholder {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: var(--text-dim);
  gap: 6px;
}
.ph-big {
  font-size: 18px;
}
.ph-small {
  font-size: 12px;
}

.scene-item {
  position: absolute;
  border: 1px dashed rgba(255, 255, 255, 0.35);
  cursor: move;
  user-select: none;
  background: rgba(255, 255, 255, 0.03);
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
}
.scene-item:hover {
  border-color: var(--accent-hover);
}
.scene-item.selected {
  border: 2px solid var(--accent);
  background: rgba(14, 99, 156, 0.12);
}
.scene-item.hidden {
  opacity: 0.25;
  pointer-events: none; /* 隐藏项不拦截点击，可穿透到下方元素 */
}
.item-label {
  font-size: 10px;
  color: #eee;
  background: rgba(0, 0, 0, 0.55);
  padding: 1px 5px;
  border-radius: 2px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 80%;
  pointer-events: none;
}
.item-btns {
  display: none;
}
.scene-item.selected .item-btns {
  display: flex;
  gap: 2px;
  padding: 2px;
}
.mini {
  background: rgba(0, 0, 0, 0.6);
  color: #fff;
  border: none;
  font-size: 9px;
  cursor: pointer;
  padding: 1px 4px;
  border-radius: 2px;
}
.mini.del:hover {
  color: var(--red);
}

/* OBS 风格缩放手柄 */
.rs-handle {
  position: absolute;
  width: 8px;
  height: 8px;
  background: #fff;
  border: 1px solid var(--accent);
  border-radius: 1px;
  cursor: pointer;
  z-index: 20;
  box-shadow: 0 0 2px rgba(0, 0, 0, 0.6);
}
.rs-nw {
  left: -4px;
  top: -4px;
  cursor: nwse-resize;
}
.rs-n {
  left: 50%;
  top: -4px;
  transform: translateX(-50%);
  cursor: ns-resize;
}
.rs-ne {
  right: -4px;
  top: -4px;
  cursor: nesw-resize;
}
.rs-e {
  right: -4px;
  top: 50%;
  transform: translateY(-50%);
  cursor: ew-resize;
}
.rs-se {
  right: -4px;
  bottom: -4px;
  cursor: nwse-resize;
}
.rs-s {
  left: 50%;
  bottom: -4px;
  transform: translateX(-50%);
  cursor: ns-resize;
}
.rs-sw {
  left: -4px;
  bottom: -4px;
  cursor: nesw-resize;
}
.rs-w {
  left: -4px;
  top: 50%;
  transform: translateY(-50%);
  cursor: ew-resize;
}
.drop-tip {
  position: absolute;
  bottom: 12px;
  font-size: 11px;
  color: var(--text-dim);
  background: rgba(0, 0, 0, 0.5);
  padding: 4px 12px;
  border-radius: 10px;
  pointer-events: none;
}
</style>
