<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, errMsg } from '../../api'
import type { PrizeAdminView } from '../../types'

const prizes = ref<PrizeAdminView[]>([])
const error = ref('')
const newName = ref('')
const newEmoji = ref('')

const singleCost = ref<number | null>(null)
const doubleCost = ref<number | null>(null)
const costSaved = ref('')

async function load() {
  prizes.value = (await api<{ prizes: PrizeAdminView[] }>('GET', '/api/prizes')).prizes
  const cfg = await api<{ singleCost: number; doubleCost: number }>('GET', '/api/wheel-config')
  singleCost.value = cfg.singleCost
  doubleCost.value = cfg.doubleCost
}

async function create() {
  error.value = ''
  try {
    await api('POST', '/api/prizes', {
      name: newName.value,
      emoji: newEmoji.value || undefined,
    })
    newName.value = ''
    newEmoji.value = ''
    load()
  } catch (e) {
    error.value = errMsg(e)
  }
}

async function toggle(pz: PrizeAdminView) {
  try {
    await api('PATCH', `/api/prizes/${pz.id}`, { active: !pz.active })
    load()
  } catch (e) {
    error.value = errMsg(e)
  }
}

async function saveCost() {
  error.value = ''
  costSaved.value = ''
  try {
    await api('PUT', '/api/wheel-config', {
      singleCost: singleCost.value,
      doubleCost: doubleCost.value,
    })
    costSaved.value = '定价已更新'
    setTimeout(() => (costSaved.value = ''), 2000)
  } catch (e) {
    error.value = errMsg(e)
  }
}

onMounted(load)
</script>

<template>
  <div>
    <h2>奖品与定价</h2>
    <p class="err">{{ error }}</p>

    <div class="card cost">
      <h3>转盘定价</h3>
      <div class="cost-row">
        <label>单抽
          <input v-model.number="singleCost" type="number" min="1" max="999" />
        </label>
        <label>保险双抽
          <input v-model.number="doubleCost" type="number" min="1" max="999" />
        </label>
        <button :disabled="!singleCost || !doubleCost" @click="saveCost">保存</button>
        <span v-if="costSaved" class="ok-note">{{ costSaved }}</span>
      </div>
      <p class="hint">约束：1 ≤ 单抽价 ≤ 双抽价 ≤ 999</p>
    </div>

    <div class="card add">
      <div class="add-row">
        <input v-model="newEmoji" placeholder="emoji 🎁" style="max-width: 90px" />
        <input v-model="newName" placeholder="奖品名，如：水果糖1颗" />
        <button :disabled="!newName" @click="create">添加</button>
      </div>
    </div>

    <div class="list">
      <div v-for="pz in prizes" :key="pz.id" class="card row" :class="{ off: !pz.active }">
        <span class="emoji">{{ pz.emoji }}</span>
        <span class="name">{{ pz.name }}</span>
        <span class="ops">
          <button class="ghost" @click="toggle(pz)">{{ pz.active ? '停用' : '启用' }}</button>
        </span>
      </div>
      <p v-if="prizes.length === 0" class="empty">
        转盘还没有奖品；去总览页一键导入示例（水果糖/巧克力/自选零食）
      </p>
    </div>
  </div>
</template>

<style scoped>
h2 {
  margin-bottom: 14px;
}
.cost {
  margin-bottom: 12px;
}
.cost h3 {
  font-size: 16px;
  margin-bottom: 10px;
}
.cost-row {
  display: flex;
  align-items: center;
  gap: 14px;
  flex-wrap: wrap;
}
.cost-row label {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
  color: var(--c-ink);
  font-size: 14px;
}
.cost-row input {
  width: 90px;
}
.hint {
  font-size: 12px;
  color: var(--c-sub);
  margin-top: 8px;
}
.add-row {
  display: flex;
  gap: 8px;
}
.list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 14px;
}
.row {
  display: flex;
  align-items: center;
  gap: 12px;
}
.row.off {
  opacity: 0.6;
}
.emoji {
  font-size: 22px;
}
.name {
  flex: 1;
  font-size: 15px;
  font-weight: 600;
}
.ops .ghost {
  padding: 6px 10px;
  font-size: 13px;
}
.empty {
  color: var(--c-sub);
  padding: 20px 0;
}
</style>
