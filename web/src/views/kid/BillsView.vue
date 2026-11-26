<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { api } from '../../api'
import BillRow from '../../components/BillRow.vue'
import EmptyState from '../../components/ui/EmptyState.vue'
import ListStatus from '../../components/ui/ListStatus.vue'
import PageHeader from '../../components/ui/PageHeader.vue'
import type { LedgerPage, LedgerRow } from '../../types'

const rows = ref<LedgerRow[]>([])
const nextBefore = ref<number | null>(null)
const loading = ref(false)
const done = ref(false)

async function loadMore() {
  if (loading.value || done.value) return
  loading.value = true
  try {
    const url = nextBefore.value === null ? '/api/ledger' : `/api/ledger?before=${nextBefore.value}`
    const page = await api<LedgerPage>('GET', url)
    rows.value.push(...page.rows)
    nextBefore.value = page.nextBefore
    if (page.nextBefore === null) done.value = true
  } finally {
    loading.value = false
  }
}

function onScroll() {
  const bottom = document.documentElement.scrollHeight - window.innerHeight - 200
  if (window.scrollY >= bottom) loadMore()
}

onMounted(() => {
  loadMore()
  window.addEventListener('scroll', onScroll, { passive: true })
})
onUnmounted(() => window.removeEventListener('scroll', onScroll))
</script>

<template>
  <div>
    <PageHeader title="我的账单" />
    <EmptyState v-if="rows.length === 0 && !loading" text="还没有记录，快去完成第一个积分项吧！" />
    <div class="flex flex-col gap-2 stagger">
      <BillRow v-for="r in rows" :key="r.id" :row="r" />
    </div>
    <ListStatus :loading="loading" :done="done && rows.length > 0" />
  </div>
</template>
