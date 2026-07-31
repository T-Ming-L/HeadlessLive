<script setup>
import { onMounted } from "vue";
import { refreshStatus } from "./api";
import { state, ui } from "./store";
import { connectWS } from "./ws";
import { toast } from "./toast";
import SourcePanel from "./components/SourcePanel.vue";
import SceneCanvas from "./components/SceneCanvas.vue";
import PropertyPanel from "./components/PropertyPanel.vue";
import BottomBar from "./components/BottomBar.vue";
import LogPanel from "./components/LogPanel.vue";
import AddSourceDialog from "./components/AddSourceDialog.vue";
import OutputManager from "./components/OutputManager.vue";
import ContextMenu from "./components/ContextMenu.vue";
import BiliAssistant from "./components/BiliAssistant.vue";

onMounted(async () => {
  try {
    await refreshStatus();
  } catch (e) {
    toast(`加载状态失败: ${e.message}`, "error");
  }
  connectWS();
});
</script>

<template>
  <div class="app">
    <header class="topbar">
      <div class="logo">🎬 HeadlessLive</div>
      <div class="conn" :class="{ on: state.connected }">
        {{ state.connected ? "● 已连接" : "○ 连接中" }}
      </div>
      <div class="spacer"></div>
      <button class="btn ghost" @click="ui.showLogs = !ui.showLogs">
        📋 日志
      </button>
      <button class="btn ghost" @click="ui.showOutputs = !ui.showOutputs">
        📡 输出管理
      </button>
      <button class="btn ghost bili-btn" @click="ui.showBili = true">
        🔴 B站助手
      </button>
    </header>

    <div class="main">
      <SourcePanel />
      <SceneCanvas />
      <PropertyPanel />
    </div>

    <BottomBar />

    <LogPanel v-if="ui.showLogs" />
    <AddSourceDialog
      v-if="ui.showAddSource"
      @close="ui.showAddSource = false"
    />
    <OutputManager v-if="ui.showOutputs" @close="ui.showOutputs = false" />
    <BiliAssistant v-if="ui.showBili" @close="ui.showBili = false" />
    <ContextMenu />
  </div>
</template>
