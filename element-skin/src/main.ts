import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import 'element-plus/theme-chalk/dark/css-vars.css'
import './assets/main.css'

import App from './App.vue'
import router from './router'
import { applyPageAppearance } from './storage/appearance'

const app = createApp(App)

app.use(createPinia())
router.afterEach((to, _from, failure) => {
  if (!failure) applyPageAppearance(to.path)
})
app.use(router)
app.use(ElementPlus)

// HTML 已提供正确底色；初次路由确认后才挂载，避免暂时把仪表盘当作首页。
router.isReady().then(() => app.mount('#app'))
