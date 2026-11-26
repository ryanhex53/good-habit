<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import { useSession } from '../session'
import { useTheme } from '../composables/useTheme'
import BaseIcon from '../components/ui/BaseIcon.vue'
import ThemeToggle from '../components/ui/ThemeToggle.vue'

const router = useRouter()
const route = useRoute()
const { me, logout } = useSession()
const { theme, toggle: toggleTheme } = useTheme()

// 手机页签与桌面页签共用；顺序 = 侧栏三组（日常 / 设置 / 记录）的展开顺序，两处要保持一致
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
      <span class="inline-flex size-8 shrink-0 items-center justify-center rounded-lg bg-primary text-primary-content">
        <BaseIcon name="star" class="size-4" />
      </span>
      <span class="text-lg font-semibold">好习惯</span>
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
          :class="isActive(l.to) ? 'tab-active font-semibold text-brand-ink' : ''"
          :aria-current="isActive(l.to) ? 'page' : undefined"
        >
          {{ l.label }}
        </RouterLink>
      </div>
    </nav>

    <!-- 桌面侧栏：品牌标记 + 分组导航（日常 / 设置 / 记录）+ 账户区 -->
    <aside class="sticky top-0 hidden h-dvh flex-col border-r border-base-300 bg-base-100 px-4 py-5 lg:flex">
      <div class="mb-2 flex items-center gap-3 px-2">
        <span class="inline-flex size-9 shrink-0 items-center justify-center rounded-xl bg-primary text-primary-content">
          <BaseIcon name="star" class="size-5" />
        </span>
        <div class="min-w-0">
          <div class="text-title">好习惯</div>
          <div class="text-xs text-base-content/55">家长端</div>
        </div>
      </div>

      <nav class="flex-1 overflow-y-auto" aria-label="家长端导航">
        <div class="px-3 pt-3 pb-1.5 text-xs font-medium text-base-content/50">日常</div>
        <ul class="menu w-full gap-0.5 p-0">
          <li>
            <RouterLink
              to="/parent/overview"
              class="min-h-11 content-center rounded-xl"
              :class="isActive('/parent/overview') ? 'bg-primary/10 font-semibold text-brand-ink' : ''"
              :aria-current="isActive('/parent/overview') ? 'page' : undefined"
            >
              <BaseIcon name="list" class="size-5" />
              总览
            </RouterLink>
          </li>
          <li>
            <RouterLink
              to="/parent/earn"
              class="min-h-11 content-center rounded-xl"
              :class="isActive('/parent/earn') ? 'bg-primary/10 font-semibold text-brand-ink' : ''"
              :aria-current="isActive('/parent/earn') ? 'page' : undefined"
            >
              <BaseIcon name="plus" class="size-5" />
              登记得分
            </RouterLink>
          </li>
        </ul>

        <div class="px-3 pt-4 pb-1.5 text-xs font-medium text-base-content/50">设置</div>
        <ul class="menu w-full gap-0.5 p-0">
          <li>
            <RouterLink
              to="/parent/children"
              class="min-h-11 content-center rounded-xl"
              :class="isActive('/parent/children') ? 'bg-primary/10 font-semibold text-brand-ink' : ''"
              :aria-current="isActive('/parent/children') ? 'page' : undefined"
            >
              <BaseIcon name="users" class="size-5" />
              孩子
            </RouterLink>
          </li>
          <li>
            <RouterLink
              to="/parent/items"
              class="min-h-11 content-center rounded-xl"
              :class="isActive('/parent/items') ? 'bg-primary/10 font-semibold text-brand-ink' : ''"
              :aria-current="isActive('/parent/items') ? 'page' : undefined"
            >
              <BaseIcon name="check" class="size-5" />
              积分项
            </RouterLink>
          </li>
          <li>
            <RouterLink
              to="/parent/prizes"
              class="min-h-11 content-center rounded-xl"
              :class="isActive('/parent/prizes') ? 'bg-primary/10 font-semibold text-brand-ink' : ''"
              :aria-current="isActive('/parent/prizes') ? 'page' : undefined"
            >
              <BaseIcon name="gift" class="size-5" />
              奖品与定价
            </RouterLink>
          </li>
        </ul>

        <div class="px-3 pt-4 pb-1.5 text-xs font-medium text-base-content/50">记录</div>
        <ul class="menu w-full gap-0.5 p-0">
          <li>
            <RouterLink
              to="/parent/draws"
              class="min-h-11 content-center rounded-xl"
              :class="isActive('/parent/draws') ? 'bg-primary/10 font-semibold text-brand-ink' : ''"
              :aria-current="isActive('/parent/draws') ? 'page' : undefined"
            >
              <BaseIcon name="star" class="size-5" />
              待发放
            </RouterLink>
          </li>
          <li>
            <RouterLink
              to="/parent/bills"
              class="min-h-11 content-center rounded-xl"
              :class="isActive('/parent/bills') ? 'bg-primary/10 font-semibold text-brand-ink' : ''"
              :aria-current="isActive('/parent/bills') ? 'page' : undefined"
            >
              <BaseIcon name="receipt" class="size-5" />
              账单
            </RouterLink>
          </li>
        </ul>
      </nav>

      <div class="mt-4 flex items-center justify-between gap-2 border-t border-base-300 pt-3">
        <span class="min-w-0 truncate text-sm text-base-content/60">{{ me?.displayName }}</span>
        <div class="flex shrink-0 items-center gap-1">
          <!-- 账户区按效果图用桌面小尺寸圆钮；ThemeToggle 尺寸固定 44px，故此处内联同一主题逻辑 -->
          <button
            type="button"
            class="btn btn-ghost btn-circle btn-sm"
            :aria-label="theme === 'dark' ? '切换到浅色模式' : '切换到深色模式'"
            @click="toggleTheme"
          >
            <BaseIcon :name="theme === 'dark' ? 'sun' : 'moon'" class="size-4" />
          </button>
          <button
            type="button"
            class="btn btn-ghost btn-circle btn-sm"
            aria-label="退出登录"
            @click="doLogout"
          >
            <BaseIcon name="log-out" class="size-4" />
          </button>
        </div>
      </div>
    </aside>

    <main class="mx-auto w-full max-w-5xl p-4 lg:px-10 lg:py-8">
      <RouterView v-slot="{ Component }">
        <Transition name="page" mode="out-in" appear>
          <component :is="Component" />
        </Transition>
      </RouterView>
    </main>
  </div>
</template>
