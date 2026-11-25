<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useSession } from '../session'

const router = useRouter()
const { me, logout } = useSession()

async function doLogout() {
  await logout()
  router.push('/login')
}
</script>

<template>
  <div class="kid-shell">
    <header class="kid-top">
      <span class="who">{{ me?.displayName }}</span>
      <button class="ghost mini" @click="doLogout">退出</button>
    </header>
    <main class="kid-main">
      <RouterView />
    </main>
    <nav class="kid-tabs">
      <RouterLink to="/kid/home">首页</RouterLink>
      <RouterLink to="/kid/wheel">转盘</RouterLink>
      <RouterLink to="/kid/bills">账单</RouterLink>
    </nav>
  </div>
</template>

<style scoped>
.kid-shell {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}
.kid-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
}
.who {
  font-weight: 700;
  font-size: 18px;
}
.mini {
  padding: 6px 12px;
  font-size: 13px;
}
.kid-main {
  flex: 1;
  max-width: 640px;
  width: 100%;
  margin: 0 auto;
  padding: 0 16px 84px;
}
.kid-tabs {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  background: var(--c-card);
  border-top: 1px solid #eee;
  padding: 6px 0 max(6px, env(safe-area-inset-bottom));
  z-index: 10;
}
.kid-tabs a {
  flex: 1;
  text-align: center;
  padding: 8px 0;
  text-decoration: none;
  color: var(--c-sub);
  font-size: 15px;
}
.kid-tabs a.router-link-active {
  color: var(--c-primary);
  font-weight: 700;
}
</style>
