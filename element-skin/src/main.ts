import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import 'element-plus/theme-chalk/dark/css-vars.css'
import './assets/main.css'
import axios from 'axios'

import App from './App.vue'
import router from './router'

const motionDisabled = localStorage.getItem('motionDisabled') === '1'
document.documentElement.classList.toggle('motion-off', motionDisabled)

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(ElementPlus)

// axios 全局基础配置，VITE_API_BASE 可用于后端地址
axios.defaults.baseURL = import.meta.env.VITE_API_BASE || ''
app.config.globalProperties.$http = axios

app.mount('#app')
