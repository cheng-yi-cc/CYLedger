<template>
    <div v-if="visible.length" class="dca-notice-backdrop">
        <section role="alertdialog" aria-modal="true" aria-labelledby="dca-notice-title" class="dca-notice" @keydown.esc="dismiss" @keydown.tab.prevent="dismissButton?.focus()">
            <h2 id="dca-notice-title">定投余额提醒</h2>
            <div v-for="alert in visible" :key="alert.accountId + alert.paymentInstrumentId">
                <strong>{{ alert.accountName }} · {{ dcaSymbols[alert.paymentInstrumentId] }}</strong>
                <p v-if="alert.paused">有定投因余额不足已自动暂停。补足后，请到该交易所账户的“每日定投”手动恢复，暂停期间不补买。</p>
                <p v-else>余额不足以覆盖未来 3 天的定投，请及时补充稳定币。</p>
                <p>当前 {{ alert.balance }} · 未来 3 天需要 {{ alert.required }} {{ dcaSymbols[alert.paymentInstrumentId] }}</p>
            </div>
            <button ref="dismissButton" type="button" autofocus @click="dismiss">知道了</button>
        </section>
    </div>
</template>
<script setup lang="ts">
import { ref, watch, nextTick, onMounted, onUnmounted } from 'vue';
import moment from 'moment-timezone';
import { cryptoDCAState, dcaSymbols, syncCryptoDCAOnOpen, type CryptoDCAAlert } from '@/lib/crypto-dca.ts';
import { getCurrentUserInfo, isUserUnlocked } from '@/lib/userstate.ts';
import { useUserStore } from '@/stores/user.ts';
const users=useUserStore();
const visible = ref<CryptoDCAAlert[]>([]);
const dismissButton=ref<HTMLButtonElement>();
let previousFocus: HTMLElement | undefined;
const seen = new Set<string>();
let timer: ReturnType<typeof setInterval> | undefined;
function key(alert: CryptoDCAAlert): string {
    return `cy-dca-reminder:${getCurrentUserInfo()?.username || ''}:${alert.accountId}:${alert.paymentInstrumentId}:${alert.paused ? 'paused' : 'low'}:${moment().format('YYYY-MM-DD')}`;
}
function wasSeen(alert: CryptoDCAAlert): boolean {
    if (seen.has(key(alert))) return true;
    try { return sessionStorage.getItem(key(alert)) === '1'; } catch { return false; }
}
function dismiss(): void {
    for (const alert of visible.value) { seen.add(key(alert)); try { sessionStorage.setItem(key(alert), '1'); } catch { /* Private browsing may disable storage. */ } }
    visible.value = [];
    previousFocus?.focus();
}
watch(cryptoDCAState, state => {
    if (!isUserUnlocked()) { visible.value = []; return; }
    visible.value = (state?.alerts || []).filter(alert => !wasSeen(alert));
    if (visible.value.length) { if (document.activeElement instanceof HTMLElement && document.activeElement !== dismissButton.value) previousFocus=document.activeElement; void nextTick(()=>dismissButton.value?.focus()); }
});
watch(()=>users.currentUserBasicInfo?.username,()=>{ visible.value=[]; cryptoDCAState.value=undefined; tick(); });
function tick(): void { if (!isUserUnlocked()) visible.value = []; void syncCryptoDCAOnOpen(); }
onMounted(() => { tick(); timer = setInterval(tick, 60000); document.addEventListener('visibilitychange', tick); window.addEventListener('online', tick); });
onUnmounted(() => { if (timer) clearInterval(timer); document.removeEventListener('visibilitychange', tick); window.removeEventListener('online', tick); });
</script>
<style scoped>
.dca-notice-backdrop{position:fixed;inset:0;z-index:15000;display:grid;place-items:center;padding:24px;background:#0007;color:var(--cy-text,#242a30)}.dca-notice{width:min(100%,440px);max-height:80vh;overflow:auto;padding:24px;border-radius:20px;background:var(--cy-card,#fff);box-shadow:0 20px 70px #0003}.dca-notice h2{font-size:20px;margin:0 0 20px}.dca-notice p{font-size:14px;line-height:1.8;overflow-wrap:anywhere}.dca-notice button{width:100%;padding:13px;border:0;border-radius:10px;background:var(--cy-accent,#35776f);color:white;font:inherit;cursor:pointer;margin-top:12px}
</style>
