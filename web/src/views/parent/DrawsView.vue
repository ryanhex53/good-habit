<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, errMsg } from '../../api'
import { fmtTime, type ChildView, type DrawListItem } from '../../types'

const draws = ref<DrawListItem[]>([])
const children = ref<ChildView[]>([])
const error = ref('')
const toast = ref('')
const loading = ref(false)

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
    toast.value = `已发放：${d.prizeName}（${d.childName}）`
    setTimeout(() => (toast.value = ''), 2200)
    load()
  } catch (e) {
    error.value = errMsg(e)
  }
}

onMounted(load)
</script>

<template>
  <div>
    <h2>待发放奖品</h2>
    <p class="err">{{ error }}</p>
    <p v-if="toast" class="ok-note">{{ toast }}</p>

    <p v-if="!loading && draws.length === 0" class="empty">
      没有待发放的奖品 —— 孩子中奖后会出现在这里
    </p>

    <div class="list">
      <div v-for="d in draws" :key="d.uuid" class="card row">
        <div class="info">
          <p class="who">{{ d.childName }} 抽中 <b>{{ d.prizeName }}</b></p>
          <p class="sub">
            {{ d.mode === 'double' ? '保险抽' : '单抽' }} · {{ d.cost }} 分 ·
            {{ fmtTime(d.createdAt) }}
          </p>
        </div>
        <button @click="fulfill(d)">标记已发放</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
h2 {
  margin-bottom: 14px;
}
.empty {
  color: var(--c-sub);
  padding: 30px 0;
  text-align: center;
}
.list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}
.who {
  font-size: 15px;
}
.who b {
  color: var(--c-primary);
}
.sub {
  font-size: 12px;
  color: var(--c-sub);
  margin-top: 3px;
}
</style>
