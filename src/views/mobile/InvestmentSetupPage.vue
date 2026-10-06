<template>
  <f7-page class="cy-mobile-surface cy-investment-page" :class="{'cy-fund-setup':type==='FUND'}" @page:beforein="start" @page:afterin="focusName">
    <f7-navbar v-if="type==='FUND'" :title="title"><f7-nav-left><f7-link :aria-label="editing?'取消编辑基金':'取消新增基金'" @click="f7router.back()"><svg class="fund-nav-symbol" viewBox="0 0 24 24" aria-hidden="true"><path d="M6 6 18 18M18 6 6 18" /></svg></f7-link></f7-nav-left><f7-nav-right><f7-link :disabled="busy||loading||searching" aria-label="保存基金" @click="save"><f7-preloader v-if="busy" /><svg v-else class="fund-nav-symbol" viewBox="0 0 24 24" aria-hidden="true"><path d="m6 12 4 4 9-8" /></svg></f7-link></f7-nav-right></f7-navbar>
    <f7-navbar v-else :title="title" back-link="理财"><f7-nav-right v-if="!['CRYPTO','MONETARY'].includes(type)"><f7-link :disabled="busy||loading" aria-label="保存理财" @click="save"><f7-preloader v-if="busy" /><f7-icon v-else f7="checkmark" /></f7-link></f7-nav-right></f7-navbar>
    <main class="inv-body">
      <p v-if="error" class="cy-message" role="alert">{{ error }} <button v-if="type==='FUND'&&searchFailed" type="button" class="inv-link" @click="searchLater(searchField==='code'?symbol:name,searchField)">重试查询</button></p>
      <p v-if="loading" class="cy-empty">正在加载…</p>
      <template v-else-if="type==='CRYPTO'">
        <section class="inv-card"><f7-link href="/crypto/add?kind=EXCHANGE" class="inv-list-button">加密货币交易所<f7-icon f7="chevron_right" /></f7-link><f7-link href="/crypto/add?kind=WALLET" class="inv-list-button">加密钱包<f7-icon f7="chevron_right" /></f7-link></section>
        <section v-if="cryptoAccounts.length" class="inv-card"><p class="inv-muted">录入到已有账户</p><f7-link v-for="a in cryptoAccounts" :key="a.id" :href="`/crypto/convert?mode=opening&accountId=${a.id}`" class="inv-list-button">{{ a.name }}<f7-icon f7="chevron_right" /></f7-link></section>
      </template>
      <template v-else-if="type==='MONETARY'">
        <p class="inv-caption">选择基金资金所在的账户，绑定后自动记录逐日收益。已有账户余额会直接沿用。</p>
        <section class="inv-card"><f7-link v-for="a in cashChoices" :key="a.id" :href="`/account/income?id=${a.id}`" class="inv-list-button"><span>{{ a.name }}</span><span>{{ ledgerMoney(a.value,false) }} <f7-icon f7="chevron_right" /></span></f7-link><p v-if="!cashChoices.length" class="cy-empty">先建立一个人民币资金账户</p></section>
        <f7-link href="/account/add?preset=savings&monetary=1" class="inv-list-button">新建货币基金账户<f7-icon f7="plus" /></f7-link>
      </template>
      <form v-else-if="type==='FUND'" class="fund-form" @submit.prevent="save">
        <section class="fund-card fund-fields" @keydown.enter.prevent="nextFundField">
          <label class="fund-field"><span>基金名称</span><input ref="nameInput" v-model="name" maxlength="64" placeholder="输入名称进行查询" aria-label="基金名称" autocomplete="off" enterkeyhint="next" @input="nameChanged" /></label>
          <label class="fund-field"><span>基金代码</span><input v-model="symbol" maxlength="6" placeholder="输入代码进行查询" aria-label="基金代码" inputmode="numeric" autocomplete="off" :disabled="editing" enterkeyhint="next" @input="searchLater(symbol,'code')" /><f7-preloader v-if="searching" class="fund-query-loading" /></label>
          <div v-if="candidates.length" class="fund-candidates" :class="{'fund-candidates-name':searchField==='name'}"><button v-for="candidate in candidates" :key="candidate.provider+candidate.providerId" type="button" @click="choose(candidate)"><span>{{ candidate.symbol }}</span><span>{{ candidate.name }}</span></button></div>
          <label class="fund-field"><span>持仓成本</span><input v-model="unitCost" inputmode="decimal" maxlength="80" placeholder="请输入每份额的持仓成本价" aria-label="持仓成本" enterkeyhint="next" /></label>
          <label class="fund-field"><span>持仓份额</span><input v-model="quantity" inputmode="decimal" maxlength="80" placeholder="请输入持仓份额" aria-label="持仓份额" enterkeyhint="next" /></label>
          <label class="fund-field"><span>盈亏偏差</span><input v-model="profile.profitOffset" inputmode="decimal" maxlength="80" placeholder="默认为0，当持有收益存在偏差时设置" aria-label="盈亏偏差" enterkeyhint="next" /></label>
          <label class="fund-field"><span>备注</span><input v-model="profile.note" maxlength="300" placeholder="请输入备注" aria-label="备注" enterkeyhint="done" /></label>
        </section>
        <section class="fund-card fund-options">
          <button class="fund-option" type="button" @click="openGroups"><span>所属分组</span><span class="fund-option-value">{{ profile.group }}<f7-icon f7="chevron_right" /></span></button>
          <label class="fund-option"><span>计入总资产</span><input class="fund-switch" type="checkbox" role="switch" :checked="!profile.excludeFromTotal" aria-label="计入总资产" @change="profile.excludeFromTotal=!($event.target as HTMLInputElement).checked" /></label>
          <label class="fund-option"><span>盈亏计入总资产</span><input class="fund-switch" type="checkbox" role="switch" :checked="!profile.excludeProfit" aria-label="盈亏计入总资产" @change="profile.excludeProfit=!($event.target as HTMLInputElement).checked" /></label>
          <button class="fund-option" type="button" @click="openBooks"><span>选择生效账本<small>账户仅显示在指定账本</small></span><span class="fund-option-value"><span>{{ selectedBookNames }}</span><f7-icon f7="chevron_right" /></span></button>
        </section>
        <p class="fund-tip">提示：<br />盈亏偏差：部分平台会将分红或手续费计入盈亏，设置该数值使累计盈亏与三方平台相同<br />初次创建理财账户不会生成交易记录，如要生成，可将持仓份额设为0，再添加买入卖出记录</p>
      </form>
      <form v-else @submit.prevent="save">
        <section class="inv-card inv-form">
          <label class="inv-row"><span>{{ type==='FUND'?'基金名称':'理财名称' }}</span><input v-model="name" maxlength="64" :placeholder="searchable?'输入名称进行查询':'输入理财名称'" aria-label="理财名称" @input="nameChanged" /></label>
          <label v-if="type!=='OTHER'" class="inv-row"><span>{{ type==='FUND'?'基金代码':'资产代码' }}</span><input v-model="symbol" maxlength="24" placeholder="输入代码进行查询" aria-label="资产代码" :disabled="editing" @input="searchLater(symbol)" /></label>
          <label class="inv-row"><span>{{ type==='OTHER'?'持有金额':'持有份额' }}</span><input v-model="quantity" inputmode="decimal" :placeholder="type==='OTHER'?'输入持有金额':'没有持仓可填0'" aria-label="持有份额" /></label>
          <label v-if="type!=='OTHER'" class="inv-row"><span>持仓成本价</span><input v-model="unitCost" inputmode="decimal" placeholder="每份人民币成本，可留空" aria-label="持仓成本价" /></label>
          <label class="inv-row"><span>盈亏偏差</span><input v-model="profile.profitOffset" inputmode="decimal" placeholder="默认0，可填负数" aria-label="盈亏偏差" /></label>
          <label class="inv-row"><span>备注</span><input v-model="profile.note" maxlength="300" placeholder="输入备注" aria-label="备注" /></label>
        </section>
        <section v-if="searching||candidates.length" class="inv-card inv-results"><p v-if="searching" class="inv-muted">正在查询…</p><button v-for="candidate in candidates" :key="candidate.provider+candidate.providerId" type="button" @click="choose(candidate)"><strong>{{ candidate.name }}</strong><small>{{ candidate.symbol }} · {{ candidate.market }} · {{ candidate.currency }}</small></button></section>
        <p v-if="selected" class="inv-caption">已选择 {{ selected.name }} · {{ selected.symbol }} · {{ selected.market }} · {{ selected.currency }}</p>
        <section class="inv-card inv-form">
          <label class="inv-row"><span>所属分组</span><input v-model="profile.group" list="investment-groups" maxlength="32" placeholder="输入或选择分组" aria-label="所属分组" /></label>
          <datalist id="investment-groups"><option v-for="g in groups" :key="g" :value="g" /></datalist>
          <label class="inv-row"><span>计入总资产</span><input type="checkbox" :checked="!profile.excludeFromTotal" @change="profile.excludeFromTotal=!($event.target as HTMLInputElement).checked" /></label>
          <label class="inv-row"><span>盈亏计入总资产</span><input type="checkbox" :checked="!profile.excludeProfit" @change="profile.excludeProfit=!($event.target as HTMLInputElement).checked" /></label>
          <button class="inv-list-button" type="button" @click="showBooks=!showBooks"><span>选择生效账本</span><span class="inv-muted">{{ profile.bookIds.length?`已选${profile.bookIds.length}个`:'全部账本' }} <f7-icon f7="chevron_down" /></span></button>
          <template v-if="showBooks"><label v-for="book in books.allBooks" :key="book.id" class="inv-row"><span>{{ book.name }}</span><input v-model="profile.bookIds" type="checkbox" :value="book.id" /></label></template>
        </section>
        <details class="inv-card inv-details"><summary>更多设置</summary>
          <label v-if="!editing" class="inv-row"><span>所属投资账户</span><select v-model="profile.accountId" aria-label="所属投资账户"><option value="">单独建立本项理财</option><option v-for="a in accounts.filter(a=>!['EXCHANGE','WALLET'].includes(a.kind))" :key="a.id" :value="a.id">{{ a.name }}</option></select></label>
          <label class="inv-row"><span>{{ editing?'校准日期':'持仓日期' }}</span><input v-model="date" type="datetime-local" required aria-label="持仓日期" /></label>
          <div class="inv-row"><span>记入账本</span><BookPicker v-model="bookId" compact /></div>
          <label v-if="!editing&&!selected&&type!=='OTHER'" class="inv-row"><span>手动参考价格</span><input v-model="manualPrice" inputmode="decimal" placeholder="每份人民币价格，可留空" aria-label="手动参考价格" /></label>
          <label v-if="!editing&&searchable" class="inv-row"><span>使用手动估值</span><input v-model="manual" type="checkbox" /></label>
          <p>成本按人民币填写；留空保留为未知。盈亏偏差只调整累计收益的显示，不修改余额。关闭“盈亏计入总资产”时，该项按剩余成本计入。</p>
        </details>
        <p class="inv-caption">{{ editing?'修改份额或成本会保存一笔持仓校准记录，可在流水中查看和撤销。':'已有持仓不会扣付款账户。想记录一次真实买入，可先填0份，再从详情页买入。' }}</p>
        <button class="inv-save" :disabled="busy||searching">{{ busy?'正在保存…':'保存' }}</button>
      </form>
    </main>
    <f7-sheet class="fund-group-sheet fund-overlay" v-model:opened="showGroups" backdrop swipe-to-close>
      <header><strong>选择所属分组</strong><button type="button" @click="openNewGroup"><f7-icon f7="plus_circle" />新增分组</button></header>
      <div class="fund-group-list"><button v-for="group in fundGroups" :key="group" type="button" @click="profile.group=group;showGroups=false">{{ group }}</button></div>
    </f7-sheet>
    <f7-sheet class="fund-new-group-sheet fund-overlay" v-model:opened="showNewGroup" backdrop @sheet:opened="groupInput?.focus()">
      <header><strong>新增组名</strong><button type="button" @click="addGroup">保存</button></header>
      <form @submit.prevent="addGroup"><input ref="groupInput" v-model="newGroup" maxlength="32" placeholder="请输入账户分组名" aria-label="账户分组名" /><button type="button" aria-label="清空分组名" @click="newGroup=''"><f7-icon f7="xmark_circle_fill" /></button></form>
    </f7-sheet>
    <f7-popup class="fund-books-dialog fund-overlay" v-model:opened="showFundBooks" :close-by-backdrop-click="true">
      <h2>选择账本</h2>
      <label class="fund-all-books"><input v-model="allBooksDraft" type="checkbox" aria-label="全部账本" @change="draftBookIds=[]" /><strong>全部账本</strong></label>
      <div class="fund-book-list"><label v-for="book in books.allBooks" :key="book.id"><input :checked="!allBooksDraft&&draftBookIds.includes(book.id)" type="checkbox" :aria-label="book.name" @change="toggleDraftBook(book.id,($event.target as HTMLInputElement).checked)" /><span>{{ book.name }}</span></label></div>
      <footer><button type="button" @click="showFundBooks=false">取消</button><button type="button" :disabled="!allBooksDraft&&!draftBookIds.length" @click="confirmBooks">完成</button></footer>
    </f7-popup>
  </f7-page>
