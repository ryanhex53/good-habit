<script setup lang="ts">
// 内联 SVG 图标集：路径取自 Lucide（ISC License，https://lucide.dev）、
// Feather（MIT）与 Tabler Icons（MIT），按本项目的线条粗细做了统一裁剪，无第三方运行时依赖。
// 调用方必须传 class 指定尺寸，如 class="size-5"。
const paths: Record<string, string[]> = {
  home: ['M3 11l9-9 9 9', 'M5 13v7a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-7', 'M12 22v-6'],
  // 转盘：轮辐 + 中心轴
  wheel: [
    'M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18',
    'M12 12L12 3',
    'M12 12L21 12',
    'M12 12L5.6 5.6',
    'M12 12L18.4 18.4',
    'M12 9.8a2.2 2.2 0 1 0 0 4.4 2.2 2.2 0 0 0 0-4.4',
  ],
  // 账单：文件 + 行
  receipt: [
    'M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6z',
    'M14 2v6h6',
    'M16 13H8',
    'M16 17H8',
    'M10 9H8',
  ],
  users: [
    'M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2',
    'M13 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0',
    'M22 21v-2a4 4 0 0 0-3-3.87',
    'M16 3.13a4 4 0 0 1 0 7.75',
  ],
  list: ['M8 6h13', 'M8 12h13', 'M8 18h13', 'M3.5 6h.01', 'M3.5 12h.01', 'M3.5 18h.01'],
  gift: [
    'M3 9h18',
    'M5 9v10a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V9',
    'M12 9v12',
    'M12 9c-1.5-3-6-3-6-1s4.5 2 6 1',
    'M12 9c1.5-3 6-3 6-1s-4.5 2-6 1',
  ],
  check: ['M20 6L9 17l-5-5'],
  plus: ['M12 5v14', 'M5 12h14'],
  sun: [
    'M16 12a4 4 0 1 1-8 0 4 4 0 0 1 8 0',
    'M12 2v2',
    'M12 20v2',
    'M4.93 4.93l1.41 1.41',
    'M17.66 17.66l1.41 1.41',
    'M2 12h2',
    'M20 12h2',
    'M6.34 17.66l-1.41 1.41',
    'M19.07 4.93l-1.41 1.41',
  ],
  moon: ['M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9z'],
  'log-out': ['M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4', 'M16 17l5-5-5-5', 'M21 12H9'],
  star: ['M12 2l3.09 6.26L22 9.27l-5 4.87L18.18 22 12 18.56 5.82 22 7 14.14l-5-4.87 6.91-1.01L12 2z'],
}

defineProps<{ name: string }>()
</script>

<template>
  <svg
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="2"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path v-for="(d, i) in paths[name] ?? []" :key="i" :d="d" />
  </svg>
</template>
