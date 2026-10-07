<template>
    <f7-page class="cy-mobile-surface cy-asset-surface" @page:afterin="load">
        <f7-navbar title="标签管理" back-link="返回">
            <f7-nav-right>
                <f7-link :class="{ disabled: busy || loading }" @click="create()">新增</f7-link>
                <f7-link icon-f7="ellipsis_vertical" aria-label="标签管理更多操作" @click="menuOpen = true" />
            </f7-nav-right>
        </f7-navbar>
        <main class="management-body">
            <label v-if="searchOpen" class="management-search"><f7-icon f7="search" /><input v-model="query" type="search" placeholder="搜索标签" aria-label="搜索标签" /><button class="management-action" aria-label="关闭搜索" @click="query = ''; searchOpen = false"><f7-icon f7="xmark" /></button></label>
            <label v-if="groups.length > 1" class="management-filter">标签分组<select v-model="groupId" aria-label="标签分组"><option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option></select></label>
            <p v-if="showHidden" class="management-state"><f7-icon f7="eye_slash" size="14" />正在显示隐藏标签<button @click="showHidden = false">收起</button></p>
            <p v-if="error" class="cy-message" role="alert">{{ error }}<button @click="load">重试</button></p>
            <p v-if="loading" class="cy-empty">正在加载标签…</p>
            <template v-else>
                <section v-for="row in rows" :key="row.parent.id" class="management-card">
                    <div class="management-head">
                        <button class="management-main" :class="{ 'management-hidden': row.parent.hidden }" :aria-label="`编辑${row.parent.name}`" @click="edit(row.parent)">
                            <i class="management-tag-dot" /><span class="management-name">{{ row.parent.name }}<small v-if="row.parent.hidden">已隐藏</small><small v-else-if="row.parent.groupId !== groupId">{{ groupName(row.parent.groupId) }}</small></span>
                        </button>
                        <button class="management-action" :disabled="busy" :aria-label="`为${row.parent.name}新增子标签`" @click="create(row.parent)"><f7-icon f7="plus" /></button>
                        <button class="management-action" :aria-label="`${row.parent.name}的更多操作`" @click="selected = row.parent; actionsOpen = true"><f7-icon f7="text_alignleft" /></button>
                    </div>
                    <div v-if="row.children.length" class="tag-children">
                        <button v-for="child in row.children" :key="child.id" :class="{ 'management-hidden': child.hidden }" :aria-label="`${child.name}的更多操作`" @click="selected = child; actionsOpen = true">{{ child.name }}<f7-icon v-if="child.hidden" f7="eye_slash" size="12" /></button>
                    </div>
                </section>
                <p v-if="!rows.length" class="cy-empty">{{ query ? '没有找到匹配的标签' : '还没有标签' }}<br /><f7-link v-if="!query" @click="create()">新增第一个标签</f7-link></p>
            </template>
        </main>
        <f7-actions :opened="menuOpen" @actions:closed="menuOpen = false">
            <f7-actions-group>
                <f7-actions-button @click="searchOpen = true">搜索标签</f7-actions-button>
                <f7-actions-button @click="showHidden = !showHidden">{{ showHidden ? '收起隐藏标签' : '显示隐藏标签' }}</f7-actions-button>
                <f7-actions-button :class="{ disabled: groupTags.length < 2 }" @click="openOrder">调整标签顺序</f7-actions-button>
                <f7-actions-button @click="f7router.navigate('/tag/group/list')">管理标签分组</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group><f7-actions-button bold>取消</f7-actions-button></f7-actions-group>
        </f7-actions>
        <f7-actions :opened="actionsOpen" @actions:closed="actionsOpen = false">
            <f7-actions-group>
                <f7-actions-label>{{ selected?.name }}</f7-actions-label>
                <f7-actions-button @click="selected && edit(selected)">编辑标签</f7-actions-button>
                <f7-actions-button v-if="selected && isRoot(selected)" @click="create(selected)">新增子标签</f7-actions-button>
                <f7-actions-button @click="viewBills">查看账单</f7-actions-button>
                <f7-actions-button :class="{ disabled: busy }" @click="toggleHidden">{{ selected?.hidden ? '显示标签' : '隐藏标签' }}</f7-actions-button>
                <f7-actions-button color="red" :class="{ disabled: busy }" @click="confirmDelete">删除标签</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group><f7-actions-button bold>取消</f7-actions-button></f7-actions-group>
        </f7-actions>
        <f7-popup class="cy-mobile-surface cy-asset-surface" :opened="editorOpen" :close-by-backdrop-click="!busy" :close-on-escape="!busy" @popup:closed="editorOpen = false">
            <f7-page>
                <f7-navbar :title="draft.id ? '编辑标签' : '新增标签'">
                    <f7-nav-left><f7-link :class="{ disabled: busy }" @click="editorOpen = false">取消</f7-link></f7-nav-left>
                    <f7-nav-right><f7-link icon-f7="checkmark_alt" aria-label="保存标签" :class="{ disabled: busy || !draft.name.trim() }" @click="save" /></f7-nav-right>
                </f7-navbar>
                <main class="management-body">
                    <p v-if="editError" class="cy-message" role="alert">{{ editError }}</p>
                    <form class="management-form" @submit.prevent="save">
                        <label class="management-field"><span>标签名称</span><input v-model="draft.name" :disabled="busy" maxlength="64" placeholder="填写标签名称" aria-label="标签名称" /></label>
                        <label class="management-field"><span>一级标签</span><select v-model="draft.parentId" :disabled="busy || hasChildren(draft.id)" aria-label="一级标签"><option value="0">无（设为一级标签）</option><option v-for="parent in parents.filter(item => item.id !== draft.id)" :key="parent.id" :value="parent.id">{{ parent.name }}{{ parent.hidden ? '（已隐藏）' : '' }}</option></select></label>
                        <label v-if="groups.length > 1" class="management-field"><span>所属分组</span><select v-model="draft.groupId" :disabled="busy" aria-label="所属分组"><option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option></select></label>
                    </form>
                    <p v-if="hasChildren(draft.id)" class="management-caption">此标签已有子标签，保留为一级标签。</p>
                </main>
            </f7-page>
        </f7-popup>
        <ManagementOrderPopup v-model:open="orderOpen" :title="groups.length > 1 ? `${groupName(groupId)} · 排序` : '标签排序'" :items="orderItems" allow-name-sort :busy="busy" :error="orderError" @save="saveOrder" />
    </f7-page>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue';