</template>
<script setup lang="ts">
import {computed,ref,onUnmounted,nextTick} from 'vue';
import type {Router} from 'framework7/types';
import moment from 'moment-timezone';
import {investments,investmentError} from '@/lib/investments.ts';
import {blankHolding,useInvestmentData,investmentGroup,validInvestmentNumber,invalidateInvestmentData} from '@/lib/investment-mobile.ts';
import {LedgerDecimal,ledgerMoney} from '@/lib/ledger-display.ts';
import {generateRandomUUID} from '@/lib/misc.ts';
import BookPicker from '@/components/mobile/BookPicker.vue';
import type {Instrument,InstrumentCandidate,HoldingSetup} from '@/models/investment.ts';
const props=defineProps<{f7route:Router.Route;f7router:Router.Router}>();
const {wealth,accounts,rows,books,loading,error,zone,load}=useInvestmentData();
const type=ref(props.f7route.query['type']||'FUND'),editing=!!props.f7route.query['instrumentId'];
const profile=ref(blankHolding(props.f7route.query['accountId']||'',props.f7route.query['instrumentId']||''));
const name=ref(''),symbol=ref(''),quantity=ref(''),unitCost=ref(''),manualPrice=ref(''),manual=ref(false),date=ref(''),bookId=ref(''),busy=ref(false),showBooks=ref(false),searching=ref(false);
const selected=ref<InstrumentCandidate>(),candidates=ref<InstrumentCandidate[]>([]),asset=ref<Instrument>();
const nameInput=ref<HTMLInputElement>(),groupInput=ref<HTMLInputElement>(),searchField=ref<'name'|'code'>('name');
const searchFailed=ref(false);
const showGroups=ref(false),showNewGroup=ref(false),showFundBooks=ref(false),newGroup=ref(''),addedGroups=ref<string[]>([]),allBooksDraft=ref(true),draftBookIds=ref<string[]>([]);
const fundGroups=computed(()=>[...new Set(['基金','期货','股票（沪深）','港股','美股',...rows.value.map(r=>r.profile.group).filter(Boolean),...addedGroups.value])]);
const selectedBookNames=computed(()=>profile.value.bookIds.length?books.allBooks.filter(b=>profile.value.bookIds.includes(b.id)).map(b=>b.name).join('、'):'全部账本');
const market=computed(()=>({FUND:'CN_FUND',CN:'CN',HK:'HK',US:'US'} as Record<string,string>)[type.value]||'');
const searchable=computed(()=>!!market.value&&!editing&&!manual.value);
const title=computed(()=>(editing?'编辑':'新增')+({FUND:'基金',CN:'股票',HK:'港股',US:'美股',OTHER:'理财',METAL:'期货 / 贵金属',CRYPTO:'加密货币',MONETARY:'货币基金'} as Record<string,string>)[type.value]);
const groups=computed(()=>[...new Set(['基金','股票','港股','美股','期货 / 贵金属','其他理财',...rows.value.map(r=>r.group)])]);
const cryptoAccounts=computed(()=>accounts.value.filter(a=>['EXCHANGE','WALLET'].includes(a.kind)));
const cashChoices=computed(()=>wealth.value?.cashAccounts.filter(a=>a.currency==='CNY'&&!a.liability)||[]);
let initialized=false,timer:ReturnType<typeof setTimeout>|undefined,searchVersion=0,initialUnitCost='',expectedQuantity='0',expectedCost:string|null='0';
const requestKey=generateRandomUUID();
if(type.value==='FUND'&&!editing)profile.value.profitOffset='';
async function start():Promise<void>{if(initialized)return;await load();if(error.value)return;initialized=true;bookId.value=books.defaultBookId;date.value=moment().tz(zone.value).format('YYYY-MM-DDTHH:mm');if(editing){const row=rows.value.find(r=>r.position.accountId===profile.value.accountId&&r.position.instrumentId===profile.value.instrumentId);if(!row){error.value='找不到这项理财';return;}asset.value=row.asset;type.value=row.asset?.type==='STOCK'?row.asset.market==='HK'?'HK':row.asset.market==='US'?'US':'CN':row.asset?.type||'OTHER';profile.value=JSON.parse(JSON.stringify(row.profile));name.value=row.name;symbol.value=row.asset?.symbol||'';quantity.value=row.position.quantity;expectedQuantity=row.position.quantity;expectedCost=row.position.cost;unitCost.value=row.position.averageCost?new LedgerDecimal(row.position.averageCost).toDecimalPlaces(18).toString():'';initialUnitCost=unitCost.value;}else{profile.value.group=type.value==='METAL'?'期货 / 贵金属':investmentGroup({type:type.value==='FUND'?'FUND':type.value==='OTHER'?'OTHER':'STOCK',market:market.value} as Instrument);}await focusName();}
function nameChanged():void{if(editing)return;searchLater(name.value,'name');}
function searchLater(query:string,field:'name'|'code'='code'):void{
  clearTimeout(timer);const version=++searchVersion;searchField.value=field;
  if(type.value==='FUND'&&selected.value){if(field==='code'&&name.value===selected.value.name)name.value='';if(field==='name'&&symbol.value===selected.value.symbol)symbol.value='';}
  selected.value=undefined;candidates.value=[];error.value='';searchFailed.value=false;const text=query.trim();
  if(!searchable.value||text.length<2||(type.value==='FUND'&&field==='code'&&!/^\d{6}$/.test(text))){searching.value=false;return;}
  searching.value=true;timer=setTimeout(async()=>{try{const found=await investments.searchInstruments(text,market.value);if(version!==searchVersion)return;const exact=found.filter(item=>item.type==='FUND'&&item.market==='CN_FUND'&&item.symbol===text);if(type.value==='FUND'&&field==='code'&&exact.length===1){choose(exact[0]!);}else{candidates.value=found;if(type.value==='FUND'&&!found.length)error.value='未找到该基金，请核对名称或代码';}}catch(e){if(version===searchVersion){searchFailed.value=true;error.value=type.value==='FUND'?'基金查询暂不可用，请检查网络后重试':investmentError(e);}}finally{if(version===searchVersion)searching.value=false;}},350);
}
function choose(value:InstrumentCandidate):void{searchVersion++;clearTimeout(timer);selected.value={...value};name.value=value.name;symbol.value=value.symbol;candidates.value=[];searching.value=false;error.value='';}
async function focusName():Promise<void>{await nextTick();if(type.value==='FUND'&&!editing&&!name.value&&!symbol.value)nameInput.value?.focus();}
function nextFundField(event:KeyboardEvent):void{const target=event.target as HTMLInputElement;const fields=Array.from(target.closest('.fund-fields')?.querySelectorAll<HTMLInputElement>('input:not(:disabled)')||[]);const next=fields[fields.indexOf(target)+1];if(next)next.focus();else target.blur();}
function blurInput():void{if(document.activeElement instanceof HTMLElement)document.activeElement.blur();}
function openGroups():void{blurInput();showGroups.value=true;}
function openNewGroup():void{showGroups.value=false;newGroup.value='';showNewGroup.value=true;}
function addGroup():void{const group=newGroup.value.trim();if(!group)return;addedGroups.value.push(group);profile.value.group=group;blurInput();showNewGroup.value=false;}
function openBooks():void{blurInput();draftBookIds.value=[...profile.value.bookIds];allBooksDraft.value=!draftBookIds.value.length;showFundBooks.value=true;}
function toggleDraftBook(id:string,checked:boolean):void{allBooksDraft.value=false;draftBookIds.value=checked?[...new Set([...draftBookIds.value,id])]:draftBookIds.value.filter(item=>item!==id);}
function confirmBooks():void{if(!allBooksDraft.value&&!draftBookIds.value.length)return;profile.value.bookIds=allBooksDraft.value?[]:[...draftBookIds.value];showFundBooks.value=false;}
async function save():Promise<void>{if(busy.value||!initialized)return;error.value='';try{validInvestmentNumber(quantity.value,'持有份额');validInvestmentNumber(profile.value.profitOffset||'0','盈亏偏差',false,true);if(!name.value.trim())throw Error('请输入理财名称');if(searchable.value&&!selected.value)throw Error(type.value==='FUND'?'请输入有效基金代码，或按名称查询并选择基金':'请从查询结果选择资产；找不到时可在更多设置中选择手动估值');const q=new LedgerDecimal(quantity.value);let cost:string|null=unitCost.value?new LedgerDecimal(validInvestmentNumber(unitCost.value,'成本价')).mul(q).toDecimalPlaces(18).toString():null;if(type.value==='OTHER')cost=q.toString();if(editing&&quantity.value===expectedQuantity&&unitCost.value===initialUnitCost)cost=expectedCost;if(q.isZero())cost='0';const at=moment.tz(date.value,'YYYY-MM-DDTHH:mm',true,zone.value);if(!at.isValid())throw Error('请选择有效日期');const data:HoldingSetup={profile:{...profile.value,name:name.value.trim()},instrument:selected.value||asset.value||{name:name.value.trim(),symbol:symbol.value||'自定义',type:type.value==='FUND'?'FUND':['CN','HK','US'].includes(type.value)?'STOCK':'OTHER'},quantity:q.toString(),cost,price:editing?'':type.value==='OTHER'?'1':manualPrice.value,bookId:bookId.value,occurredAt:at.unix(),expectedQuantity,expectedCost};busy.value=true;const saved=editing?await investments.updateHolding(data,requestKey):await investments.setupHolding(data,requestKey);invalidateInvestmentData();if(editing||type.value==='FUND')props.f7router.back();else props.f7router.navigate('/investments/position?'+new URLSearchParams({accountId:saved.accountId,instrumentId:saved.instrumentId}),{reloadCurrent:true});}catch(e){error.value=investmentError(e);}finally{busy.value=false;}}
onUnmounted(()=>{clearTimeout(timer);searchVersion++;});
</script>
