<script setup lang="ts">
defineProps<{ balance: number }>()
</script>

<template>
  <div class="bal-card">
    <p class="label">我的积分</p>
    <p class="num">{{ balance }}</p>
    <p class="hint">完成好事就能加分，攒够了去转盘抽奖</p>
  </div>
</template>

<style scoped>
.bal-card {
  background: linear-gradient(135deg, #ff9a3c, #e8630a);
  color: #fff;
  border-radius: 20px;
  padding: 22px 20px;
  box-shadow: 0 6px 18px rgba(232, 99, 10, 0.35);
}
.label {
  font-size: 14px;
  opacity: 0.9;
}
.num {
  font-size: 52px;
  font-weight: 800;
  line-height: 1.1;
  margin: 4px 0;
}
.hint {
  font-size: 13px;
  opacity: 0.85;
}
</style>
