<template>
  <a-card :bordered="false" class="card">
    <template #title>模型管理</template>
    <a-tabs v-model:activeKey="activeTab">
      <a-tab-pane key="classifier" tab="轻量分类模型">
        <div class="toolbar">
          <span class="hint">配置 MODEL 路径使用的 MiniLM / DistilBERT ONNX HTTP 服务。</span>
          <a-button type="primary" @click="openClassifierCreate">新增分类模型</a-button>
        </div>
        <a-table :dataSource="classifiers" :columns="classifierColumns" :loading="loadingClassifiers" :rowKey="(r: ClassifierConfig) => r.id" :pagination="false">
          <template #bodyCell="{ column, record }">
            <template v-if="column.dataIndex === 'status'"><a-tag :color="statusColor(record.status)">{{ record.status }}</a-tag></template>
            <template v-else-if="column.dataIndex === 'is_active'"><a-tag :color="record.is_active ? 'green' : 'default'">{{ record.is_active ? '生效中' : '未启用' }}</a-tag></template>
            <template v-else-if="column.dataIndex === 'threshold'">{{ Math.round(record.threshold * 100) }}%</template>
            <template v-else-if="column.dataIndex === 'action'">
              <a-space>
                <a-button size="small" @click="openClassifierEdit(record)">编辑</a-button>
                <a-button size="small" type="primary" :disabled="record.is_active" @click="enableClassifier(record.id)">启用</a-button>
                <a-popconfirm title="确认删除该分类模型配置？" @confirm="removeClassifier(record.id)">
                  <a-button size="small" danger :disabled="record.is_active">删除</a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </template>
        </a-table>
      </a-tab-pane>

      <a-tab-pane key="llm" tab="边缘小 LLM">
        <div class="toolbar">
          <span class="hint">配置 EDGE_LLM 路径使用的 OpenAI 兼容服务，例如 llama.cpp / vLLM。</span>
          <a-button type="primary" @click="openProviderCreate">新增 LLM Provider</a-button>
        </div>
        <a-table :dataSource="providers" :columns="providerColumns" :loading="loadingProviders" :rowKey="(r: EdgeProvider) => r.id" :pagination="false">
          <template #bodyCell="{ column, record }">
            <template v-if="column.dataIndex === 'status'"><a-tag :color="statusColor(record.status)">{{ record.status }}</a-tag></template>
            <template v-else-if="column.dataIndex === 'is_active'"><a-tag :color="record.is_active ? 'green' : 'default'">{{ record.is_active ? '生效中' : '未启用' }}</a-tag></template>
            <template v-else-if="column.dataIndex === 'action'">
              <a-space>
                <a-button size="small" @click="openProviderEdit(record)">编辑</a-button>
                <a-button size="small" type="primary" :disabled="record.is_active" @click="enableProvider(record.id)">启用</a-button>
                <a-popconfirm title="确认删除该 LLM 配置？" @confirm="removeProvider(record.id)">
                  <a-button size="small" danger :disabled="record.is_active">删除</a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </template>
        </a-table>
      </a-tab-pane>

      <a-tab-pane key="cloud" tab="云端大模型">
        <a-alert class="cloud-notice" type="info" show-icon>
          <template #message>API Key 安全说明</template>
          <template #description>
            API Key 由 server 统一保存并下发给 cloud-gateway。点击可以切换为明文展示。
          </template>
        </a-alert>
        <div class="toolbar">
          <span class="hint">当前配置：{{ cloudConfigText }}</span>
          <a-button type="primary" @click="openCloudProviderCreate">新增云端 Provider</a-button>
        </div>
        <a-table :dataSource="cloudProviders" :columns="cloudProviderColumns" :loading="loadingCloudProviders" :rowKey="(r: CloudProvider) => r.id" :pagination="false">
          <template #bodyCell="{ column, record }">
            <template v-if="column.dataIndex === 'status'"><a-tag :color="statusColor(record.status)">{{ record.status }}</a-tag></template>
            <template v-else-if="column.dataIndex === 'is_active'"><a-tag :color="record.is_active ? 'green' : 'default'">{{ record.is_active ? '生效中' : '未启用' }}</a-tag></template>
            <template v-else-if="column.dataIndex === 'api_key'"><a-input-password :value="record.api_key" readonly size="small" /></template>
            <template v-else-if="column.dataIndex === 'action'">
              <a-space>
                <a-button size="small" :disabled="record.is_active" @click="openCloudProviderEdit(record)">编辑</a-button>
                <a-button size="small" type="primary" :disabled="record.is_active" @click="enableCloudProvider(record.id)">启用</a-button>
                <a-popconfirm title="确认删除该云端模型配置？" @confirm="removeCloudProvider(record.id)">
                  <a-button size="small" danger :disabled="record.is_active">删除</a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </template>
        </a-table>
      </a-tab-pane>
    </a-tabs>
  </a-card>

  <a-drawer v-model:open="classifierDrawer" :title="editingClassifier?.id ? '编辑分类模型' : '新增分类模型'" width="620" @close="resetClassifierForm">
    <a-form layout="vertical" :model="classifierForm">
      <a-form-item label="名称"><a-input v-model:value="classifierForm.name" /></a-form-item>
      <a-form-item label="Endpoint"><a-input v-model:value="classifierForm.endpoint" placeholder="http://127.0.0.1:18080/predict" /></a-form-item>
      <a-form-item label="模型版本"><a-input v-model:value="classifierForm.model" placeholder="bgl-minilm" /></a-form-item>
      <a-form-item label="超时 ms"><a-input-number v-model:value="classifierForm.timeout_ms" style="width: 100%" :min="100" /></a-form-item>
      <a-form-item label="告警阈值"><a-slider v-model:value="classifierForm.threshold" :min="0" :max="1" :step="0.01" /></a-form-item>
      <a-form-item label="状态">
        <a-select v-model:value="classifierForm.status">
          <a-select-option value="draft">draft</a-select-option>
          <a-select-option value="published">published</a-select-option>
          <a-select-option value="disabled">disabled</a-select-option>
        </a-select>
      </a-form-item>
      <a-button type="primary" block :loading="saving" @click="saveClassifier">保存</a-button>
    </a-form>
  </a-drawer>

  <a-drawer v-model:open="providerDrawer" :title="editingProvider?.id ? '编辑 LLM Provider' : '新增 LLM Provider'" width="680" @close="resetProviderForm">
    <a-form layout="vertical" :model="providerForm">
      <a-form-item label="名称"><a-input v-model:value="providerForm.name" /></a-form-item>
      <a-form-item label="Provider">
        <a-select v-model:value="providerForm.provider">
          <a-select-option value="llama_cpp">llama_cpp</a-select-option>
          <a-select-option value="openai_compat">openai_compat</a-select-option>
          <a-select-option value="vllm">vllm</a-select-option>
          <a-select-option value="custom_http">custom_http</a-select-option>
        </a-select>
      </a-form-item>
      <a-form-item label="Endpoint"><a-input v-model:value="providerForm.endpoint" placeholder="http://host:18081/v1/chat/completions" /></a-form-item>
      <a-form-item label="模型名"><a-input v-model:value="providerForm.model" /></a-form-item>
      <a-form-item label="超时 ms"><a-input-number v-model:value="providerForm.timeout_ms" style="width: 100%" :min="100" /></a-form-item>
      <a-form-item label="Max Tokens"><a-input-number v-model:value="providerForm.max_tokens" style="width: 100%" :min="1" /></a-form-item>
      <a-form-item label="Temperature"><a-slider v-model:value="providerForm.temperature" :min="0" :max="1" :step="0.01" /></a-form-item>
      <a-form-item label="Top P"><a-slider v-model:value="providerForm.top_p" :min="0" :max="1" :step="0.01" /></a-form-item>
      <a-form-item label="Response Path"><a-input v-model:value="providerForm.response_path" /></a-form-item>
      <a-form-item label="Health Path"><a-input v-model:value="providerForm.healthcheck_path" placeholder="/health，可为空" /></a-form-item>
      <a-form-item label="状态">
        <a-select v-model:value="providerForm.status">
          <a-select-option value="draft">draft</a-select-option>
          <a-select-option value="published">published</a-select-option>
          <a-select-option value="disabled">disabled</a-select-option>
        </a-select>
      </a-form-item>
      <a-button type="primary" block :loading="saving" @click="saveProvider">保存</a-button>
    </a-form>
  </a-drawer>

  <a-drawer v-model:open="cloudProviderDrawer" :title="editingCloudProvider?.id ? '编辑云端 Provider' : '新增云端 Provider'" width="700" @close="resetCloudProviderForm">
    <a-form layout="vertical" :model="cloudProviderForm">
      <a-form-item label="名称" required><a-input v-model:value="cloudProviderForm.name" placeholder="deepseek-prod" /></a-form-item>
      <a-form-item label="Provider" required>
        <a-select v-model:value="cloudProviderForm.provider" @change="applyCloudProviderPreset">
          <a-select-option value="deepseek">deepseek</a-select-option>
          <a-select-option value="qwen">qwen</a-select-option>
          <a-select-option value="openai_compat">openai_compat</a-select-option>
          <a-select-option value="custom_http">custom_http</a-select-option>
        </a-select>
      </a-form-item>
      <a-form-item label="Endpoint" required><a-input v-model:value="cloudProviderForm.endpoint" placeholder="https://api.deepseek.com/v1/chat/completions" /></a-form-item>
      <a-form-item label="模型名" required><a-input v-model:value="cloudProviderForm.model" placeholder="deepseek-chat" /></a-form-item>
      <a-form-item label="API Key 环境变量名" required>
        <a-input v-model:value="cloudProviderForm.api_key_env" placeholder="DEEPSEEK_API_KEY" />
        <div class="field-help">用于标识和兼容旧部署，不包含真实 Key。</div>
      </a-form-item>
      <a-form-item label="API Key" required>
        <a-input-password v-model:value="cloudProviderForm.api_key" placeholder="输入真实 API Key" autocomplete="new-password" />
        <div class="field-help">点击右侧眼睛图标可在明文和星号之间切换。</div>
      </a-form-item>
      <a-row :gutter="12">
        <a-col :span="12"><a-form-item label="超时 ms"><a-input-number v-model:value="cloudProviderForm.timeout_ms" style="width: 100%" :min="1000" /></a-form-item></a-col>
        <a-col :span="12"><a-form-item label="Max Tokens"><a-input-number v-model:value="cloudProviderForm.max_tokens" style="width: 100%" :min="1" /></a-form-item></a-col>
      </a-row>
      <a-form-item label="Temperature"><a-slider v-model:value="cloudProviderForm.temperature" :min="0" :max="2" :step="0.01" /></a-form-item>
      <a-form-item label="Top P"><a-slider v-model:value="cloudProviderForm.top_p" :min="0.01" :max="1" :step="0.01" /></a-form-item>
      <a-form-item label="Seed（可选）"><a-input-number v-model:value="cloudProviderForm.seed" style="width: 100%" /></a-form-item>
      <a-form-item label="Response Path"><a-input v-model:value="cloudProviderForm.response_path" /></a-form-item>
      <a-form-item label="Health Path（可选）"><a-input v-model:value="cloudProviderForm.health_path" placeholder="/health" /></a-form-item>
      <a-form-item v-if="editingCloudProvider" label="状态">
        <a-select v-model:value="cloudProviderForm.status">
          <a-select-option value="draft">draft</a-select-option>
          <a-select-option value="published">published</a-select-option>
          <a-select-option value="disabled">disabled</a-select-option>
        </a-select>
      </a-form-item>
      <a-button type="primary" block :loading="savingCloudProvider" @click="saveCloudProvider">保存配置</a-button>
    </a-form>
  </a-drawer>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue';
