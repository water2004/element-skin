import fs from 'node:fs'
import path from 'node:path'
import crypto from 'node:crypto'
import { execFileSync } from 'node:child_process'
import { createRequire } from 'node:module'

const root = process.cwd()
const require = createRequire(path.join(root, 'element-skin/package.json'))
const ts = require('typescript')
const { parse } = require('@vue/compiler-sfc')
const { baseParse, NodeTypes, parserOptions } = require('@vue/compiler-dom')
const postcss = require('postcss')
const base = execFileSync('git', ['rev-parse', 'a2485e5'], { encoding: 'utf8' }).trim()
const out = path.join(root, 'doc/前端规范化清单-a2485e5')
fs.mkdirSync(out, { recursive: true })
const tree = execFileSync('git', ['ls-tree', '-r', base, '--', 'element-skin'], {
  encoding: 'utf8',
})
const entries = tree
  .trim()
  .split('\n')
  .map((line) => {
    const match = /^(\d+) (\S+) (\S+)\t(.+)$/.exec(line)
    if (!match) throw new Error(`Unexpected tree entry: ${line}`)
    return { mode: match[1], blob: match[3], file: match[4] }
  })
const sources = new Map()
const rows = []
const coverage = []
const components = []
const pages = []
const diagnostics = []
const defs = new Map()
const refs = []
const seen = new Set()
const fileCounts = new Map()
const json = (value) => JSON.stringify(value)
const compact = (value) => value.replace(/\s+/g, ' ').trim()
const frozen = /\/OAuthWebhookEndpoint(?:Editor|Form)\.vue$/

