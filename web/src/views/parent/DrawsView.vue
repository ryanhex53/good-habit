<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, errMsg } from '../../api'
import { useToast } from '../../composables/useToast'
import EmptyState from '../../components/ui/EmptyState.vue'
import PageHeader from '../../components/ui/PageHeader.vue'
import RowCard from '../../components/ui/RowCard.vue'
import { fmtTime, type ChildView, type DrawListItem } from '../../types'

const draws = ref<DrawListItem[]>([])
const children = ref<ChildView[]>([])
const error = ref('')
const loading = ref(false)
const toast = useToast()

async function load() {
  loading.value = true
  try {
    draws.value = (await api<{ draws: DrawListItem[] }>('GET', '/api/draws?state=unfulfilled')).draws
    children.value = (await api<{ children: ChildView[] }>('GET', '/api/children')).children
  } finally {
    loading.value = false
  }
}

async function fulfill(d: DrawListItem) {
  error.value = ''
  try {
    await api('PATCH', `/api/draws/${d.uuid}/fulfill`)
    toast.show(`已发放：${d.prizeName}（${d.childName}）`, 2200)
    load()
  } catch (e) {
    error.value = errMsg(e)
  }
}

onMounted(load)
</script>

<template>
  <div>
    <PageHeader title="待发放奖品" />
    <div v-if="error" class="alert alert-error py-2 text-sm" role="alert">{{ error }}</div>

    <div v-if="loading" class="flex justify-center py-10">
      <span class="loading loading-spinner" />
    </div>
    <EmptyState
      v-else-if="draws.length === 0"
      text="没有待发放的奖品"
      sub="孩子中奖后会出现在这里"
    />

    <div v-else class="flex flex-col gap-3">
      <RowCard v-for="d in draws" :key="d.uuid">
        <p class="text-sm">
          {{ d.childName }} 抽中
          <b class="text-primary">{{ d.prizeName }}</b>
        </p>
        <p class="mt-0.5 text-xs text-base-content/60">
          {{ d.mode === 'double' ? '保险抽' : '单抽' }} · {{ d.cost }} 分 · {{ fmtTime(d.createdAt) }}
        </p>
        <template #actions>
          <button class="btn btn-primary btn-sm max-lg:min-h-11" @click="fulfill(d)">
            标记已发放
          </button>
        </template>
      </RowCard>
    </div>
  </div>
</template>
