<script setup>
</script>

<template>
  <div class="app-container">
    <el-card>
      <h1>欢迎来到皮肤站</h1>
      <p>请登录或注册以管理你的皮肤；管理员可以在管理面板管理用户和邀请码。</p>
      <div style="margin-top:16px">
        <router-link to="/login"><el-button type="primary">登录</el-button></router-link>
        <router-link to="/register" style="margin-left:8px"><el-button>注册</el-button></router-link>
      </div>
    </el-card>
  </div>
</template>
