<script setup>
import { ref, computed, onMounted } from "vue";
import { api } from "../api";
import { state, sourceTypeLabel } from "../store";
import { toast } from "../toast";

const emit = defineEmits(["close"]);

const TYPES = [
  "video_device",
  "audio_device",
  "image",
  "text",
  "media_file",
  "screen",
  "rtmp_source",
  "color",
];

const type = ref("video_device");
const form = ref({
  name: "",
  enabled: true,
  device_path: "/dev/video0",
  pixel_format: "yuyv422",
  resolution: "1920x1080",
  fps: 30,
  color_space: "bt709",
  audio_device: "usb",
  sample_rate: 48000,
  channels: 2,
  volume: 1.0,
  file_path: "",
  loop: true,
  text: "直播文字",
  font_size: 48,
  font_color: "#ffffff",
  browser_w: 1280,
  browser_h: 720,
  browser_fps: 30,
  display: ":0",
  color: "#2e2e2e",
});
const addToScene = ref(true);
const videoDevs = ref([]);
const audioDevs = ref([]);

const typeLabel = computed(() => sourceTypeLabel(type.value));

onMounted(async () => {
  try {
    videoDevs.value = await api.videoDevices();
  } catch (_) {}
  try {
    audioDevs.value = await api.audioDevices();
  } catch (_) {}
});

function pickType(t) {
  type.value = t;
  // 默认名称
  if (!form.value.name) {
    const base = sourceTypeLabel(t);
    form.value.name = base.split(" ")[1] || t;
  }
}

// 每种源类型允许的字段（避免创建时带上无关字段）
const TYPE_FIELDS = {
  video_device: [
    "name",
    "enabled",
    "device_path",
    "pixel_format",
    "resolution",
    "fps",
    "color_space",
  ],
  audio_device: [
    "name",
    "enabled",
    "audio_device",
    "sample_rate",
    "channels",
    "volume",
  ],
  image: ["name", "enabled", "file_path", "loop"],
  text: ["name", "enabled", "text", "font_size", "font_color"],
  media_file: ["name", "enabled", "file_path", "loop"],
  screen: [
    "name",
    "enabled",
    "display",
    "browser_w",
    "browser_h",
    "browser_fps",
  ],
  rtmp_source: ["name", "enabled", "file_path"],
  color: ["name", "enabled", "color"],
};

// 图片/媒体：先上传到服务器，自动填路径
async function doUpload() {
  const input = document.createElement("input");
  input.type = "file";
  input.accept = type.value === "image" ? "image/*" : "video/*,audio/*";
  input.onchange = async () => {
    const file = input.files[0];
    if (!file) return;
    try {
      const url =
        type.value === "image" ? "/api/upload/image" : "/api/upload/media";
      const r = await api.upload(url, file);
      form.value.file_path = r.path;
      toast("上传成功，路径已填入", "ok");
    } catch (e) {
      toast(e.message, "error");
    }
  };
  input.click();
}

async function create() {
  const allowed = TYPE_FIELDS[type.value] || ["name", "enabled"];
  const payload = { type: type.value };
  for (const k of allowed) {
    if (form.value[k] !== undefined) payload[k] = form.value[k];
  }
  if (!payload.name) payload.name = sourceTypeLabel(type.value);
  try {
    const src = await api.createSource(payload);
    state.sources.push(src);

    // 可选加入当前场景
    if (addToScene.value && src.type !== "audio_device") {
      const sc =
        state.scenes.find((s) => s.id === state.currentSceneId) ||
        state.scenes[0];
      if (sc) {
        sc.items.push({
          source_id: src.id,
          x: 0,
          y: 0,
          width: Math.min(640, sc.canvas_w),
          height: Math.min(360, sc.canvas_h),
          opacity: 1,
          z_index: sc.items.length,
          visible: true,
        });
        await api.updateScene(sc.id, sc);
      }
    }
    toast(`已创建：${src.name}`, "ok");
    emit("close");
  } catch (e) {
    toast(e.message, "error");
  }
}
</script>