import type { TableColumnsType } from 'ant-design-vue';
import { message } from 'ant-design-vue';
import {
  activateClassifier, activateCloudProvider, activateProvider, createClassifier, createCloudProvider, createProvider,
  deleteClassifier, deleteCloudProvider, deleteProvider, listClassifiers, listCloudProviders,
  listProviders, updateClassifier, updateCloudProvider, updateProvider,
  type ClassifierConfig, type ClassifierPayload, type CloudProvider, type CloudProviderPayload,
  type EdgeProvider, type ModelStatus, type ProviderPayload
} from '../api/models';

const activeTab = ref('classifier');
const classifiers = ref<ClassifierConfig[]>([]);
const providers = ref<EdgeProvider[]>([]);
const cloudProviders = ref<CloudProvider[]>([]);
const cloudConfigText = ref('尚未配置生效中的云端模型');
const loadingClassifiers = ref(false);
const loadingProviders = ref(false);
const saving = ref(false);
const classifierDrawer = ref(false);
const providerDrawer = ref(false);
const cloudProviderDrawer = ref(false);
const editingClassifier = ref<ClassifierConfig>();
const editingProvider = ref<EdgeProvider>();
const editingCloudProvider = ref<CloudProvider>();
const loadingCloudProviders = ref(false);
const savingCloudProvider = ref(false);

