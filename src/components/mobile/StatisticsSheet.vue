<template>
    <f7-sheet class="cy-stat-sheet cy-mobile-surface" :opened="open" :style="{height: compact ? 'auto' : '88%'}" swipe-to-close swipe-handler=".cy-stat-sheet-bar" @sheet:closed="$emit('update:open', false)">
        <div class="cy-stat-sheet-bar"><button @click="$emit('update:open', false)">关闭</button><strong>{{ title }}</strong><slot name="action" /></div>
        <div class="cy-stat-sheet-body"><slot /></div>
    </f7-sheet>
</template>
<script setup lang="ts">defineProps<{ open: boolean; title: string; compact?: boolean }>(); defineEmits<{ 'update:open': [value: boolean] }>();</script>
<style scoped>
.cy-stat-sheet{background:var(--cy-card);color:var(--cy-ink);border-radius:18px 18px 0 0;max-height:88%}.cy-stat-sheet-bar{position:relative;height:52px;display:flex;gap:12px;align-items:center;justify-content:space-between;padding:0 18px;border-bottom:1px solid var(--cy-line)}.cy-stat-sheet-bar strong{position:absolute;left:50%;transform:translateX(-50%);max-width:60%;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.cy-stat-sheet-bar :deep(button){border:0;background:none;color:var(--cy-accent);font:inherit;padding:5px 0}.cy-stat-sheet-body{padding:18px;overflow-y:auto;height:calc(100% - 52px);box-sizing:border-box;padding-bottom:calc(30px + env(safe-area-inset-bottom))}
</style>