<template>
  <div class="modal-mask" @click.self="emit('close')">
    <div class="modal">
      <div class="modal-head">
        <span>＋ 添加源</span>
        <button class="btn small ghost" @click="emit('close')">✕</button>
      </div>
      <div class="modal-body">
        <!-- 类型选择 -->
        <div class="type-grid">
          <div
            v-for="t in TYPES"
            :key="t"
            class="type-cell"
            :class="{ active: type === t }"
            @click="pickType(t)"
          >
            {{ sourceTypeLabel(t) }}
          </div>
        </div>

        <!-- 通用 -->
        <div class="field-row" style="margin-top: 12px">
          <div class="field">
            <label>名称</label><input v-model="form.name" />
          </div>
          <div class="field">
            <label>启用</label>
            <select v-model="form.enabled">
              <option :value="true">是</option>
              <option :value="false">否</option>
            </select>
          </div>
        </div>

        <!-- 采集卡 -->
        <template v-if="type === 'video_device'">
          <div class="field">
            <label>设备</label>
            <select v-model="form.device_path">
              <option v-for="d in videoDevs" :key="d.path" :value="d.path">
                {{ d.name }}
              </option>
              <option value="/dev/video0">/dev/video0</option>
            </select>
          </div>
          <div class="field-row">
            <div class="field">
              <label>像素格式</label>
              <select v-model="form.pixel_format">
                <option value="yuyv422">yuyv422</option>
                <option value="mjpeg">mjpeg</option>
                <option value="nv12">nv12</option>
              </select>
            </div>
            <div class="field">
              <label>分辨率</label>
              <select v-model="form.resolution">
                <option value="1920x1080">1920x1080</option>
                <option value="1280x720">1280x720</option>
                <option value="640x480">640x480</option>
              </select>
            </div>
          </div>
          <div class="field-row">
            <div class="field">
              <label>帧率</label
              ><input type="number" v-model.number="form.fps" />
            </div>
            <div class="field">
              <label>色彩空间</label>
              <select v-model="form.color_space">
                <option value="bt709">BT.709</option>
                <option value="bt601">BT.601</option>
                <option value="smpte170m">SMPTE 170M</option>
                <option value="">自动</option>
              </select>
            </div>
          </div>
        </template>

        <!-- 音频 -->
        <template v-else-if="type === 'audio_device'">
          <div class="field">
            <label>设备（usb = 自动探测 USB 声卡）</label>
            <select v-model="form.audio_device">
              <option value="usb">🖥 USB 声卡（自动探测）</option>
              <option v-for="d in audioDevs" :key="d.path" :value="d.path">
                {{ d.usb ? "🔌 " : "" }}{{ d.path }}
              </option>
            </select>
          </div>
          <div class="field-row">
            <div class="field">
              <label>采样率</label>
              <select v-model.number="form.sample_rate">
                <option :value="44100">44100</option>
                <option :value="48000">48000</option>
                <option :value="96000">96000</option>
              </select>
            </div>
            <div class="field">
              <label>声道</label>
              <select v-model.number="form.channels">
                <option :value="1">单</option>
                <option :value="2">双</option>
              </select>
            </div>
          </div>
        </template>

        <!-- 图片/媒体 -->
        <template v-else-if="type === 'image' || type === 'media_file'">
          <div class="field">
            <label>文件路径</label>
            <div class="upload-row">
              <input
                v-model="form.file_path"
                :placeholder="
                  type === 'image' ? 'uploads/xxx.png' : 'uploads/xxx.mp4'
                "
              />
              <button class="btn small" @click="doUpload">
                ⬆ 上传{{ type === "image" ? "图片" : "文件" }}
              </button>
            </div>
          </div>
          <label class="check"
            ><input type="checkbox" v-model="form.loop" /> 循环{{
              type === "media_file" ? "播放" : ""
            }}</label
          >
        </template>

        <!-- 文字 -->
        <template v-else-if="type === 'text'">
          <div class="field">
            <label>内容</label
            ><textarea rows="2" v-model="form.text"></textarea>
          </div>
          <div class="field-row">
            <div class="field">
              <label>字号</label
              ><input type="number" v-model.number="form.font_size" />
            </div>
            <div class="field">
              <label>颜色</label
              ><input
                type="color"
                v-model="form.font_color"
                style="height: 28px; padding: 2px"
              />
            </div>
          </div>
        </template>

        <!-- 屏幕 -->
        <template v-else-if="type === 'screen'">
          <div class="field">
            <label>X11 显示</label
            ><input v-model="form.display" placeholder=":0" />
          </div>
          <div class="field-row">
            <div class="field">
              <label>宽</label
              ><input type="number" v-model.number="form.browser_w" />
            </div>
            <div class="field">
              <label>高</label
              ><input type="number" v-model.number="form.browser_h" />
            </div>
          </div>
        </template>

        <!-- RTMP 拉流 -->
        <template v-else-if="type === 'rtmp_source'">
          <div class="field">
            <label>拉流地址</label
            ><input v-model="form.file_path" placeholder="rtmp://..." />
          </div>
        </template>

        <!-- 纯色 -->
        <template v-else-if="type === 'color'">
          <div class="field">
            <label>颜色</label
            ><input
              type="color"
              v-model="form.color"
              style="height: 30px; padding: 2px"
            />
          </div>
        </template>

        <label class="check" v-if="type !== 'audio_device'">
          <input type="checkbox" v-model="addToScene" /> 创建后加入当前场景
        </label>
      </div>
      <div class="modal-foot">
        <button class="btn ghost" @click="emit('close')">取消</button>
        <button class="btn" @click="create">✓ 创建（{{ typeLabel }}）</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.type-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 6px;
}
.type-cell {
  border: 1px solid var(--border);
  border-radius: 3px;
  padding: 8px 4px;
  text-align: center;
  cursor: pointer;
  font-size: 11px;
}
.type-cell:hover {
  background: var(--bg-hover);
}
.type-cell.active {
  background: var(--accent);
  border-color: var(--accent);
  color: #fff;
}
.upload-row {
  display: flex;
  gap: 6px;
  align-items: center;
}
.upload-row input {
  flex: 1;
  min-width: 0;
}
.upload-row .btn {
  flex-shrink: 0;
}
.check {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  margin-top: 10px;
  cursor: pointer;
}
.check input {
  width: auto;
}
</style>
