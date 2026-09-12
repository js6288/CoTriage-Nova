<template>
  <div class="page">
    <a-card :bordered="false" class="card" :loading="loading">
      <template #title>
        <div class="title-row">
          <div>
            <div class="title">设备详情</div>
            <div class="subtitle">设备ID：{{ device?.node_name || nodeName }}</div>
          </div>
          <div class="actions">
            <a-button @click="back">返回</a-button>
          </div>
        </div>
      </template>

      <div v-if="device" class="grid">
        <a-card size="small" class="panel" :bordered="false">
          <template #title>基础信息</template>
          <div class="kv">
            <div class="k">设备ID</div>
            <div class="v mono">{{ device.node_name }}</div>
          </div>
          <div class="kv">
            <div class="k">设备名</div>
            <div class="v">{{ device.equipment }}</div>
          </div>
          <div class="kv">
            <div class="k">车间</div>
            <div class="v">{{ device.site }}</div>
          </div>
        </a-card>

        <a-card size="small" class="panel" :bordered="false">
          <template #title>运行状态</template>
          <div class="row">
            <div class="label">设备状态</div>
            <div class="value">
              <a-tag :color="deviceStatusColor(device.status)">{{ device.status }}</a-tag>
            </div>
          </div>
          <div class="row">
            <div class="label">监视器状态</div>
            <div class="value">
              <a-tag :color="collectorStatusColor(device.collector_status)">{{ device.collector_status }}</a-tag>
            </div>
          </div>
        </a-card>

        <a-card size="small" class="panel" :bordered="false">
          <template #title>健康检查</template>
          <div class="metric">
            <div class="metric-name">该节点最近一次上报事件时间</div>
            <div class="metric-value mono">{{ formatLastEventTime(device.last_event_time) }}</div>
          </div>
          <div class="metric">
            <div class="metric-name">最近一分钟事件上报速率</div>
            <div class="metric-value">
              <span class="rate">{{ device.ingest_rate_1m }}</span>
              <span class="unit">条/分钟</span>
            </div>
          </div>
        </a-card>
      </div>

      <a-empty v-else description="未找到该设备" />
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import dayjs from 'dayjs';

import { listDevices, type DeviceItem } from '../api/devices';

const route = useRoute();
const router = useRouter();

const nodeName = computed(() => String(route.params.nodeName || ''));

const loading = ref(false);
const device = ref<DeviceItem | null>(null);

async function load() {
  if (!nodeName.value) return;

  loading.value = true;
  try {
    // MVP: 复用 list 接口并在前端过滤。后续如增加 GET /devices/:node 可替换。
    const data = await listDevices({ limit: 1000, offset: 0 });
    device.value = data.items.find((x) => x.node_name === nodeName.value) ?? null;
  } finally {
    loading.value = false;
  }
}

function back() {
  router.push({ name: 'devices' });
}

function deviceStatusColor(status: DeviceItem['status']) {
  if (status === 'Ready') return 'green';
  if (status === 'NotReady') return 'red';
  return 'gold';
}

function collectorStatusColor(status: DeviceItem['collector_status']) {
  if (status === 'Running') return 'green';
  if (status === 'CrashLoopBackOff' || status === 'ImagePullBackOff') return 'red';
  if (status === 'Pending') return 'gold';
  if (status === 'NotFound') return 'default';
  return 'blue';
}

function formatLastEventTime(ts: string | null) {
  if (!ts) return '-';
  const d = dayjs(ts);
  if (!d.isValid()) return ts;
  return d.add(0, 'hour').format('YYYY年MM月DD日 HH:mm:ss');
}

load();
</script>

<style scoped>
.page {
  /* 平铺占满管理台内容区宽度 */
  width: 100%;
}
.card {
  background: rgba(255, 255, 255, 0.9);
  border: 1px solid rgba(24, 144, 255, 0.12);
}
.title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.title {
  font-size: 18px;
  font-weight: 700;
  color: rgba(15, 42, 74, 0.9);
}
.subtitle {
  margin-top: 2px;
  font-size: 12px;
  color: rgba(15, 42, 74, 0.55);
}
.actions {
  display: flex;
  align-items: center;
}
.grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 12px;
}
@media (min-width: 900px) {
  .grid {
    grid-template-columns: 1fr 1fr;
  }
  .grid :deep(.panel:nth-child(3)) {
    grid-column: 1 / -1;
  }
}
.panel {
  background: rgba(246, 251, 255, 0.9);
  border: 1px solid rgba(24, 144, 255, 0.12);
}
.kv {
  display: grid;
  grid-template-columns: 110px 1fr;
  padding: 10px 0;
  border-bottom: 1px dashed rgba(24, 144, 255, 0.16);
}
.kv:last-child {
  border-bottom: none;
}
.k {
  color: rgba(15, 42, 74, 0.65);
}
.v {
  color: rgba(15, 42, 74, 0.9);
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
}
.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 0;
  border-bottom: 1px dashed rgba(24, 144, 255, 0.16);
}
.row:last-child {
  border-bottom: none;
}
.label {
  color: rgba(15, 42, 74, 0.65);
}
.value {
  color: rgba(15, 42, 74, 0.9);
}
.metric {
  padding: 10px 0;
  border-bottom: 1px dashed rgba(24, 144, 255, 0.16);
}
.metric:last-child {
  border-bottom: none;
}
.metric-name {
  font-size: 12px;
  color: rgba(15, 42, 74, 0.6);
  margin-bottom: 6px;
}
.metric-value {
  font-size: 14px;
  color: rgba(15, 42, 74, 0.9);
}
.rate {
  font-size: 24px;
  font-weight: 800;
  color: #1677ff;
}
.unit {
  margin-left: 8px;
  color: rgba(15, 42, 74, 0.55);
}
</style>
