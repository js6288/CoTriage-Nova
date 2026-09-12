<template>
  <a-card :bordered="false" class="card">
    <template #title>模板知识库</template>
    <template #extra>
      <a-space>
        <a-button @click="publishActive" :loading="publishing">发布并启用当前模板</a-button>
        <a-button type="primary" @click="openCreate">新增模板</a-button>
      </a-space>
    </template>

    <a-table :dataSource="rows" :columns="columns" :loading="loading" :rowKey="(r: EdgeTemplate) => r.id" :pagination="false">
      <template #bodyCell="{ column, record }">
        <template v-if="column.dataIndex === 'edge_pred'">
          <a-tag :color="predColor(record.edge_pred)">{{ record.edge_pred }}</a-tag>
        </template>
        <template v-else-if="column.dataIndex === 'confidence'">{{ Math.round(record.confidence * 100) }}%</template>
        <template v-else-if="column.dataIndex === 'status'">
          <a-tag :color="record.status === 'published' ? 'blue' : record.status === 'disabled' ? 'default' : 'gold'">{{ record.status }}</a-tag>
        </template>
        <template v-else-if="column.dataIndex === 'action'">
          <a-space>
            <a-button size="small" @click="openEdit(record)">编辑</a-button>
            <a-popconfirm title="确认删除该模板？" @confirm="remove(record.id)">
              <a-button size="small" danger>删除</a-button>
            </a-popconfirm>
          </a-space>
        </template>
      </template>
    </a-table>
  </a-card>

  <a-drawer v-model:open="drawerOpen" :title="editing?.id ? '编辑模板' : '新增模板'" width="680" @close="resetForm">
    <a-form layout="vertical" :model="form">
      <a-form-item label="日志模板"><a-textarea v-model:value="form.template_text" :rows="3" /></a-form-item>
      <a-form-item label="原始日志示例"><a-textarea v-model:value="form.raw_example" :rows="3" /></a-form-item>
      <a-form-item label="来源"><a-input v-model:value="form.source" placeholder="manual / imported / labeled" /></a-form-item>
      <a-form-item label="输出分类">
        <a-select v-model:value="form.edge_pred">
          <a-select-option v-for="item in predOptions" :key="item" :value="item">{{ item }}</a-select-option>
        </a-select>
      </a-form-item>
      <a-form-item label="严重度"><a-slider v-model:value="form.severity" :min="0" :max="100" /></a-form-item>
      <a-form-item label="置信度"><a-slider v-model:value="form.confidence" :min="0" :max="1" :step="0.01" /></a-form-item>
      <a-form-item label="证据"><a-select v-model:value="form.evidence" mode="tags" /></a-form-item>
      <a-form-item label="建议编码"><a-input v-model:value="form.advice_code" /></a-form-item>
      <a-form-item label="处理建议"><a-textarea v-model:value="form.advice" :rows="3" /></a-form-item>
      <a-form-item label="状态">
        <a-select v-model:value="form.status">
          <a-select-option value="draft">draft</a-select-option>
          <a-select-option value="published">published</a-select-option>
          <a-select-option value="disabled">disabled</a-select-option>
        </a-select>
      </a-form-item>
      <a-button type="primary" block :loading="saving" @click="save">保存</a-button>
    </a-form>
  </a-drawer>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue';
import type { TableColumnsType } from 'ant-design-vue';
import { message } from 'ant-design-vue';
import { activateTemplateSet, createTemplate, deleteTemplate, listTemplates, updateTemplate, type EdgePred, type EdgeTemplate, type TemplatePayload } from '../api/edgeKnowledge';

const predOptions: EdgePred[] = ['NORMAL', 'ABNORMAL', 'APP_ERROR', 'NET_ERROR', 'DISK_ERROR', 'INFRA_ERROR', 'SECURITY', 'UNKNOWN'];

const rows = ref<EdgeTemplate[]>([]);
const loading = ref(false);
const saving = ref(false);
const publishing = ref(false);
const drawerOpen = ref(false);
const editing = ref<EdgeTemplate>();

const form = reactive<TemplatePayload>({
  template_text: '',
  raw_example: '',
  source: 'manual',
  edge_pred: 'ABNORMAL',
  severity: 60,
  confidence: 0.9,
  evidence: [],
  advice_code: '',
  advice: '',
  status: 'draft'
});

const columns: TableColumnsType = [
  { title: '模板', dataIndex: 'template_text', ellipsis: true },
  { title: '来源', dataIndex: 'source', width: 110 },
  { title: '分类', dataIndex: 'edge_pred', width: 120 },
  { title: '严重度', dataIndex: 'severity', width: 90 },
  { title: '置信度', dataIndex: 'confidence', width: 90 },
  { title: '状态', dataIndex: 'status', width: 110 },
  { title: '操作', dataIndex: 'action', width: 150 }
];

async function reload() {
  loading.value = true;
  try {
    rows.value = (await listTemplates()).items;
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  resetForm();
  drawerOpen.value = true;
}

function openEdit(row: EdgeTemplate) {
  editing.value = row;
  Object.assign(form, JSON.parse(JSON.stringify(row)));
  drawerOpen.value = true;
}

function resetForm() {
  editing.value = undefined;
  Object.assign(form, {
    template_text: '',
    raw_example: '',
    source: 'manual',
    edge_pred: 'ABNORMAL',
    severity: 60,
    confidence: 0.9,
    evidence: [],
    advice_code: '',
    advice: '',
    status: 'draft'
  });
}

async function save() {
  saving.value = true;
  try {
    if (editing.value?.id) await updateTemplate(editing.value.id, form);
    else await createTemplate(form);
    message.success('模板已保存');
    drawerOpen.value = false;
    await reload();
  } finally {
    saving.value = false;
  }
}

async function remove(id: number) {
  await deleteTemplate(id);
  message.success('模板已删除');
  await reload();
}

async function publishActive() {
  publishing.value = true;
  try {
    await activateTemplateSet(`templates-${Date.now()}`, rows.value.filter(r => r.status !== 'disabled').map(r => r.id));
    message.success('模板集已发布并启用');
  } finally {
    publishing.value = false;
  }
}

function predColor(value: EdgePred) {
  return value === 'NORMAL' ? 'green' : value === 'UNKNOWN' ? 'default' : 'orange';
}

reload();
</script>

<style scoped>
.card {
  background: rgba(255, 255, 255, 0.92);
  border: 1px solid rgba(24, 144, 255, 0.12);
}
</style>
