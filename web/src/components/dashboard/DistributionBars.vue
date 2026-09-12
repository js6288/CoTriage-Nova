<template>
  <div v-if="entries.length" class="distribution">
    <div v-for="item in entries" :key="item[0]" class="row">
      <div class="row-head"><span>{{ item[0] }}</span><b>{{ item[1] }}</b></div>
      <div class="track"><div class="fill" :style="{ width: `${(item[1] / max) * 100}%`, background: color }"></div></div>
    </div>
  </div>
  <a-empty v-else :image="simpleImage" description="等待推理数据" />
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { Empty } from 'ant-design-vue';

const props = defineProps<{ items: Record<string, number>; color: string }>();
const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;
const entries = computed(() => Object.entries(props.items).sort((a, b) => b[1] - a[1]));
const max = computed(() => Math.max(1, ...entries.value.map((item) => item[1])));
</script>

<style scoped>
.distribution { display: flex; flex-direction: column; gap: 13px; }
.row-head { display: flex; justify-content: space-between; color: #52677b; font-size: 12px; margin-bottom: 5px; }
.row-head b { color: #16324f; }
.track { height: 7px; background: #eef5fb; border-radius: 5px; overflow: hidden; }
.fill { height: 100%; border-radius: 5px; min-width: 3px; }
</style>
