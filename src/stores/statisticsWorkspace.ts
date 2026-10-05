import { ref, watch } from 'vue';
import { defineStore } from 'pinia';
import { useUserStore } from '@/stores/user.ts';
import { defaultStatisticsPreferences, statisticsWorkspace, type StatisticsPreferences, type StatisticsBudget, type StatisticsNote, type StatisticsAuxiliary } from '@/lib/statistics-workspace.ts';

export const useStatisticsWorkspaceStore = defineStore('statisticsWorkspace', () => {
    const user = useUserStore();
    const preferences = ref(defaultStatisticsPreferences());
    const budgets = ref<StatisticsBudget[]>([]), notes = ref<StatisticsNote[]>([]);
    const auxiliary = ref<StatisticsAuxiliary>({ debtActions: {}, feeIds: [] });
    const loaded = ref(false);
    let generation = 0, pending: Promise<void> | undefined;
    watch(() => user.currentUserBasicInfo?.username, () => {
        generation++; pending = undefined; loaded.value = false;
        budgets.value = []; notes.value = []; preferences.value = defaultStatisticsPreferences(); auxiliary.value = { debtActions: {}, feeIds: [] };
    }, { flush: 'sync' });
    async function load(force = false): Promise<void> {
        if (pending) return pending;
        if (loaded.value && !force) return;
        const version = generation;
        pending = Promise.all([statisticsWorkspace.preferences(), statisticsWorkspace.budgets(), statisticsWorkspace.notes(), statisticsWorkspace.auxiliary()]).then(([p, b, n, a]) => {
            if (version !== generation) return;
            preferences.value = p; budgets.value = b; notes.value = n; auxiliary.value = a; loaded.value = true;
        }).finally(() => { if (version === generation) pending = undefined; });
        return pending;
    }
    async function savePreferences(input: StatisticsPreferences): Promise<void> {
        const version = generation, result = await statisticsWorkspace.savePreferences(input);
        if (version === generation) preferences.value = result;
    }
    async function saveBudget(input: StatisticsBudget): Promise<void> {
        const version = generation, result = await statisticsWorkspace.saveBudget(input);
        if (version === generation) budgets.value = [...budgets.value.filter(b => b.id !== result.id), result];
    }
    async function deleteBudget(input: StatisticsBudget): Promise<void> {
        const version = generation; await statisticsWorkspace.deleteBudget(input);
        if (version === generation) budgets.value = budgets.value.filter(b => b.id !== input.id);
    }
    async function saveNote(input: StatisticsNote): Promise<void> {
        const version = generation, result = await statisticsWorkspace.saveNote(input);
        if (version === generation) notes.value = [...notes.value.filter(n => n.id !== result.id), result];
    }
    return { preferences, budgets, notes, auxiliary, loaded, load, savePreferences, saveBudget, deleteBudget, saveNote };
});
