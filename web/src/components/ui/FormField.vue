<script setup lang="ts">
// label + 控件插槽 + 提示/错误 的表单字段包装。
defineProps<{ label: string; forId?: string; hint?: string; error?: string }>()
</script>

<template>
  <div class="w-full">
    <label v-if="forId" :for="forId" class="label mb-1 justify-start text-sm text-base-content/70">
      {{ label }}
    </label>
    <span v-else class="label mb-1 justify-start text-sm text-base-content/70">{{ label }}</span>
    <slot />
    <p v-if="error" class="label mt-1 justify-start text-xs text-error">{{ error }}</p>
    <p v-else-if="hint" class="label mt-1 justify-start text-xs text-base-content/50">
      {{ hint }}
    </p>
  </div>
</template>