const classifierForm = reactive<ClassifierPayload>({
  name: '',
  endpoint: 'http://127.0.0.1:18080/predict',
  model: 'bgl-minilm',
  timeout_ms: 1500,
  threshold: 0.05,
  status: 'draft'
});

const providerForm = reactive<ProviderPayload>({
  name: '',
  provider: 'llama_cpp',
  endpoint: 'http://100.85.92.31:18081/v1/chat/completions',
  model: 'qwen1.5-1.8b-chat',
  timeout_ms: 30000,
  max_tokens: 384,
  temperature: 0.1,
  top_p: 1,
  seed: null,
  healthcheck_path: '/health',
  response_path: 'choices[0].message.content',
  status: 'draft'
});

const cloudProviderForm = reactive<CloudProviderPayload>({
  name: '',
  provider: 'deepseek',
  endpoint: 'https://api.deepseek.com/v1/chat/completions',
  model: 'deepseek-chat',
  api_key_env: 'DEEPSEEK_API_KEY',
  api_key: '',
  timeout_ms: 20000,
  max_tokens: 512,
  temperature: 0.1,
  top_p: 0.9,
  seed: null,
  health_path: null,
  response_path: 'choices[0].message.content',
  status: 'draft'
});

const classifierColumns: TableColumnsType = [
  { title: '名称', dataIndex: 'name' },
  { title: 'Endpoint', dataIndex: 'endpoint', ellipsis: true },
  { title: '模型', dataIndex: 'model', width: 150 },
  { title: '阈值', dataIndex: 'threshold', width: 90 },
  { title: '状态', dataIndex: 'status', width: 110 },
  { title: '生效', dataIndex: 'is_active', width: 100 },
  { title: '操作', dataIndex: 'action', width: 210 }
];

