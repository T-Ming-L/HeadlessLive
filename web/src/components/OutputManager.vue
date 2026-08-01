<script setup>
import { ref, onMounted } from "vue";
import { api } from "../api";
import { state } from "../store";
import { toast } from "../toast";

const emit = defineEmits(["close"]);

const form = ref({
  name: "",
  type: "rtmp",
  scene_id: "",
  enabled: true,
  url: "",
  stream_key: "",
  encoder: "libx264",
  bitrate: 5000,
  keyframe: 60,
  file_path: "",
  format: "mp4",
});
const editing = ref(null); // 正在编辑的输出 ID
const scenes = () => state.scenes;
const outputs = () => state.outputs;

function reset() {
  editing.value = null;
  form.value = {
    name: "",
    type: "rtmp",
    scene_id: state.currentSceneId || scenes()[0]?.id || "",
    enabled: true,
    url: "",
    stream_key: "",
    encoder: "libx264",
    bitrate: 5000,
    keyframe: 60,
    file_path: "",
    format: "mp4",
  };
}

function edit(o) {
  editing.value = o.id;
  form.value = {
    name: o.name,
    type: o.type,
    scene_id: o.scene_id,
    enabled: o.enabled,
    url: o.url || "",
    stream_key: o.stream_key || "",
    encoder: o.encoder || "libx264",
    bitrate: o.bitrate || 5000,
    keyframe: o.keyframe || 60,
    file_path: o.file_path || "",
    format: o.format || "mp4",
  };
}

async function save() {
  try {
    if (editing.value) {
      const upd = await api.updateOutput(editing.value, form.value);
      Object.assign(
        outputs().find((o) => o.id === editing.value),
        upd,
      );
      toast("已更新", "ok");
    } else {
      const created = await api.createOutput(form.value);
      state.outputs.push(created);
      toast("已创建", "ok");
    }
    reset();
  } catch (e) {
    toast(e.message, "error");
  }
}

async function remove(o) {
  if (!confirm(`删除输出「${o.name}」？`)) return;
  try {
    await api.deleteOutput(o.id);
    state.outputs = state.outputs.filter((x) => x.id !== o.id);
  } catch (e) {
    toast(e.message, "error");
  }
}

async function start(o) {
  try {
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
function st(o) {
  return state.outputStatus[o.id] || { state: "idle" };
}
</script>

<template>
  <div class="modal-mask" @click.self="emit('close')">
    <div class="modal wide">
      <div class="modal-head">
        <span>📡 输出管理</span>
        <button class="btn small ghost" @click="emit('close')">✕</button>
      </div>
      <div class="modal-body">
        <!-- 列表 -->
        <div class="out-list">
          <div
            v-for="o in outputs()"
            :key="o.id"
            class="out-row"
            :class="{ running: st(o).state === 'running' }"
          >
            <div class="o-info">
              <div class="o-name">
                {{ o.type.toUpperCase() }} · {{ o.name }}
              </div>
              <div class="o-desc">
                <span v-if="o.type === 'rtmp'"
                  >{{ (o.url || "").replace(/\/+$/, "")
                  }}{{ o.stream_key ? "/" + o.stream_key : "" }}</span
                >
                <span v-else-if="o.type === 'record'"
                  >{{ o.file_path || "自动命名" }} ({{ o.format }})</span
                >
                <span v-else>{{ o.url }}</span>
                <span class="st" :class="st(o).state">{{
                  st(o).state === "running"
                    ? "运行中"
                    : st(o).state === "error"
                      ? "出错"
                      : "空闲"
                }}</span>
              </div>
            </div>
            <div class="o-ops">
              <button
                v-if="st(o).state !== 'running'"
                class="btn small"
                @click="start(o)"
              >
                ▶
              </button>
              <button v-else class="btn small danger" @click="stop(o)">
                ■
              </button>
              <button class="btn small ghost" @click="edit(o)">✎</button>
              <button class="btn small ghost" @click="remove(o)">🗑</button>
            </div>
          </div>
          <div v-if="outputs().length === 0" class="empty">暂无输出</div>
        </div>

        <!-- 表单 -->
        <div class="form-title">{{ editing ? "编辑输出" : "新建输出" }}</div>
        <div class="field-row">
          <div class="field">
            <label>名称</label><input v-model="form.name" />
          </div>
          <div class="field">
            <label>类型</label>
            <select v-model="form.type">
              <option value="rtmp">RTMP 推流</option>
              <option value="record">本地录制</option>
              <option value="srt">SRT</option>
            </select>
          </div>
        </div>
        <div class="field-row">
          <div class="field">
            <label>绑定场景</label>
            <select v-model="form.scene_id">
              <option v-for="s in scenes()" :key="s.id" :value="s.id">
                {{ s.name }}
              </option>
            </select>
          </div>
          <div class="field">
            <label>编码器</label>
            <select v-model="form.encoder">
              <option value="h264_vaapi">h264_vaapi</option>
              <option value="h264_qsv">h264_qsv</option>
              <option value="libx264">libx264</option>
              <option value="hevc_vaapi">hevc_vaapi</option>
            </select>
          </div>
        </div>
        <template v-if="form.type === 'rtmp' || form.type === 'srt'">
          <div class="field">
            <label>{{ form.type === "rtmp" ? "推流地址" : "SRT 地址" }}</label>
            <input
              v-model="form.url"
              :placeholder="
                form.type === 'rtmp'
                  ? 'rtmp://live.example.com/live/'
                  : 'srt://...'
              "
            />
          </div>
          <div class="field" v-if="form.type === 'rtmp'">
            <label>流密钥</label><input v-model="form.stream_key" />
          </div>
        </template>
        <template v-if="form.type === 'record'">
          <div class="field-row">
            <div class="field">
              <label>文件路径（留空自动命名）</label
              ><input v-model="form.file_path" />
            </div>
            <div class="field">
              <label>格式</label>
              <select v-model="form.format">
                <option value="mp4">mp4</option>
                <option value="mkv">mkv</option>
                <option value="flv">flv</option>
              </select>
            </div>
          </div>
        </template>
        <div class="field-row">
          <div class="field">
            <label>码率 (kbps)</label
            ><input type="number" v-model.number="form.bitrate" />
          </div>
          <div class="field">
            <label>关键帧间隔</label
            ><input type="number" v-model.number="form.keyframe" />
          </div>
        </div>

        <div class="modal-foot">
          <button class="btn ghost" @click="reset" v-if="editing">
            取消编辑
          </button>
          <button class="btn" @click="save">
            {{ editing ? "✓ 保存修改" : "✓ 创建输出" }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.modal.wide {
  min-width: 560px;
}
.out-list {
  margin-bottom: 14px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.out-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  border: 1px solid var(--border);
  border-radius: 3px;
  padding: 8px 10px;
  background: var(--bg);
}
.out-row.running {
  border-color: var(--green);
}
.o-name {
  font-size: 12px;
  font-weight: 700;
}
.o-desc {
  font-size: 11px;
  color: var(--text-dim);
  display: flex;
  align-items: center;
  gap: 8px;
}
.st {
  padding: 1px 8px;
  border-radius: 8px;
  border: 1px solid var(--border);
  font-size: 10px;
}
.st.running {
  color: var(--green);
  border-color: var(--green);
}
.st.error {
  color: var(--red);
  border-color: var(--red);
}
.o-ops {
  display: flex;
  gap: 4px;
}
.form-title {
  font-weight: 700;
  margin-bottom: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--border);
}
.empty {
  color: var(--text-dim);
  text-align: center;
  padding: 16px 0;
}
.modal-foot {
  padding: 0;
  border: none;
}
</style>
