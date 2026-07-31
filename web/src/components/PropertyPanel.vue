<script setup>
import { ref, computed, onMounted } from "vue";
import { api } from "../api";
import { state, findSource, sourceTypeLabel } from "../store";
import { toast } from "../toast";

const selectedItem = computed(() => state.selectedItem);
const selectedSource = computed(() => findSource(state.selectedSourceId));
const sourceName = computed(() => {
  if (!selectedItem.value) return "";
  const s = findSource(selectedItem.value.source_id);
  return s ? s.name : selectedItem.value.source_id;
});

// 当前场景的场景项（层列表，可选中被遮挡项）
const sceneItems = computed(() => {
  const sc =
    state.scenes.find((s) => s.id === state.currentSceneId) || state.scenes[0];
  return sc ? sc.items : [];
});
function layerName(it) {
  const s = findSource(it.source_id);
  return s ? s.name : it.source_id;
}
function selectItem(it) {
  state.selectedItem = it;
  state.selectedSourceId = null;
}
function toggleVisible(it) {
  it.visible = !it.visible;
  saveItem();
}

// 设备列表（用于下拉）
const videoDevs = ref([]);
const audioDevs = ref([]);
onMounted(async () => {
  try {
    videoDevs.value = await api.videoDevices();
  } catch (_) {}
  try {
    audioDevs.value = await api.audioDevices();
  } catch (_) {}
});

// ---- 保存（防抖）----
let timer = null;
function debounce(fn) {
  clearTimeout(timer);
  timer = setTimeout(fn, 500);
}
function saveItem() {
  debounce(async () => {
    try {
      const sc =
        state.scenes.find((s) => s.id === state.currentSceneId) ||
        state.scenes[0];
      if (sc) await api.updateScene(sc.id, sc);
    } catch (e) {
      toast(e.message, "error");
    }
  });
}
function saveSource() {
  debounce(async () => {
    const s = selectedSource.value;
    if (!s) return;
    try {
      await api.updateSource(s.id, s);
    } catch (e) {
      toast(e.message, "error");
    }
  });
}

// 移除场景项
function removeItem() {
  const it = selectedItem.value;
  const sc =
    state.scenes.find((s) => s.id === state.currentSceneId) || state.scenes[0];
  if (!it || !sc) return;
  sc.items = sc.items.filter((i) => i !== it);
  state.selectedItem = null;
  debounce(async () => {
    try {
      await api.updateScene(sc.id, sc);
    } catch (e) {
      toast(e.message, "error");
    }
  });
}

// 上传
async function doUpload(kind) {
  const input = document.createElement("input");
  input.type = "file";
  input.accept = kind === "image" ? "image/*" : "video/*,audio/*";
  input.onchange = async () => {
    const file = input.files[0];
    if (!file) return;
    try {
      const url = kind === "image" ? "/api/upload/image" : "/api/upload/media";
      const r = await api.upload(url, file);
      if (selectedSource.value) {
        selectedSource.value.file_path = r.path;
        saveSource();
        toast("上传成功", "ok");
      }
    } catch (e) {
      toast(e.message, "error");
    }
  };
  input.click();
}
</script>