const providerColumns: TableColumnsType = [
  { title: '名称', dataIndex: 'name' },
  { title: 'Provider', dataIndex: 'provider', width: 130 },
  { title: 'Endpoint', dataIndex: 'endpoint', ellipsis: true },
  { title: '模型', dataIndex: 'model', width: 150 },
  { title: '状态', dataIndex: 'status', width: 110 },
  { title: '生效', dataIndex: 'is_active', width: 100 },
  { title: '操作', dataIndex: 'action', width: 210 }
];

const cloudProviderColumns: TableColumnsType = [
  { title: '名称', dataIndex: 'name', width: 150 },
  { title: 'Provider', dataIndex: 'provider', width: 130 },
  { title: 'Endpoint', dataIndex: 'endpoint', ellipsis: true },
  { title: '模型', dataIndex: 'model', width: 150 },
  { title: 'API Key', dataIndex: 'api_key', width: 220 },
  { title: '状态', dataIndex: 'status', width: 100 },
  { title: '生效', dataIndex: 'is_active', width: 90 },
  { title: '操作', dataIndex: 'action', width: 210 }
];

async function reloadClassifiers() {
  loadingClassifiers.value = true;
  try { classifiers.value = (await listClassifiers()).items; }
  finally { loadingClassifiers.value = false; }
}

async function reloadProviders() {
  loadingProviders.value = true;
  try { providers.value = (await listProviders()).items; }
  finally { loadingProviders.value = false; }
}

async function reloadCloudProviders() {
  loadingCloudProviders.value = true;
  try {
    cloudProviders.value = (await listCloudProviders()).items;
    const activeProvider = cloudProviders.value.find(provider => provider.is_active);
    cloudConfigText.value = activeProvider
      ? `${activeProvider.name} / ${activeProvider.model}（版本 ${activeProvider.config_version ?? 0}）`
      : '尚未配置生效中的云端模型';
  } finally {
    loadingCloudProviders.value = false;
  }
}

function openClassifierCreate() { resetClassifierForm(); classifierDrawer.value = true; }
function openClassifierEdit(row: ClassifierConfig) { editingClassifier.value = row; Object.assign(classifierForm, row); classifierDrawer.value = true; }
function resetClassifierForm() { editingClassifier.value = undefined; Object.assign(classifierForm, { name: '', endpoint: 'http://127.0.0.1:18080/predict', model: 'bgl-minilm', timeout_ms: 1500, threshold: 0.05, status: 'draft' as ModelStatus }); }

async function saveClassifier() {
  saving.value = true;
  try {
    if (editingClassifier.value?.id) await updateClassifier(editingClassifier.value.id, classifierForm); else await createClassifier(classifierForm);
    message.success('分类模型配置已保存');
    classifierDrawer.value = false;
    await reloadClassifiers();
  } finally { saving.value = false; }
}

async function enableClassifier(id: number) { await activateClassifier(id); message.success('分类模型已启用'); await reloadClassifiers(); }
async function removeClassifier(id: number) { await deleteClassifier(id); message.success('分类模型已删除'); await reloadClassifiers(); }

