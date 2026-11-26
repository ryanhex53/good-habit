<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useToast } from '../../composables/useToast'

const { current } = useToast()
const route = useRoute()

// 孩子端手机上抬高，避开底部 Tab 栏
const lifted = computed(() => route.path.startsWith('/kid'))
</script>

<template>
  <!-- pointer-events-none：提示不参与点击，不遮挡下方控件 -->
  <Transition name="toast-fade">
    <div
      v-if="current"
      class="toast toast-center toast-bottom pointer-events-none z-50"
      :class="lifted ? 'mb-24 lg:mb-8' : 'mb-8'"
      role="status"
      aria-live="polite"
    >
      <div class="alert alert-success border-none py-2 shadow-overlay">
        <span class="text-sm font-medium">{{ current.message }}</span>
      </div>
    </div>
  </Transition>
</template>
