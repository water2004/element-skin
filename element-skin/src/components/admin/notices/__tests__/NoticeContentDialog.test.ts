import { createApp, h, nextTick, reactive } from 'vue'
import ElementPlus from 'element-plus'
import { it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { parse, compileStyle } from '@vue/compiler-sfc'
import NoticeContentDialog from '../NoticeContentDialog.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import { expectedNoticeDraft } from '../../../common/__tests__/fixtures/dialogFlows'
import { renderMarkdown } from '@/utils/markdown'
import { noticePreviewContent } from './fixtures/noticeContent'

it('bounds long notice previews inside their own scroll region without constraining other dialogs', async () => {
  const host = document.createElement('div')
  document.body.append(host)
  const form = reactive({
    ...expectedNoticeDraft,
    display_mode: 'detail' as const,
    content_markdown: noticePreviewContent.short,
  })
  const app = createApp({
    render: () =>
      h('div', [
        h(NoticeContentDialog, {
          modelValue: true,
          createMode: true,
          form,
          previewHtml: renderMarkdown(form.content_markdown),
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
  const desktopStyle = document.createElement('style')
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
    const preview = dialog.querySelector('.notice-content-preview')!
    const scrollRegion = preview.querySelector<HTMLElement>('[aria-label="公告预览"]')!
    const header = dialog.querySelector('.el-dialog__header')
    const footer = dialog.querySelector('.el-dialog__footer')
    expect(scrollRegion.getAttribute('role')).toBe('region')
    expect(scrollRegion.tabIndex).toBe(0)
    expect(scrollRegion.textContent).toContain(noticePreviewContent.short)
    expect(getComputedStyle(preview).height).toBe('50dvh')
    expect(getComputedStyle(scrollRegion).overflow).toBe('auto')
    expect(getComputedStyle(scrollRegion).overscrollBehavior).toBe('contain')
    expect(getComputedStyle(scrollRegion).minHeight).toBe('0px')

    form.content_markdown = noticePreviewContent.long
    await nextTick()
    expect(scrollRegion.querySelectorAll('article > div p')).toHaveLength(
      noticePreviewContent.paragraphCount,
    )
    expect(scrollRegion.textContent).toContain(noticePreviewContent.lastParagraph)
    expect(dialog.querySelector('[aria-label="公告预览"]')).toBe(scrollRegion)
    expect(dialog.querySelector('.el-dialog__header')).toBe(header)
    expect(dialog.querySelector('.el-dialog__footer')).toBe(footer)
    expect(getComputedStyle(preview).height).toBe('50dvh')

    // JSDOM 不执行视口媒体查询，激活组件的实际桌面规则以验证双列高度边界。
    const desktopRules = [...style.sheet!.cssRules].filter(
      (rule): rule is CSSMediaRule =>
        rule instanceof CSSMediaRule && rule.conditionText === '(min-width: 64rem)',
    )
    expect(desktopRules).toHaveLength(1)
    desktopStyle.textContent = desktopRules
      .flatMap((rule) => [...rule.cssRules].map((child) => child.cssText))
      .join('\n')
    document.head.append(desktopStyle)
    expect(getComputedStyle(preview).height).toBe('auto')
    expect(getComputedStyle(preview).contain).toBe('size')
    expect(getComputedStyle(preview).alignSelf).toBe('stretch')
    expect(getComputedStyle(scrollRegion).overflow).toBe('auto')
    const other = document.querySelector('.ui-dialog:not(.notice-content-dialog)')!
    expect(getComputedStyle(other).display).toBe('block')
    expect(getComputedStyle(other.querySelector('.el-dialog__body')!).overflowY).toBe('visible')
  } finally {
    app.unmount()
    host.remove()
    style.remove()
    desktopStyle.remove()
  }
})
