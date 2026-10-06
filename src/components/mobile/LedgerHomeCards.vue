<template><section class="cy-home-cards" aria-label="数据小卡片"><f7-link v-for="card in cards" :key="card.id" :href="card.href" class="cy-data-card" :style="{gridColumn:`span ${card.width}`,color:card.color||undefined,'--card-background':card.background||'var(--cy-card)','--card-image':card.image?`url(${card.image})`:'none','--card-opacity':card.opacity??1}"><header>{{ card.title }}<small>{{ card.caption }}</small></header><strong v-if="card.value!==undefined" class="cy-card-value">{{ card.value }}</strong><small v-if="card.note" class="cy-card-note">{{ card.note }}</small>
    <div v-if="card.cells" class="cy-card-heatmap"><i v-for="cell in card.cells" :key="cell.label" :title="`${cell.label} · ${cell.amount}`" :style="{opacity:cell.opacity}" /></div>
    <svg v-if="card.points" viewBox="0 0 240 64" role="img" :aria-label="card.title" class="cy-card-line"><polyline :points="card.points" fill="none" stroke="currentColor" stroke-width="2" /><line x1="0" y1="62" x2="240" y2="62" stroke="currentColor" opacity=".12" /></svg>
    <svg v-if="card.flow" viewBox="0 0 300 160" class="cy-card-flow" role="img" aria-label="收入流向支出与结余"><path v-for="flow in card.flow.paths" :key="flow.d" :d="flow.d" :stroke="flow.color" :stroke-width="flow.width" fill="none" opacity=".28"/><g v-for="node in card.flow.nodes" :key="node.label"><rect :x="node.x" :y="node.y" width="6" :height="node.height" :fill="node.color"/><text :x="node.x===0?10:290" :y="node.y+node.height/2+3" :text-anchor="node.x===0?'start':'end'" fill="currentColor">{{node.label}}</text></g></svg>
    <div v-for="row in card.rows" :key="row.label" class="cy-card-row"><div><span>{{ row.label }}</span><b>{{ row.value }}</b></div><i v-if="row.width!==undefined" :style="{width:`${row.width}%`}" /></div>