function location(source, offset) {
  const before = source.slice(0, offset)
  const line = (before.match(/\n/g) || []).length + 1
  const col = offset - before.lastIndexOf('\n')
  return { line, col }
}
function region(file) {
  if (frozen.test(file)) return '冻结 Webhook'
  if (file.includes('/easter-eggs/')) return '彩蛋展示'
  if (/\/SkinViewer|\/CapeViewer|\/HomeView|\/hero|\/render|\/avatar/i.test(file))
    return '渲染与首页'
  if (file.includes('/components/admin/')) return '管理业务'
  if (file.includes('/components/dashboard/')) return '用户业务'
  if (/\/components\/(ui|common|layout|textures)\//.test(file) || file.endsWith('/AppLayout.vue'))
    return '共享组件与布局'
  if (file.includes('/assets/')) return '全局样式'
  if (file.includes('/storage/')) return '浏览器存储'
  if (file.includes('/api/')) return '接口契约'
  if (file.includes('/permissions/')) return '权限与导航'
  if (file.includes('/views/')) return '路由页面'
  if (file.includes('/composables/')) return '共享交互逻辑'
  return file.includes('/src/') ? '工具与应用入口' : '构建与工程配置'
}
function classify(file, kind, key, value, context) {
  const all = `${key} ${value} ${context}`
  const area = region(file)
  let category = '语义字符串／语言基元'
  let stage = '保留或按所属领域核对'
  let assessment = '语法采集；不是缺陷结论'
  if (kind === 'CSS 变量定义') {
    category = '已有变量定义'
    stage = '3.2／3.3 已有基础'
    assessment = '保留来源；作用域及消费另查'
  } else if (
    kind === 'CSS 声明' ||
    kind === 'CSS 条件' ||
    kind === '模板 class' ||
    kind === '模板 style'
  ) {
    if (/font|line-height|letter-spacing|text-|leading-|tracking-/.test(all)) category = '文字层级'
    else if (
      /width|height|padding|margin|gap|grid|flex|border-radius|max-w|min-w|p[xytrbl]?-|m[xytrbl]?-|w-|h-|rounded/.test(
        all,
      )
    )
      category = '布局尺寸与间距'
    else category = '视觉声明／条件'
    if (/(#[\da-f]{3,8}\b|rgba?\(|hsla?\(|gradient|shadow)/i.test(value)) category += '＋颜色／图层'
    if (/animation|transition|transform|duration|delay|easing|keyframes/.test(all))
      category += '＋动效'
    if (/position|z-index|overflow|scrollbar/.test(all)) category += '＋定位／滚动'
    if (kind === '模板 class' && !/\[[^\]]+\]/.test(value))
      assessment = '现有 utility／语义类；不机械改为变量'
    else if (/^var\(--[^)]+\)$/.test(value.trim())) assessment = '已引用变量；核对来源和作用域'
    else assessment = '局部值候选；需按用途和重复程度决定迁移或保留'
    stage = /animation|transition|transform/.test(all)
      ? '3.5 已有规则／3.6.5 指定反馈'
      : '3.6.7 布局与文字'
  } else if (kind === '模板属性') {
    category = /max|min|step|length|precision|rows|limit|accept/i.test(key)
      ? '表单约束／格式'
      : /width|height|size|span|gutter|label-width/.test(key)
        ? '组件尺寸'
        : '组件参数／文案'
    stage = /width|height|size|span|gutter|label-width/.test(key)
      ? '3.6.7／3.7'
      : '3.6.8 流程／所属业务契约'
    assessment = '需区分 UI 参数、文案及后端契约'
  } else if (kind.startsWith('TS ') || kind === '模板表达式') {
    if (/^\d|^-\d/.test(value) || value === 'true' || value === 'false') {
      category =
        /status|code/i.test(context) && /^[1-5]\d\d$/.test(value)
          ? '协议状态码'
          : /timeout|interval|ttl|cache|duration|retry|countdown|expire|poll/i.test(all)
            ? '计时／缓存／重试'
            : /limit|page|length|min|max|size|quota|count/i.test(all)
              ? '数量／边界／默认值'
              : '数值／布尔基元'
      assessment =
        category === '协议状态码'
          ? '契约值保留；非设计 token'
          : '按符号上下文人工归类；不预判魔法数字'
      stage =
        category === '计时／缓存／重试' ? '阶段 4／所属存储或渲染模块' : '所属模块／3.6.8 契约核对'
    } else if (/^--/.test(value)) {
      category = '运行时变量键'
      assessment = '已有变量来源或消费'
      stage = '已有基础／作用域核对'
    } else if (/https?:\/\//.test(value)) category = 'URL／地址／示例'
    else if (
      /^\/(v2|oauth|authserver|sessionserver|api|users|minecraft)|\.owned$|\.any$|\.self$/.test(
        value,
      )
    )
      category = '路由／权限／协议'
    else if (/[\u3400-\u9fff]/.test(value)) category = '用户文案／业务枚举'
    else if (/^[@.]?\.?\//.test(value)) category = '路径／依赖／选择器'
  }
  if (area === '彩蛋展示') {
    stage = '彩蛋展示例外／专项'
    assessment += '；不可批量套普通控件规则'
  }
  if (area === '渲染与首页') {
    stage = '渲染／首页专项；普通 UI 值另核'
    assessment += '；像素格式和几何值需保留含义'
  }
  if (area === '浏览器存储') stage = '阶段 4／存储契约'
  if (area === '构建与工程配置') {
    category = '工程配置／依赖契约'
    stage = '构建配置专项'
    assessment = '记录显式配置；不纳入视觉变量替换'
  }
  if (area === '冻结 Webhook') {
    stage = '冻结范围'
    assessment = '只读登记，不迁移、不恢复 stash'
  }
  return { area, category, stage, assessment }
}
function add(file, offset, kind, key, value, context = '') {
  const source = sources.get(file)
  const { line, col } = location(source, offset)
  const id = `${file}:${offset}:${kind}:${key}:${value}`
  if (seen.has(id)) return
  seen.add(id)
  const text = source.split(/\r?\n/)[line - 1]?.trim() || ''
  const classification = classify(file, kind, key, value, context)
  rows.push({
    file,
    line,
    col,
    kind,
    key,
    value,
    context: compact(context),
    source: text,
    ...classification,
  })
  fileCounts.set(file, (fileCounts.get(file) || 0) + 1)
  for (const match of String(value).matchAll(/var\(\s*(--[\w-]+)/g))
    refs.push({ file, line, name: match[1], carrier: `${kind} ${key}` })
}
function define(name, file, offset, source) {
  const { line } = location(sources.get(file), offset)
  if (!defs.has(name)) defs.set(name, [])
  defs.get(name).push({ file, line, source })
}
function css(file, raw, offset, origin) {
  let ast
  try {
    ast = postcss.parse(raw, { from: file })
  } catch (error) {
    diagnostics.push({ file, origin, error: error.reason || error.message })
    return
  }
  ast.walkDecls((node) => {
    const start = offset + (node.source.start.offset || 0)
    const kind = node.prop.startsWith('--') ? 'CSS 变量定义' : 'CSS 声明'
    add(
      file,
      start,
      kind,
      node.prop,
      node.value,
      `${origin} ${node.parent.type === 'rule' ? node.parent.selector : node.parent.name || ''}`,
    )
    if (node.prop.startsWith('--')) define(node.prop, file, start, origin)
  })
  ast.walkAtRules((node) => {
    if (node.params)
      add(
        file,
        offset + (node.source.start.offset || 0),
        'CSS 条件',
        `@${node.name}`,
        node.params,
        origin,
      )
  })
}
function symbol(node) {
  const items = []
  for (let p = node.parent; p && items.length < 4; p = p.parent) {
    if (
      ts.isPropertyAssignment(p) ||
      ts.isVariableDeclaration(p) ||
      ts.isParameter(p) ||
      ts.isFunctionDeclaration(p) ||
      ts.isMethodDeclaration(p) ||
      ts.isPropertySignature(p)
    ) {
      if (p.name) items.push(p.name.getText())
    } else if (ts.isCallExpression(p) || ts.isNewExpression(p))
      items.push(p.expression.getText().slice(0, 80))
  }
  return items.join(' ← ')
}
function script(file, raw, offset, origin = 'script') {
  const expression = origin === '模板表达式'
  const input = expression ? `(${raw})` : raw
  const sourceOffset = expression ? offset - 1 : offset
  const ast = ts.createSourceFile(
    file.endsWith('.json') ? 'input.json' : 'input.ts',
    input,
    ts.ScriptTarget.Latest,
    true,
    file.endsWith('.json') ? ts.ScriptKind.JSON : ts.ScriptKind.TS,
  )
  if (origin !== '模板表达式')
    for (const diagnostic of ast.parseDiagnostics)
      diagnostics.push({
        file,
        origin,
        error: ts.flattenDiagnosticMessageText(diagnostic.messageText, ' '),
      })
  const visit = (node) => {
    let start = node.getStart(ast)
    const isNumber = ts.isNumericLiteral(node)
    const isString = ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)
    const isBool =
      node.kind === ts.SyntaxKind.TrueKeyword || node.kind === ts.SyntaxKind.FalseKeyword
    const isRegex = ts.isRegularExpressionLiteral(node)
    if (isNumber || isString || isBool || isRegex) {
      let value = isString ? node.text : node.getText(ast)
      if (
        isNumber &&
        ts.isPrefixUnaryExpression(node.parent) &&
        [ts.SyntaxKind.MinusToken, ts.SyntaxKind.PlusToken].includes(node.parent.operator)
      ) {
        value = node.parent.getText(ast)
        start = node.parent.getStart(ast)
      }
      const context = symbol(node)
      const kind =
        origin === '模板表达式'
          ? origin
          : isNumber
            ? 'TS 数值'
            : isBool
              ? 'TS 布尔'
              : isRegex
                ? 'TS 正则'
                : 'TS 字符串'
      add(file, sourceOffset + start, kind, origin, value, context)
      if (isString && /^--[\w-]+$/.test(value)) {
        if (ts.isPropertyAssignment(node.parent) && node.parent.name === node)
          define(value, file, sourceOffset + start, '运行时 style 属性键')
        else if (
          ts.isCallExpression(node.parent) &&
          node.parent.arguments[0] === node &&
          /\.setProperty$/.test(node.parent.expression.getText(ast))
        )
          define(value, file, sourceOffset + start, '运行时 setProperty 键')
      }
      if (
        ts.isNoSubstitutionTemplateLiteral(node) &&
        /\{[\s\S]*:[\s\S]*\}/.test(value) &&
        /@keyframes|\.[a-z_-]+|html[.# ]/.test(value)
      )
        css(file, input.slice(start + 1, node.end - 1), sourceOffset + start + 1, 'TS CSS 模板')
    }
    if (ts.isTemplateExpression(node)) {
      add(file, sourceOffset + start, 'TS 模板骨架', origin, node.getText(ast), symbol(node))
    }
    ts.forEachChild(node, visit)
  }
  visit(ast)
}
function template(file, raw, offset) {
  let ast
  try {
    ast = baseParse(raw, {
      ...parserOptions,
      onError: (error) => diagnostics.push({ file, origin: 'template', error: error.message }),
    })
  } catch (error) {
    diagnostics.push({ file, origin: 'template', error: error.message })
    return
  }
  const visit = (node) => {
    if (node.type === NodeTypes.ELEMENT) {
      components.push({
        file,
        line: location(sources.get(file), offset + node.loc.start.offset).line,
        tag: node.tag,
      })
      for (const prop of node.props) {
        if (prop.type === NodeTypes.ATTRIBUTE && prop.value) {
          add(
            file,
            offset + prop.loc.start.offset,
            prop.name === 'class'
              ? '模板 class'
              : prop.name === 'style'
                ? '模板 style'
                : '模板属性',
            prop.name,
            prop.value.content,
            node.tag,
          )
          if (prop.name === 'style')
            css(
              file,
              `x{${prop.value.content}}`,
              offset + prop.value.loc.start.offset - 1,
              '模板 style（声明行号为属性位置）',
            )
        } else if (prop.type === NodeTypes.DIRECTIVE && prop.exp)
          script(file, prop.exp.content, offset + prop.exp.loc.start.offset, '模板表达式')
      }
    } else if (node.type === NodeTypes.INTERPOLATION)
      script(file, node.content.content, offset + node.content.loc.start.offset, '模板表达式')
    if (node.children) for (const child of node.children) visit(child)
  }
  visit(ast)
}
function csv(name, headers, data) {
  const escape = (value) => {
    let text = String(value ?? '')
    if (/^[=+@]|^-\D/.test(text)) text = "'" + text
    return '"' + text.replaceAll('"', '""') + '"'
  }
  fs.writeFileSync(
    path.join(out, name),
    '\ufeff' + [headers, ...data].map((row) => row.map(escape).join(',')).join('\r\n') + '\r\n',
  )
}
for (const entry of entries) {
  const file = entry.file
  let state = '纳入'
  let reason = '产品源码或配置'
  if (/\/__tests__\/|\.test\.|\.spec\.|\/fixtures\//.test(file)) {
    state = '排除'
    reason = '测试／fixture，不作为产品硬编码'
  } else if (/\.(svg|woff2?|png|jpe?g|gif|ico|webp|txt)$/i.test(file)) {
    state = '资源例外'
    reason = '静态图形／字体／许可资源，不作布局数值迁移'
  } else if (/\.md$/.test(file)) {
    state = '文档参考'
    reason = '非产品值清单'
  } else if (/(lock\.json|lock\.yaml)$/.test(file)) {
    state = '依赖锁文件'
    reason = '保留依赖解析结果，不逐项当配置审查'
  } else if (frozen.test(file)) {
    state = '只读冻结'
    reason = '登记源码，不改动，不恢复本地 stash'
  }
  if (state === '纳入' || state === '只读冻结') {
    const source = execFileSync('git', ['show', `${base}:${file}`], {
      encoding: 'utf8',
      maxBuffer: 20 * 1024 * 1024,
    })
    sources.set(file, source)
    if (file.endsWith('.vue')) {
      const { descriptor, errors } = parse(source, { filename: file })
      for (const error of errors) diagnostics.push({ file, origin: 'sfc', error: String(error) })
      if (descriptor.script)
        script(file, descriptor.script.content, descriptor.script.loc.start.offset)
      if (descriptor.scriptSetup)
        script(file, descriptor.scriptSetup.content, descriptor.scriptSetup.loc.start.offset)
      if (descriptor.template)
        template(file, descriptor.template.content, descriptor.template.loc.start.offset)
      for (const block of descriptor.styles)
        css(
          file,
          block.content,
          block.loc.start.offset,
          block.scoped ? 'scoped style' : 'global style',
        )
    } else if (file.endsWith('.css')) css(file, source, 0, 'CSS 文件')
    else if (/\.(ts|json)$/.test(file))
      script(file, source, 0, file.includes('/src/') ? 'script' : '工程配置')
    else if (file.endsWith('.html'))
      template(
        file,
        source.replace(/^<!DOCTYPE[^>]*>/i, (match) => ' '.repeat(match.length)),
        0,
      )
    else {
      let currentOffset = 0
      for (const line of source.split(/\r?\n/)) {
        if (line.trim() && !/^\s*[#;]/.test(line))
          add(
            file,
            currentOffset,
            '工程声明',
            line.includes('=') ? line.split('=')[0].trim() : '路径／声明',
            line.includes('=') ? line.slice(line.indexOf('=') + 1).trim() : line.trim(),
            '文本配置',
          )
        const next = source.indexOf('\n', currentOffset)
        currentOffset = next === -1 ? source.length : next + 1
      }
    }
    coverage.push({
      ...entry,
      state,
      reason,
      lines: source.split('\n').length,
      sha256: crypto.createHash('sha256').update(source).digest('hex'),
    })
  } else coverage.push({ ...entry, state, reason, lines: '', sha256: '' })
}
rows.sort(
  (a, b) =>
    a.file.localeCompare(b.file, 'en') ||
    a.line - b.line ||
    a.col - b.col ||
    a.kind.localeCompare(b.kind),
)
rows.forEach((row, index) => {
  row.id = `V${String(index + 1).padStart(5, '0')}`
})

const routerFile = 'element-skin/src/router/index.ts'
const routerAst = ts.createSourceFile(
  routerFile,
  sources.get(routerFile),
  ts.ScriptTarget.Latest,
  true,
)
const imports = new Map()
for (const node of routerAst.statements)
  if (ts.isImportDeclaration(node) && node.importClause?.name) {
    const name = node.importClause.name.text
    const target = node.moduleSpecifier.text
    if (target.startsWith('@/')) imports.set(name, `element-skin/src/${target.slice(2)}`)
    else if (target.startsWith('.'))
      imports.set(name, path.posix.normalize(path.posix.join('element-skin/src/router', target)))
  }
const property = (node, key) =>
  node.properties?.find((p) => p.name && p.name.getText(routerAst).replaceAll(/["']/g, '') === key)
    ?.initializer
function routes(array, parent = '') {
  for (const item of array.elements || []) {
    if (!ts.isObjectLiteralExpression(item)) continue
    const routePath = property(item, 'path')?.text || ''
    const route = routePath.startsWith('/')
      ? routePath
      : `${parent}/${routePath}`.replaceAll(/\/+/g, '/')
    const symbol = property(item, 'component')?.getText(routerAst) || ''
    const file = imports.get(symbol) || ''
    const source = sources.get(file) || ''
    const header = /<PageHeader\b/.test(source)
      ? '共享 PageHeader'
      : /class="page-header/.test(source)
        ? '旧 page-header'
        : /<h1\b/.test(source)
          ? '局部 h1'
          : '路由容器／无本级页头'
    const rootTag = source.match(/<template>\s*<([\w-]+)([\s\S]*?)>/)
    const rootClass = rootTag?.[2].match(/\bclass="([^"]+)"/)?.[1] || ''
    const title =
      source.match(/<PageHeader\b[^>]*\btitle="([^"]+)"/)?.[1] ||
      source.match(/<h1\b[^>]*>([\s\S]*?)<\/h1>/)?.[1] ||
      ''
    const widths = [
      ...source.matchAll(/(?:[a-z]+:)*max-w-(?:\[[^\]]+\]|[\w-]+)|max-width:\s*[^;\n)]+;/g),
    ].map((m) => m[0])
    pages.push({
      route,
      file,
      symbol,
      line: routerAst.getLineAndCharacterOfPosition(item.getStart(routerAst)).line + 1,
      header,
      title: compact(title),
      rootClass,
      widths: [...new Set(widths)].join(' ; '),
      shell: ['/login', '/register', '/reset-password'].includes(route)
        ? '认证页无顶栏／footer'
        : route === '/'
          ? '首页一屏流式例外'
          : '普通 AppShell；20px 外边距',
      redirect: property(item, 'redirect')?.getText(routerAst) || '',
    })
    const child = property(item, 'children')
    if (child && ts.isArrayLiteralExpression(child)) routes(child, route)
  }
}
const walkRouter = (node) => {
  if (
    ts.isPropertyAssignment(node) &&
    node.name.getText(routerAst) === 'routes' &&
    ts.isArrayLiteralExpression(node.initializer)
  )
    routes(node.initializer)
  ts.forEachChild(node, walkRouter)
}
walkRouter(routerAst)
const refRows = [
  ...new Map(refs.map((ref) => [`${ref.file}:${ref.line}:${ref.name}`, ref])).values(),
].map((ref) => ({
  ...ref,
  origins: [...new Set((defs.get(ref.name) || []).map((d) => `${d.file}:${d.line}`))].join(' ; '),
  state: defs.has(ref.name)
    ? '有本地声明／运行时键（未判定层叠与作用域）'
    : ref.name.startsWith('--el-')
      ? 'Element Plus 库变量'
      : '未静态找到声明；需人工核对运行时／拼接',
}))
const counts = (list) =>
  Object.fromEntries(
    [...new Set(list)].sort().map((key) => [key, list.filter((item) => item === key).length]),
  )
// 处理组为人工编制的领域归属；行内辅助标签仍只是语法规则，不能据此宣布缺陷或已验收。
const groups = [
  [
    'G01',
    '全局主题与设计变量',
    /\/assets\/(base\.css|main\.css|styles\/(?!animations))/,
    '已有来源，保留并核对映射',
    '颜色、字号、间距、圆角、主题首帧与 Element Plus 覆盖；变量文件本身允许定义原值。',
    '3.6.7；主题高风险回归',
  ],
  [
    'G02',
    '共享动效样式',
    /\/assets\/styles\/animations\.css$/,
    '已有来源，业务采用仍需核对',
    '分层时长、错峰、减少动态效果、初始可见性；不将全部动画同值化。',
    '3.5 已闭环；3.6.5 指定恢复',
  ],
  [
    'G03',
    '应用壳 顶栏 账号与 footer',
    /\/components\/(AppLayout\.vue|layout\/)/,
    '待规范决策／局部已实施',
    '64px 顶栏、20px 页边距、768／1100 断点、12px 安全间距、旧页头及版本回退；吸顶尚未实施。',
    '3.6.7 先壳布局；阶段 4 取数',
  ],
  [
    'G04',
    '公共页头 工具栏 搜索与操作布局',
    /\/components\/common\/(PageHeader|ActionBar|CardActions|SearchBar)\.vue$/,
    '待统一尺寸语义',
    '页头 24／14px 与旧 32／16px 并存；30／40px 底距、56px 图标、92px 操作基宽。',
    '3.6.7',
  ],
  [
    'G05',
    '基础按钮 卡片 选择与选项',
    /\/components\/ui\/(UiButton|UiCard|UiSegmented|UiOptionCard)\.vue$/,
    '已有组件；指定反馈待实施',
    '按钮层级与语义色、loading 稳定占位、卡片默认无阴影、选择态；未来 icon-swap／光晕不默认推广。',
    '3.6.5；3.6.7',
  ],
  [
    'G06',
    '弹窗与预览分栏',
    /\/components\/ui\/(UiDialog|UiViewerLayout)\.vue$/,
    '已有来源；移动尺寸待审',
    '500／1120／800px 宽度变量、560／340px 预览高度及 900px 分栏断点；退出语义不得被尺寸迁移改写。',
    '3.6.8；3.7 待确认',
  ],
  [
    'G07',
    '共享预览与角色卡片',
    /\/components\/common\/(PreviewCard|ProfileCard|ProfilePreviewStage)\.vue$/,
    '本轮已整合；保留局部例外',
    '静态卡片、交互预览、8px 角标、角色快捷区；预览尺寸不是页面宽度。',
    '3.6.2—3.6.4 已闭环；3.7 待确认',
  ],
  [
    'G08',
    '标识符 名称 身份与状态',
    /\/components\/common\/(IdentifierValue|ViewerNameEditor|ViewerIdentity|ProfileIdentifiers|OfficialBindingTag|DirtyTag)\.vue$/,
    '已实施；不扩大为全站已迁移',
    '前八字符摘要、320px 全文浮层、150ms 隐藏、显式名称保存；状态与普通信息不全部标签化。',
    '3.6.6 字段提示；3.6.8 推广',
  ],
  [
    'G09',
    '材质展示与精度契约',
    /\/components\/textures\//,
    '标准与协议值保留；展示复用',
    '皮肤 64×64／64×32，披风 64×32／22×17；密度判定与实际宽度分开，Alex／Steve 仅为显示名。',
    '现行规则保留；3.7 预览布局待审',
  ],
  [
    'G10',
    'WebGL 预览与头像渲染',
    /\/components\/(SkinViewer|CapeViewer)\.vue$|\/composables\/useAvatar\.ts$/,
    '领域参数例外，专项解释',
    '256px 头像、镜头位置、缩放、材质坐标、静态快照与资源释放；不得改成通用 UI 尺寸。',
    'skinview3d 独立专项',
  ],
  [
    'G11',
    '首页与 hero 场景',
    /\/views\/HomeView\.vue$|\/composables\/useHeroScene\.ts$/,
    '高风险例外；分清布局与渲染',
    '背景 fixed、流式内容、900ms 转场、60fps、DPR 1.5 上限、画面比例与媒体时长；保留连续缩放修复。',
    '首页专项；3.6.7／3.7 仅布局',
  ],
  [
    'G12',
    '会话 路由 权限与请求认证',
    /\/stores\/session\.ts$|\/src\/permissions\/|\/router\/|\/api\/client\.ts$/,
    '契约保留，重复来源需对照',
    '权限字符串、保护路由、401 重试与会话状态；数值或路径不同不自动说明越权。',
    '所属权限契约；阶段 4 请求协调',
  ],
  [
    'G13',
    'API 包装与接口类型',
    /\/api\//,
    '接口契约保留，默认 limit 待核对',
    'method／path／枚举／状态码不是设计 token；OAuth 包装的 50／100 与页面分页分开。',
    '阶段 4；API 契约核对',
  ],
  [
    'G14',
    '存储与渲染缓存',
    /\/storage\//,
    '独立配置来源；需核对重复与版本',
    '头像 2MiB、快照 20MiB、IndexedDB 版本／键、分页持久化、主题和站点回退文案。',
    '阶段 4／存储专项',
  ],
  [
    'G15',
    '脏值 退出与详情写锁',
    /\/composables\/(useDirtySnapshot|useUnsavedChangesGuard|useViewerSession)\.ts$/,
    '已有语义，推广时复用',
    '草稿、写锁、路由参数比较、确认文案、生命周期；布尔初值不机械抽常量。',
    '3.6.6／3.6.8；阶段 4 结果协调',
  ],
  [
    'G16',
    '分页与分页控件',
    /\/composables\/useCursorPagination\.ts$|\/common\/CursorPager\.vue$/,
    '默认值与候选项待归一',
    '[5,10,20,30]、默认 20、存储上限 100 与各业务 limit 分开。',
    '3.6.7 呈现；阶段 4 读取策略',
  ],
  [
    'G17',
    '公共皮肤库页面',
    /\/views\/SkinLibraryView\.vue$/,
    '卡片已收敛；页面规则待迁移',
    '旧页头、筛选／分页、240px 画廊栅格、确认空态；保留添加不冒泡与收藏数契约。',
    '3.6.7；阶段 4',
  ],
  [
    'G18',
    '个人角色与正版导入流程',
    /\/components\/dashboard\/roles\//,
    '已整合详情；表单与协议分开复核',
    '名称当前 1–16、正版绑定、远端导入、多步退出与写保护；不得擅自改成 Mojang 3–16。',
    '3.6.8；角色命名契约另议',
  ],
  [
    'G19',
    '个人衣柜与上传材质',
    /\/components\/dashboard\/wardrobe\//,
    '详情已整合；上传与目标清单待复核',
    '目标角色 limit 100、PNG／类型／模型／公开表单、名称草稿与删除现有语义。',
    '3.6.8；阶段 4 目标列表',
  ],
  [
    'G20',
    '个人资料与重设邮箱',
    /\/components\/dashboard\/profile\//,
    '现状保留；布局和计时需复核',
    '600px 内层表单、标签宽、邮箱验证码 60s／1000ms、头像有意 hover 动效。',
    '3.6.7／3.6.8；阶段 4 计时',
  ],
  [
    'G21',
    '仪表盘与 Fallback 状态',
    /\/components\/dashboard\/home\//,
    '展示局部值与刷新策略分开',
    '4 条公告、60s 更新、24h 历史、12～144 桶及图表标签；720px 是局部区域，不是整页最大宽。',
    '3.6.7；阶段 4',
  ],
  [
    'G22',
    '个人第三方应用与授权',
    /\/components\/dashboard\/oauth\//,
    '已知读取闪烁留待阶段 4',
    '开发者／授权分区、30 天撤销记录、5 个 Webhook 上限、应用表单；冻结编辑器另组。',
    '3.6.7／3.6.8；阶段 4',
  ],
  [
    'G23',
    '个人关联身份',
    /\/components\/dashboard\/identities\//,
    '领域表单边界待核对',
    '标签 80 字符、提供方分组、绑定／解除与跳转返回；不以角色正版绑定代替身份绑定。',
    '3.6.7／3.6.8',
  ],
  [
    'G24',
    '站点与邮件设置',
    /\/components\/admin\/settings\//,
    '明确保存例外保留；默认值待核对',
    '900px 页宽、1024KB、5 次／15 分钟、7 天、邮件 TTL 300 秒及字段约束；已有对象仍需分组保存。',
    '3.6.6 样板；3.6.7／3.6.8',
  ],
  [
    'G25',
    'Fallback 配置与白名单',
    /\/components\/admin\/mojang\//,
    '业务默认来源重复需核对',
    '1100px 页宽、600 秒默认／60 秒下限、端点 TTL／调度枚举、白名单与分组保存。',
    '3.6.7／3.6.8；阶段 4',
  ],
  [
    'G26',
    '首页媒体配置',
    /\/components\/admin\/homepage\//,
    '混合提交语义保留，布局需专项',
    '1040px 页宽、图片／Panorama 时长轨迹与拖排参数；编辑暂存后保存，上传删除即时生效。',
    '3.6.7；3.6.8 谨慎推广',
  ],
  [
    'G27',
    '用户管理与权限编辑',
    /\/components\/admin\/users\//,
    '领域与表格尺寸分开复核',
    '1000px 页宽、表格列宽、详情布局、24／72／168／720 小时封禁选项与默认、密码与权限范围。',
    '3.6.7／3.6.8；权限契约另核',
  ],
  [
    'G28',
    '公告管理与编辑',
    /\/components\/admin\/notices\//,
    '明确提交表单与正文排版分开',
    '1100px 页宽、编辑字数、时间、目标、弹窗宽度；富文本标题不直接套页面 h1。',
    '3.6.7／3.6.8',
  ],
  [
    'G29',
    '邀请码管理',
    /\/components\/admin\/invites\//,
    '表格与业务约束待核对',
    '1000px 页宽、次数／数量／期限、列宽与创建表单。',
    '3.6.7／3.6.8',
  ],
  [
    'G30',
    '管理第三方应用',
    /\/components\/admin\/oauth\//,
    '表格与审核契约待核对',
    '1200px 页宽、筛选组、表格列宽、审核理由 500 字及详情命令。',
    '3.6.7／3.6.8；阶段 4',
  ],
  [
    'G31',
    '管理身份提供方',
    /\/components\/admin\/identity\//,
    '表单布局与协议参数分开',
    '1000px 页宽、OIDC／Yggdrasil 枚举、URL／回调说明、秘密字段与创建编辑复用。',
    '3.6.7／3.6.8；外部协议保留',
  ],
  [
    'G32',
    '管理材质与角色',
    /\/components\/admin\/(textures|roles)\//,
    '卡片详情已整合；整页规则待迁移',
    '共享 PageHeader 但整页全宽、画廊栅格、管理命令与对象名称；Hash 搜索尚无契约。',
    '3.6.7；阶段 4；搜索未来工作',
  ],
  [
    'G33',
    '彩蛋配置页面',
    /\/components\/admin\/easter-eggs\//,
    '设置页面，不等同动画特效例外',
    '1000px 页宽、分组保存、日期／开关／预览；表单仍按普通交互规则整理。',
    '3.6.6 推广；3.6.7／3.6.8',
  ],
  [
    'G34',
    '登录 注册与重设密码',
    /\/views\/(LoginView|RegisterView|ResetPassword)\.vue$/,
    '认证页窄表单例外；文案约束需核对',
    '440px 表单、用户名 3–20、邮箱策略、60 秒验证码／1.5 秒返回；与角色命名不同。',
    '3.6.7／3.6.8；阶段 4 计时',
  ],
  [
    'G35',
    '通知中心与指示器',
    /\/views\/NotificationsView\.vue$|\/composables\/useNotificationIndicator\.ts$/,
    '已知空态闪烁；正文单独排版',
    '列表／详情比例、空态容器、正文标题、更新按钮、轮询状态；错误不当作暂无通知。',
    '3.6.5 按钮；3.6.7；阶段 4',
  ],
  [
    'G36',
    'OAuth 授权与设备流程',
    /\/views\/OAuth(Authorize|Device)View\.vue$|\/components\/oauth\//,
    '协议状态保留；面板与命令需核对',
    '同意／拒绝、设备码、回调地址、轮询间隔与错误；不是普通注册或设置页。',
    '3.6.8；阶段 4／OAuth 契约',
  ],
  [
    'G37',
    '路由容器',
    /\/views\/(AdminView|UserDashboard)\.vue$/,
    '目前只有全宽容器',
    '没有共享 page-width 分类；子页自行 max-width。',
    '3.6.7',
  ],
  [
    'G38',
    '入口 主题与通用工具',
    /\/src\/(App\.vue|main\.ts|utils\/)|\/composables\/useTheme\.ts$|\/common\/EmailSuffixInput\.vue$/,
    '领域常量保留；重复回退文案需查',
    '应用启动、邮箱长度、32 位 UUID 分段、Markdown 净化、错误展示、内部跳转及站点文字回退。',
    '所属工具；3.6.8 文案；阶段 4',
  ],
  [
    'G39',
    '权限选择与显示组件',
    /\/components\/permissions\//,
    '语义映射保留；不套材质 tag 限制',
    '权限分组、危险程度、选择与说明；细粒度授权与纯显示配色分开。',
    '3.6.7／3.6.8；权限契约',
  ],
  [
    'G40',
    '节庆彩蛋运行时',
    /\/easter-eggs\//,
    '合理展示例外；仍需清理与降级',
    '几何、随机粒子、调色盘、日期与超高层级逐专项解释；不批量改成普通按钮规则。',
    '彩蛋专项；减少动态效果回归',
  ],
  [
    'G41',
    '工程与构建配置',
    (file) => !file.includes('/src/'),
    '工程契约保留，配置作用域明确',
    'appVersion、开发代理、构建阈值、依赖、TS／格式化／测试配置；非生产运行时设计 token。',
    '构建配置专项',
  ],
  [
    'G42',
    '冻结 Webhook 编辑器',
    frozen,
    '只读冻结',
    '基线中已跟踪的两份源码只登记；不恢复 stash，不迁移到当前产品。',
    '冻结范围',
  ],
]
function groupFor(file) {
  if (frozen.test(file)) return groups.find((g) => g[0] === 'G42')
  const group = groups.find((g) => (typeof g[2] === 'function' ? g[2](file) : g[2].test(file)))
  if (!group) throw new Error(`Unassigned file: ${file}`)
  return group
}
for (const row of rows) row.group = groupFor(row.file)[0]
const groupRows = groups.map((g) => {
  const files = coverage
    .filter((c) => ['纳入', '只读冻结'].includes(c.state) && groupFor(c.file)[0] === g[0])
    .map((c) => c.file)
  return {
    id: g[0],
    name: g[1],
    status: g[3],
    scope: g[4],
    stage: g[5],
    files,
    records: rows.filter((r) => r.group === g[0]).length,
  }
})
const summary = {
  base,
  files: entries.length,
  included: sources.size,
  coverage: counts(coverage.map((row) => row.state)),
  rows: rows.length,
  kinds: counts(rows.map((row) => row.kind)),
  areas: counts(rows.map((row) => row.area)),
  routes: pages.length,
  tokenDefinitions: defs.size,
  tokenReferences: refRows.length,
  diagnostics,
  runtime: {
    node: process.version,
    typescript: ts.version,
    vueCompiler: require('@vue/compiler-sfc/package.json').version,
    postcss: require('postcss/package.json').version,
  },
}
csv(
  '文件覆盖.csv',
  ['路径', 'Git blob', '状态', '范围说明', '行数', '源码 SHA256', '采集记录数'],
  coverage.map((row) => [
    row.file,
    row.blob,
    row.state,
    row.reason,
    row.lines,
    row.sha256,
    fileCounts.get(row.file) || 0,
  ]),
)
csv(
  '值与规则.csv',
  [
    'ID',
    '处理组',
    '路径',
    '行',
    '列',
    '载体',
    '属性或来源',
    '值_JSON',
    '符号或选择器',
    '领域',
    '辅助分类',
    '初步处理口径',
    '建议归属',
    '原始行_JSON',
  ],
  rows.map((row) => [
    row.id,
    row.group,
    row.file,
    row.line,
    row.col,
    row.kind,
    row.key,
    json(row.value),
    row.context,
    row.area,
    row.category,
    row.assessment,
    row.stage,
    json(row.source),
  ]),
)
csv(
  '处理分组.csv',
  ['处理组', '专题', '当前结论', '核对范围', '实施归属', '文件数', '采集记录数', '全部文件路径'],
  groupRows.map((g) => [
    g.id,
    g.name,
    g.status,
    g.scope,
    g.stage,
    g.files.length,
    g.records,
    g.files.join(' ; '),
  ]),
)
csv(
  '路由页面.csv',
  [
    '路由',
    '页面源码',
    '组件',
    '路由声明行',
    '页头来源',
    '标题或标题表达式',
    '根 class',
    '文件内最大宽度候选_不全是页面宽度',
    '共享壳',
    '重定向',
  ],
  pages.map((row) => [
    row.route,
    row.file,
    row.symbol,
    row.line,
    row.header,
    row.title,
    row.rootClass,
    row.widths,
    row.shell,
    row.redirect,
  ]),
)
csv(
  '组件入口.csv',
  ['页面或组件路径', '行', '标签'],
  components
    .filter((row) => /^[A-Z]|^el-|^h[1-6]$|^(main|header|form|table)$/.test(row.tag))
    .map((row) => [row.file, row.line, row.tag]),
)
csv(
  '变量引用.csv',
  ['路径', '行', '变量名', '载体', '本地定义位置', '解析边界'],
  refRows.map((row) => [row.file, row.line, row.name, row.carrier, row.origins, row.state]),
)
csv(
  '变量定义.csv',
  ['变量名', '路径', '行', '来源'],
  [...defs].flatMap(([name, locations]) => locations.map((d) => [name, d.file, d.line, d.source])),
)
fs.writeFileSync(path.join(out, '基线与采集.json'), JSON.stringify(summary, null, 2) + '\n')
fs.writeFileSync(
  path.join(process.env.TEMP, 'element-skin-audit-data.json'),
  JSON.stringify({
    summary,
    rows,
    coverage,
    pages,
    components,
    refs: refRows,
    defs: [...defs],
    groups: groupRows,
  }),
)
console.log(JSON.stringify(summary, null, 2))
