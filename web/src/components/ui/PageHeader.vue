<script setup lang="ts">
defineProps<{ title: string; sub?: string }>()
</script>

<template>
  <div class="mb-5 flex items-end justify-between gap-3">
    <div class="min-w-0">
      <h2 class="truncate text-title">{{ title }}</h2>
      <p v-if="sub" class="text-sm text-base-content/60">{{ sub }}</p>
    </div>
    <div v-if="$slots.actions" class="flex shrink-0 items-center gap-2">
      <slot name="actions" />
    </div>
  </div>
</template>
