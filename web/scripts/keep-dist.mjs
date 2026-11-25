// postbuild：vite 的 emptyOutDir 会清掉 dist（含 .keep 占位），
// 构建后重建它，保证干净检出时 //go:embed all:dist 也能编译。
import fs from 'node:fs'

fs.writeFileSync(new URL('../../internal/webui/dist/.keep', import.meta.url), '')
console.log('kept internal/webui/dist/.keep')
