<template>
  <div :class="['metric-card', { warning }]">
    <div class="label">
      <span>{{ label }}</span>
      <a-tooltip v-if="tooltip" :title="tooltip">
        <span class="help-icon">!</span>
      </a-tooltip>
    </div>
    <div><span class="value">{{ value }}</span><span class="suffix">{{ suffix }}</span></div>
  </div>
</template>

<script setup lang="ts">
defineProps<{ label: string; value: number; suffix: string; warning?: boolean; tooltip?: string }>();
</script>

<style scoped>
.metric-card { position: relative; overflow: hidden; background: linear-gradient(180deg, rgba(255,255,255,.98), rgba(248,251,255,.94)); border: 1px solid rgba(219, 234, 254, .96); border-radius: 14px; padding: 16px 18px; box-shadow: 0 12px 28px rgba(15,42,74,.055); }
.metric-card.warning { border-color: #fed7aa; background: linear-gradient(180deg, #fffaf3, #fff7ed); }
.label { display: flex; align-items: center; gap: 6px; color: #66788a; font-size: 13px; margin-bottom: 8px; }
.help-icon { display: inline-flex; align-items: center; justify-content: center; width: 15px; height: 15px; border: 1px solid #94a3b8; border-radius: 50%; color: #64748b; font-size: 11px; font-weight: 700; line-height: 1; cursor: help; }
.value { color: #0f2a4a; font-size: 29px; line-height: 1; font-weight: 800; letter-spacing: -.02em; }
.suffix { color: #789; margin-left: 6px; font-size: 12px; }
</style>
