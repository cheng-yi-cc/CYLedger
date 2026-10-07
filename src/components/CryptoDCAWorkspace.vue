<template>
    <div class="dca-workspace">
        <header class="dca-heading"><div><h2>每日定投</h2><p>固定投入，按计划积累</p></div><button :disabled="busy" @click="newPlan">＋ 添加计划</button></header>
        <p class="dca-hint">在这里设置与交易所一致的定投。按公开分钟参考价自动记账，实际成交与手续费可能不同，可在投资流水中修订。</p>
        <p v-if="error" class="dca-error" role="alert">{{ error }}</p>
        <p v-if="notice" role="status" class="dca-hint">{{ notice }}</p>
        <section v-if="editing" ref="editorElement" class="dca-card dca-editor">
            <h3>{{ draft.id ? '调整定投' : '新建定投' }}</h3>
            <form @submit.prevent="save">
                <label v-if="!accountId">交易所账户<select v-model="draft.accountId" required :disabled="!!draft.id"><option disabled value="">选择交易所</option><option v-for="account in exchanges" :key="account.id" :value="account.id">{{ account.name }}</option></select></label>
                <div class="dca-grid">
                    <label>买入币种<select v-model="draft.instrumentId" :disabled="!!draft.id"><option value="crypto:bitcoin">比特币 BTC</option><option value="crypto:ethereum">以太坊 ETH</option><option value="crypto:solana">Solana SOL</option></select></label>
                    <label>支付币种<select v-model="draft.paymentInstrumentId" :disabled="!!draft.id"><option value="crypto:tether">USDT</option><option value="crypto:usd-coin">USDC</option></select></label>
                    <label>每日投入（{{ symbol(draft.paymentInstrumentId) }}）<input v-model.trim="draft.amount" inputmode="decimal" maxlength="128" required placeholder="例如 10" /></label>
                    <label>每天时间<input v-model="draft.dailyTime" type="time" step="60" required /></label>
                    <label>开始日期<input v-model="draft.startDate" type="date" min="2015-01-01" required :disabled="!!draft.id" /></label>
                    <label>记录账本<select v-model="draft.bookId" required><option v-for="book in books.activeBooks" :key="book.id" :value="book.id">{{ book.name }}</option></select></label>
                </div>
                <p class="dca-hint">时区：{{ draft.timeZone }}。时间精确到分钟，行情公布后执行。余额不足未来 3 天用量会弹窗提醒；不够一次扣款时自动暂停。</p>
                <p v-if="draft.id" class="dca-hint">调整从下次计划时间生效，旧的待处理安排停止，已完成交易保留。更换买入或支付币种请另建计划。</p>
                <p v-else class="dca-hint">开始日期已过去时，会按历史行情补记；补记不能使历史持仓变成负数。请先录入账户实际持有的稳定币。</p>
                <div class="dca-actions"><button class="primary" :disabled="busy">{{ busy ? '正在保存…' : '保存并启用' }}</button><button type="button" :disabled="busy" @click="editing = false">取消</button></div>
            </form>
        </section>
        <p v-if="loading" class="dca-hint">正在读取定投…</p>
        <section v-for="alert in alerts" :key="alert.accountId + alert.paymentInstrumentId" class="dca-warning">
            <strong>{{ alert.paused ? '有计划因余额不足暂停' : '稳定币余额即将不足' }}</strong>
            <p>{{ alert.accountName }} · 余额 {{ alert.balance }} {{ symbol(alert.paymentInstrumentId) }}，未来 3 天需 {{ alert.required }}。</p>
        </section>
        <div v-if="!plans.length && !loading && !editing" class="dca-card dca-empty"><h3>让每天的买入有据可查</h3><p>为 BTC 和 ETH 分别添加计划，设置每日投入和时间。</p><button class="primary" @click="newPlan">添加第一条定投</button></div>
        <section v-for="plan in plans" :key="plan.id" class="dca-card">
            <div class="dca-heading"><div><h3>{{ symbol(plan.instrumentId) }} 每日定投</h3><p>{{ accountName(plan.accountId) }} · 每天 {{ plan.dailyTime }}</p></div><span class="dca-state" :class="{paused: !plan.enabled}">{{ plan.enabled ? '进行中' : '已暂停' }}</span></div>
            <p class="dca-amount">{{ plan.amount }} <span>{{ symbol(plan.paymentInstrumentId) }} / 天</span></p>
            <dl class="dca-grid dca-metrics"><div><dt>累计投入</dt><dd>{{ stats(plan).spent }} {{ symbol(plan.paymentInstrumentId) }}</dd></div><div><dt>累计买入</dt><dd>{{ stats(plan).acquired }} {{ symbol(plan.instrumentId) }}</dd></div><div><dt>累计买入成本</dt><dd>{{ money(stats(plan).cost) }}</dd></div><div><dt>有效记录</dt><dd>{{ stats(plan).count }} 笔</dd></div></dl>
            <p class="dca-hint">{{ plan.status }}<template v-if="plan.enabled"> · 下一次 {{ plan.nextDate }} {{ plan.dailyTime }}</template><br />{{ plan.timeZone }}</p>
            <details><summary>本账户 {{ symbol(plan.instrumentId) }} 的成本与收益</summary><dl class="dca-grid dca-metrics"><div><dt>当前持仓数量</dt><dd>{{ position(plan)?.quantity || '0' }}</dd></div><div><dt>剩余成本</dt><dd>{{ money(position(plan)?.cost) }}</dd></div><div><dt>平均单位成本</dt><dd>{{ money(position(plan)?.averageCost) }}</dd></div><div><dt>未实现盈亏</dt><dd>{{ money(position(plan)?.unrealizedPnl) }}</dd></div></dl><p class="dca-hint">与该账户已有同币种持仓合并计算；累计买入不代表当前剩余持仓。收益随参考行情变化，缺报价或成本时显示未知。</p></details>
            <div class="dca-actions"><button :disabled="busy" @click="editPlan(plan)">调整</button><button :disabled="busy" @click="askToggle(plan)">{{ plan.enabled ? '暂停定投' : '恢复定投' }}</button></div>
        </section>
        <Teleport to="body"><div v-if="toggleCandidate" class="dca-confirm-backdrop"><section class="dca-card dca-confirm" role="alertdialog" aria-modal="true" aria-labelledby="dca-toggle-title" @keydown.esc="!busy && (toggleCandidate = undefined)" @keydown.tab.prevent="cycleConfirmFocus">
            <h3 id="dca-toggle-title">{{ toggleCandidate.enabled ? '暂停这条定投？' : '恢复这条定投？' }}</h3>
            <p>{{ toggleCandidate.enabled ? '停止未完成安排，已完成的交易保留。' : '从下次计划时间继续，暂停期间不补买。请确认交易所中的定投也已恢复。' }}</p>
            <div class="dca-actions"><button ref="confirmButton" class="primary" :disabled="busy" @click="toggle">确认{{ toggleCandidate.enabled ? '暂停' : '恢复' }}</button><button ref="cancelButton" :disabled="busy" @click="toggleCandidate = undefined">取消</button></div>
        </section></div></Teleport>
        <section class="dca-card"><div class="dca-heading"><h3>最近执行记录</h3><button :disabled="busy" @click="sync">{{ busy ? '处理中…' : '检查并补记' }}</button></div><p class="dca-hint">应用运行时自动检查；重新打开会补记。缺少历史行情或汇率时保留待处理，暂停期间不补买。未设置交易所手续费，参考记账手续费为 0。</p><p v-if="!days.length" class="dca-hint">到计划时间后，这里显示执行结果。</p>
            <article v-for="day in days" :key="day.id" class="dca-day"><div><strong>{{ day.date }} · {{ symbol(day.instrumentId) }}</strong><span>{{ {pending:'待处理',done:'已入账',skipped:'未买入'}[day.status] }}</span></div><p>{{ day.amount }} {{ symbol(day.paymentInstrumentId) }} · {{ day.message }}</p><p v-if="eventFor(day.eventId)?.voided" class="dca-hint">对应投资流水已撤销，此日期不会再次自动买入。</p><details v-if="eventFor(day.eventId)?.dca"><summary>查看记账依据</summary><p>价格时间 {{ date(eventFor(day.eventId)!.dca!.priceTime) }} · {{ eventFor(day.eventId)!.dca!.source }}</p><p>支付币 {{ eventFor(day.eventId)!.dca!.paymentPrice }} USD / 枚；买入币 {{ eventFor(day.eventId)!.dca!.targetPrice }} USD / 枚</p><p>人民币汇率 {{ eventFor(day.eventId)!.dca!.fxRate }} · {{ eventFor(day.eventId)!.dca!.fxDate }} · {{ eventFor(day.eventId)!.dca!.fxSource }}</p><p v-if="eventFor(day.eventId)!.version > 1" class="dca-hint">交易已人工修订，以上保留最初自动记账依据。</p></details></article>
        </section>
    </div>
