<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { api } from '../../api'
import BillRow from '../../components/BillRow.vue'
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
  <div class="bills">
    <h2>我的账单</h2>
    <p v-if="rows.length === 0 && !loading" class="empty">还没有记录，快去完成第一个积分项吧！</p>
    <div class="list">
      <BillRow v-for="r in rows" :key="r.id" :row="r" />
    </div>
    <p v-if="loading" class="loading">加载中…</p>
    <p v-else-if="done && rows.length > 0" class="loading">— 没有更多了 —</p>
  </div>
</template>

<style scoped>
h2 {
  margin: 6px 0 14px;
  font-size: 20px;
}
.list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.empty {
  color: var(--c-sub);
  padding: 40px 0;
  text-align: center;
}
.loading {
  text-align: center;
  color: var(--c-sub);
  font-size: 13px;
  padding: 14px 0 30px;
}
</style>
