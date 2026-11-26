<script setup lang="ts">
defineProps<{ balance: number }>()
</script>

<template>
  <!-- 记分板：橙金渐变 + 超大等宽数字 -->
  <div
    class="card overflow-hidden border-0 bg-linear-to-br from-accent to-primary text-primary-content shadow-lg shadow-primary/30"
  >
    <div class="card-body gap-1 py-6">
      <p class="self-start rounded-full bg-black/25 px-2.5 py-0.5 text-sm">我的积分</p>
      <p class="text-6xl font-extrabold leading-none tabular-nums">{{ balance }}</p>
      <p class="mt-1 self-start rounded-lg bg-black/25 px-2.5 py-1 text-xs">
        完成好事就能加分，攒够了去转盘抽奖
      </p>
    </div>
  </div>
</template>
