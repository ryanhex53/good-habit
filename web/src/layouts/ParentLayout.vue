<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useSession } from '../session'

const router = useRouter()
const { me, logout } = useSession()

const links = [
  { to: '/parent/overview', label: '总览' },
  { to: '/parent/earn', label: '登记得分' },
  { to: '/parent/children', label: '孩子' },
  { to: '/parent/items', label: '积分项' },
  { to: '/parent/prizes', label: '奖品与定价' },
  { to: '/parent/draws', label: '待发放' },
  { to: '/parent/bills', label: '账单' },
]

async function doLogout() {
  await logout()
  router.push('/login')
}
</script>

<template>
  <div class="parent-shell">
    <!-- 手机端顶栏（含登出，桌面端隐藏） -->
    <div class="mobile-bar">
      <span class="m-brand">好习惯</span>
      <span class="m-who">{{ me?.displayName }}</span>
      <button class="ghost mini" @click="doLogout">退出</button>
    </div>
    <aside class="side">
      <div class="brand">好习惯 · 家长端</div>
      <nav>
        <RouterLink v-for="l in links" :key="l.to" :to="l.to">{{ l.label }}</RouterLink>
      </nav>
      <div class="side-foot">
        <span>{{ me?.displayName }}</span>
        <button class="ghost mini" @click="doLogout">退出</button>
      </div>
    </aside>
    <main class="main">
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.parent-shell {
  display: flex;
  min-height: 100vh;
}
.side {
  width: 220px;
  background: var(--c-card);
  border-right: 1px solid #eee;
  display: flex;
  flex-direction: column;
  padding: 18px 14px;
  position: sticky;
  top: 0;
  height: 100vh;
}
.brand {
  font-weight: 800;
  margin-bottom: 18px;
  padding: 0 8px;
}
.side nav {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1;
}
.side nav a {
  padding: 10px 12px;
  border-radius: 10px;
  text-decoration: none;
  color: var(--c-ink);
  font-size: 15px;
}
.side nav a.router-link-active {
  background: #fdeadd;
  color: var(--c-primary);
  font-weight: 700;
}
.side-foot {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 8px;
  font-size: 14px;
  color: var(--c-sub);
}
.mini {
  padding: 6px 12px;
  font-size: 13px;
}
.main {
  flex: 1;
  padding: 20px;
  max-width: 980px;
}

/* ≤768px：侧栏折叠为顶部横向 Tab，另设含登出的顶栏 */
.mobile-bar {
  display: none;
}
@media (max-width: 768px) {
  .parent-shell {
    flex-direction: column;
  }
  .mobile-bar {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 14px;
    background: var(--c-card);
    border-bottom: 1px solid #eee;
  }
  .m-brand {
    font-weight: 800;
  }
  .m-who {
    flex: 1;
    font-size: 14px;
    color: var(--c-sub);
  }
  .side {
    width: 100%;
    height: auto;
    position: static;
    padding: 10px 12px;
    border-right: none;
    border-bottom: 1px solid #eee;
  }
  .brand {
    display: none;
  }
  .side-foot {
    display: none;
  }
  .side nav {
    flex-direction: row;
    overflow-x: auto;
    gap: 6px;
  }
  .side nav a {
    white-space: nowrap;
    padding: 8px 12px;
    font-size: 14px;
  }
  .main {
    padding: 14px;
  }
}
</style>
