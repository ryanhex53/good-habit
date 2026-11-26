<script setup lang="ts">
// 列表行卡片：leading（头像/emoji）/ 主内容（默认插槽）/ actions。
</script>

<template>
  <div class="card bg-base-100 border border-base-300 shadow-sm">
    <div class="card-body flex-row flex-wrap items-center gap-3 py-4 max-lg:gap-2">
      <div v-if="$slots.leading" class="flex shrink-0 items-center gap-3">
        <slot name="leading" />
      </div>
      <div class="min-w-0 flex-1">
        <slot />
      </div>
      <div v-if="$slots.actions" class="flex flex-wrap items-center gap-2">
        <slot name="actions" />
      </div>
    </div>
  </div>
</template>
