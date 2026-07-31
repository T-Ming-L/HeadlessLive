import { reactive } from "vue";

// 全局右键菜单状态（各组件调用 showMenu 弹出）
export const menu = reactive({
  visible: false,
  x: 0,
  y: 0,
  items: [],
});

// items: [{ label, danger?, disabled?, onClick }, { sep: true }, ...]
export function showMenu(e, items) {
  menu.items = items;
  menu.x = e.clientX;
  menu.y = e.clientY;
  menu.visible = true;
}

export function hideMenu() {
  menu.visible = false;
  menu.items = [];
}
