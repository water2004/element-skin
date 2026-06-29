import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'
import AdminView from '../views/AdminView.vue'
import RegisterView from '../views/RegisterView.vue'
import LoginView from '../views/LoginView.vue'
import ResetPassword from '../views/ResetPassword.vue'
import UserDashboard from '../views/UserDashboard.vue'
import SkinLibraryView from '../views/SkinLibraryView.vue'
import NotificationsView from '../views/NotificationsView.vue'

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
import AdminHomepageMedia from '@/components/admin/AdminHomepageMedia.vue'
import AdminEmail from '@/components/admin/AdminEmail.vue'
import AdminEasterEggs from '@/components/admin/AdminEasterEggs.vue'
import AdminTexturesList from '@/components/admin/AdminTexturesList.vue'
import AdminRolesList from '@/components/admin/AdminRolesList.vue'
import AdminNotices from '@/components/admin/AdminNotices.vue'
import { getMe } from '@/api/me'
import { installEasterEggRouterHooks } from '@/easter-eggs'
import { canAccessAdminPath, firstAccessibleAdminPath } from '@/permissions/adminPages'

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
      path: '/reset-password',
      name: 'reset-password',
      component: ResetPassword,
    },
    {
      path: '/admin',
      component: AdminView,
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
          path: 'email',
          name: 'admin-email',
          component: AdminEmail,
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
        {
          path: 'homepage-media',
          name: 'admin-homepage-media',
          component: AdminHomepageMedia,
        },
        {
          path: 'notices',
          name: 'admin-notices',
          component: AdminNotices,
        },
        {
          path: 'easter-eggs',
          name: 'admin-easter-eggs',
          component: AdminEasterEggs,
        },
        {
          path: 'textures',
          name: 'admin-textures',
          component: AdminTexturesList,
        },
        {
          path: 'roles',
          name: 'admin-roles',
          component: AdminRolesList,
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
      path: '/skin-library',
      name: 'skin-library',
      component: SkinLibraryView,
    },
    {
      path: '/notifications',
      name: 'notifications',
      component: NotificationsView,
    },
    {
      path: '/notifications/:id',
      name: 'notification-detail',
      component: NotificationsView,
    },
  ],
})

router.beforeEach(async (to) => {
  if (to.path !== '/admin' && !to.path.startsWith('/admin/')) return true

  try {
    const res = await getMe()
    const permissions = res.data.permissions ?? []
    const firstAdminPath = firstAccessibleAdminPath(permissions)
    if (!firstAdminPath) return { path: '/dashboard/home' }
    if (to.path === '/admin' || to.path === '/admin/') return { path: firstAdminPath }
    if (canAccessAdminPath(to.path, permissions)) return true
    return { path: firstAdminPath }
  } catch {
    return { path: '/login' }
  }
})

installEasterEggRouterHooks(router)

export default router
