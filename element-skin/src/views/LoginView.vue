<template>
  <el-card>
    <h2>登录</h2>
    <el-form :model="form">
      <el-form-item label="Email">
        <el-input v-model="form.email" />
      </el-form-item>
      <el-form-item label="密码">
        <el-input v-model="form.password" type="password" />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="login">登录</el-button>
      </el-form-item>
    </el-form>
  </el-card>
</template>

<script setup>
import { reactive } from 'vue'
import axios from 'axios'

const form = reactive({ email: '', password: '' })

async function login() {
  try {
    // 使用 authserver 登录接口
    const res = await axios.post('/authserver/authenticate', {
      username: form.email,
      password: form.password,
      requestUser: true,
    })
    alert('登录成功')
    // 存储 token 简单示例
    if (res.data.accessToken) localStorage.setItem('accessToken', res.data.accessToken)
    if (res.data.token) localStorage.setItem('jwt', res.data.token)
  } catch (e) {
    alert('登录失败: ' + (e.response?.data?.errorMessage || e.message))
  }
}
</script>
