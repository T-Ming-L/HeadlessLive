<script setup>
import { ref, watch, nextTick } from "vue";
import { state, ui } from "../store";

const listRef = ref(null);

// 新日志自动滚到底
watch(
  () => state.logs.length,
  async () => {
    await nextTick();
    if (listRef.value) listRef.value.scrollTop = listRef.value.scrollHeight;
  },
);

function clear() {
  state.logs = [];
}
</script>

<template>
  <div class="log-drawer panel">
    <div class="panel-title">
      <span>📋 日志（{{ state.logs.length }}）</span>
      <div>
        <button class="btn small ghost" @click="clear">清空</button>
        <button class="btn small ghost" @click="ui.showLogs = false">✕</button>
      </div>
    </div>
    <div class="log-list" ref="listRef">
      <div v-for="(l, i) in state.logs" :key="i" class="log-line">{{ l }}</div>
      <div v-if="state.logs.length === 0" class="empty">等待日志...</div>
    </div>
  </div>
</template>

<style scoped>
.log-drawer {
  position: fixed;
  right: 12px;
  bottom: 56px;
  width: 560px;
  max-width: 90vw;
  height: 300px;
  display: flex;
  flex-direction: column;
  z-index: 900;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.5);
}
.log-list {
  flex: 1;
  overflow-y: auto;
  padding: 6px 10px;
}
.log-line {
  font-size: 11px;
  color: var(--text-dim);
  padding: 1px 0;
  font-family: "JetBrains Mono", Consolas, monospace;
  white-space: pre-wrap;
  word-break: break-all;
  border-bottom: 1px solid rgba(60, 60, 60, 0.3);
}
.log-line:last-child {
  color: var(--text);
}
.empty {
  color: var(--text-dim);
  text-align: center;
  padding: 30px 0;
}
</style>
