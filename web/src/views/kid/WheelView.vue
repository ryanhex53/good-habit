<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { api, errMsg } from '../../api'
import TickNumber from '../../components/TickNumber.vue'
import WheelCanvas from '../../components/WheelCanvas.vue'
import ResultModal from '../../components/ResultModal.vue'
import ChoiceModal from '../../components/ChoiceModal.vue'
import type { ChooseResp, DrawResp, Prize, WheelInfo } from '../../types'

const route = useRoute()
const wheelRef = ref<{ spin: (i: number) => Promise<void> } | null>(null)

const prizes = ref<Prize[]>([])
const singleCost = ref(5)
const doubleCost = ref(7)
const balance = ref(0)
const loaded = ref(false)

// 扣分浮动标签：−N，750ms 后消失
const floatText = ref('')
let floatTimer: ReturnType<typeof setTimeout> | undefined

const prefersReduced = () =>
  window.matchMedia('(prefers-reduced-motion: reduce)').matches

// 抽奖扣分的浮动标签：接口返回处飘出 −N，750ms 后清空；减少动态效果时直接不显示
function showFloat(cost: number) {
  if (prefersReduced()) return
  floatText.value = '−' + cost // − 是 U+2212 减号，不是连字符
  if (floatTimer !== undefined) clearTimeout(floatTimer)
  floatTimer = setTimeout(() => {
    floatText.value = ''
    floatTimer = undefined
  }, 750)
}

onBeforeUnmount(() => {
  if (floatTimer !== undefined) clearTimeout(floatTimer)
})

type Phase = 'idle' | 'spin' | 'first' | 'spin2' | 'choose' | 'result'
const phase = ref<Phase>('idle')
const firstPrize = ref<Prize | null>(null)
const pendingUUID = ref('')
const choices = ref<Prize[]>([])
const resultPrize = ref<Prize | null>(null)
const resultBalance = ref(0)
const toast = ref('')

const busy = computed(
  () => phase.value !== 'idle' && phase.value !== 'result' && phase.value !== 'choose',
)
const canSingle = computed(() => !busy.value && prizes.value.length > 0 && balance.value >= singleCost.value)
const canDouble = computed(() => !busy.value && prizes.value.length > 0 && balance.value >= doubleCost.value)

async function load() {
  const w = await api<WheelInfo>('GET', '/api/wheel')
  prizes.value = w.prizes
  singleCost.value = w.singleCost
  doubleCost.value = w.doubleCost
  balance.value = w.balance
  loaded.value = true
  return w
}

function indexOf(prize: Prize): number {
  const i = prizes.value.findIndex((p) => p.id === prize.id)
  return i >= 0 ? i : 0
}

async function draw(mode: 'single' | 'double') {
  toast.value = ''
  phase.value = 'spin'
  try {
    const resp = await api<DrawResp>('POST', '/api/draws', { mode })
    // 余额在接口返回处立即更新并飘出 −N，不等转盘转完（那时结果弹窗会盖住这一行）
    balance.value = resp.entry.balanceAfter
    showFloat(mode === 'single' ? singleCost.value : doubleCost.value)
    if (mode === 'single') {
      const prize = resp.draw.prize!
      await wheelRef.value?.spin(indexOf(prize))
      resultPrize.value = prize
      resultBalance.value = resp.entry.balanceAfter
      phase.value = 'result'
    } else {
      const cs = resp.draw.choices!
      firstPrize.value = cs[0]
      pendingUUID.value = resp.draw.uuid
      choices.value = cs
      await wheelRef.value?.spin(indexOf(cs[0]))
      phase.value = 'first'
    }
  } catch (e) {
    phase.value = 'idle'
    toast.value = errMsg(e)
    load().catch(() => undefined)
  }
}

async function spinSecond() {
  phase.value = 'spin2'
  try {
    await wheelRef.value?.spin(indexOf(choices.value[1]))
    phase.value = 'choose'
  } catch {
    phase.value = 'choose'
  }
}

async function pick(index: number) {
  if (!pendingUUID.value) return
  phase.value = 'spin'
  try {
    const resp = await api<ChooseResp>(
      'POST',
      `/api/draws/${pendingUUID.value}/choose`,
      { pick: index },
    )
    resultPrize.value = resp.draw.prize
    resultBalance.value = balance.value
    pendingUUID.value = ''
    choices.value = []
    firstPrize.value = null
    phase.value = 'result'
  } catch (e) {
    phase.value = 'choose'
    toast.value = errMsg(e)
  }
}

function closeResult() {
  phase.value = 'idle'
  resultPrize.value = null
  load().catch(() => undefined)
}

onMounted(async () => {
  await load()
  // 有待选局（无论从横幅跳来还是直接点 Tab 进来）都直接进入选择界面
  const resume = route.query.resume
  const w = await api<WheelInfo>('GET', '/api/wheel')
  if (w.pending && (!resume || w.pending.uuid === resume)) {
    pendingUUID.value = w.pending.uuid
    choices.value = w.pending.choices
    phase.value = 'choose'
  }
})
</script>

<template>
  <div class="pt-2">
    <p v-if="loaded" class="relative flex items-baseline justify-center gap-1.5">
      <span class="text-base text-base-content/60">当前积分</span>
      <TickNumber :value="balance" enter class="text-stat text-base-content" />
      <span class="text-sm text-base-content/55">分</span>
      <span v-if="floatText" class="float-chip">{{ floatText }}</span>
    </p>
    <WheelCanvas ref="wheelRef" :prizes="prizes" />
    <!-- 选择弹窗打开时，提示改渲染在弹窗内（弹窗处于顶层，行内提示会被遮住） -->
    <div v-if="toast && phase !== 'choose'" class="alert alert-error py-2 text-sm" role="alert">
      {{ toast }}
    </div>

    <div class="mt-4 flex gap-3 max-lg:flex-col">
      <button
        class="btn btn-primary h-auto min-h-16 flex-1 flex-col items-center justify-center gap-0.5 py-3 text-base leading-snug max-lg:btn-lg"
        :disabled="!canSingle"
        @click="draw('single')"
      >
        <span>抽一次</span>
        <small class="text-xs opacity-80">{{ singleCost }} 分</small>
      </button>
      <button
        class="btn btn-secondary h-auto min-h-16 flex-1 flex-col items-center justify-center gap-0.5 py-3 text-base leading-snug max-lg:btn-lg"
        :disabled="!canDouble"
        @click="draw('double')"
      >
        <span>保险抽两次</span>
        <small class="text-xs opacity-80">{{ doubleCost }} 分 · 挑喜欢的</small>
      </button>
    </div>

    <!-- 双抽第一轮结果 -->
    <div v-if="phase === 'first'" class="surface mt-4 animate-pop border-warning bg-warning/10">
      <div class="card-body flex-row flex-wrap items-center justify-between gap-3 py-4">
        <p class="font-semibold">第一抽：{{ firstPrize?.emoji }} {{ firstPrize?.name }}</p>
        <button class="btn btn-ghost max-lg:btn-lg" @click="spinSecond">再转一次 ⟳</button>
      </div>
    </div>

    <ChoiceModal v-if="phase === 'choose'" :choices="choices" :error="toast" @pick="pick" />
    <ResultModal
      v-if="phase === 'result' && resultPrize"
      :prize="resultPrize"
      :balance="resultBalance"
      @close="closeResult"
    />
  </div>
</template>
