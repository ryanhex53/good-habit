<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, errMsg } from '../../api'
import { useDialogs } from '../../composables/useDialogs'
import { useToast } from '../../composables/useToast'
import EmptyState from '../../components/ui/EmptyState.vue'
import PageHeader from '../../components/ui/PageHeader.vue'
import RowCard from '../../components/ui/RowCard.vue'
import type { PrizeAdminView } from '../../types'

const prizes = ref<PrizeAdminView[]>([])
const error = ref('')
const newName = ref('')
const newEmoji = ref('')

const singleCost = ref<number | null>(null)
const doubleCost = ref<number | null>(null)

const dialogs = useDialogs()
const toast = useToast()

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

async function rename(pz: PrizeAdminView) {
  const name = await dialogs.prompt({ title: '修改名称', label: '新名称', value: pz.name })
  if (!name || name.trim() === '') return
  try {
    await api('PATCH', `/api/prizes/${pz.id}`, { name: name.trim() })
    load()
  } catch (e) {
    error.value = errMsg(e)
  }
}

async function changeEmoji(pz: PrizeAdminView) {
  const emoji = await dialogs.prompt({
    title: '修改 emoji',
    label: '新 emoji（留空使用 🎁）',
    value: pz.emoji,
    allowEmpty: true,
  })
  if (emoji === null) return
  try {
    await api('PATCH', `/api/prizes/${pz.id}`, { emoji: emoji.trim() || '🎁' })
    load()
  } catch (e) {
    error.value = errMsg(e)
  }
}

async function saveCost() {
  error.value = ''
  try {
    await api('PUT', '/api/wheel-config', {
      singleCost: singleCost.value,
      doubleCost: doubleCost.value,
    })
    toast.show('定价已更新', 2000)
  } catch (e) {
    error.value = errMsg(e)
  }
}

onMounted(load)
</script>

<template>
  <div>
    <PageHeader title="奖品与定价" />
    <div v-if="error" class="alert alert-error py-2 text-sm" role="alert">{{ error }}</div>

    <div class="card border border-base-300 bg-base-100 shadow-sm">
      <div class="card-body gap-3">
        <h3 class="text-base font-bold">转盘定价</h3>
        <div class="flex flex-wrap items-end gap-3">
          <div>
            <label class="label mb-1 justify-start text-sm text-base-content/70" for="single-cost">
              单抽
            </label>
            <input
              id="single-cost"
              v-model.number="singleCost"
              type="number"
              min="1"
              max="999"
              class="input w-24 text-base max-lg:input-lg"
            />
          </div>
          <div>
            <label class="label mb-1 justify-start text-sm text-base-content/70" for="double-cost">
              保险双抽
            </label>
            <input
              id="double-cost"
              v-model.number="doubleCost"
              type="number"
              min="1"
              max="999"
              class="input w-24 text-base max-lg:input-lg"
            />
          </div>
          <button
            class="btn btn-primary max-lg:btn-lg"
            :disabled="!singleCost || !doubleCost"
            @click="saveCost"
          >
            保存
          </button>
        </div>
        <p class="text-xs text-base-content/50">约束：1 ≤ 单抽价 ≤ 双抽价 ≤ 999</p>
      </div>
    </div>

    <div class="card mt-4 border border-base-300 bg-base-100 shadow-sm">
      <div class="card-body gap-3">
        <h3 class="text-base font-bold">添加奖品</h3>
        <!-- 手机上竖向堆叠（join-vertical），lg 起横向拼接 -->
        <div class="join w-full max-lg:join-vertical">
          <input
            v-model="newEmoji"
            class="input join-item w-full text-base max-lg:input-lg lg:w-24"
            placeholder="emoji 🎁"
          />
          <input
            v-model="newName"
            class="input join-item w-full text-base max-lg:input-lg lg:w-auto lg:min-w-36 lg:flex-1"
            placeholder="奖品名，如：水果糖1颗"
          />
          <button
            class="btn btn-primary join-item max-lg:btn-lg"
            :disabled="!newName"
            @click="create"
          >
            添加
          </button>
        </div>
      </div>
    </div>

    <div class="mt-4 flex flex-col gap-3">
      <RowCard v-for="pz in prizes" :key="pz.id" :class="!pz.active ? 'opacity-60' : ''">
        <p class="flex items-center gap-2">
          <span class="text-2xl">{{ pz.emoji }}</span>
          <span class="font-semibold">{{ pz.name }}</span>
        </p>
        <template #actions>
          <button class="btn btn-ghost max-lg:min-h-11 lg:btn-sm" @click="rename(pz)">改名</button>
          <button class="btn btn-ghost max-lg:min-h-11 lg:btn-sm" @click="changeEmoji(pz)">
            改 emoji
          </button>
          <button class="btn btn-ghost max-lg:min-h-11 lg:btn-sm" @click="toggle(pz)">
            {{ pz.active ? '停用' : '启用' }}
          </button>
        </template>
      </RowCard>
    </div>
    <EmptyState
      v-if="prizes.length === 0"
      text="转盘还没有奖品"
      sub="可添加：水果糖 / 巧克力 / 自选零食之类的小奖品"
    />
  </div>
</template>