</template>
<script setup lang="ts">
import { computed, ref, nextTick, onMounted, onUnmounted, watch } from 'vue';
import moment from 'moment-timezone';
import { useBooksStore } from '@/stores/books.ts';
import { useLedgerScopeStore } from '@/stores/ledgerScope.ts';
import { investments, investmentError } from '@/lib/investments.ts';
import { LedgerDecimal } from '@/lib/ledger-display.ts';
import { generateRandomUUID } from '@/lib/misc.ts';
import { createValuationRefresh } from '@/lib/valuation-refresh.ts';
import { cryptoDCA, cryptoDCAState, dcaSymbols, type CryptoDCAPlan, type CryptoDCAInput } from '@/lib/crypto-dca.ts';
import type { InvestmentAccount, InvestmentEvent, WealthSummary } from '@/models/investment.ts';
const props = defineProps<{ accountId?: string }>();
const emit = defineEmits<{ changed: [] }>();
const books = useBooksStore(), ledgerScope = useLedgerScopeStore();
const accounts = ref<InvestmentAccount[]>([]), events = ref<InvestmentEvent[]>([]), wealth = ref<WealthSummary>();
const valuationRefresh=createValuationRefresh(value=>{wealth.value=value;});
const error = ref(''), notice = ref(''), busy = ref(false), loading = ref(false), editing = ref(false), toggleCandidate = ref<CryptoDCAPlan>();
function fresh(): CryptoDCAInput { return { id:'', revision:0, requestKey:generateRandomUUID(), accountId:props.accountId || accounts.value.find(a=>a.kind==='EXCHANGE')?.id || '', instrumentId:'crypto:bitcoin', paymentInstrumentId:'crypto:tether', amount:'', dailyTime:'09:00', startDate:moment().tz(ledgerScope.timeZone).format('YYYY-MM-DD'), timeZone:ledgerScope.timeZone, bookId:books.defaultBookId }; }
const draft = ref<CryptoDCAInput>(fresh());
const editorElement=ref<HTMLElement>(),confirmButton=ref<HTMLButtonElement>(),cancelButton=ref<HTMLButtonElement>();
let toggleTrigger: HTMLElement | undefined;
function askToggle(plan:CryptoDCAPlan):void { toggleTrigger=document.activeElement instanceof HTMLElement ? document.activeElement : undefined; toggleCandidate.value=plan; }
function cycleConfirmFocus():void { (document.activeElement===confirmButton.value ? cancelButton.value : confirmButton.value)?.focus(); }
watch(toggleCandidate,value=>{void nextTick(()=>{if(value)confirmButton.value?.focus();else toggleTrigger?.focus();});});
function showEditor():void { editing.value=true; error.value=''; void nextTick(()=>editorElement.value?.scrollIntoView({block:'start',behavior:'smooth'})); }
const exchanges = computed(()=>accounts.value.filter(a=>a.kind==='EXCHANGE'));
const plans = computed(()=>(cryptoDCAState.value?.plans || []).filter(p=>!props.accountId || p.accountId===props.accountId));
const days = computed(()=>(cryptoDCAState.value?.days || []).filter(d=>(!props.accountId || d.accountId===props.accountId) && plans.value.some(p=>p.id===d.planId)).slice(0,60));
const alerts = computed(()=>(cryptoDCAState.value?.alerts || []).filter(a=>!props.accountId || a.accountId===props.accountId));
function symbol(id:string):string { return dcaSymbols[id] || id; }
function accountName(id:string):string { return accounts.value.find(a=>a.id===id)?.name || ''; }
function date(at:number):string { return moment.unix(at).tz(ledgerScope.timeZone).format('YYYY-MM-DD HH:mm'); }
function money(value:string|null|undefined):string { return value===null || value===undefined ? '未知' : `¥ ${new LedgerDecimal(value).toFixed(2)}`; }
function position(plan:CryptoDCAPlan) { return wealth.value?.positions.find(p=>p.accountId===plan.accountId && p.instrumentId===plan.instrumentId); }
function eventFor(id:string) { return events.value.find(e=>e.id===id); }
function stats(plan:CryptoDCAPlan) {
    let spent = new LedgerDecimal(0), acquired = new LedgerDecimal(0), cost = new LedgerDecimal(0), known = true, count = 0;
    for (const event of events.value.filter(e=>e.dca?.planId===plan.id && !e.voided && e.instrumentId===plan.instrumentId && e.settlementInstrumentId===plan.paymentInstrumentId)) {
        count++; spent=spent.plus(event.amount).plus(event.fee || '0'); acquired=acquired.plus(event.quantity);
        if (event.exchangeRate) cost=cost.plus(new LedgerDecimal(event.amount).plus(event.fee || '0').times(event.exchangeRate)); else known=false;
    }
    return { spent:spent.toFixed(), acquired:acquired.toFixed(), cost:known ? cost.toFixed() : null, count };
}
function newPlan():void { draft.value=fresh(); showEditor(); }
function editPlan(plan:CryptoDCAPlan):void { draft.value={...plan, requestKey:generateRandomUUID()}; showEditor(); }
async function loadFacts():Promise<void> { const [a,e,s]=await Promise.all([investments.accounts(),investments.events(),investments.summary()]); accounts.value=a; events.value=e; wealth.value=s; }
async function load():Promise<void> { loading.value=true; try { await Promise.all([cryptoDCA.list(),loadFacts(),books.loadBooks()]); } catch(e) { error.value=investmentError(e); } finally { loading.value=false; } }
async function save():Promise<void> {
    if(busy.value) return; error.value=''; busy.value=true;
    try {
        if (!/^(?:0|[1-9]\d*)(?:\.\d{1,18})?$/.test(draft.value.amount) || !new LedgerDecimal(draft.value.amount).gt(0)) throw Error('请输入大于零的普通十进制数量，最多 18 位小数');
        await cryptoDCA.save(draft.value); editing.value=false; notice.value='定投已保存并启用。'; await cryptoDCA.sync(true); await load(); emit('changed');
    } catch(e) { error.value=investmentError(e); } finally { busy.value=false; }
}
async function toggle():Promise<void> {
    if(!toggleCandidate.value || busy.value) return; busy.value=true; error.value='';
    try { await cryptoDCA.enabled(toggleCandidate.value,!toggleCandidate.value.enabled); toggleCandidate.value=undefined; await load(); emit('changed'); } catch(e) { error.value=investmentError(e); } finally { busy.value=false; }
}
async function sync():Promise<void> {
    if(busy.value) return; busy.value=true; error.value='';
    try { const result=await cryptoDCA.sync(true); notice.value=result.created ? `已补记 ${result.created} 笔定投。` : '已检查；没有新增买入。未完成安排的原因见执行记录。'; await loadFacts(); emit('changed'); } catch(e) { error.value=investmentError(e); } finally { busy.value=false; }
}
watch(()=>cryptoDCAState.value, state=>{ if(state?.created) void loadFacts().catch(e=>{error.value=investmentError(e);}); });
onMounted(()=>{void load();valuationRefresh.start();});
onUnmounted(valuationRefresh.stop);
</script>
<style scoped>
.dca-editor{scroll-margin-top:76px}
.dca-workspace{color:var(--cy-text,#252e31);max-width:780px;margin:auto}.dca-heading{display:flex;justify-content:space-between;align-items:center;gap:14px}.dca-heading h2{font-size:23px;margin:0}.dca-heading h3,.dca-card h3{font-size:17px;margin:0 0 8px}.dca-heading p{margin:4px 0 0;font-size:12px;color:var(--cy-muted,#6b757c)}.dca-hint{font-size:12px;color:var(--cy-muted,#6b757c);line-height:1.8}.dca-card{padding:21px;margin:18px 0;background:var(--cy-card,#fff);border:1px solid var(--cy-line,#e5e9e7);border-radius:16px}.dca-state{font-size:11px;padding:6px 10px;border-radius:20px;color:var(--cy-accent,#387f79);background:var(--cy-soft,#edf6f2);white-space:nowrap}.dca-state.paused{background:#f4eee2;color:#7f6032}.dca-amount{font-size:30px;font-weight:650;margin:23px 0}.dca-amount span{font-size:12px;font-weight:400;color:var(--cy-muted,#6b757c)}.dca-grid{display:grid;grid-template-columns:1fr 1fr;gap:16px}.dca-metrics{margin:20px 0}.dca-metrics dt{font-size:11px;color:var(--cy-muted,#6b757c);margin-bottom:7px}.dca-metrics dd{font-size:14px;margin:0;overflow-wrap:anywhere}.dca-workspace button{font:inherit;font-size:13px;padding:10px 14px;border:1px solid var(--cy-line,#d7dfdc);border-radius:10px;background:var(--cy-card,#fff);color:var(--cy-accent,#387f79);cursor:pointer}.dca-workspace button:disabled{opacity:.5;cursor:wait}.dca-workspace button.primary{background:var(--cy-accent,#387f79);border-color:transparent;color:#fff}.dca-actions{display:flex;gap:10px;margin-top:18px}.dca-editor label{display:grid;gap:8px;font-size:12px;margin:10px 0}.dca-editor input,.dca-editor select{box-sizing:border-box;width:100%;min-width:0;padding:12px;border:1px solid var(--cy-line,#d7dfdc);border-radius:8px;background:var(--cy-card,#fff);color:inherit;font:inherit;font-size:15px}.dca-editor input:disabled,.dca-editor select:disabled{opacity:.65}.dca-error,.dca-warning{padding:14px;border-radius:12px;background:#fff5df;color:#805816;font-size:13px;line-height:1.8}.dca-warning p{margin:8px 0 0}.dca-empty{padding:30px 22px;text-align:center}.dca-empty p{font-size:13px;color:var(--cy-muted,#6b757c);line-height:1.9}.dca-workspace details{margin:15px 0;font-size:12px;line-height:1.8;overflow-wrap:anywhere}.dca-workspace summary{cursor:pointer;color:var(--cy-accent,#387f79)}.dca-day{padding:17px 0;border-bottom:1px solid var(--cy-line,#e5e9e7)}.dca-day:last-child{border:0}.dca-day>div{display:flex;justify-content:space-between;gap:8px;font-size:13px}.dca-day>div>span{color:var(--cy-muted,#6b757c);white-space:nowrap}.dca-day>p{font-size:12px;line-height:1.7;overflow-wrap:anywhere}.dca-workspace :focus-visible{outline:2px solid var(--cy-accent,#387f79);outline-offset:3px}@media(max-width:360px){.dca-grid{grid-template-columns:1fr}.dca-card{padding:16px}}
.dca-confirm-backdrop{position:fixed;inset:0;z-index:14000;display:grid;place-items:center;padding:24px;background:#0007;color:var(--cy-text,#252e31)}.dca-confirm{width:min(100%,420px);box-sizing:border-box;box-shadow:0 20px 70px #0003}.dca-confirm p{font-size:14px;line-height:1.8}.dca-confirm button{font:inherit;font-size:13px;padding:11px 16px;border:1px solid var(--cy-line,#d7dfdc);border-radius:10px;background:var(--cy-card,#fff);color:var(--cy-accent,#387f79);cursor:pointer}.dca-confirm button.primary{background:var(--cy-accent,#387f79);color:#fff}.dca-confirm button:disabled{opacity:.5}
</style>
