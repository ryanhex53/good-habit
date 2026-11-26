<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import { useSession } from '../session'
import BaseIcon from '../components/ui/BaseIcon.vue'
import ThemeToggle from '../components/ui/ThemeToggle.vue'

const router = useRouter()
const route = useRoute()
const { me, logout } = useSession()

const links = [
  { to: '/parent/overview', label: '总览', icon: 'list' },
  { to: '/parent/earn', label: '登记得分', icon: 'plus' },
  { to: '/parent/children', label: '孩子', icon: 'users' },
  { to: '/parent/items', label: '积分项', icon: 'check' },
  { to: '/parent/prizes', label: '奖品与定价', icon: 'gift' },
  { to: '/parent/draws', label: '待发放', icon: 'star' },
  { to: '/parent/bills', label: '账单', icon: 'receipt' },
]

function isActive(to: string): boolean {
  return route.path === to
}

async function doLogout() {
  await logout()
  router.push('/login')
}
</script>

<template>
  <div class="min-h-dvh bg-base-200 lg:grid lg:grid-cols-[240px_minmax(0,1fr)]">
    <!-- 手机顶栏（含登出，桌面隐藏） -->
    <header
      class="sticky top-0 z-30 flex items-center gap-2 border-b border-base-300 bg-base-100 px-3 py-2 pt-[calc(env(safe-area-inset-top)+0.5rem)] lg:hidden"
    >
      <span class="font-extrabold">好习惯</span>
      <span class="min-w-0 flex-1 truncate text-sm text-base-content/60">
        {{ me?.displayName }}
      </span>
      <ThemeToggle />
      <button
        type="button"
        class="btn btn-ghost btn-circle min-h-11 min-w-11"
        aria-label="退出登录"
        @click="doLogout"
      >
        <BaseIcon name="log-out" class="size-5" />
      </button>
    </header>

    <!-- 手机页签条：<nav> 自身是横向滚动容器，页签组只占内容宽度（不撑破页面） -->
    <nav
      class="overflow-x-auto border-b border-base-300 bg-base-100 px-2 py-2 [scrollbar-width:none] lg:hidden [&::-webkit-scrollbar]:hidden"
    >
      <div class="tabs tabs-box w-max min-w-full">
        <RouterLink
          v-for="l in links"
          :key="l.to"
          :to="l.to"
          class="tab min-h-11 whitespace-nowrap"
          :class="isActive(l.to) ? 'tab-active text-brand-ink' : ''"
          :aria-current="isActive(l.to) ? 'page' : undefined"
        >
          {{ l.label }}
        </RouterLink>
      </div>
    </nav>

    <!-- 桌面侧栏 -->
    <aside class="sticky top-0 hidden h-dvh flex-col border-r border-base-300 bg-base-100 p-4 lg:flex">
      <div class="mb-6 flex items-center justify-between px-2">
        <div class="text-lg font-extrabold">好习惯 · 家长端</div>
        <ThemeToggle />
      </div>
      <ul class="menu w-full flex-1 gap-1 p-0">
        <li v-for="l in links" :key="l.to">
          <RouterLink
            :to="l.to"
            class="min-h-11"
            :class="isActive(l.to) ? 'menu-active bg-primary/10 font-bold text-brand-ink' : ''"
            :aria-current="isActive(l.to) ? 'page' : undefined"
          >
            <BaseIcon :name="l.icon" class="size-5" />
            {{ l.label }}
          </RouterLink>
        </li>
      </ul>
      <div
        class="mt-4 flex items-center justify-between gap-2 border-t border-base-300 px-2 pt-3 text-sm text-base-content/60"
      >
        <span class="min-w-0 truncate">{{ me?.displayName }}</span>
        <button type="button" class="btn btn-ghost btn-sm gap-2" @click="doLogout">
          <BaseIcon name="log-out" class="size-4" />
          退出
        </button>
      </div>
    </aside>

    <main class="mx-auto w-full max-w-5xl p-4 lg:p-8">
      <RouterView />
    </main>
  </div>
</template>
