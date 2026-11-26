<script setup lang="ts">
import { useTheme } from '../../composables/useTheme'
import BaseIcon from './BaseIcon.vue'

const { theme, toggle } = useTheme()

defineProps<{ compact?: boolean }>()
</script>

<template>
  <button
    type="button"
    class="btn btn-ghost btn-circle"
    :class="compact ? 'btn-sm' : 'min-h-11 min-w-11'"
    :aria-label="theme === 'dark' ? '切换到浅色模式' : '切换到深色模式'"
    @click="toggle"
  >
    <BaseIcon :name="theme === 'dark' ? 'sun' : 'moon'" :class="compact ? 'size-4' : 'size-5'" />
  </button>
</template>
