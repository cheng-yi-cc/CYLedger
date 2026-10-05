import { computed, onScopeDispose, ref, watch } from 'vue';
import { defineStore } from 'pinia';
import moment from 'moment-timezone';
import { useUserStore } from '@/stores/user.ts';

export const useLedgerScopeStore = defineStore('ledgerScope', () => {
    const users = useUserStore();
    const now = ref(Date.now());
    const timeZone = ref(moment.tz.guess(true));
    const currentDay = computed(() => moment(now.value).tz(timeZone.value).format('YYYY-MM-DD'));
    function syncClock(): void {
        timeZone.value = moment.tz.guess(true);
        now.value = Date.now();
    }
    const clockTimer = window.setInterval(syncClock, 1000);
    window.addEventListener('focus', syncClock);
    document.addEventListener('visibilitychange', syncClock);
    onScopeDispose(() => {
        window.clearInterval(clockTimer);
        window.removeEventListener('focus', syncClock);
        document.removeEventListener('visibilitychange', syncClock);
    });
    const month = ref(moment().tz(timeZone.value).format('YYYY-MM'));
    const selectedDay = ref(moment().tz(timeZone.value).format('YYYY-MM-DD'));
    const accountId = ref(''), categoryId = ref(''), tagId = ref(''), type = ref(0);
    const filterKey = computed(() => [accountId.value,categoryId.value,tagId.value,type.value,timeZone.value].join('|'));
    function resetFilters(): void { accountId.value = ''; categoryId.value = ''; tagId.value = ''; type.value = 0; }
    watch(() => users.currentUserBasicInfo?.username, () => {
        resetFilters(); month.value = moment().tz(timeZone.value).format('YYYY-MM'); selectedDay.value = moment().tz(timeZone.value).format('YYYY-MM-DD');
    });
    return { timeZone, currentDay, syncClock, month, selectedDay, accountId, categoryId, tagId, type, filterKey, resetFilters };
});