function openProviderCreate() { resetProviderForm(); providerDrawer.value = true; }
function openProviderEdit(row: EdgeProvider) { editingProvider.value = row; Object.assign(providerForm, row); providerDrawer.value = true; }
function resetProviderForm() { editingProvider.value = undefined; Object.assign(providerForm, { name: '', provider: 'llama_cpp', endpoint: 'http://100.85.92.31:18081/v1/chat/completions', model: 'qwen1.5-1.8b-chat', timeout_ms: 30000, max_tokens: 384, temperature: 0.1, top_p: 1, seed: null, healthcheck_path: '/health', response_path: 'choices[0].message.content', status: 'draft' as ModelStatus }); }

async function saveProvider() {
  saving.value = true;
  try {
    if (editingProvider.value?.id) await updateProvider(editingProvider.value.id, providerForm); else await createProvider(providerForm);
    message.success('LLM Provider 配置已保存');
    providerDrawer.value = false;
    await reloadProviders();
  } finally { saving.value = false; }
}

async function enableProvider(id: number) { await activateProvider(id); message.success('LLM Provider 已启用'); await reloadProviders(); }
async function removeProvider(id: number) { await deleteProvider(id); message.success('LLM Provider 已删除'); await reloadProviders(); }

function openCloudProviderCreate() { resetCloudProviderForm(); cloudProviderDrawer.value = true; }
function openCloudProviderEdit(row: CloudProvider) {
  editingCloudProvider.value = row;
  Object.assign(cloudProviderForm, row);
  cloudProviderDrawer.value = true;
}
function resetCloudProviderForm() {
  editingCloudProvider.value = undefined;
  Object.assign(cloudProviderForm, {
    name: '', provider: 'deepseek', endpoint: 'https://api.deepseek.com/v1/chat/completions',
    model: 'deepseek-chat', api_key_env: 'DEEPSEEK_API_KEY', api_key: '', timeout_ms: 20000,
    max_tokens: 512, temperature: 0.1, top_p: 0.9, seed: null, health_path: null,
    response_path: 'choices[0].message.content', status: 'draft' as ModelStatus
  });
}
function applyCloudProviderPreset(provider: string) {
  if (editingCloudProvider.value) return;
  if (provider === 'deepseek') Object.assign(cloudProviderForm, { endpoint: 'https://api.deepseek.com/v1/chat/completions', model: 'deepseek-chat', api_key_env: 'DEEPSEEK_API_KEY' });
  if (provider === 'qwen') Object.assign(cloudProviderForm, { endpoint: 'https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions', model: 'qwen-plus', api_key_env: 'DASHSCOPE_API_KEY' });
  if (provider === 'openai_compat') Object.assign(cloudProviderForm, { endpoint: '', model: '', api_key_env: 'OPENAI_API_KEY' });
  if (provider === 'custom_http') Object.assign(cloudProviderForm, { endpoint: '', model: '', api_key_env: 'CLOUD_API_KEY' });
}
function validateCloudProviderForm() {
  if (!cloudProviderForm.name.trim() || !cloudProviderForm.endpoint.trim() || !cloudProviderForm.model.trim()) return '请填写名称、Endpoint 和模型名';
  if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(cloudProviderForm.api_key_env.trim())) return 'API Key 环境变量名格式不正确';
  if (!cloudProviderForm.api_key?.trim()) return '请填写 API Key';
  return '';
}
async function saveCloudProvider() {
  const validationError = validateCloudProviderForm();
  if (validationError) { message.error(validationError); return; }
  savingCloudProvider.value = true;
  try {
    if (editingCloudProvider.value?.id) await updateCloudProvider(editingCloudProvider.value.id, cloudProviderForm);
    else await createCloudProvider(cloudProviderForm);
    message.success('云端大模型配置已保存');
    cloudProviderDrawer.value = false;
    await reloadCloudProviders();
  } finally { savingCloudProvider.value = false; }
}
async function enableCloudProvider(id: number) {
  await activateCloudProvider(id);
  message.success('云端大模型配置已启用，cloud-gateway 将自动热更新');
  await reloadCloudProviders();
}
async function removeCloudProvider(id: number) {
  await deleteCloudProvider(id);
  message.success('云端大模型配置已删除');
  await reloadCloudProviders();
}

function statusColor(value: ModelStatus) {
  if (value === 'published') return 'blue';
  if (value === 'disabled') return 'default';
  return 'gold';
}

reloadClassifiers();
reloadProviders();
reloadCloudProviders();
</script>

<style scoped>
.card { background: rgba(255, 255, 255, .92); border: 1px solid rgba(24, 144, 255, .12); }
.toolbar { display: flex; justify-content: space-between; align-items: center; gap: 16px; margin-bottom: 14px; }
.hint { color: #6b7f93; }
.cloud-notice { margin-bottom: 14px; }
.field-help { margin-top: 6px; color: #8c8c8c; font-size: 12px; }
</style>
