<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { api, errMsg } from '../../api'
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
    balance.value = resp.entry.balanceAfter
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
  <div class="wheel-page">
    <div class="balance-line">当前积分 <b>{{ balance }}</b></div>
    <WheelCanvas ref="wheelRef" :prizes="prizes" />
    <p v-if="toast" class="err toast">{{ toast }}</p>

    <div class="actions">
      <button class="act single" :disabled="!canSingle" @click="draw('single')">
        抽一次<br /><small>{{ singleCost }} 分</small>
      </button>
      <button class="act double" :disabled="!canDouble" @click="draw('double')">
        保险抽两次<br /><small>{{ doubleCost }} 分 · 挑喜欢的</small>
      </button>
    </div>

    <!-- 双抽第一轮结果 -->
    <div v-if="phase === 'first'" class="card first-card">
      <p>第一抽：{{ firstPrize?.emoji }} {{ firstPrize?.name }}</p>
      <button @click="spinSecond">再转一次 ⟳</button>
    </div>

    <ChoiceModal v-if="phase === 'choose'" :choices="choices" @pick="pick" />
    <ResultModal
      v-if="phase === 'result' && resultPrize"
      :prize="resultPrize"
      :balance="resultBalance"
      @close="closeResult"
    />
  </div>
</template>

<style scoped>
.wheel-page {
  padding-top: 8px;
}
.balance-line {
  text-align: center;
  font-size: 16px;
  margin-bottom: 10px;
  color: var(--c-sub);
}
.balance-line b {
  color: var(--c-primary);
  font-size: 22px;
}
.actions {
  display: flex;
  gap: 12px;
  margin-top: 18px;
}
.act {
  flex: 1;
  padding: 14px 8px;
  font-size: 17px;
  font-weight: 700;
  line-height: 1.35;
}
.act small {
  font-size: 12px;
  font-weight: 400;
  opacity: 0.9;
}
.act.single {
  background: linear-gradient(135deg, #ff9a3c, #e8630a);
}
.act.double {
  background: linear-gradient(135deg, #8f5cf7, #6a3de8);
}
.toast {
  text-align: center;
  margin-top: 10px;
}
.first-card {
  margin-top: 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  animation: pop 0.25s;
}
.first-card p {
  font-size: 15px;
  font-weight: 600;
}
@keyframes pop {
  from {
    transform: scale(0.8);
    opacity: 0;
  }
}
</style>
