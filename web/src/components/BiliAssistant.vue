<script setup>
import { ref, onMounted, onUnmounted } from "vue";
import QRCode from "qrcode";
import { api, refreshStatus } from "../api";
import { state } from "../store";
import { toast } from "../toast";

const emit = defineEmits(["close"]);

// 状态
const loading = ref(true);
const loggedIn = ref(false);
const roomId = ref("");
const title = ref("");
const isLive = ref(false);
const statusMsg = ref("");

// 二维码
const qrImg = ref("");
const qrKey = ref("");
const qrTimer = ref(null);
const countdown = ref(0);
let countdownTimer = null;

// 分区
const areas = ref([]);
const parentArea = ref("");
const childArea = ref("");

// 开播结果
const stream = ref(null); // { rtmp_addr, rtmp_code }
const busy = ref(false);

onMounted(async () => {
  await loadStatus();
});

onUnmounted(() => {
  stopQR();
});

async function loadStatus() {
  loading.value = true;
  try {
    const r = await api.get("/api/bili/status");
    loggedIn.value = !!r.logged_in;
    if (r.logged_in) {
      stopQR();
      qrImg.value = "";
      roomId.value = r.room_id || "";
      title.value = r.title || "";
      isLive.value = !!r.is_live;
      if (r.area_id) childArea.value = String(r.area_id);
      if (r.parent_area_id) parentArea.value = String(r.parent_area_id);
      loadAreas();
      // 未开播时清掉旧的开播结果
      if (!r.is_live) stream.value = null;
    } else {
      startQR();
    }
  } catch (e) {
    statusMsg.value = "无法连接 B 站服务: " + e.message;
  }
  loading.value = false;
}

// ---- 二维码登录 ----
async function startQR() {
  stopQR();
  statusMsg.value = "正在生成二维码...";
  try {
    const r = await api.get("/api/bili/qrcode");
    qrKey.value = r.qrcode_key;
    qrImg.value = await QRCode.toDataURL(r.url, {
      width: 220,
      margin: 1,
      color: { dark: "#000000", light: "#ffffff" },
    });
    statusMsg.value = "请使用 B 站手机客户端扫码登录";
    countdown.value = 180;
    countdownTimer = setInterval(() => {
      countdown.value -= 1;
      if (countdown.value <= 0) {
        stopQR();
        startQR();
      }
    }, 1000);
    qrTimer.value = setInterval(poll, 2000);
  } catch (e) {
    statusMsg.value = "生成二维码失败: " + e.message;
  }
}

async function poll() {
  if (!qrKey.value) return;
  try {
    const r = await api.get(
      "/api/bili/poll?key=" + encodeURIComponent(qrKey.value),
    );
    if (r.code === 0) {
      stopQR();
      statusMsg.value = "登录成功！";
      await loadStatus();
    } else if (r.code === 86101) {
      statusMsg.value = "等待扫码中...";
    } else if (r.code === 86090) {
      statusMsg.value = "已扫码，请在手机上确认登录";
    } else if (r.code === 86038) {
      statusMsg.value = "二维码已过期，正在刷新...";
      startQR();
    } else {
      statusMsg.value = r.message || "未知状态";
    }
  } catch (e) {
    statusMsg.value = "轮询失败: " + e.message;
  }
}

function stopQR() {
  if (qrTimer.value) {
    clearInterval(qrTimer.value);
    qrTimer.value = null;
  }
  if (countdownTimer) {
    clearInterval(countdownTimer);
    countdownTimer = null;
  }
}

async function refreshQR() {
  await startQR();
}

async function logout() {
  if (!confirm("退出 B 站登录？")) return;
  try {
    await api.post("/api/bili/logout");
    loggedIn.value = false;
    stream.value = null;
    await startQR();
    toast("已退出登录", "ok");
  } catch (e) {
    toast(e.message, "error");
  }
}

// ---- 分区 ----
async function loadAreas() {
  try {
    const r = await api.get("/api/bili/areas");
    areas.value = r.areas || [];
  } catch (e) {
    toast("加载分区失败: " + e.message, "error");
  }
}
function parentList() {
  return areas.value.filter((a) => a.list && a.list.length);
}
function childList() {
  const p = areas.value.find((a) => String(a.id) === String(parentArea.value));
  return p ? p.list || [] : [];
}

