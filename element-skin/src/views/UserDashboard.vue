<template>
  <div class="dashboard-content">
    <router-view />
  </div>
</template>

<script setup lang="ts">
// This component now only serves as a layout container for the dashboard's nested routes.
// The main layout, navigation, and user data are handled by AppLayout.vue.
</script>

<style scoped>
.dashboard-content {
  width: 100%;
  height: 100%;
}
</style>