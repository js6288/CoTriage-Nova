import { createRouter, createWebHistory } from 'vue-router';

import HomePage from '../views/HomePage.vue';
import ManageLayout from '../views/ManageLayout.vue';
import DevicesListPage from '../views/DevicesListPage.vue';
import DeviceDetailPage from '../views/DeviceDetailPage.vue';
import DashboardPage from '../views/DashboardPage.vue';
import RulesManagePage from '../views/RulesManagePage.vue';
import TemplatesManagePage from '../views/TemplatesManagePage.vue';
import ModelsManagePage from '../views/ModelsManagePage.vue';

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: HomePage },
    { path: '/dashboard', name: 'dashboard', component: DashboardPage },
    {
      path: '/manage',
      component: ManageLayout,
      children: [
        { path: '', redirect: { name: 'devices' } },
        { path: 'devices', name: 'devices', component: DevicesListPage },
        { path: 'devices/:nodeName', name: 'deviceDetail', component: DeviceDetailPage, props: true },
        { path: 'rules', name: 'rules', component: RulesManagePage },
        { path: 'templates', name: 'templates', component: TemplatesManagePage },
        { path: 'models', name: 'models', component: ModelsManagePage }
      ]
    }
  ]
});

export default router;
