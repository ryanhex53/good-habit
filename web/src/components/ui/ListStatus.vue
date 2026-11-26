<script setup lang="ts">
// 列表底部的加载/到底状态行
defineProps<{ loading?: boolean; done?: boolean }>()
</script>

<template>
  <p v-if="loading" class="py-4 text-center text-sm text-base-content/50">
    <span class="loading loading-spinner loading-sm align-[-2px]" />
    加载中…
  </p>
  <p v-else-if="done" class="py-4 text-center text-sm text-base-content/40">— 没有更多了 —</p>
</template>
