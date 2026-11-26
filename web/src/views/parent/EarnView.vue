<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { api, errMsg } from '../../api'
import { useToast } from '../../composables/useToast'
import FormField from '../../components/ui/FormField.vue'
import PageHeader from '../../components/ui/PageHeader.vue'
import type { ChildView, EntryResp, ItemView } from '../../types'

const route = useRoute()

const children = ref<ChildView[]>([])
const items = ref<ItemView[]>([])
const activeChildren = computed(() => children.value.filter((c) => c.active))
const activeItems = computed(() => items.value.filter((i) => i.active))

const selected = ref<number | null>(null)
const note = ref('')
const error = ref('')

// 手动调整
const adjDelta = ref<number | null>(null)
const adjNote = ref('')

// 双击防抖：toast 显示期间忽略点击（语义与原本地 toast ref 一致）
const toast = useToast()
const toastVisible = ref(false)
let toastTimer = 0

function flash(msg: string) {
  toast.show(msg)
  toastVisible.value = true
  clearTimeout(toastTimer)
  toastTimer = window.setTimeout(() => (toastVisible.value = false), 2200)
}

onBeforeUnmount(() => clearTimeout(toastTimer))

async function load() {
  children.value = (await api<{ children: ChildView[] }>('GET', '/api/children')).children
  items.value = (await api<{ items: ItemView[] }>('GET', '/api/reward-items')).items
  if (selected.value === null) {
    const fromQuery = Number(route.query.child)
    const found = activeChildren.value.find((c) => c.id === fromQuery)
    selected.value = found ? found.id : (activeChildren.value[0]?.id ?? null)
  }
}

async function earn(item: ItemView) {
  if (selected.value === null || toastVisible.value) return
  error.value = ''
  try {
    const res = await api<EntryResp>('POST', '/api/earnings', {
      childId: selected.value,
      rewardItemId: item.id,
      note: note.value || undefined,
    })
    // 更新本地余额
    const c = children.value.find((x) => x.id === selected.value)
    if (c) c.balance = res.entry.balanceAfter
    flash(`已记 ${res.entry.delta > 0 ? '+' : ''}${res.entry.delta} 分（当前 ${res.entry.balanceAfter} 分）`)
    note.value = ''
  } catch (e) {
    error.value = errMsg(e)
  }
}

async function adjust() {
  if (selected.value === null || adjDelta.value === null || adjDelta.value === 0) return
  error.value = ''
  try {
    const res = await api<EntryResp>('POST', '/api/adjustments', {
      childId: selected.value,
      delta: adjDelta.value,
      note: adjNote.value,
    })
    const c = children.value.find((x) => x.id === selected.value)
    if (c) c.balance = res.entry.balanceAfter
    flash(`已调整 ${res.entry.delta > 0 ? '+' : ''}${res.entry.delta} 分（当前 ${res.entry.balanceAfter} 分）`)
    adjDelta.value = null
    adjNote.value = ''
  } catch (e) {
    error.value = errMsg(e)
  }
}

onMounted(load)
</script>

<template>
  <div class="lg:grid lg:grid-cols-[minmax(0,1fr)_320px] lg:items-start lg:gap-6">
    <div class="min-w-0">
      <PageHeader title="登记得分" />

      <div v-if="error" class="alert alert-error py-2 text-sm" role="alert">{{ error }}</div>

      <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-4">
        <button
          v-for="it in activeItems"
          :key="it.id"
          class="btn h-auto min-h-24 flex-col justify-center gap-1 border border-base-300 bg-base-100 py-4 shadow-sm max-lg:btn-lg"
          :disabled="selected === null"
          @click="earn(it)"
        >
          <span class="text-lg font-extrabold text-brand-ink tabular-nums">+{{ it.points }} 分</span>
          <span class="max-w-full truncate text-sm font-normal">{{ it.title }}</span>
        </button>
      </div>
      <p v-if="activeItems.length === 0" class="py-8 text-center text-sm text-base-content/60">
        还没有积分项，去「积分项」页添加，或到总览页用一键导入
      </p>
    </div>

    <div class="flex flex-col gap-4 max-lg:mt-4">
      <div class="card border border-base-300 bg-base-100 shadow-sm">
        <div class="card-body gap-3">
          <h3 class="text-base font-bold">给谁登记</h3>
          <div class="flex flex-wrap gap-2">
            <button
              v-for="c in activeChildren"
              :key="c.id"
              class="btn btn-soft rounded-full btn-sm max-lg:btn-lg"
              :class="selected === c.id ? 'btn-primary' : ''"
              @click="selected = c.id"
            >
              {{ c.displayName }} · {{ c.balance }}分
            </button>
            <span v-if="activeChildren.length === 0" class="text-sm text-base-content/60">
              先去「孩子」页添加孩子
            </span>
          </div>
          <FormField label="备注（可不填）" for-id="note">
            <input
              id="note"
              v-model="note"
              class="input w-full text-base max-lg:input-lg"
              placeholder="今天完成得很好"
            />
          </FormField>
        </div>
      </div>

      <div class="card border border-base-300 bg-base-100 shadow-sm">
        <div class="card-body gap-3">
          <h3 class="text-base font-bold">手动调整</h3>
          <div class="flex flex-col gap-2">
            <input
              v-model.number="adjDelta"
              type="number"
              class="input w-full text-base max-lg:input-lg"
              placeholder="±分值（如 -20 或 10）"
            />
            <input
              v-model="adjNote"
              class="input w-full text-base max-lg:input-lg"
              placeholder="原因（必填）"
            />
            <button
              class="btn btn-primary max-lg:btn-lg"
              :disabled="selected === null || !adjDelta || !adjNote"
              @click="adjust"
            >
              调整
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
