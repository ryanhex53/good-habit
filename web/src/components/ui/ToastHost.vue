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
  <Transition name="toast-fade">
    <div
      v-if="current"
      class="toast toast-center toast-bottom z-50"
      :class="lifted ? 'mb-24 lg:mb-8' : 'mb-8'"
      role="status"
      aria-live="polite"
    >
      <div class="alert alert-success border-none py-2 shadow-lg">
        <span class="text-sm font-medium">{{ current.message }}</span>
      </div>
    </div>
  </Transition>
</template>
