import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'
import AdminView from '../views/AdminView.vue'
import RegisterView from '../views/RegisterView.vue'
import LoginView from '../views/LoginView.vue'
import UserDashboard from '../views/UserDashboard.vue'

// Dashboard Components
import DashboardWardrobe from '@/components/dashboard/DashboardWardrobe.vue'
import DashboardRoles from '@/components/dashboard/DashboardRoles.vue'
import DashboardProfile from '@/components/dashboard/DashboardProfile.vue'
import DashboardHome from '@/components/dashboard/DashboardHome.vue'

// Admin Components
import AdminSettings from '@/components/admin/AdminSettings.vue'
import AdminUserList from '@/components/admin/AdminUserList.vue'
import AdminInviteList from '@/components/admin/AdminInviteList.vue'
import AdminMojang from '@/components/admin/AdminMojang.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'home',
      component: HomeView,
    },
    {
      path: '/login',
      name: 'login',
      component: LoginView,
    },
    {
      path: '/register',
      name: 'register',
      component: RegisterView,
    },
    {
      path: '/admin',
      component: AdminView,
      redirect: '/admin/settings',
      children: [
        {
          path: 'settings',
          name: 'admin-settings',
          component: AdminSettings,
        },
        {
          path: 'mojang',
          name: 'admin-mojang',
          component: AdminMojang,
        },
        {
          path: 'users',
          name: 'admin-users',
          component: AdminUserList,
        },
        {
          path: 'invites',
          name: 'admin-invites',
          component: AdminInviteList,
        },
      ],
    },
    {
      path: '/dashboard',
      component: UserDashboard,
      redirect: '/dashboard/home',
      children: [
        {
          path: 'home',
          name: 'dashboard-home',
          component: DashboardHome,
        },
        {
          path: 'wardrobe',
          name: 'dashboard-wardrobe',
          component: DashboardWardrobe,
        },
        {
          path: 'roles',
          name: 'dashboard-roles',
          component: DashboardRoles,
        },
        {
          path: 'profile',
          name: 'dashboard-profile',
          component: DashboardProfile,
        },
      ],
    },
    {
      path: '/about',
      name: 'about',
      // route level code-splitting
      // this generates a separate chunk (About.[hash].js) for this route
      // which is lazy-loaded when the route is visited.
      component: () => import('../views/AboutView.vue'),
    },
  ],
})

export default router