<template>
  <aside class="prop-panel panel">
    <div class="panel-title">属性</div>

    <!-- 画布场景项层列表（OBS 风格，可选中被遮挡项） -->
    <div class="layer-list" v-if="sceneItems.length">
      <div class="section-title">画布场景项</div>
      <div
        v-for="(it, i) in sceneItems"
        :key="it.uid || 'l' + i"
        class="layer-item"
        :class="{ sel: state.selectedItem === it }"
        @click="selectItem(it)"
      >
        <span class="layer-eye" @click.stop="toggleVisible(it)">{{
          it.visible ? "👁" : "🚫"
        }}</span>
        <span class="layer-name" :title="layerName(it)">{{
          layerName(it)
        }}</span>
        <span class="layer-z">Z{{ it.z_index ?? 0 }}</span>
      </div>
    </div>

    <!-- 场景项属性 -->
    <div v-if="selectedItem" class="prop-body">
      <div class="section-title">🎯 场景项 · {{ sourceName }}</div>

      <div class="field-row">
        <div class="field">
          <label>X</label
          ><input
            type="number"
            v-model.number="selectedItem.x"
            @input="saveItem"
          />
        </div>
        <div class="field">
          <label>Y</label
          ><input
            type="number"
            v-model.number="selectedItem.y"
            @input="saveItem"
          />
        </div>
      </div>
      <div class="field-row">
        <div class="field">
          <label>宽</label
          ><input
            type="number"
            v-model.number="selectedItem.width"
            @input="saveItem"
          />
        </div>
        <div class="field">
          <label>高</label
          ><input
            type="number"
            v-model.number="selectedItem.height"
            @input="saveItem"
          />
        </div>
      </div>

      <div class="field">
        <label
          >透明度 {{ Math.round((selectedItem.opacity ?? 1) * 100) }}%</label
        >
        <input
          type="range"
          min="0"
          max="1"
          step="0.05"
          v-model.number="selectedItem.opacity"
          @input="saveItem"
        />
      </div>

      <div class="field">
        <label>层级 (Z)</label>
        <input
          type="number"
          v-model.number="selectedItem.z_index"
          @input="saveItem"
        />
      </div>

      <div class="field">
        <label>裁剪（0 = 不裁剪）</label>
        <div class="field-row">
          <div class="field">
            <input
              type="number"
              placeholder="X"
              v-model.number="selectedItem.crop_x"
              @input="saveItem"
            />
          </div>
          <div class="field">
            <input
              type="number"
              placeholder="Y"
              v-model.number="selectedItem.crop_y"
              @input="saveItem"
            />
          </div>
          <div class="field">
            <input
              type="number"
              placeholder="W"
              v-model.number="selectedItem.crop_w"
              @input="saveItem"
            />
          </div>
          <div class="field">
            <input
              type="number"
              placeholder="H"
              v-model.number="selectedItem.crop_h"
              @input="saveItem"
            />
          </div>
        </div>
      </div>

      <label class="check"
        ><input
          type="checkbox"
          v-model="selectedItem.visible"
          @change="saveItem"
        />
        可见</label
      >

      <div class="actions">
        <button class="btn danger small" @click="removeItem">
          ✕ 从场景移除
        </button>
      </div>
    </div>

    <!-- 源属性 -->
    <div v-else-if="selectedSource" class="prop-body">
      <div class="section-title">
        {{ sourceTypeLabel(selectedSource.type) }} · {{ selectedSource.name }}
      </div>

      <div class="field">
        <label>名称</label>
        <input v-model="selectedSource.name" @input="saveSource" />
      </div>
      <label class="check"
        ><input
          type="checkbox"
          v-model="selectedSource.enabled"
          @change="saveSource"
        />
        启用</label
      >

      <!-- 采集卡 -->
      <template v-if="selectedSource.type === 'video_device'">
        <div class="field">
          <label>设备路径</label>
          <select v-model="selectedSource.device_path" @change="saveSource">
            <option v-for="d in videoDevs" :key="d.path" :value="d.path">
              {{ d.name }}
            </option>
            <option
              :value="selectedSource.device_path"
              v-if="
                !videoDevs.find((d) => d.path === selectedSource.device_path)
              "
            >
              {{ selectedSource.device_path }}
            </option>
          </select>
        </div>
        <div class="field">
          <label>像素格式</label>
          <select v-model="selectedSource.pixel_format" @change="saveSource">
            <option value="yuyv422">yuyv422</option>
            <option value="mjpeg">mjpeg</option>
            <option value="nv12">nv12</option>
          </select>
        </div>
        <div class="field">
          <label>分辨率</label>
          <select v-model="selectedSource.resolution" @change="saveSource">
            <option value="1920x1080">1920x1080</option>
            <option value="1280x720">1280x720</option>
            <option value="640x480">640x480</option>
            <option
              :value="selectedSource.resolution"
              v-if="
                !['1920x1080', '1280x720', '640x480'].includes(
                  selectedSource.resolution,
                )
              "
            >
              {{ selectedSource.resolution }}
            </option>
          </select>
        </div>
        <div class="field">
          <label>帧率</label
          ><input
            type="number"
            v-model.number="selectedSource.fps"
            @input="saveSource"
          />
        </div>
        <div class="field">
          <label>色彩空间</label>
          <select v-model="selectedSource.color_space" @change="saveSource">
            <option value="bt709">BT.709</option>
            <option value="bt601">BT.601</option>
            <option value="smpte170m">SMPTE 170M</option>
            <option value="">自动</option>
          </select>
        </div>
      </template>

      <!-- 音频 -->
      <template v-else-if="selectedSource.type === 'audio_device'">
        <div class="field">
          <label>音频设备（usb = 自动探测 USB 声卡）</label>
          <select v-model="selectedSource.audio_device" @change="saveSource">
            <option value="usb">🖥 USB 声卡（自动探测）</option>
            <option v-for="d in audioDevs" :key="d.path" :value="d.path">
              {{ d.usb ? "🔌 " : "" }}{{ d.path }}
            </option>
          </select>
        </div>
        <div class="field-row">
          <div class="field">
            <label>采样率</label>
            <select
              v-model.number="selectedSource.sample_rate"
              @change="saveSource"
            >
              <option :value="44100">44100</option>
              <option :value="48000">48000</option>
              <option :value="96000">96000</option>
            </select>
          </div>
          <div class="field">
            <label>声道</label>
            <select
              v-model.number="selectedSource.channels"
              @change="saveSource"
            >
              <option :value="1">单声道</option>
              <option :value="2">双声道</option>
            </select>
          </div>
        </div>
        <div class="field">
          <label
            >音量 {{ Math.round((selectedSource.volume ?? 1) * 100) }}%</label
          >
          <input
            type="range"
            min="0"
            max="3"
            step="0.1"
            v-model.number="selectedSource.volume"
            @input="saveSource"
          />
        </div>
      </template>

      <!-- 图片/媒体 -->
      <template
        v-else-if="
          selectedSource.type === 'image' ||
          selectedSource.type === 'media_file'
        "
      >
        <div class="field">
          <label>文件路径</label
          ><input v-model="selectedSource.file_path" @input="saveSource" />
        </div>
        <button
          class="btn small"
          @click="doUpload(selectedSource.type === 'image' ? 'image' : 'media')"
        >
          ⬆ 上传{{ selectedSource.type === "image" ? "图片" : "媒体" }}
        </button>
        <div class="field" v-if="selectedSource.type === 'media_file'">
          <label class="check"
            ><input
              type="checkbox"
              v-model="selectedSource.loop"
              @change="saveSource"
            />
            循环</label
          >
        </div>
      </template>

      <!-- 文字 -->
      <template v-else-if="selectedSource.type === 'text'">
        <div class="field">
          <label>内容</label
          ><textarea
            rows="3"
            v-model="selectedSource.text"
            @input="saveSource"
          ></textarea>
        </div>
        <div class="field-row">
          <div class="field">
            <label>字号</label
            ><input
              type="number"
              v-model.number="selectedSource.font_size"
              @input="saveSource"
            />
          </div>
          <div class="field">
            <label>颜色</label
            ><input
              type="color"
              v-model="selectedSource.font_color"
              @input="saveSource"
              style="height: 28px; padding: 2px"
            />
          </div>
        </div>
      </template>

      <!-- 浏览器 -->
      <template v-else-if="selectedSource.type === 'browser'">
        <div class="field">
          <label>URL</label
          ><input
            v-model="selectedSource.url"
            @input="saveSource"
            placeholder="https://..."
          />
        </div>
        <div class="field-row">
          <div class="field">
            <label>渲染宽</label
            ><input
              type="number"
              v-model.number="selectedSource.browser_w"
              @input="saveSource"
            />
          </div>
          <div class="field">
            <label>渲染高</label
            ><input
              type="number"
              v-model.number="selectedSource.browser_h"
              @input="saveSource"
            />
          </div>
        </div>
        <div class="field">
          <label>帧率</label
          ><input
            type="number"
            v-model.number="selectedSource.browser_fps"
            @input="saveSource"
          />
        </div>
      </template>

      <!-- 屏幕 -->
      <template v-else-if="selectedSource.type === 'screen'">
        <div class="field">
          <label>X11 显示</label
          ><input
            v-model="selectedSource.display"
            @input="saveSource"
            placeholder=":0"
          />
        </div>
        <div class="field-row">
          <div class="field">
            <label>捕获宽</label
            ><input
              type="number"
              v-model.number="selectedSource.browser_w"
              @input="saveSource"
            />
          </div>
          <div class="field">
            <label>捕获高</label
            ><input
              type="number"
              v-model.number="selectedSource.browser_h"
              @input="saveSource"
            />
          </div>
        </div>
      </template>

      <!-- 纯色 -->
      <template v-else-if="selectedSource.type === 'color'">
        <div class="field">
          <label>颜色</label
          ><input
            type="color"
            v-model="selectedSource.color"
            @input="saveSource"
            style="height: 28px; padding: 2px"
          />
        </div>
      </template>

      <!-- RTMP 拉流 -->
      <template v-else-if="selectedSource.type === 'rtmp_source'">
        <div class="field">
          <label>拉流地址</label
          ><input
            v-model="selectedSource.file_path"
            @input="saveSource"
            placeholder="rtmp://..."
          />
        </div>
      </template>

      <!-- 源滤镜（暂显示数量） -->
      <div
        class="field"
        v-if="selectedSource.filters && selectedSource.filters.length"
      >
        <label>滤镜</label>
        <div class="filters">
          {{ selectedSource.filters.map((f) => f.type).join(", ") }}
        </div>
      </div>
    </div>

    <div v-else class="prop-body empty">
      <div>在左侧选择「源」</div>
      <div>或在画布上选择元素</div>
      <div class="dim">即可编辑属性</div>
    </div>
  </aside>
