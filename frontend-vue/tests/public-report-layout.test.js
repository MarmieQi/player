import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { parse, compileScript } from 'vue/compiler-sfc'
import { createSSRApp, h } from 'vue'
import { renderToString } from 'vue/server-renderer'

const source = await readFile(new URL('../src/components/PublicReportLayout.vue', import.meta.url), 'utf8')
const { descriptor } = parse(source)
const compiled = compileScript(descriptor, { id: 'public-report-test', inlineTemplate: true })
const code = compiled.content.replace(/from ["'](vue|lucide-vue-next)["']/g, (_, name) => `from ${JSON.stringify(import.meta.resolve(name))}`)
const Layout = (await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)).default
const render = (props, slots) => renderToString(createSSRApp({ render: () => h(Layout, props, slots) }))

test('report shell does not render data slots until ready, including loading and error states', async () => {
  const slots = { header: () => { throw new Error('Data not ready') } }
  const empty = await render({ ready: false }, slots)
  assert.ok(!empty.includes('log-share-hero'))
  const loading = await render({ loading: true, ready: true }, slots)
  assert.ok(loading.includes('正在打开日志'))
  const error = await render({ ready: true, error: '链接过期', errorTitle: '无法打开分析' }, slots)
  assert.ok(error.includes('role="alert"'))
  assert.ok(error.includes('无法打开分析'))
  assert.ok(error.includes('链接过期'))
})

test('both report types share the hero, summary, content and footer containers', async () => {
  const html = await render({ ready: true }, {
    header: () => h('h1', '报告标题'),
    summary: () => h('article', '摘要'),
    default: () => h('section', { class: 'log-share-content' }, '证据或日志'),
    footer: () => '服务器信息'
  })
  for (const className of ['log-share-page', 'log-share-hero', 'log-share-summary', 'log-share-content', 'log-share-footer']) {
    assert.ok(html.includes(`class="${className}"`), className)
  }
  assert.ok(html.indexOf('报告标题') < html.indexOf('证据或日志'))
  assert.ok(html.indexOf('证据或日志') < html.indexOf('服务器信息'))
})
