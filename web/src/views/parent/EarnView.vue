<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { api, ApiError, errMsg } from '../../api'
import { useDialogs } from '../../composables/useDialogs'
import { useToast } from '../../composables/useToast'
import TickNumber from '../../components/TickNumber.vue'
import FormField from '../../components/ui/FormField.vue'
import PageHeader from '../../components/ui/PageHeader.vue'
import type { ChildView, EntryResp, ItemView } from '../../types'

const route = useRoute()
const dialogs = useDialogs()

const children = ref<ChildView[]>([])
const items = ref<ItemView[]>([])
const activeChildren = computed(() => children.value.filter((c) => c.active))
// 登记页只显示选中孩子适用的积分项：childIds 为空 = 所有孩子；未选中孩子时只按启用状态过滤
const activeItems = computed(() =>
  items.value.filter(
    (i) => i.active && (selected.value === null || i.childIds.length === 0 || i.childIds.includes(selected.value)),
  ),
)

// 空状态文案：家庭没有启用中的积分项时给添加指引；有但选中孩子一个都不适用时点名孩子
const emptyText = computed<string>(() => {
  if (!items.value.some((i) => i.active)) return '还没有积分项，先去「积分项」页添加'
  const c = children.value.find((x) => x.id === selected.value)
  return c ? `「${c.displayName}」暂无可登记的积分项` : '还没有积分项，先去「积分项」页添加'
})

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

// 登记确认：点积分项先弹确认框，确认后才发请求；confirming 防重入（连点只开一个弹窗）。
// 成功后在刚点的积分项上方飘出 +N，750ms 后移除；减少动态效果时不飘。
const confirming = ref(false)
const float = ref<{ id: number; text: string } | null>(null)
let floatTimer = 0

onBeforeUnmount(() => {
  clearTimeout(toastTimer)
  clearTimeout(floatTimer)
})

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
  if (confirming.value) return
  if (selected.value === null || toastVisible.value) return
  const child = children.value.find((c) => c.id === selected.value)
  if (!child) return
  error.value = ''
  confirming.value = true
  try {
    const ok = await dialogs.confirm({
      title: '登记得分',
      message: `给 ${child.displayName} 记一笔「${item.title}」，+${item.points} 分。`,
      confirmText: '登记',
    })
    if (!ok) return
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
      if (!window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
        float.value = { id: item.id, text: '+' + item.points }
        clearTimeout(floatTimer)
        floatTimer = window.setTimeout(() => (float.value = null), 750)
      }
    } catch (e) {
      // 陈旧标签页：范围刚被别的标签页改掉 → 立即刷新，别让过期格子留在页面上
      if (e instanceof ApiError && e.code === 'item_not_applicable') load()
      error.value = errMsg(e)
    }
  } finally {
    confirming.value = false
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
  <!--
    手机（DOM 顺序）：给谁登记 → 积分项 → 手动调整（与原版一致）。
    lg 起两栏：两张卡片（给谁登记、手动调整）在左 320px 列，积分项在右。
  -->
  <div>
    <PageHeader title="登记得分" />
    <div v-if="error" class="alert alert-error mb-4 py-2 text-sm" role="alert">{{ error }}</div>

    <div
      class="mt-5 flex flex-col gap-6 lg:grid lg:grid-cols-[320px_minmax(0,1fr)] lg:items-start lg:gap-x-8 lg:gap-y-6"
    >
      <section class="lg:col-start-1 lg:row-start-1">
        <div class="surface p-5">
          <div class="flex flex-col gap-3">
            <h3 class="text-base font-semibold">给谁登记</h3>
            <div class="flex flex-wrap gap-2">
              <button
                v-for="c in activeChildren"
                :key="c.id"
                type="button"
                class="btn rounded-full max-lg:btn-lg"
                :class="
                  selected === c.id
                    ? 'btn-primary border-primary/25 bg-primary/10 text-brand-ink hover:border-primary/25 hover:bg-primary/15 hover:text-brand-ink'
                    : 'btn-soft'
                "
                :aria-pressed="selected === c.id"
                @click="selected = c.id"
              >
                <span class="font-medium">{{ c.displayName }}</span>
                <TickNumber :value="c.balance" class="tabular-nums" />
                <span class="text-xs">分</span>
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
      </section>

      <!-- 积分项：白底卡片，数字放大；点击先确认再登记 -->
      <section class="min-w-0 lg:col-start-2 lg:row-span-2 lg:row-start-1">
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-4">
          <span v-for="it in activeItems" :key="it.id" class="relative block">
            <button
              type="button"
              class="btn h-auto min-h-24 w-full flex-col justify-center gap-1 rounded-2xl border-base-300 bg-base-100 px-3 py-4 hover:border-primary/40 active:scale-[0.97] max-lg:btn-lg"
              :disabled="selected === null"
              @click="earn(it)"
            >
              <span class="text-stat text-brand-ink">
                +{{ it.points }}<span class="ml-1 text-sm font-medium">分</span>
              </span>
              <span class="w-full truncate text-sm font-normal text-base-content/75">{{ it.title }}</span>
            </button>
            <span v-if="float && float.id === it.id" class="float-chip" aria-hidden="true">{{ float.text }}</span>
          </span>
        </div>
        <p v-if="activeItems.length === 0" class="py-8 text-center text-sm text-base-content/60">
          {{ emptyText }}
        </p>
      </section>

      <section class="lg:col-start-1 lg:row-start-2">
        <div class="surface p-5">
          <h3 class="text-base font-semibold">手动调整</h3>
          <div class="mt-3 flex flex-col gap-3">
            <FormField label="分值" for-id="adj-delta">
              <input
                id="adj-delta"
                v-model.number="adjDelta"
                type="number"
                class="input w-full text-base max-lg:input-lg"
                placeholder="扣分填负数，如 -20"
              />
            </FormField>
            <FormField label="原因" for-id="adj-note">
              <input
                id="adj-note"
                v-model="adjNote"
                class="input w-full text-base max-lg:input-lg"
                placeholder="必填"
              />
            </FormField>
            <button
              type="button"
              class="btn btn-outline max-lg:btn-lg"
              :disabled="selected === null || !adjDelta || !adjNote"
              @click="adjust"
            >
              调整
            </button>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>
