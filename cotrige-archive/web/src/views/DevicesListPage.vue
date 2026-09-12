<template>
  <div>
    <a-card :bordered="false" class="card">
      <template #title>设备管理</template>

      <a-form layout="inline" :model="filters" @submit.prevent>
        <a-form-item label="车间">
          <a-input v-model:value="filters.site" placeholder="workshop-a" allow-clear style="width: 180px" />
        </a-form-item>
        <a-form-item label="设备">
          <a-input v-model:value="filters.equipment" placeholder="press-01" allow-clear style="width: 180px" />
        </a-form-item>
        <a-form-item label="状态">
          <a-select v-model:value="filters.status" allow-clear placeholder="全部" style="width: 160px">
            <a-select-option value="Ready">Ready</a-select-option>
            <a-select-option value="NotReady">NotReady</a-select-option>
            <a-select-option value="Unknown">Unknown</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item>
          <a-button type="primary" :loading="loading" @click="reload">查询</a-button>
        </a-form-item>
      </a-form>

      <div style="margin-top: 14px">
        <a-table
          :dataSource="rows"
          :columns="columns"
          :loading="loading"
          :rowKey="(r: DeviceItem) => r.node_name"
          :pagination="false"
          :customRow="customRow"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.dataIndex === 'status'">
              <a-tag :color="statusColor(record.status)">{{ record.status }}</a-tag>
            </template>
          </template>
        </a-table>
        <div class="meta">共 {{ total }} 台设备</div>
      </div>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue';
import { useRouter } from 'vue-router';
import type { TableColumnsType } from 'ant-design-vue';

import { listDevices, type DeviceItem, type DeviceStatus } from '../api/devices';

const router = useRouter();

const loading = ref(false);
const rows = ref<DeviceItem[]>([]);
const total = ref(0);

const filters = reactive<{ site: string; equipment: string; status?: DeviceStatus }>(
  { site: '', equipment: '', status: undefined }
);

const columns = computed<TableColumnsType>(() => [
  { title: '设备ID', dataIndex: 'node_name' },
  { title: '设备名称', dataIndex: 'equipment' },
  { title: '设备状态', dataIndex: 'status' }
]);

function statusColor(status: DeviceStatus) {
  if (status === 'Ready') return 'green';
  if (status === 'NotReady') return 'red';
  return 'gold';
}

async function reload() {
  loading.value = true;
  try {
    const data = await listDevices({
      site: filters.site || undefined,
      equipment: filters.equipment || undefined,
      status: filters.status
    });
    rows.value = data.items;
    total.value = data.total;
  } finally {
    loading.value = false;
  }
}

function onRowClick(record: DeviceItem) {
  router.push({ name: 'deviceDetail', params: { nodeName: record.node_name } });
}

function customRow(record: DeviceItem) {
  return {
    onClick: () => onRowClick(record)
  };
}

reload();
</script>

<style scoped>
.card {
  background: rgba(255, 255, 255, 0.9);
  border: 1px solid rgba(24, 144, 255, 0.12);
}
.meta {
  margin-top: 12px;
  color: rgba(15, 42, 74, 0.65);
}

:deep(.ant-table-tbody > tr) {
  cursor: pointer;
}
</style>
