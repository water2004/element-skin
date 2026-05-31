<template>
  <div class="admin-content">
    <router-view />
  </div>
</template>

<script setup lang="ts">
// This component now only serves as a layout container for the admin panel's nested routes.
// The main layout and navigation are handled by AppLayout.vue.
</script>

<style scoped>
.admin-content {
  width: 100%;
  height: 100%;
}
</style>