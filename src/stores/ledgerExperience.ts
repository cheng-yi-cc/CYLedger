import { ref, watch } from 'vue';
import { defineStore } from 'pinia';
import { defaultLedgerPreferences, parseLedgerPreferences, type LedgerPreferences } from '@/lib/ledger-preferences.ts';
import { useUserStore } from '@/stores/user.ts';

export const useLedgerExperienceStore = defineStore('ledgerExperience', () => {
    const users = useUserStore();
    const preferences = ref<LedgerPreferences>(defaultLedgerPreferences());
    const history = ref<string[]>([]);
    const lastAccountId = ref('');
    let loading = false;
    const key = () => `cy_ledger_experience_${users.currentUserBasicInfo?.username || 'local'}`;
    function load() {
        loading = true;
        try {
            const data = JSON.parse(localStorage.getItem(key()) || '{}');
            preferences.value = parseLedgerPreferences(data.preferences);
            history.value = Array.isArray(data.history) ? data.history.filter((item: unknown): item is string => typeof item === 'string' && item.length <= 255).slice(0, 30) : [];
            lastAccountId.value = typeof data.lastAccountId === 'string' ? data.lastAccountId : '';
        } catch { preferences.value = defaultLedgerPreferences(); history.value = []; lastAccountId.value = ''; }
        loading = false;
    }
    watch(() => users.currentUserBasicInfo?.username, load, { immediate: true, flush: 'sync' });
    watch([preferences, history, lastAccountId], () => {
        if (!loading) localStorage.setItem(key(), JSON.stringify({ preferences: preferences.value, history: history.value, lastAccountId: lastAccountId.value }));
    }, { deep: true, flush: 'sync' });
    function set<K extends keyof LedgerPreferences>(name: K, value: LedgerPreferences[K]) { preferences.value = parseLedgerPreferences({ ...preferences.value, [name]: value }); }
    function rememberSearch(value: string) { const query = value.trim().slice(0, 255); if (query) history.value = [query, ...history.value.filter(item => item !== query)].slice(0, 30); }
    function forgetSearch(value?: string) { history.value = value === undefined ? [] : history.value.filter(item => item !== value); }
    function rememberAccount(categoryId: string, accountId: string) {
        lastAccountId.value = accountId;
        if (categoryId && accountId) set('categoryAccounts', { ...preferences.value.categoryAccounts, [categoryId]: accountId });
    }
    function vibrate() { if (preferences.value.vibration) navigator.vibrate?.(12); }
    return { preferences, history, lastAccountId, set, rememberSearch, forgetSearch, rememberAccount, vibrate, load };
});
