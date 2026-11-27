// 与后端 JSON 形状一一对应的共享类型（Task 15/16 的响应契约）。
export interface Prize {
  id: number
  name: string
  emoji: string
}

export interface PendingDraw {
  uuid: string
  choices: Prize[]
  deadline: number
}

export interface WheelInfo {
  prizes: Prize[]
  singleCost: number
  doubleCost: number
  balance: number
  pending: PendingDraw | null
}

export interface GuideItem {
  id: number
  title: string
  points: number
}

export type LedgerKind = 'earn' | 'draw' | 'adjust'

export interface LedgerRow {
  id: number
  createdAt: number
  kind: LedgerKind
  delta: number
  balanceAfter: number
  itemTitle: string
  prizeName: string
  mode: string
  note: string
}

export interface LedgerPage {
  rows: LedgerRow[]
  nextBefore: number | null
}

// 抽奖响应（Task 15 契约）
export interface DrawEntryInfo {
  delta: number
  balanceAfter: number
  kind: string
}

export interface DrawResp {
  draw: {
    uuid: string
    mode: string
    state: string
    prize?: Prize
    choices?: Prize[]
  }
  entry: DrawEntryInfo
}

export interface ChooseResp {
  draw: {
    uuid: string
    prize: Prize
    state: string
  }
}

// 家长端（Task 11/12/14/16 契约）
export interface ChildView {
  id: number
  username: string
  displayName: string
  balance: number
  active: boolean
}

// 适用范围：'all' = 所有孩子；number[] = 恰好这些孩子，[] = 指定模式但零选中
export type ItemScope = 'all' | number[]

export interface ItemView {
  id: number
  title: string
  points: number
  active: boolean
  sort: number
  // 适用孩子；空数组 = 所有孩子
  childIds: number[]
}

export interface PrizeAdminView {
  id: number
  name: string
  emoji: string
  active: boolean
  sort: number
}

export interface OverviewChild {
  id: number
  displayName: string
  balance: number
  unfulfilled: number
  hasPending: boolean
}

export interface OverviewInfo {
  children: OverviewChild[]
  todayEarns: number
}

export interface DrawListItem {
  uuid: string
  childName: string
  mode: string
  cost: number
  prizeName: string
  state: string
  fulfilledAt: number
  createdAt: number
}

export interface EntryResp {
  entry: {
    delta: number
    balanceAfter: number
    kind: string
    itemTitle?: string
  }
}

// fmtTime：Unix 毫秒 → "M/D HH:mm"（浏览器本地时区）。
export function fmtTime(ms: number): string {
  const d = new Date(ms)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getMonth() + 1}/${d.getDate()} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}
