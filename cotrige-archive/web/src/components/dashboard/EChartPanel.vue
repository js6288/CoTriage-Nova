<template>
  <div ref="chartEl" class="echart-panel"></div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, shallowRef, ref, watch } from 'vue';
import * as echarts from 'echarts/core';
import { BarChart, LineChart, PieChart } from 'echarts/charts';
import {
  GridComponent,
  LegendComponent,
  TooltipComponent,
  TitleComponent
} from 'echarts/components';
import { CanvasRenderer } from 'echarts/renderers';
import type { ECharts, EChartsCoreOption } from 'echarts/core';

echarts.use([
  BarChart,
  LineChart,
  PieChart,
  GridComponent,
  LegendComponent,
  TooltipComponent,
  TitleComponent,
  CanvasRenderer
]);

const props = defineProps<{
  option: EChartsCoreOption;
}>();

const chartEl = ref<HTMLDivElement>();
const chart = shallowRef<ECharts>();
let resizeObserver: ResizeObserver | undefined;

function render() {
  if (!chart.value || !props.option) return;
  chart.value.setOption(props.option, true);
}

onMounted(() => {
  if (!chartEl.value) return;
  chart.value = echarts.init(chartEl.value);
  render();
  resizeObserver = new ResizeObserver(() => chart.value?.resize());
  resizeObserver.observe(chartEl.value);
});

watch(() => props.option, render, { deep: true });

onBeforeUnmount(() => {
  resizeObserver?.disconnect();
  chart.value?.dispose();
});
</script>

<style scoped>
.echart-panel {
  width: 100%;
  height: 260px;
}
</style>