</f7-link></section></template>
<script setup lang="ts">
import {computed,onMounted,ref,watch} from 'vue';
import moment from 'moment-timezone';
import {LedgerDecimal,ledgerMoney,ledgerTotals} from '@/lib/ledger-display.ts';
import {useMobileLedger,type LedgerEntry} from '@/lib/mobile-ledger.ts';
import {useLedgerExperienceStore} from '@/stores/ledgerExperience.ts';
import {useStatisticsWorkspaceStore} from '@/stores/statisticsWorkspace.ts';
import {useLedgerScopeStore} from '@/stores/ledgerScope.ts';
import {useBooksStore} from '@/stores/books.ts';
import {useAccountsStore} from '@/stores/account.ts';
import {useTransactionTagsStore} from '@/stores/transactionTag.ts';
import {budgetReports,groupReport,reportBreakdown,reportBuckets} from '@/lib/statistics-report.ts';
import {ledgerCardNames,ledgerMonthRange,type LedgerCard} from '@/lib/ledger-preferences.ts';
import {investments} from '@/lib/investments.ts';
import type {WealthSummary,WealthSnapshot,InvestmentQuote,Instrument} from '@/models/investment.ts';
import {ledgerWorkspace,wishProgress,type LedgerWorkspaceItem,type LedgerWish} from '@/lib/ledger-workspace.ts';
import {assetTools,mergeCreditDue,mergeDebtDue} from '@/lib/asset-tools.ts';
import {creditAccounts} from '@/lib/credit-accounts.ts';
import {debtAccounts} from '@/lib/debt-accounts.ts';
import {reimbursements,type ReimbursementClaim} from '@/lib/reimbursements.ts';
import type {CalendarEvent} from '@/lib/calendar-events.ts';
const props=defineProps<{entries:LedgerEntry[];month:string}>(),experience=useLedgerExperienceStore(),workspace=useStatisticsWorkspaceStore(),scope=useLedgerScopeStore(),books=useBooksStore(),accounts=useAccountsStore(),tags=useTransactionTagsStore();
const claims=ref<ReimbursementClaim[]>([]);
const summary=ref<WealthSummary>(),history=ref<WealthSnapshot[]>([]),wishes=ref<LedgerWorkspaceItem<LedgerWish>[]>([]),due=ref<CalendarEvent[]>([]),quotes=ref<InvestmentQuote[]>([]),instruments=ref<Instrument[]>([]),failures=ref<string[]>([]);
const completeLedger=useMobileLedger({bookIds:()=>[],applyFilters:()=>false});
let generation=0;
async function load(){if(!experience.preferences.cards.length)return;const gen=++generation,errors:string[]=[];await completeLedger.load(0,moment().add(100,'year').unix());if(completeLedger.error.value)errors.push('entries');await Promise.all([
    ['reimbursement',async()=>{claims.value=await reimbursements.list();}],['asset',async()=>{summary.value=await investments.summary();}],['assetTrend',async()=>{history.value=await investments.history();}],['wish',async()=>{wishes.value=await ledgerWorkspace.wishes();}],
    ['repayment',async()=>{const [events,credits,debts]=await Promise.all([assetTools.pending(),creditAccounts.reports(scope.timeZone),debtAccounts.reports()]);due.value=mergeDebtDue(mergeCreditDue(events,credits),debts,accounts.allAccountsMap);}],
    ['gold',async()=>{[quotes.value,instruments.value]=await Promise.all([investments.quotes(),investments.instruments()]);}]
].map(async([kind,action])=>{const needed=experience.preferences.cards.some(c=>c.kind===kind||(kind==='asset'&&['credit','debt'].includes(c.kind)));if(needed)try{await (action as ()=>Promise<void>)();}catch{errors.push(String(kind));}}));if(gen===generation)failures.value=errors;}
onMounted(load);watch(()=>[props.entries,experience.preferences.cards.map(c=>c.kind).join(',')],()=>void load());
interface Row{label:string;value:string;width?:number}
interface DisplayCard{ id:string;width:number;color:string;background:string;image?:string;opacity?:number;title:string;caption:string;href:string;value?:string;note?:string;rows:Row[];points?:string;flow?:ReturnType<typeof makeFlow>;cells?:{label:string;amount:string;opacity:number}[] }
function money(value:string|null|undefined){return value==null?'待换算':ledgerMoney(value,false);}
function spark(values:(string|null)[]){if(values.length<2||values.some(x=>x===null))return '';const nums=values.map(x=>new LedgerDecimal(x!)),min=LedgerDecimal.min(...nums),range=LedgerDecimal.max(...nums).minus(min);return nums.map((n,i)=>`${(i*240/(nums.length-1)).toFixed(2)},${range.isZero()?32:60-n.minus(min).div(range).mul(56).toNumber()}`).join(' ');}
function makeFlow(entries:LedgerEntry[],income:string,expense:string){
    const inflow=new LedgerDecimal(income),outflow=new LedgerDecimal(expense),max=LedgerDecimal.max(inflow,outflow);
    if(inflow.lt(0)||outflow.lt(0)||!max.gt(0))return undefined;
    const rows=(type:number)=>{const groups=new Map<string,InstanceType<typeof LedgerDecimal>>();for(const entry of entries)if(entry.type===type&&!entry.excludeFromStatistics&&entry.cny!==null)groups.set(entry.primaryCategory,(groups.get(entry.primaryCategory)||new LedgerDecimal(0)).plus(entry.cny));return [...groups].filter(([,value])=>value.gt(0)).sort((a,b)=>b[1].cmp(a[1])).map(([name,value])=>({name,value}));};
    const compact=(values:ReturnType<typeof rows>)=>values.length<=4?values:[...values.slice(0,3),{name:'其他',value:values.slice(3).reduce((sum,row)=>sum.plus(row.value),new LedgerDecimal(0))}];
    const left=compact(rows(2)),right=compact(rows(3));if(outflow.gt(inflow))left.push({name:'结余补足',value:outflow.minus(inflow)});if(inflow.gt(outflow))right.push({name:'结余',value:inflow.minus(outflow)});
    // Refunds can make category gross totals exceed net totals; avoid a misleading graph.
    if(!left.reduce((s,r)=>s.plus(r.value),new LedgerDecimal(0)).eq(max)||!right.reduce((s,r)=>s.plus(r.value),new LedgerDecimal(0)).eq(max))return undefined;
    const nodes:{x:number;y:number;height:number;label:string;color:string}[]=[],paths:{d:string;width:number;color:string}[]=[];let ly=8,ry=8;
    const leftNodes=left.map(row=>{const height=row.value.div(max).mul(96).toNumber();const node={x:0,y:ly,height,label:row.name,color:'#269785'};ly+=height+14;nodes.push(node);return node;});
    const rightNodes=right.map(row=>{const height=row.value.div(max).mul(96).toNumber();const node={x:294,y:ry,height,label:row.name,color:row.name==='结余'?'#269785':'#e65757'};ry+=height+14;nodes.push(node);return node;});
    const rightOffset=rightNodes.map(()=>0);for(const leftNode of leftNodes){let offset=0;for(let i=0;i<rightNodes.length;i++){const rightNode=rightNodes[i]!,width=leftNode.height*rightNode.height/96,y1=leftNode.y+offset+width/2,y2=rightNode.y+rightOffset[i]!+width/2;paths.push({d:`M6 ${y1} C120 ${y1},180 ${y2},294 ${y2}`,width,color:rightNode.color});offset+=width;rightOffset[i]!+=width;}}return{nodes,paths};
}
function build(card:LedgerCard):DisplayCard{
    const now=moment().tz(scope.timeZone),range=ledgerMonthRange(props.month,scope.timeZone,experience.preferences.monthStart);let from=range.start,to=range.end;
    if(card.period==='today'){from=now.clone().startOf('day');to=now.clone().endOf('day');}else if(card.period==='week'||card.kind==='week'){from=now.clone().startOf('isoWeek');to=from.clone().endOf('isoWeek');}else if(card.period==='year'){from=now.clone().startOf('year');to=now.clone().endOf('year');}else if(card.period==='all'){from=moment.tz('1900-01-01',scope.timeZone);to=now.clone().add(100,'year');}
    const cardBooks=card.bookIds.length?card.bookIds:books.selectedBookIds;
    const filtered=completeLedger.entries.value.filter(e=>e.time>=from.unix()&&e.time<=to.unix()&&(!cardBooks.length||cardBooks.includes(e.bookId))&&(!card.accountIds.length||card.accountIds.includes(e.accountId)||card.accountIds.includes(e.destinationAccountId))&&(!card.categoryIds.length||card.categoryIds.includes(e.categoryId)||card.categoryIds.includes(e.primaryCategoryId))&&(!card.tagIds.length||e.tagIds.some(id=>card.tagIds.includes(id))));
    const total=ledgerTotals(filtered),view:DisplayCard={...card,title:card.title||ledgerCardNames[card.kind],caption:card.period==='all'?'全部':from.format('M.D')+'—'+to.format('M.D'),rows:[],href:`/transaction/list?minTime=${from.unix()}&maxTime=${to.unix()}&bookIds=${card.bookIds.join(',')}&accountIds=${card.accountIds.join(',')}`};
    if(failures.value.includes('entries')||failures.value.includes(card.kind)||(['credit','debt'].includes(card.kind)&&failures.value.includes('asset'))){view.note='暂时无法加载，点击查看';view.href=['asset','credit','debt','assetTrend'].includes(card.kind)?'/investments':'/settings/cards';return view;}
    if(['income','expense','balance'].includes(card.kind))view.value=total.complete?money(total[card.kind as 'income'|'expense'|'balance']):'待换算';
    else if(card.kind==='count')view.value=`${filtered.length} 笔`;
    else if(card.kind==='budget'||card.kind==='budgetCompact'){
        view.href='/statistics/budgets';const reports=budgetReports(workspace.budgets,completeLedger.entries.value,props.month,workspace.preferences).filter(r=>(!card.bookIds.length||card.bookIds.includes(r.budget.bookId))&&(!card.categoryIds.length||card.categoryIds.includes(r.budget.categoryId)));
        if(!reports.length)view.note='点击设置预算';else{const overall=reports.filter(r=>r.budget.categoryId==='0'&&r.budget.kind==='monthly');view.value=overall.length?money(overall.some(r=>r.remaining===null)?null:overall.reduce((sum,r)=>sum.plus(r.remaining!),new LedgerDecimal(0)).toString()):undefined;if(card.kind==='budget')view.rows=reports.slice(0,4).map(r=>({label:r.budget.name||'月度预算',value:money(r.remaining),width:r.available&&new LedgerDecimal(r.available).gt(0)?LedgerDecimal.max(0,LedgerDecimal.min(100,new LedgerDecimal(r.spent).div(r.available).mul(100))).toNumber():0}));}
    }else if(['asset','credit','debt'].includes(card.kind)){
        view.caption='当前';view.href='/investments';if(summary.value){let cash=summary.value.cashAccounts.filter(a=>!card.accountIds.length||card.accountIds.includes(a.id));if(card.kind!=='asset')cash=cash.filter(a=>card.kind==='credit'?accounts.allAccountsMap[a.id]?.category===3:[5,6].includes(accounts.allAccountsMap[a.id]?.category||0));view.value=card.kind==='asset'&&!card.accountIds.length?money(summary.value.netAssets):money(cash.some(a=>a.value===null)?null:cash.reduce((sum,a)=>sum.plus(a.value!),new LedgerDecimal(0)).toString());view.rows=cash.slice(0,3).map(a=>({label:a.name,value:money(a.value)}));}else view.note='正在加载';
    }else if(card.kind==='assetTrend'){
        view.href='/investments';const snapshots=history.value.filter(s=>!s.invalidated&&s.recordedAt>=from.unix()&&s.recordedAt<=to.unix()).sort((a,b)=>a.recordedAt-b.recordedAt);view.points=spark(snapshots.map(s=>s.complete?s.netAssets:null));view.value=snapshots.length?money(snapshots.at(-1)?.netAssets):undefined;if(!view.points)view.note=snapshots.some(s=>!s.complete)?'部分历史估值缺失':'至少两个历史快照后显示趋势';
    }else if(card.kind==='wish'){
        view.href='/wishes';view.caption='储蓄计划';view.rows=wishes.value.filter(w=>!w.data.archived&&(!card.wishId||card.wishId===w.id)).slice(0,3).map(w=>{const p=wishProgress(w.data,completeLedger.entries.value,accounts.allAccountsMap,now);return{label:w.data.name,value:`${money(p.saved)} / ${money(w.data.target)}`,width:p.percent};});if(!view.rows.length)view.note='点击添加愿望';
    }else if(card.kind==='repayment'){
        view.href='/calendar/due';view.caption='待处理';view.rows=due.value.filter(d=>!d.completed&&d.kind==='repayment'&&(!card.accountIds.length||card.accountIds.includes(d.accountId))).slice(0,4).map(d=>({label:`${d.date.slice(5)} ${d.accountName}`,value:`${d.currency} ${money(d.amount)}`}));if(!view.rows.length)view.note='暂无待还款';
    }else if(card.kind==='reimbursement'){
        view.href='/assets/reimbursements';const ids=new Set(filtered.map(e=>e.id)),pending=claims.value.filter(c=>!c.closed&&!c.ended&&new LedgerDecimal(c.pending).gt(0)&&ids.has(c.transactionId));view.value=money(pending.some(c=>c.currency!=='CNY')?null:pending.reduce((sum,c)=>sum.plus(c.pending),new LedgerDecimal(0)).toString());view.note=`${pending.length} 笔待报销`;
    }else if(card.kind==='gold'){
        view.href='/investments/manage';view.caption='已绑定报价';const gold=instruments.value.filter(i=>/黄金|gold|XAU/i.test(`${i.name} ${i.symbol}`));view.rows=gold.slice(0,3).map(i=>{const q=quotes.value.find(q=>q.instrumentId===i.id);return{label:i.name,value:q?`${q.currency} ${money(q.price)}`:'缺失报价'};});if(!view.rows.length)view.note='点击绑定黄金标的';
    }else if(card.kind==='category'||card.kind==='tag'||card.kind==='rank'){
        const groups=reportBreakdown(groupReport(filtered,card.kind==='tag'?'tag':'category',{tags:tags.allTransactionTagsMap,type:3}),'expense');view.rows=groups.items.slice(0,4).map(g=>({label:g.name,value:g.complete?money(g.amount):'待换算',width:g.width}));if(!view.rows.length)view.note='暂无支出记录';view.href='/statistics';
    }else if(card.kind==='heatmap'||card.kind==='line'||card.kind==='week'){
        if(card.period==='all'){from=now.clone().subtract(11,'month').startOf('month');to=now.clone().endOf('month');}const buckets=reportBuckets(filtered,from,to,to.diff(from,'days')>90?'month':'day');
        if(card.kind==='heatmap'){const max=LedgerDecimal.max(1,...buckets.map(b=>new LedgerDecimal(b.expense).abs()));view.cells=buckets.map(b=>({label:b.label,amount:b.complete?money(b.expense):'待换算',opacity:b.complete?(new LedgerDecimal(b.expense).abs().div(max).mul(.85).toNumber()+.15):.06}));}else view.points=spark(buckets.map(b=>b.complete?b.expense:null));view.value=total.complete?money(total.expense):'待换算';view.note='支出';view.href='/statistics';
    }else{
        if(card.kind==='sankey'&&total.complete)view.flow=makeFlow(filtered,total.income,total.expense);
        view.href='/statistics';const maximum=LedgerDecimal.max(1,new LedgerDecimal(total.income).abs(),new LedgerDecimal(total.expense).abs(),new LedgerDecimal(total.balance).abs());view.rows=[{label:'收入',value:total.income},{label:'支出',value:total.expense},{label:'结余',value:total.balance}].map(r=>({...r,value:total.complete?money(r.value):'待换算',width:card.kind==='sankey'||card.kind==='flow'?new LedgerDecimal(r.value).abs().div(maximum).mul(100).toNumber():undefined}));
    }
    return view;
}
const cards=computed(()=>experience.preferences.cards.filter(c=>!c.bookIds.length||!books.selectedBookIds.length||c.bookIds.some(id=>books.selectedBookIds.includes(id))).map(build));
</script>
<style scoped>.cy-home-cards{display:grid;grid-template-columns:repeat(6,minmax(0,1fr));gap:9px;margin:12px 0}.cy-data-card{display:block;min-width:0;padding:13px 12px;position:relative;isolation:isolate;border-radius:12px;color:var(--cy-ink);box-sizing:border-box}.cy-data-card::before{content:"";position:absolute;inset:0;z-index:-1;border-radius:inherit;background:var(--card-background);background-image:var(--card-image);background-size:cover;background-position:center;opacity:var(--card-opacity)}.cy-data-card header{font-size:12px;display:flex;flex-wrap:wrap;gap:4px;justify-content:space-between;align-items:center}.cy-data-card header small{font-size:9px;opacity:.5}.cy-card-value{display:block;font-size:clamp(16px,4.8vw,25px);font-weight:550;margin-top:13px;overflow-wrap:anywhere;font-variant-numeric:tabular-nums}.cy-card-note{display:block;font-size:10px;opacity:.6;line-height:1.6;margin-top:9px}.cy-card-row{margin-top:12px;position:relative;padding-bottom:5px}.cy-card-row>div{display:flex;justify-content:space-between;gap:5px;font-size:10px;position:relative;z-index:1}.cy-card-row span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.cy-card-row b{font-weight:500;white-space:nowrap}.cy-card-row>i{display:block;position:absolute;height:3px;bottom:0;left:0;background:var(--cy-accent);opacity:.55;min-width:0;border-radius:2px}.cy-card-line{width:100%;height:64px;color:var(--cy-accent);margin-top:8px}.cy-card-flow{width:100%;height:auto;margin-top:12px}.cy-card-flow text{font-size:10px}.cy-card-heatmap{display:grid;grid-template-columns:repeat(7,1fr);gap:3px;margin-top:12px}.cy-card-heatmap i{aspect-ratio:1;background:var(--cy-accent);border-radius:2px;max-height:20px}</style>
