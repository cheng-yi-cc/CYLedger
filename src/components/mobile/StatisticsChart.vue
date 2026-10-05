<template>
    <v-chart class="cy-stat-chart" :style="{height: `${height || 230}px`}" :option="styledOption" autoresize @click="clicked" />
</template>
<script setup lang="ts">
import { computed } from 'vue';
import VChart from 'vue-echarts';
import { use } from 'echarts/core';
import { CanvasRenderer } from 'echarts/renderers';
import { BarChart, LineChart, PieChart, SankeyChart } from 'echarts/charts';
import { GridComponent, TooltipComponent, LegendComponent, DataZoomComponent } from 'echarts/components';
import type { EChartsOption } from 'echarts';
import { useEnvironmentsStore } from '@/stores/environment.ts';
use([CanvasRenderer, BarChart, LineChart, PieChart, SankeyChart, GridComponent, TooltipComponent, LegendComponent, DataZoomComponent]);
const props = defineProps<{ option: EChartsOption; height?: number }>();
const emit = defineEmits<{ select: [value: { name: string; dataIndex: number; seriesName: string }] }>();
const environments = useEnvironmentsStore();
const styledOption = computed<EChartsOption>(() => ({
    backgroundColor: 'transparent', animationDuration: 250,
    textStyle: { color: environments.framework7DarkMode ? '#b9c6cc' : '#56666b', fontFamily: 'inherit', fontSize: 10 },
    ...props.option
}));
function clicked(value: unknown) { emit('select', value as { name: string; dataIndex: number; seriesName: string }); }
</script>
<style scoped>.cy-stat-chart{width:100%;min-width:0;touch-action:pan-y}</style>
