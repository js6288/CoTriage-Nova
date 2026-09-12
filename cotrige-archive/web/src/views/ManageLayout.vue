<template>
  <a-layout class="layout">
    <a-layout-sider class="sider" width="220">
      <div class="brand">CoTriage</div>
      <a-menu theme="light" mode="inline" :selectedKeys="selectedKeys" @click="onMenuClick">
        <a-menu-item key="devices">设备管理</a-menu-item>
        <a-menu-item key="rules">规则管理</a-menu-item>
        <a-menu-item key="templates">模板知识库</a-menu-item>
        <a-menu-item key="models">模型管理</a-menu-item>
      </a-menu>
    </a-layout-sider>

    <a-layout>
      <a-layout-header class="header">
        <div class="header-title">管理台</div>
        <div class="header-actions">
          <a-button class="dashboard-btn" type="primary" @click="goDashboard">分诊大盘</a-button>
          <a-tooltip title="返回首页">
            <a-button class="home-btn" type="text" @click="goHome" aria-label="返回首页">
              <HomeOutlined />
            </a-button>
          </a-tooltip>
        </div>
      </a-layout-header>
      <a-layout-content class="content">
        <RouterView />
      </a-layout-content>
    </a-layout>
  </a-layout>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { HomeOutlined } from '@ant-design/icons-vue';

const route = useRoute();
const router = useRouter();

const selectedKeys = computed(() => {
  if (route.name === 'devices' || route.name === 'deviceDetail') return ['devices'];
  if (route.name === 'rules') return ['rules'];
  if (route.name === 'templates') return ['templates'];
  if (route.name === 'models') return ['models'];
  return [];
});

function onMenuClick(info: any) {
  if (info?.key === 'devices') router.push({ name: 'devices' });
  if (info?.key === 'rules') router.push({ name: 'rules' });
  if (info?.key === 'templates') router.push({ name: 'templates' });
  if (info?.key === 'models') router.push({ name: 'models' });
}

function goHome() {
  router.push({ name: 'home' });
}

function goDashboard() {
  router.push({ name: 'dashboard' });
}
</script>

<style scoped>
.layout {
  min-height: 100vh;
  background: linear-gradient(180deg, #f5fbff 0%, #eaf4ff 100%);
}
.sider {
  background: rgba(255, 255, 255, 0.9);
  border-right: 1px solid rgba(24, 144, 255, 0.12);
}
.brand {
  height: 56px;
  display: flex;
  align-items: center;
  padding: 0 18px;
  font-size: 18px;
  font-weight: 700;
  color: #0f2a4a;
}
.header {
  background: rgba(255, 255, 255, 0.8);
  border-bottom: 1px solid rgba(24, 144, 255, 0.12);
  display: flex;
  align-items: center;
  padding: 0 24px;
  justify-content: space-between;
}
.home-btn {
  padding: 0;
  width: 36px;
  height: 32px;
  border-radius: 999px;
  color: rgba(15, 42, 74, 0.85);
  border: 1px solid rgba(24, 144, 255, 0.18);
  background: rgba(255, 255, 255, 0.7);
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.home-btn:hover {
  background: rgba(24, 144, 255, 0.08);
  border-color: rgba(24, 144, 255, 0.35);
}
.header-title {
  font-size: 16px;
  font-weight: 600;
  color: rgba(15, 42, 74, 0.85);
}
.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}
.dashboard-btn {
  border-radius: 999px;
}
.content {
  padding: 20px;
}
</style>
