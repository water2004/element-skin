import { createApp, h, nextTick } from 'vue'
import ElementPlus from 'element-plus'
import { it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { parse, compileStyle } from '@vue/compiler-sfc'
import NoticeContentDialog from '../NoticeContentDialog.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import { expectedNoticeDraft } from '../../../common/__tests__/fixtures/dialogFlows'

it('applies the scroll boundary to the teleported notice without constraining other dialogs', async () => {
  const host = document.createElement('div')
  document.body.append(host)
  const app = createApp({
    render: () =>
      h('div', [
        h(NoticeContentDialog, {
          modelValue: true,
          createMode: true,
          form: { ...expectedNoticeDraft, display_mode: 'detail' },
          previewHtml: '',
          dirty: false,
          saving: false,
        }),
        h(UiDialog, { modelValue: true, title: '其他表单' }, () => '其他内容'),
      ]),
  })
  app.use(ElementPlus)
  const filename = 'src/components/admin/notices/NoticeContentDialog.vue'
  const { descriptor } = parse(readFileSync(filename, 'utf8'))
  const style = document.createElement('style')
  style.textContent = descriptor.styles
    .map(
      (block) =>
        compileStyle({
          source: block.content,
          id: 'data-v-notice-test',
          filename,
          scoped: block.scoped,
        }).code,
    )
    .join('\n')
  document.head.append(style)
  try {
    app.mount(host)
    await nextTick()
    const dialog = document.querySelector('.notice-content-dialog')!
    expect(host.contains(dialog)).toBe(false)
    expect(getComputedStyle(dialog).display).toBe('flex')
    expect(getComputedStyle(dialog).flexDirection).toBe('column')
    const body = dialog.querySelector('.el-dialog__body')!
    expect(getComputedStyle(body).overflowY).toBe('auto')
    expect(getComputedStyle(body).minHeight).toBe('0px')
    for (const region of ['.el-dialog__header', '.el-dialog__footer']) {
      expect(getComputedStyle(dialog.querySelector(region)!).flexShrink).toBe('0')
    }
    expect(dialog.querySelector<HTMLTextAreaElement>('textarea[maxlength="20000"]')!.rows).toBe(8)
    expect(dialog.querySelector('.el-dialog__footer')?.textContent).toContain('下一步：发布设置')
    const other = document.querySelector('.ui-dialog:not(.notice-content-dialog)')!
    expect(getComputedStyle(other).display).toBe('block')
    expect(getComputedStyle(other.querySelector('.el-dialog__body')!).overflowY).toBe('visible')
  } finally {
    app.unmount()
    host.remove()
    style.remove()
  }
})
