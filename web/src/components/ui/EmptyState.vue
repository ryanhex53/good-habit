<script setup lang="ts">
defineProps<{ text: string; sub?: string }>()
</script>

<template>
  <div class="flex flex-col items-center justify-center gap-1 py-10 text-center">
    <div class="rounded-full bg-base-200 p-4 text-2xl">
      <slot name="icon">🌱</slot>
    </div>
    <p class="text-sm text-base-content/60">{{ text }}</p>
    <p v-if="sub" class="text-xs text-base-content/40">{{ sub }}</p>
    <div v-if="$slots.default" class="mt-2">
      <slot />
    </div>
  </div>
</template>