</template>

<style scoped>
.prop-panel {
  width: 260px;
  flex-shrink: 0;
  margin: 8px 8px 8px 0;
  overflow-y: auto;
}
.layer-list {
  padding: 12px;
  border-bottom: 1px solid var(--border);
}
.layer-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 3px 6px;
  border-radius: 3px;
  cursor: pointer;
  font-size: 12px;
}
.layer-item:hover {
  background: var(--bg-hover);
}
.layer-item.sel {
  background: var(--bg-active);
}
.layer-eye {
  cursor: pointer;
  font-size: 11px;
}
.layer-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.layer-z {
  font-size: 10px;
  color: var(--text-dim);
}
.prop-body {
  padding: 12px;
}
.section-title {
  font-weight: 700;
  margin-bottom: 10px;
  padding-bottom: 6px;
  border-bottom: 1px solid var(--border);
}
.empty {
  color: var(--text-dim);
  text-align: center;
  padding: 30px 0;
  line-height: 1.9;
}
.empty .dim {
  font-size: 11px;
  color: #666;
}
.check {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  margin: 6px 0;
  cursor: pointer;
}
.check input {
  width: auto;
}
.actions {
  margin-top: 12px;
  display: flex;
  gap: 6px;
}
.filters {
  font-size: 11px;
  color: var(--yellow);
}
</style>
