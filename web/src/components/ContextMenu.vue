<script setup>
import { ref, watch, onUnmounted } from "vue";
import { menu, hideMenu } from "../contextmenu";

const box = ref(null);

// 点击任意处关闭（菜单自身用 pointerdown.stop 阻止冒泡，保证点击项能触发 onClick）
function onDocDown() {
  hideMenu();
}
function onKey(e) {
  if (e.key === "Escape") hideMenu();
}
function onBlur() {
  hideMenu();
}
function onScroll() {
  hideMenu();
}

watch(
  () => menu.visible,
  (v) => {
    if (v) {
      document.addEventListener("pointerdown", onDocDown);
      document.addEventListener("keydown", onKey);
      document.addEventListener("scroll", onScroll, true);
      window.addEventListener("blur", onBlur);
      // 视口边缘自动翻转，避免菜单超出屏幕
      requestAnimationFrame(() => {
        const el = box.value;
        if (!el) return;
        const r = el.getBoundingClientRect();
        if (r.right > window.innerWidth) {
          el.style.left = Math.max(4, menu.x - r.width) + "px";
        }
        if (r.bottom > window.innerHeight) {
          el.style.top = Math.max(4, menu.y - r.height) + "px";
        }
      });
    } else {
      document.removeEventListener("pointerdown", onDocDown);
      document.removeEventListener("keydown", onKey);
      document.removeEventListener("scroll", onScroll, true);
      window.removeEventListener("blur", onBlur);
    }
  },
);
onUnmounted(() => {
  document.removeEventListener("pointerdown", onDocDown);
  document.removeEventListener("keydown", onKey);
  document.removeEventListener("scroll", onScroll, true);
  window.removeEventListener("blur", onBlur);
});

function click(item) {
  hideMenu();
  if (item.onClick) item.onClick();
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="menu.visible"
      ref="box"
      class="ctx-menu"
      :style="{ left: menu.x + 'px', top: menu.y + 'px' }"
      @pointerdown.stop
      @contextmenu.prevent
    >
      <div
        v-for="(it, i) in menu.items"
        :key="i"
        class="ctx-item"
        :class="{ danger: it.danger, disabled: it.disabled, sep: it.sep }"
        @click="click(it)"
      >
        <template v-if="it.sep"></template>
        <template v-else>{{ it.label }}</template>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.ctx-menu {
  position: fixed;
  z-index: 10000;
  min-width: 160px;
  background: var(--bg-panel);
  border: 1px solid var(--border);
  border-radius: 6px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.55);
  padding: 4px;
  font-size: 12px;
}
.ctx-item {
  padding: 6px 12px;
  border-radius: 4px;
  cursor: pointer;
  color: #ddd;
  white-space: nowrap;
  user-select: none;
}
.ctx-item:hover {
  background: var(--accent);
  color: #fff;
}
.ctx-item.danger {
  color: #ff6b6b;
}
.ctx-item.danger:hover {
  background: #c0392b;
  color: #fff;
}
.ctx-item.disabled {
  opacity: 0.45;
  cursor: default;
  pointer-events: none;
}
.ctx-item.sep {
  padding: 0;
  margin: 4px 6px;
  height: 1px;
  background: var(--border);
  cursor: default;
  pointer-events: none;
}
</style>
