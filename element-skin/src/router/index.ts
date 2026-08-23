import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'
import AdminView from '../views/AdminView.vue'
import RegisterView from '../views/RegisterView.vue'
import LoginView from '../views/LoginView.vue'
import ResetPassword from '../views/ResetPassword.vue'
import UserDashboard from '../views/UserDashboard.vue'
import SkinLibraryView from '../views/SkinLibraryView.vue'
import NotificationsView from '../views/NotificationsView.vue'
import OAuthAuthorizeView from '../views/OAuthAuthorizeView.vue'
import OAuthDeviceView from '../views/OAuthDeviceView.vue'

// Dashboard Components
import DashboardWardrobe from '@/components/dashboard/wardrobe/DashboardWardrobe.vue'
import DashboardRoles from '@/components/dashboard/roles/DashboardRoles.vue'
import DashboardProfile from '@/components/dashboard/profile/DashboardProfile.vue'
import DashboardHome from '@/components/dashboard/home/DashboardHome.vue'
import DashboardOAuthApps from '@/components/dashboard/oauth/DashboardOAuthApps.vue'
import DashboardOAuthAppForm from '@/components/dashboard/oauth/DashboardOAuthAppForm.vue'
import DashboardIdentities from '@/components/dashboard/identities/DashboardIdentities.vue'

// Admin Components
import AdminSettings from '@/components/admin/settings/AdminSettings.vue'
import AdminUserList from '@/components/admin/users/AdminUserList.vue'
import AdminInviteList from '@/components/admin/invites/AdminInviteList.vue'
import AdminMojang from '@/components/admin/mojang/AdminMojang.vue'
import AdminHomepageMedia from '@/components/admin/homepage/AdminHomepageMedia.vue'
import AdminEmail from '@/components/admin/settings/AdminEmail.vue'
import AdminEasterEggs from '@/components/admin/easter-eggs/AdminEasterEggs.vue'
import AdminTexturesList from '@/components/admin/textures/AdminTexturesList.vue'
import AdminRolesList from '@/components/admin/roles/AdminRolesList.vue'
import AdminNotices from '@/components/admin/notices/AdminNotices.vue'
import AdminOAuthApps from '@/components/admin/oauth/AdminOAuthApps.vue'
import AdminIdentityProviders from '@/components/admin/identity/AdminIdentityProviders.vue'
import AdminIdentityProviderForm from '@/components/admin/identity/AdminIdentityProviderForm.vue'
import { getMe } from '@/api/me'
import { installEasterEggRouterHooks } from '@/easter-eggs'
import { canAccessAdminPath, firstAccessibleAdminPath } from '@/permissions/adminPages'
import {
  canAccessSitePath,
  firstAccessibleSitePath,
  isProtectedSitePath,
} from '@/permissions/sitePages'

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
        {
          path: 'oauth-apps',
          name: 'admin-oauth-apps',
          component: AdminOAuthApps,
        },
        {
          path: 'identity-providers',
          name: 'admin-identity-providers',
          component: AdminIdentityProviders,
        },
        {
          path: 'identity-providers/new',
          name: 'admin-identity-provider-create',
          component: AdminIdentityProviderForm,
        },
        {
          path: 'identity-providers/:provider_id/edit',
          name: 'admin-identity-provider-edit',
          component: AdminIdentityProviderForm,
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
        {
          path: 'oauth',
          name: 'dashboard-oauth',
          component: DashboardOAuthApps,
        },
        {
          path: 'oauth/apps/new',
          name: 'dashboard-oauth-app-create',
          component: DashboardOAuthAppForm,
        },
        {
          path: 'oauth/apps/:client_id/edit',
          name: 'dashboard-oauth-app-edit',
          component: DashboardOAuthAppForm,
        },
        {
          path: 'identities',
          name: 'dashboard-identities',
          component: DashboardIdentities,
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
    {
      path: '/oauth/authorize',
      name: 'oauth-authorize',
      component: OAuthAuthorizeView,
    },
    {
      path: '/oauth/device',
      name: 'oauth-device',
      component: OAuthDeviceView,
    },
  ],
})

router.beforeEach(async (to) => {
  if (to.path === '/admin' || to.path.startsWith('/admin/')) {
    try {
      const res = await getMe()
      const permissions = res.data.permissions ?? []
      const firstAdminPath = firstAccessibleAdminPath(permissions)
      if (!firstAdminPath) return { path: firstAccessibleSitePath(permissions) ?? '/' }
      if (to.path === '/admin' || to.path === '/admin/') return { path: firstAdminPath }
      if (canAccessAdminPath(to.path, permissions)) return true
      return { path: firstAdminPath }
    } catch {
      return loginRedirect(to.fullPath)
    }
  }

  if (!isProtectedSitePath(to.path)) return true

  try {
    const res = await getMe()
    const permissions = res.data.permissions ?? []
    const firstSitePath = firstAccessibleSitePath(permissions)
    if (!firstSitePath) return { path: '/' }
    if (to.path === '/dashboard' || to.path === '/dashboard/') return { path: firstSitePath }
    if (canAccessSitePath(to.path, permissions)) return true
    return { path: firstSitePath }
  } catch {
    return loginRedirect(to.fullPath)
  }
})

function loginRedirect(returnTo: string) {
  return { path: '/login', query: { redirect: returnTo } }
}

installEasterEggRouterHooks(router)

export default router
