import { readFileSync } from 'node:fs'
import path from 'node:path'
import { compileStyle } from '@vue/compiler-sfc'

export function installMotionStyles(component?: { source: string; scopeId: string }) {
  const files = [
    'node_modules/element-plus/dist/index.css',
    'src/assets/styles/appearance.css',
    'src/assets/styles/tokens.css',
    'src/assets/styles/animations.css',
    'src/assets/styles/element-overrides.css',
  ]
  const styles = document.createElement('style')
  const css = files.map((file) => readFileSync(path.join(process.cwd(), file), 'utf8'))
  if (component) {
    const source = readFileSync(path.join(process.cwd(), component.source), 'utf8')
    const result = compileStyle({
      source: source.match(/<style[^>]*>([\s\S]*?)<\/style>/)![1]!,
      filename: component.source,
      id: component.scopeId,
      scoped: true,
    })
    if (result.errors.length) throw result.errors[0]
    css.push(result.code)
  }
  styles.textContent = css.join('\n')
  document.head.append(styles)
  return {
    remove: () => styles.remove(),
    // JSDOM 不驱动媒体查询，将实际 reduce 规则作为激活的样式层来验证其效果。
    reduce() {
      const reduced = document.createElement('style')
      reduced.textContent = [...styles.sheet!.cssRules]
        .filter(
          (rule): rule is CSSMediaRule =>
            rule instanceof CSSMediaRule &&
            rule.conditionText === '(prefers-reduced-motion: reduce)',
        )
        .flatMap((rule) => [...rule.cssRules].map((child) => child.cssText))
        .join('\n')
      document.head.append(reduced)
      return () => reduced.remove()
    },
  }
}

export function motionCss(element: Element, property: string): string {
  return getComputedStyle(element)
    .getPropertyValue(property)
    .trim()
    .replace(/var\((--[\w-]+)\)/g, (_, name: string) => motionCss(element, name))
}
