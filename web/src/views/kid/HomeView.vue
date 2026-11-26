<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api } from '../../api'
import BalanceCard from '../../components/BalanceCard.vue'
import GuideList from '../../components/GuideList.vue'
import PendingBanner from '../../components/PendingBanner.vue'
import type { GuideItem, PendingDraw, WheelInfo } from '../../types'

const balance = ref(0)
const pending = ref<PendingDraw | null>(null)
const items = ref<GuideItem[]>([])

async function load() {
  const wheel = await api<WheelInfo>('GET', '/api/wheel')
  balance.value = wheel.balance
  pending.value = wheel.pending
  const guide = await api<{ items: GuideItem[] }>('GET', '/api/guide')
  items.value = guide.items
}

onMounted(load)
</script>

<template>
  <div class="flex flex-col gap-4 pt-2">
    <PendingBanner v-if="pending" :pending="pending" />
    <BalanceCard :balance="balance" />
    <GuideList :items="items" />
  </div>
</template>
