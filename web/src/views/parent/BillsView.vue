<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { api } from '../../api'
import BillRow from '../../components/BillRow.vue'
import type { ChildView, LedgerPage, LedgerRow } from '../../types'

const children = ref<ChildView[]>([])
const childId = ref(0) // 0 = 全家
const rows = ref<LedgerRow[]>([])
const nextBefore = ref<number | null>(null)
const done = ref(false)
const loading = ref(false)

async function loadMore() {
  if (loading.value || done.value) return
  loading.value = true
  try {
    const params = new URLSearchParams()
    if (childId.value !== 0) params.set('childId', String(childId.value))
    if (nextBefore.value !== null) params.set('before', String(nextBefore.value))
    const qs = params.toString()
    const page = await api<LedgerPage>('GET', qs ? `/api/ledger?${qs}` : '/api/ledger')
    rows.value.push(...page.rows)
    nextBefore.value = page.nextBefore
    if (page.nextBefore === null) done.value = true
  } finally {
    loading.value = false
  }
}

async function switchChild() {
  rows.value = []
  nextBefore.value = null
  done.value = false
  loadMore()
}

function onScroll() {
  const bottom = document.documentElement.scrollHeight - window.innerHeight - 200
  if (window.scrollY >= bottom) loadMore()
}

onMounted(() => {
  loadMore()
  api<{ children: ChildView[] }>('GET', '/api/children').then((r) => {
    children.value = r.children.filter((c) => c.active)
  })
  window.addEventListener('scroll', onScroll, { passive: true })
})
onUnmounted(() => window.removeEventListener('scroll', onScroll))
</script>

<template>
  <div>
    <h2>家庭账单</h2>
    <div class="filters">
      <select v-model.number="childId" @change="switchChild">
        <option :value="0">全部孩子</option>
        <option v-for="c in children" :key="c.id" :value="c.id">{{ c.displayName }}</option>
      </select>
    </div>
    <p v-if="rows.length === 0 && !loading" class="empty">还没有账单记录</p>
    <div class="list">
      <BillRow v-for="r in rows" :key="r.id" :row="r" />
    </div>
    <p v-if="loading" class="loading">加载中…</p>
    <p v-else-if="done && rows.length > 0" class="loading">— 没有更多了 —</p>
  </div>
</template>

<style scoped>
h2 {
  margin-bottom: 12px;
}
.filters select {
  max-width: 200px;
  margin-bottom: 14px;
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