// ---- 开播/停播 ----
async function toggleLive() {
  if (isLive.value) {
    busy.value = true;
    try {
      await api.post("/api/bili/stop");
      isLive.value = false;
      toast("已停播", "ok");
    } catch (e) {
      toast(e.message, "error");
    }
    busy.value = false;
    return;
  }
  if (!childArea.value) {
    toast("请选择直播分区", "error");
    return;
  }
  busy.value = true;
  try {
    const r = await api.post("/api/bili/start", {
      area: String(childArea.value),
    });
    if (r.qr) {
      // 需要人脸验证
      qrImg.value = await QRCode.toDataURL(r.qr, { width: 220, margin: 1 });
      statusMsg.value = "需要人脸验证：" + (r.message || "请用手机扫码验证");
      toast("开播需人脸验证，请扫码", "warning");
    } else if (r.message) {
      toast(r.message, "error");
    } else {
      stream.value = { rtmp_addr: r.rtmp_addr, rtmp_code: r.rtmp_code };
      isLive.value = true;
      statusMsg.value = "开播成功，已获取推流地址和密钥";
      toast("开播成功！", "ok");
    }
  } catch (e) {
    toast(e.message, "error");
    statusMsg.value = e.message;
  }
  busy.value = false;
}

// ---- 填入输出配置 ----
async function applyToOutput() {
  if (!stream.value) return;
  const addr = stream.value.rtmp_addr;
  const code = stream.value.rtmp_code;
  const payload = {
    name: "哔哩哔哩",
    type: "rtmp",
    scene_id:
      state.currentSceneId || (state.scenes[0] && state.scenes[0].id) || "",
    url: addr,
    stream_key: code,
    encoder: "h264_vaapi",
    bitrate: 6000,
    keyframe: 120,
    enabled: true,
  };
  try {
    const existing = state.outputs.find(
      (o) =>
        o.name === "哔哩哔哩" || (o.url && o.url.indexOf("bilivideo") !== -1),
    );
    if (existing) {
      await api.updateOutput(existing.id, { ...existing, ...payload });
    } else {
      await api.createOutput(payload);
    }
    await refreshStatus();
    toast("已填入输出配置「哔哩哔哩」，到底部点 ▶ 开始 即可推流", "ok");
  } catch (e) {
    toast(e.message, "error");
  }
}

function copyText(t) {
  navigator.clipboard
    .writeText(t)
    .then(() => toast("已复制到剪贴板", "ok"))
    .catch(() => toast("复制失败", "error"));
}
</script>

