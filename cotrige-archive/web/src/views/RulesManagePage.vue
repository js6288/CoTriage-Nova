<template>
  <a-card :bordered="false" class="card">
    <template #title>规则管理</template>
    <template #extra>
      <a-space>
        <a-button @click="publishActive" :loading="publishing">发布并启用当前规则</a-button>
        <a-button type="primary" @click="openCreate">新增规则</a-button>
      </a-space>
    </template>

    <a-table :dataSource="rows" :columns="columns" :loading="loading" :rowKey="(r: EdgeRule) => r.id" :pagination="false">
      <template #bodyCell="{ column, record }">
        <template v-if="column.dataIndex === 'edge_pred'"><a-tag :color="predColor(record.edge_pred)">{{ record.edge_pred }}</a-tag></template>
        <template v-else-if="column.dataIndex === 'enabled'"><a-tag :color="record.enabled ? 'green' : 'default'">{{ record.enabled ? '启用' : '停用' }}</a-tag></template>
        <template v-else-if="column.dataIndex === 'confidence'">{{ Math.round(record.confidence * 100) }}%</template>
        <template v-else-if="column.dataIndex === 'action'">
          <a-space>
            <a-button size="small" @click="openEdit(record)">编辑</a-button>
            <a-popconfirm title="确认删除该规则？" @confirm="remove(record.id)">
              <a-button size="small" danger>删除</a-button>
            </a-popconfirm>
          </a-space>
        </template>
      </template>
    </a-table>
  </a-card>

  <a-drawer v-model:open="drawerOpen" :title="editing?.id ? '编辑规则' : '新增规则'" width="620" @close="resetForm">
    <a-form layout="vertical" :model="form">
      <a-form-item label="规则名称"><a-input v-model:value="form.name" /></a-form-item>
      <a-form-item label="描述"><a-textarea v-model:value="form.description" :rows="2" /></a-form-item>
      <a-form-item label="匹配类型">
        <a-select v-model:value="form.match_type">
          <a-select-option value="contains">contains</a-select-option>
          <a-select-option value="regex">regex</a-select-option>
          <a-select-option value="all_keywords">all_keywords</a-select-option>
          <a-select-option value="any_keywords">any_keywords</a-select-option>
        </a-select>
      </a-form-item>
      <a-form-item label="匹配字段">
        <a-select v-model:value="form.target_field">
          <a-select-option value="template_text">template_text</a-select-option>
          <a-select-option value="raw_log">raw_log</a-select-option>
        </a-select>
      </a-form-item>
      <a-form-item label="Pattern"><a-input v-model:value="form.pattern" placeholder="contains/regex 使用" /></a-form-item>
      <a-form-item label="关键词"><a-select v-model:value="form.keywords" mode="tags" placeholder="all/any keywords 使用" /></a-form-item>
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
      <a-form-item label="优先级"><a-input-number v-model:value="form.priority" style="width: 100%" /></a-form-item>
      <a-form-item label="状态">
        <a-select v-model:value="form.status">
          <a-select-option value="draft">draft</a-select-option>
          <a-select-option value="published">published</a-select-option>
          <a-select-option value="disabled">disabled</a-select-option>
        </a-select>
      </a-form-item>
      <a-form-item><a-checkbox v-model:checked="form.enabled">启用规则</a-checkbox></a-form-item>
      <a-button type="primary" block :loading="saving" @click="save">保存</a-button>
    </a-form>
  </a-drawer>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue';
import type { TableColumnsType } from 'ant-design-vue';
import { message } from 'ant-design-vue';
import { activateRuleSet, createRule, deleteRule, listRules, updateRule, type EdgePred, type EdgeRule, type RulePayload } from '../api/edgeKnowledge';

const predOptions: EdgePred[] = ['NORMAL', 'ABNORMAL', 'APP_ERROR', 'NET_ERROR', 'DISK_ERROR', 'INFRA_ERROR', 'SECURITY', 'UNKNOWN'];

const rows = ref<EdgeRule[]>([]);
const loading = ref(false);
const saving = ref(false);
const publishing = ref(false);
const drawerOpen = ref(false);
const editing = ref<EdgeRule>();

const form = reactive<RulePayload>({ name: '', description: '', match_type: 'contains', pattern: '', keywords: [], target_field: 'template_text', edge_pred: 'ABNORMAL', severity: 60, confidence: 0.9, evidence: [], advice_code: '', advice: '', priority: 100, enabled: true, status: 'draft' });

const columns: TableColumnsType = [
  { title: '名称', dataIndex: 'name' },
  { title: '匹配', dataIndex: 'pattern', ellipsis: true },
  { title: '分类', dataIndex: 'edge_pred', width: 120 },
  { title: '严重度', dataIndex: 'severity', width: 90 },
  { title: '置信度', dataIndex: 'confidence', width: 90 },
  { title: '优先级', dataIndex: 'priority', width: 90 },
  { title: '启用', dataIndex: 'enabled', width: 90 },
  { title: '操作', dataIndex: 'action', width: 150 }
];

async function reload() {
  loading.value = true;
  try { rows.value = (await listRules()).items; } finally { loading.value = false; }
}
function openCreate() { resetForm(); drawerOpen.value = true; }
function openEdit(row: EdgeRule) { editing.value = row; Object.assign(form, JSON.parse(JSON.stringify(row))); drawerOpen.value = true; }
function resetForm() { editing.value = undefined; Object.assign(form, { name: '', description: '', match_type: 'contains', pattern: '', keywords: [], target_field: 'template_text', edge_pred: 'ABNORMAL', severity: 60, confidence: 0.9, evidence: [], advice_code: '', advice: '', priority: 100, enabled: true, status: 'draft' }); }
async function save() {
  saving.value = true;
  try {
    if (editing.value?.id) await updateRule(editing.value.id, form); else await createRule(form);
    message.success('规则已保存'); drawerOpen.value = false; await reload();
  } finally { saving.value = false; }
}
async function remove(id: number) { await deleteRule(id); message.success('规则已删除'); await reload(); }
async function publishActive() {
  publishing.value = true;
  try { await activateRuleSet(`rules-${Date.now()}`, rows.value.filter(r => r.enabled).map(r => r.id)); message.success('规则集已发布并启用'); }
  finally { publishing.value = false; }
}
function predColor(value: EdgePred) { return value === 'NORMAL' ? 'green' : value === 'UNKNOWN' ? 'default' : 'orange'; }
reload();
</script>

<style scoped>
.card { background: rgba(255,255,255,.92); border: 1px solid rgba(24,144,255,.12); }
</style>
