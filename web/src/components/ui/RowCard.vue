<script setup lang="ts">
// 列表行卡片：leading（头像/emoji）/ 主内容（默认插槽）/ actions。
// 手机上 actions 换行独占一行，不与内容抢宽度；lg 起回到同一行。
</script>

<template>
  <div class="card bg-base-100 border border-base-300 shadow-sm">
    <div class="card-body flex-row flex-wrap items-center gap-3 py-4">
      <div v-if="$slots.leading" class="flex shrink-0 items-center gap-3">
        <slot name="leading" />
      </div>
      <div class="min-w-0 flex-1">
        <slot />
      </div>
      <div
        v-if="$slots.actions"
        class="flex flex-wrap items-center gap-2 max-lg:basis-full max-lg:-ml-2"
      >
        <slot name="actions" />
      </div>
    </div>
  </div>
</template>