import type { Router } from 'framework7/types';
import ManagementOrderPopup from '@/components/mobile/ManagementOrderPopup.vue';
import { useTransactionTagsStore } from '@/stores/transactionTag.ts';
import { TransactionTag } from '@/models/transaction_tag.ts';
import { useI18nUIComponents } from '@/lib/ui/mobile.ts';
import { useManagementFeedback } from '@/lib/use-management-feedback.ts';
const { errorText } = useManagementFeedback();
import services from '@/lib/services.ts';
const { f7router } = defineProps<{ f7router: Router.Router }>();
const store = useTransactionTagsStore();
const { showConfirm, showToast } = useI18nUIComponents();
const groupId = ref('0'), query = ref(''), searchOpen = ref(false), showHidden = ref(false);
const loading = ref(true), busy = ref(false), error = ref(''), editError = ref(''), orderError = ref('');
const menuOpen = ref(false), actionsOpen = ref(false), editorOpen = ref(false), orderOpen = ref(false);
const selected = ref<TransactionTag>(), draft = ref(TransactionTag.createNewTag());
const groups = computed(() => [{ id: '0', name: '默认分组' }, ...store.allTransactionTagGroups]);
const groupTags = computed(() => store.allTransactionTagsByGroupMap[groupId.value] || []);
const isRoot = (tag: TransactionTag) => !tag.parentId || tag.parentId === '0';
const allTags = computed(() => Object.values(store.allTransactionTagsMap).sort((a, b) => a.displayOrder - b.displayOrder));
const parents = computed(() => allTags.value.filter(isRoot));
function groupName(id: string) { return groups.value.find(item => item.id === id)?.name || '默认分组'; }
function hasChildren(id: string) { return !!id && allTags.value.some(tag => tag.parentId === id); }
function matches(tag: TransactionTag) { return (showHidden.value || !tag.hidden) && tag.name.toLocaleLowerCase().includes(query.value.trim().toLocaleLowerCase()); }
const rows = computed(() => {
    const roots = new Map<string, TransactionTag>();
    for (const tag of groupTags.value) {
        const parent = !isRoot(tag) ? store.allTransactionTagsMap[tag.parentId] : tag;
        if (parent) roots.set(parent.id, parent);
    }
    return [...roots.values()].sort((a, b) => a.displayOrder - b.displayOrder).map(parent => {
        const parentMatches = parent.groupId === groupId.value && matches(parent);
        const children = allTags.value.filter(tag => tag.parentId === parent.id &&
            (tag.groupId === groupId.value || parent.groupId === groupId.value) &&
            (showHidden.value || !tag.hidden) && (parentMatches || matches(tag)));
        return { parent, children, visible: parentMatches || children.length > 0 };
    }).filter(row => row.visible);
});
const orderItems = computed(() => groupTags.value.map(tag => ({ id: tag.id, hidden: tag.hidden, name: isRoot(tag) ? tag.name : `${store.allTransactionTagsMap[tag.parentId]?.name || '一级标签'} / ${tag.name}` })));
async function load() {
    if (busy.value) return;
    error.value = '';
    try { await store.loadAllTags({ force: false }); if (!groups.value.some(group => group.id === groupId.value)) groupId.value = '0'; }
    catch (cause) { error.value = errorText(cause); }
    finally { loading.value = false; }
}
function create(parent?: TransactionTag) {
    draft.value = TransactionTag.createNewTag('', parent?.groupId || groupId.value);
    draft.value.parentId = parent?.id || '0';
    editError.value = ''; editorOpen.value = true;
}
function edit(tag: TransactionTag) { draft.value = tag.clone(); editError.value = ''; editorOpen.value = true; }
async function save() {
    if (busy.value || !draft.value.name.trim()) return;
    busy.value = true; editError.value = ''; draft.value.name = draft.value.name.trim();
    try { await store.saveTag({ tag: draft.value }); groupId.value = draft.value.groupId; query.value = ''; editorOpen.value = false; }
    catch (cause) { editError.value = errorText(cause); }
    finally { busy.value = false; }
}
async function toggleHidden() {
    const tag = selected.value; if (!tag || busy.value) return;
    busy.value = true; error.value = '';
    try { await store.hideTag({ tag, hidden: !tag.hidden }); }
    catch (cause) { error.value = errorText(cause); }
    finally { busy.value = false; }
}
function confirmDelete() {
    const tag = selected.value; if (!tag || busy.value) return;
    showConfirm(`确定删除「${tag.name}」？被账单使用或含子标签的标签不能直接删除，可选择隐藏。`, () => { void remove(tag); });
}
async function remove(tag: TransactionTag) {
    if (busy.value) return; busy.value = true; error.value = '';
    try { await store.deleteTag({ tag }); showToast('标签已删除'); }
    catch (cause) { error.value = errorText(cause); }
    finally { busy.value = false; }
}
function viewBills() {
    const tag = selected.value; if (!tag) return;
    const ids = [tag.id, ...allTags.value.filter(child => child.parentId === tag.id).map(child => child.id)];
    f7router.navigate('/transaction/list?' + new URLSearchParams({ tagIds: ids.join(',') }));
}
function openOrder() { if (groupTags.value.length < 2) return; orderError.value = ''; orderOpen.value = true; }
async function saveOrder(ids: string[]) {
    if (busy.value) return; busy.value = true; orderError.value = '';
    try {
        const response = await services.moveTransactionTag({ newDisplayOrders: ids.map((id, index) => ({ id, displayOrder: index + 1 })) });
        if (!response.data.success || !response.data.result) throw new Error('标签顺序保存失败，请重试');
        store.updateTransactionTagListInvalidState(true); await store.loadAllTags({ force: false }); orderOpen.value = false;
    } catch (cause) { orderError.value = errorText(cause); }
    finally { busy.value = false; }
}
void load();
</script>
<style scoped src="@/styles/mobile/management.css"></style>
<style scoped>
.tag-children{display:flex;flex-wrap:wrap;gap:10px 12px;padding:0 17px 18px 42px}
.tag-children button{display:flex;align-items:center;gap:6px;min-height:34px;border:0;background:var(--cy-soft);color:var(--cy-accent);border-radius:7px;font-size:13px;padding:7px 11px;max-width:100%;overflow-wrap:anywhere;text-align:left}
</style>