<template>
  <div class="bili-mask" @click.self="emit('close')">
    <div class="bili-panel panel">
      <div class="bili-head">
        <span>🔴 B 站直播助手</span>
        <button class="mini x" @click="emit('close')">✕</button>
      </div>

      <div v-if="loading" class="bili-body center">加载中...</div>

      <!-- 未登录：扫码 -->
      <div v-else-if="!loggedIn" class="bili-body center">
        <div class="qr-box" v-if="qrImg">
          <img :src="qrImg" alt="登录二维码" />
        </div>
        <div v-else class="qr-box placeholder">二维码</div>
        <div class="status">{{ statusMsg }}</div>
        <div class="hint" v-if="countdown > 0">
          {{ countdown }} 秒后自动刷新
        </div>
        <button class="btn small" @click="refreshQR">🔄 刷新二维码</button>
      </div>

      <!-- 已登录：直播间管理 -->
      <div v-else class="bili-body">
        <!-- 人脸验证二维码（开播被要求人脸验证时显示） -->
        <div v-if="qrImg" class="center">
          <div class="qr-box">
            <img :src="qrImg" alt="人脸验证二维码" />
          </div>
          <div class="status">{{ statusMsg }}</div>
          <button class="btn small" @click="qrImg = ''">关闭</button>
        </div>

        <template v-else>
          <div class="row">
            <span class="lbl">直播间号</span>
            <span class="val">{{ roomId || "—" }}</span>
            <span class="live-tag" :class="{ on: isLive }">
              {{ isLive ? "● 直播中" : "○ 未开播" }}
            </span>
          </div>
          <div class="row">
            <span class="lbl">标题</span>
            <input v-model="title" class="title" readonly :title="title" />
          </div>
          <div class="row">
            <span class="lbl">分区</span>
            <select v-model="parentArea" class="sel">
              <option value="">-- 父分区 --</option>
              <option
                v-for="a in parentList()"
                :key="a.id"
                :value="String(a.id)"
              >
                {{ a.name }}
              </option>
            </select>
            <select v-model="childArea" class="sel">
              <option value="">-- 子分区 --</option>
              <option
                v-for="a in childList()"
                :key="a.id"
                :value="String(a.id)"
              >
                {{ a.name }}
              </option>
            </select>
          </div>

          <div class="row actions">
            <button
              class="btn"
              :class="{ danger: isLive }"
              :disabled="busy"
              @click="toggleLive"
            >
              {{ busy ? "处理中..." : isLive ? "■ 停播" : "▶ 开播" }}
            </button>
            <button class="btn ghost" @click="logout">退出登录</button>
          </div>

          <!-- 开播结果 -->
          <div v-if="stream" class="result">
            <div class="res-title">✅ 推流信息</div>
            <div class="row">
              <span class="lbl">推流地址</span>
              <div class="res-val">
                <code>{{ stream.rtmp_addr }}</code>
                <button class="mini" @click="copyText(stream.rtmp_addr)">
                  复制
                </button>
              </div>
            </div>
            <div class="row">
              <span class="lbl">推流密钥</span>
              <div class="res-val">
                <code>{{ stream.rtmp_code }}</code>
                <button class="mini" @click="copyText(stream.rtmp_code)">
                  复制
                </button>
              </div>
            </div>
            <button class="btn primary" @click="applyToOutput">
              ⬇ 填入输出配置并开始推流
            </button>
            <div class="hint">
              填入后到底部「输出管理」点 ▶ 开始 即可推送到 B 站
            </div>
          </div>

          <div v-if="statusMsg" class="status">{{ statusMsg }}</div>
        </template>
      </div>
    </div>
  </div>
</template>

<style scoped>
.bili-mask {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.55);
  z-index: 9000;
  display: flex;
  align-items: center;
  justify-content: center;
}
.bili-panel {
  width: 430px;
  max-width: 92vw;
  background: var(--bg-panel);
  border: 1px solid var(--border);
  border-radius: 8px;
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.6);
  overflow: hidden;
}
.bili-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
  font-weight: 700;
}
.mini.x {
  background: none;
  border: none;
  color: var(--text-dim);
  cursor: pointer;
  font-size: 14px;
}
.bili-body {
  padding: 16px;
  max-height: 72vh;
  overflow-y: auto;
}
.center {
  text-align: center;
}
.qr-box {
  width: 220px;
  height: 220px;
  margin: 4px auto 10px;
  background: #fff;
  border-radius: 8px;
  padding: 6px;
}
.qr-box img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}
.qr-box.placeholder {
  background: var(--bg);
  color: var(--text-dim);
  display: flex;
  align-items: center;
  justify-content: center;
}
.status {
  margin: 8px 0;
  font-size: 12px;
  color: var(--yellow, #e5c07b);
  min-height: 16px;
}
.hint {
  font-size: 11px;
  color: var(--text-dim);
  margin: 4px 0 10px;
}
.row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
}
.lbl {
  width: 64px;
  flex-shrink: 0;
  font-size: 12px;
  color: var(--text-dim);
}
.val {
  font-size: 13px;
}
.live-tag {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 10px;
  background: #3c3c3c;
}
.live-tag.on {
  background: #7a1f1f;
  color: #ff9d9d;
}
.title {
  flex: 1;
  min-width: 0;
}
.sel {
  flex: 1;
  min-width: 0;
}
.actions {
  margin-top: 4px;
}
.result {
  margin-top: 10px;
  border-top: 1px solid var(--border);
  padding-top: 10px;
}
.res-title {
  font-size: 13px;
  font-weight: 700;
  margin-bottom: 8px;
}
.res-val {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
}
.res-val code {
  flex: 1;
  min-width: 0;
  font-size: 11px;
  background: #111;
  padding: 4px 6px;
  border-radius: 3px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.btn.primary {
  width: 100%;
  margin-top: 6px;
}
</style>
