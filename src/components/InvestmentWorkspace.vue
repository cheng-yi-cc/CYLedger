<template>
    <div class="cy-wealth">
        <header class="cy-heading">
            <div><p class="cy-eyebrow">CYLEDGER · 资产账本</p><h1>我的资产</h1><p class="cy-muted">日常账户与投资持仓，统一以人民币查看。</p></div>
            <button class="cy-button cy-primary" @click="openEvent()">＋ 记录投资</button>
        </header>
        <p v-if="error" class="cy-message cy-error" role="alert">{{ error }} <button @click="load()">重试加载</button></p>
        <p v-if="notice" class="cy-message" role="status">{{ notice }}</p>
        <p v-if="hasLocalDraft && !editor" class="cy-message cy-warning">此浏览器有一份尚未同步的投资草稿。<button @click="restoreLocalDraft">继续填写</button><button @click="discardLocalDraft">丢弃草稿</button></p>
        <nav class="cy-tabs" aria-label="资产页面">
            <button v-for="item in tabs" :key="item.key" :aria-current="tab === item.key ? 'page' : undefined" @click="tab = item.key; closeEditor()">{{ item.name }}</button>
            <button class="cy-refresh" :disabled="loading" @click="load()">{{ loading ? '更新中…' : '刷新' }}</button>
        </nav>

        <section v-if="editor" ref="editorElement" class="cy-panel cy-editor" tabindex="-1">
            <div class="cy-section-heading"><h2>{{ editorTitle }}</h2><button class="cy-button" :disabled="saving" @click="closeEditor()">关闭</button></div>
            <p v-if="editorError" class="cy-message cy-error" role="alert">{{ editorError }}</p>
            <form v-if="editor === 'event'" @submit.prevent="previewEvent">
                <fieldset :disabled="saving || !!preview" class="cy-fields">
                    <label>操作类型<select v-model="draft.type" @change="draft.exchangeRate = draft.type === 'TRANSFER' ? '' : '1'"><option v-for="(name, key) in eventNames" :key="key" :value="key">{{ name }}</option></select></label>
                    <label>投资账户<select v-model="draft.accountId" required><option disabled value="">选择账户</option><option v-for="account in accounts" :key="account.id" :value="account.id">{{ account.name }}</option></select></label>
                    <label>资产<select v-model="draft.instrumentId" required><option disabled value="">选择资产</option><option v-for="instrument in instrumentsList" :key="instrument.id" :value="instrument.id">{{ instrument.name }} · {{ instrument.symbol }}{{ instrument.type !== 'CRYPTO' ? '（手动估值）' : '' }}</option></select></label>
                    <label>数量（{{ selectedInstrument?.symbol || '份额' }}）<input v-model="draft.quantity" inputmode="decimal" required placeholder="0.00000000" autocomplete="off" /></label>
                    <label v-if="draft.type === 'TRANSFER'">转入投资账户<select v-model="draft.toAccountId" required><option disabled value="">选择转入账户</option><option v-for="account in accounts.filter(a => a.id !== draft.accountId)" :key="account.id" :value="account.id">{{ account.name }}</option></select></label>
                    <label v-if="draft.type === 'OPENING'">持仓总成本（CNY，可留空）<input v-model="costInput" inputmode="decimal" placeholder="不填写则保留为成本未知" /><span class="cy-field-note">录入已有持仓，不扣日常账户余额。</span></label>
                    <template v-if="draft.type === 'BUY' || draft.type === 'SELL'">
                        <label>结算方式<select v-model="settlementMode"><option value="cash">日常资金账户</option><option value="asset">投资资产（例如 USDT）</option></select></label>
                        <label v-if="settlementMode === 'cash'">结算资金账户<select v-model="draft.cashAccountId" required><option disabled value="">选择账户</option><option v-for="account in cashAccounts" :key="account.id" :value="account.id">{{ account.name }} · {{ account.currency }}</option></select><span class="cy-field-note">资金不足时，服务端会阻止买入。</span></label>
                        <template v-else>
                            <label>结算投资账户<select v-model="draft.settlementAccountId" required><option disabled value="">选择账户</option><option v-for="account in accounts" :key="account.id" :value="account.id">{{ account.name }}</option></select></label>
                            <label>结算资产<select v-model="draft.settlementInstrumentId" required><option disabled value="">选择结算资产</option><option v-for="instrument in instrumentsList.filter(i => i.id !== draft.instrumentId)" :key="instrument.id" :value="instrument.id">{{ instrument.name }} · {{ instrument.symbol }}</option></select></label>
                        </template>
                        <label>成交金额（{{ settlementUnit }}）<input v-model="draft.amount" required inputmode="decimal" placeholder="不含手续费" /></label>
                        <label>手续费（{{ settlementUnit }}）<input v-model="draft.fee" required inputmode="decimal" placeholder="0" /><span class="cy-field-note">买入计入成本；卖出从所得扣除。</span></label>
                        <label>交易时折算率（1 {{ settlementUnit }} = CNY）<input v-model="draft.exchangeRate" required inputmode="decimal" /><span class="cy-field-note">保存后用于历史成本，不随当前行情变化。</span></label>
                    </template>
                    <label v-if="draft.type === 'TRANSFER'">历史参考单价（CNY，可留空）<input v-model="draft.exchangeRate" inputmode="decimal" placeholder="每份转移资产的人民币参考值" /><span class="cy-field-note">用于披露手续费参考价值；留空则费用价值未知。</span></label>
                    <label v-if="draft.type === 'TRANSFER'">转移手续费（{{ selectedInstrument?.symbol || '资产数量' }}）<input v-model="draft.fee" required inputmode="decimal" placeholder="0" /></label>
                    <label>发生时间<input v-model="eventTime" type="datetime-local" required /><span class="cy-field-note">会计时区：{{ settings.timeZone }}</span></label>
                    <label class="cy-full">备注<textarea v-model="draft.note" rows="2" maxlength="1000" placeholder="可选，例如期初录入或成交说明"></textarea></label>
                </fieldset>
                <p v-if="!accounts.length" class="cy-message">先到「账户与资产」创建一个投资账户，再录入已有持仓。</p>
                <div v-if="!preview" class="cy-actions"><button class="cy-button cy-primary" :disabled="saving || !accounts.length">{{ saving ? '正在预览…' : '预览影响' }}</button><button type="button" class="cy-button" :disabled="saving" @click="saveLocalDraft">保存本地草稿</button><span class="cy-muted">预览不会入账。</span></div>
                <section v-if="preview" class="cy-preview" aria-label="交易预览">
                    <h3>确认本次{{ eventNames[draft.type] }}</h3>
                    <p>{{ accountName(draft.accountId) }} · {{ selectedInstrument?.name }} · {{ draft.quantity }} {{ selectedInstrument?.symbol }}</p>
                    <p v-if="cashEffect">{{ cashEffect }}</p>
                    <p v-if="draft.type === 'OPENING'">{{ costInput ? `录入总成本 ${money(costInput)}` : '成本未知，不计算完整投资盈亏' }}；日常账户余额不变。</p>
                    <p v-if="draft.type === 'TRANSFER'">转入 {{ accountName(draft.toAccountId) }}，手续费 {{ draft.fee }} {{ selectedInstrument?.symbol }}，费用参考值 {{ transferFeeValue }}。</p>
                    <div class="cy-preview-positions"><div v-for="position in affectedPreview" :key="position.accountId + position.instrumentId"><strong>{{ accountName(position.accountId) }} · {{ instrumentName(position.instrumentId) }}</strong><span>保存后数量 {{ quantity(position.quantity) }}</span><span>剩余成本 {{ position.costKnown ? money(position.cost) : '未知' }}</span><span>已实现损益 {{ position.costKnown ? signedMoney(position.realizedPnl) : '成本不完整' }}</span></div></div>
                    <p class="cy-field-note">{{ draft.id ? '修订后将重算后续持仓与成本；有关历史快照会失效。' : '确认后保存交易事实，并同步更新关联资金账户。' }}</p>
                    <div class="cy-actions"><button type="button" class="cy-button" :disabled="saving" @click="preview = null; idempotencyKey = generateRandomUUID()">返回修改</button><button type="button" class="cy-button cy-primary" :disabled="saving" @click="saveEvent">{{ saving ? '正在保存…' : draft.id ? '确认修订' : '确认入账' }}</button></div>
                </section>
            </form>
            <form v-else-if="editor === 'account'" @submit.prevent="createAccount"><div class="cy-fields"><label>账户名称<input v-model="accountDraft.name" required maxlength="64" placeholder="例如：我的交易所账户" /></label><label>账户类型<select v-model="accountDraft.kind"><option value="EXCHANGE">交易所</option><option value="WALLET">个人钱包</option><option value="BROKER">证券账户</option><option value="OTHER">其他</option></select></label></div><div class="cy-actions"><button class="cy-button cy-primary" :disabled="saving">创建账户</button></div></form>
            <form v-else-if="editor === 'instrument'" @submit.prevent="createInstrument"><div class="cy-fields"><label>资产名称<input v-model="instrumentDraft.name" required maxlength="64" placeholder="基金、股票或资产的完整名称" /></label><label>展示代码<input v-model="instrumentDraft.symbol" required maxlength="24" placeholder="例如 510300" /></label><label>资产类型<select v-model="instrumentDraft.type"><option value="STOCK">股票</option><option value="FUND">基金</option><option value="CRYPTO">加密资产</option><option value="OTHER">其他资产</option></select></label></div><p class="cy-field-note">新建资产仅属于你的账本，使用手动报价。同名代码不会与已有资产自动合并。</p><div class="cy-actions"><button class="cy-button cy-primary" :disabled="saving">创建资产</button></div></form>
            <form v-else-if="editor === 'quote'" @submit.prevent="saveQuote"><div class="cy-fields"><label>资产<select v-model="quoteDraft.instrumentId" required><option v-for="instrument in instrumentsList" :key="instrument.id" :value="instrument.id">{{ instrument.name }} · {{ instrument.symbol }}</option></select></label><label>每份价格（CNY）<input v-model="quoteDraft.price" required inputmode="decimal" placeholder="0.00" /></label><label>价格时间<input v-model="quoteTime" type="datetime-local" required /><span class="cy-field-note">{{ settings.timeZone }}</span></label></div><p class="cy-field-note">此报价明确标为「手动估值」，不会修改持有数量与成本。</p><div class="cy-actions"><button class="cy-button cy-primary" :disabled="saving">保存手动报价</button><button type="button" class="cy-button" :disabled="saving" @click="restoreAutomaticQuote">恢复自动报价</button></div></form>
            <div v-else-if="editor === 'void' && voidCandidate"><p>撤销 {{ date(voidCandidate.occurredAt) }} 的{{ eventNames[voidCandidate.type] }}：{{ quantity(voidCandidate.quantity) }} {{ instrumentSymbol(voidCandidate.instrumentId) }}。</p><p>相关资金变动将撤回，后续成本与持仓重新计算。若导致后续超卖，撤销会被阻止。版本记录会保留。</p><div class="cy-actions"><button class="cy-button" @click="closeEditor">保留流水</button><button class="cy-button cy-danger" :disabled="saving" @click="voidEvent">确认撤销</button></div></div>
        </section>

        <div v-if="loading && !summary" class="cy-panel cy-empty" role="status">正在读取真实账户与持仓…</div>
        <template v-if="tab === 'overview' && summary">
            <section class="cy-overview cy-panel">
                <div class="cy-net"><p class="cy-muted">{{ summary.netAssets === null ? '已估值部分' : '净资产' }} <span class="cy-currency">CNY</span></p><p class="cy-total">{{ money(summary.netAssets ?? summary.valuedAssets) }}</p><p class="cy-field-note">{{ summary.netAssets === null ? `另有 ${summary.missingPrices} 项暂未估值` : '日常账户净余额 + 投资参考市值' }}</p></div>
                <dl class="cy-summary-grid"><div><dt>现金与日常资产</dt><dd>{{ money(summary.cashAssets) }}</dd></div><div><dt>投资参考市值</dt><dd>{{ money(summary.investmentValue) }}</dd></div><div><dt>负债</dt><dd>{{ money(summary.liabilities) }}</dd></div><div><dt>{{ summary.costComplete ? '未实现盈亏' : '未实现盈亏（成本不完整）' }}</dt><dd>{{ signedMoney(summary.unrealizedPnl) }}</dd></div></dl>
            </section>
            <p v-if="summary.missingPrices || summary.stalePrices" class="cy-message cy-warning">{{ summary.missingPrices ? `${summary.missingPrices} 项资产缺少价格或汇率。` : '' }}{{ summary.stalePrices ? `${summary.stalePrices} 项使用过期报价，仅供参考。` : '' }} <button @click="openQuote()">填写手动报价</button></p>
            <div class="cy-chart-grid">
                <section class="cy-panel"><h2>资产分布</h2><p class="cy-field-note">按已估值的正资产计算；负债另列。</p><div v-if="allocation.length" class="cy-allocation-bar" aria-hidden="true"><span v-for="item in allocation" :key="item.name" :style="{ width: item.percent + '%', background: item.color }"></span></div><p v-else class="cy-empty">添加账户或持仓后显示资产分布。</p><dl class="cy-allocation-legend"><div v-for="item in allocation" :key="item.name"><dt><i :style="{ background: item.color }"></i>{{ item.name }}</dt><dd>{{ item.percent.toFixed(1) }}% <span>{{ money(item.value) }}</span></dd></div></dl><p class="cy-field-note">负债 {{ money(summary.liabilities) }}<template v-if="summary.missingPrices"> · 另有 {{ summary.missingPrices }} 项未估值</template></p></section>
                <section class="cy-panel"><h2>净资产变化</h2><p class="cy-field-note">从开始使用后保存的实际估值记录绘制。</p><template v-if="historyPlot.points.length > 1"><div class="cy-history-range"><span>{{ money(historyPlot.max) }}</span><span>{{ money(historyPlot.min) }}</span></div><svg class="cy-history-chart" viewBox="0 0 400 130" role="img" aria-label="实际净资产估值记录，缺失记录处留空"><title>实际净资产估值记录</title><path d="M8 122H392" stroke="var(--cy-line)" /><polyline v-for="(segment, index) in historyPlot.segments" :key="index" :points="segment" fill="none" stroke="var(--cy-teal)" stroke-width="2.5" stroke-linejoin="round" /><circle v-for="point in historyPlot.points" :key="point.id" :cx="point.x" :cy="point.y" r="3" fill="var(--cy-teal)"><title>{{ date(point.time) }} · {{ money(point.value) }}</title></circle></svg><div class="cy-history-dates"><span>{{ date(historyPlot.first) }}</span><span>{{ date(historyPlot.last) }}</span></div><p v-if="historyPlot.hasGaps" class="cy-field-note">不完整或失效的记录处已断开。</p></template><div v-else class="cy-empty">{{ historyPlot.points.length ? '已有 1 条完整估值，积累更多记录后显示趋势。' : '尚无完整历史估值，缺失数据不会用零补齐。' }}</div></section>
            </div>
            <section class="cy-panel"><div class="cy-section-heading"><div><h2>投资持仓</h2><p class="cy-muted">已实现净损益 {{ signedMoney(summary.realizedPnl) }}<span v-if="!summary.costComplete"> · 成本不完整</span></p></div><div class="cy-segment" aria-label="持仓分组"><button :aria-pressed="groupBy === 'account'" @click="groupBy = 'account'">按账户</button><button :aria-pressed="groupBy === 'asset'" @click="groupBy = 'asset'">按资产</button></div></div>
                <div v-if="!positions.length" class="cy-empty"><span class="cy-empty-mark">＋</span><h3>从已有持仓开始</h3><p>先创建投资账户，再录入数量。暂时不知道成本也可以保存。</p><button class="cy-button cy-primary" @click="accounts.length ? openEvent() : openEditor('account')">{{ accounts.length ? '录入期初持仓' : '创建投资账户' }}</button></div>
                <section v-for="group in positionGroups" :key="group.name" class="cy-position-group"><h3>{{ group.name }}</h3><article v-for="position in group.items" :key="position.accountId + position.instrumentId" class="cy-position"><div class="cy-position-top"><div><strong>{{ instrumentName(position.instrumentId) }}</strong><p class="cy-muted">{{ groupBy === 'account' ? instrumentSymbol(position.instrumentId) : accountName(position.accountId) }} · {{ quantity(position.quantity) }} {{ instrumentSymbol(position.instrumentId) }}</p></div><div class="cy-align-end"><strong class="cy-value">{{ money(position.marketValue) }}</strong><span class="cy-status" :class="{ 'cy-status-warning': !position.quote || ['stale','unavailable','missing'].includes(position.quote.state.toLowerCase()) || position.quote.fxState === 'stale' }">{{ quoteState(position) }}</span></div></div><dl class="cy-position-metrics"><div><dt>剩余成本</dt><dd>{{ position.costKnown ? money(position.cost) : '成本未知' }}</dd></div><div><dt>未实现盈亏</dt><dd>{{ position.costKnown ? signedMoney(position.unrealizedPnl) : '暂不计算' }}</dd></div><div><dt>平均单位成本</dt><dd>{{ position.costKnown ? money(position.averageCost) : '—' }}</dd></div><div><dt>已实现净损益</dt><dd>{{ position.costKnown ? signedMoney(position.realizedPnl) : '成本不完整' }}</dd></div></dl><details><summary>报价与持仓操作</summary><p v-if="position.quote" class="cy-field-note">单价 {{ position.quote.price }} {{ position.quote.currency }} · {{ position.quote.source }}<br />源报价 {{ date(position.quote.sourceTime) }}<br /><template v-if="position.quote.fxRate">估值汇率 {{ position.quote.fxRate }} · {{ position.quote.fxDate || '时间未提供' }}<br />{{ position.quote.fxSource || '直接以人民币估值' }}</template></p><p v-else class="cy-field-note">暂未取得可用报价，市值不会以零代替。</p><div class="cy-actions"><button class="cy-button" @click="openEvent('BUY', position)">买入</button><button class="cy-button" @click="openEvent('SELL', position)">卖出</button><button class="cy-button" @click="openEvent('TRANSFER', position)">转移</button><button class="cy-button" @click="openQuote(position.instrumentId)">手动报价</button></div></details></article></section>
            </section>
            <section class="cy-panel"><div class="cy-section-heading"><h2>日常账户</h2><button class="cy-button" @click="emit('navigate', 'accounts')">管理账户</button></div><p v-if="!cashAccounts.length" class="cy-empty">还没有日常账户。添加银行卡、现金或支付账户后，这里会显示实际余额。</p><div v-for="account in cashAccounts" :key="account.id" class="cy-cash-row"><div><strong>{{ account.name }}</strong><p class="cy-muted">{{ account.liability ? '负债账户' : '资金账户' }} · {{ account.currency }}</p></div><div class="cy-align-end"><strong>{{ money(account.value) }}</strong><small v-if="account.currency !== 'CNY'">原币 {{ account.balance }}</small></div></div></section>
        </template>

        <section v-if="tab === 'history'" class="cy-panel"><div class="cy-section-heading"><h2>投资流水</h2><button class="cy-button" :disabled="saving" @click="downloadCSV">导出 CSV</button></div><p class="cy-field-note">记录交易事实；修订与撤销均使用当前版本校验。</p><div v-if="!events.length" class="cy-empty">还没有投资流水。录入期初持仓或记录第一笔买入后，会在这里保留记录。</div><article v-for="event in sortedEvents" :key="event.id" class="cy-event" :class="{ 'cy-voided': event.voided }"><div class="cy-position-top"><div><strong>{{ eventNames[event.type] }} · {{ instrumentName(event.instrumentId) }}</strong><p class="cy-muted">{{ date(event.occurredAt) }} · {{ accountName(event.accountId) }}</p></div><span>{{ quantity(event.quantity) }} {{ instrumentSymbol(event.instrumentId) }}</span></div><p v-if="event.type === 'BUY' || event.type === 'SELL'" class="cy-field-note">成交金额 {{ event.amount }} · 手续费 {{ event.fee }} · 交易折算率 {{ event.exchangeRate }}</p><p v-if="event.note">{{ event.note }}</p><div class="cy-event-footer"><span class="cy-muted">版本 {{ event.version }}{{ event.voided ? ' · 已撤销' : '' }}</span><div v-if="!event.voided"><button class="cy-button" @click="reviseEvent(event)">修订</button><button class="cy-button" @click="askVoid(event)">撤销</button></div></div></article></section>

        <template v-if="tab === 'accounts'"><section class="cy-panel"><div class="cy-section-heading"><h2>投资账户</h2><button class="cy-button cy-primary" @click="openEditor('account')">＋ 创建账户</button></div><p v-if="!accounts.length" class="cy-empty">用账户区分交易所、个人钱包或证券账户。</p><div v-for="account in accounts" :key="account.id" class="cy-cash-row"><strong>{{ account.name }}</strong><span class="cy-muted">{{ accountKinds[account.kind] || account.kind }}</span></div></section><section class="cy-panel"><div class="cy-section-heading"><h2>资产目录</h2><button class="cy-button" @click="openEditor('instrument')">＋ 创建资产</button></div><p class="cy-field-note">自动行情仅用于已验证映射的资产；其他资产可录入手动报价。</p><div v-for="instrument in instrumentsList" :key="instrument.id" class="cy-cash-row"><div><strong>{{ instrument.name }}</strong><p class="cy-muted">{{ instrument.symbol }} · {{ typeNames[instrument.type] }}</p></div><button class="cy-button" @click="openQuote(instrument.id)">手动报价</button></div></section></template>

        <template v-if="tab === 'settings'"><section class="cy-panel"><h2>投资设置</h2><form @submit.prevent="saveSettings"><div class="cy-fields"><label>本位币<input value="人民币 CNY" readonly /></label><label>会计时区<input v-model="settings.timeZone" required list="cy-timezones" placeholder="Asia/Shanghai" /><datalist id="cy-timezones"><option v-for="zone in timezones" :key="zone" :value="zone" /></datalist><span class="cy-field-note">日期按此时区记录与展示，初始值来自你的设备。</span></label></div><div class="cy-actions"><button class="cy-button cy-primary" :disabled="saving">保存设置</button></div></form></section><section class="cy-panel"><h2>数据管理</h2><p class="cy-muted">分别导出投资流水（含费用）与当前持仓。完整恢复请使用服务端备份工具。</p><div class="cy-actions"><button class="cy-button" :disabled="saving" @click="downloadCSV">导出投资流水 CSV</button><button class="cy-button" @click="downloadPositions">导出当前持仓 CSV</button><button class="cy-button" @click="emit('navigate', 'data')">日常流水导入与导出</button></div></section><section class="cy-panel"><h2>使用后的资产变化</h2><p class="cy-field-note">仅显示当时实际保存的估值。修订交易后失效的记录保留缺口，不用今天的持仓补造历史。</p><p v-if="!snapshots.length" class="cy-empty">尚无历史估值记录，开始使用后逐步积累。</p><div v-for="snapshot in recentSnapshots" :key="snapshot.id" class="cy-cash-row"><span>{{ date(snapshot.recordedAt) }}</span><div class="cy-align-end"><strong>{{ snapshot.invalidated ? '待重建 / 缺少历史价格' : money(snapshot.netAssets ?? snapshot.valuedAssets) }}</strong><small>{{ snapshot.invalidated ? '历史修订后失效' : snapshot.complete ? '完整估值' : '部分估值' }}</small></div></div></section></template>
    </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, useTemplateRef, watch } from 'vue';
import Decimal from 'decimal.js';
import moment from 'moment-timezone';
import { investments, investmentError } from '@/lib/investments.ts';
import { generateRandomUUID } from '@/lib/misc.ts';
import { getCurrentUserInfo } from '@/lib/userstate.ts';
import { useAccountsStore } from '@/stores/account.ts';
import { useTransactionsStore } from '@/stores/transaction.ts';
import { useOverviewStore } from '@/stores/overview.ts';
import { useStatisticsStore } from '@/stores/statistics.ts';
import type { Instrument, InvestmentAccount, InvestmentEvent, InvestmentEventType, InvestmentPosition, InvestmentPreview, InvestmentSettings, WealthSnapshot, WealthSummary } from '@/models/investment.ts';

const DecimalValue = Decimal.clone({ precision: 80 });
const dailyAccountsStore = useAccountsStore();
const dailyTransactionsStore = useTransactionsStore();
const dailyOverviewStore = useOverviewStore();
const dailyStatisticsStore = useStatisticsStore();
const emit = defineEmits<{ navigate: [destination: 'accounts' | 'data'] }>();
const tabs = [{ key: 'overview', name: '资产总览' }, { key: 'history', name: '投资流水' }, { key: 'accounts', name: '账户与资产' }, { key: 'settings', name: '设置与数据' }];
const tab = ref('overview');
const groupBy = ref('account');
const summary = ref<WealthSummary | null>(null);
const accounts = ref<InvestmentAccount[]>([]);
const instrumentsList = ref<Instrument[]>([]);
const events = ref<InvestmentEvent[]>([]);
const snapshots = ref<WealthSnapshot[]>([]);
const settings = reactive<InvestmentSettings>({ baseCurrency: 'CNY', timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'Asia/Shanghai' });
const timezones = moment.tz.names();
let settingsInitialized = false;
const loading = ref(false), saving = ref(false), error = ref(''), notice = ref(''), editorError = ref('');
const editor = ref('');
const editorElement = useTemplateRef<HTMLElement>('editorElement');
const eventNames: Record<InvestmentEventType, string> = { OPENING: '期初持仓', BUY: '买入', SELL: '卖出', TRANSFER: '账户间转移' };
const typeNames: Record<string, string> = { CRYPTO: '加密资产', STOCK: '股票', FUND: '基金', OTHER: '其他资产' };
const accountKinds: Record<string, string> = { EXCHANGE: '交易所', WALLET: '个人钱包', BROKER: '证券账户', OTHER: '其他' };
const editorTitle = computed(() => ({ event: draft.id ? '修订投资流水' : '记录投资', account: '创建投资账户', instrument: '创建私人资产', quote: '填写手动报价', void: '撤销投资流水' }[editor.value] || ''));
async function initializeSettings(initial?: InvestmentSettings): Promise<void> { const stored = initial || await investments.settings(); if (stored.timeZone) Object.assign(settings, stored); else Object.assign(settings, await investments.saveSettings({ ...settings })); settingsInitialized = true; }
function freshEvent(): InvestmentEvent { return { id: '', type: 'OPENING', accountId: '', instrumentId: '', toAccountId: '', quantity: '', amount: '0', fee: '0', cost: null, settlementInstrumentId: '', settlementAccountId: '', cashAccountId: '', exchangeRate: '1', occurredAt: 0, note: '', version: 0, voided: false }; }
const draft = reactive<InvestmentEvent>(freshEvent());
const costInput = ref(''), eventTime = ref(''), settlementMode = ref('cash');
const preview = ref<InvestmentPreview | null>(null);
const idempotencyKey = ref('');
const localDraftKey = `cyledger_investment_draft:${getCurrentUserInfo()?.username || ''}`;
const hasLocalDraft = ref(!!localStorage.getItem(localDraftKey));
const voidCandidate = ref<InvestmentEvent | null>(null);
const accountDraft = reactive({ name: '', kind: 'EXCHANGE' });
const instrumentDraft = reactive({ name: '', symbol: '', type: 'STOCK' });
const quoteDraft = reactive({ instrumentId: '', price: '' });
const quoteTime = ref('');
const cashAccounts = computed(() => summary.value?.cashAccounts || []);
const positions = computed(() => (summary.value?.positions || []).filter(position => new DecimalValue(position.quantity).gt(0)));
const sortedEvents = computed(() => [...events.value].sort((a, b) => b.occurredAt - a.occurredAt));
const recentSnapshots = computed(() => [...snapshots.value].sort((a, b) => b.recordedAt - a.recordedAt).slice(0, 30));
const selectedInstrument = computed(() => instrumentsList.value.find(item => item.id === draft.instrumentId));
const settlementUnit = computed(() => settlementMode.value === 'asset' ? instrumentSymbol(draft.settlementInstrumentId) : cashAccounts.value.find(item => item.id === draft.cashAccountId)?.currency || 'CNY');
const allocation = computed(() => {
    const values = new Map<string, Decimal>();
    if (summary.value && new DecimalValue(summary.value.cashAssets).gt(0)) values.set('日常资产', new DecimalValue(summary.value.cashAssets));
    for (const p of positions.value) {
        if (!p.marketValue || new DecimalValue(p.marketValue).lte(0)) continue;
        const name = typeNames[instrumentsList.value.find(item => item.id === p.instrumentId)?.type || 'OTHER'] || '其他资产';
        values.set(name, (values.get(name) || new DecimalValue(0)).plus(p.marketValue));
    }
    const total = [...values.values()].reduce((a, b) => a.plus(b), new DecimalValue(0));
    const colors = ['#12786f', '#68a8a0', '#8b91b2', '#cba56a', '#88958e'];
    return [...values].map(([name, value], index) => ({ name, value: value.toFixed(), percent: total.gt(0) ? value.div(total).mul(100).toNumber() : 0, color: colors[index % colors.length] }));
});
const historyPlot = computed(() => {
    const rows = [...snapshots.value].sort((a, b) => a.recordedAt - b.recordedAt);
    const valid = rows.filter(row => row.complete && !row.invalidated && row.netAssets !== null);
    const amounts = valid.map(row => new DecimalValue(row.netAssets!));
    const min = amounts.length ? DecimalValue.min(...amounts) : new DecimalValue(0);
    const max = amounts.length ? DecimalValue.max(...amounts) : new DecimalValue(0);
    const span = max.minus(min);
    const first = rows[0]?.recordedAt || 0, last = rows.at(-1)?.recordedAt || first;
    const segments: string[] = []; let segment: string[] = [];
    const points: { id: string; x: number; y: number; time: number; value: string }[] = [];
    for (const row of rows) {
        if (!row.complete || row.invalidated || row.netAssets === null) { if (segment.length) segments.push(segment.join(' ')); segment = []; continue; }
        const x = last === first ? 200 : 8 + (row.recordedAt - first) / (last - first) * 384;
        const y = span.isZero() ? 64 : 122 - new DecimalValue(row.netAssets).minus(min).div(span).mul(112).toNumber();
        segment.push(`${x},${y}`); points.push({ id: row.id, x, y, time: row.recordedAt, value: row.netAssets });
    }
    if (segment.length) segments.push(segment.join(' '));
    return { segments, points, min: min.toFixed(), max: max.toFixed(), first, last, hasGaps: valid.length !== rows.length };
});
const positionGroups = computed(() => {
    const groups = new Map<string, InvestmentPosition[]>();
    for (const position of positions.value) {
        const key = groupBy.value === 'account' ? accountName(position.accountId) : typeNames[instrumentsList.value.find(item => item.id === position.instrumentId)?.type || 'OTHER'] || '其他资产';
        groups.set(key, [...(groups.get(key) || []), position]);
    }
    return Array.from(groups, ([name, items]) => ({ name, items }));
});
const affectedPreview = computed(() => (preview.value?.positions || []).filter(item => [draft.accountId, draft.toAccountId, draft.settlementAccountId].includes(item.accountId) && [draft.instrumentId, draft.settlementInstrumentId].includes(item.instrumentId)));
const transferFeeValue = computed(() => draft.exchangeRate ? money(new DecimalValue(draft.fee || '0').mul(draft.exchangeRate)) : '未知');
const cashEffect = computed(() => {
    if (!preview.value || !['BUY', 'SELL'].includes(draft.type)) return '';
    const amount = new DecimalValue(draft.amount || '0');
    const net = draft.type === 'BUY' ? amount.plus(draft.fee || '0') : amount.minus(draft.fee || '0');
    return `${settlementMode.value === 'cash' ? cashAccounts.value.find(item => item.id === draft.cashAccountId)?.name || '资金账户' : accountName(draft.settlementAccountId)} ${draft.type === 'BUY' ? '扣除' : '增加'} ${net.toFixed()} ${settlementUnit.value}（含手续费），折算 ${money(net.mul(draft.exchangeRate))}。`;
});
function money(value: string | Decimal | null | undefined): string { if (value === null || value === undefined || value === '') return '—'; try { const amount = new DecimalValue(value); const parts = amount.abs().toFixed(2).split('.'); return `${amount.isNegative() ? '-' : ''}¥${parts[0]!.replace(/\B(?=(\d{3})+(?!\d))/g, ',')}.${parts[1]}`; } catch { return '—'; } }
function signedMoney(value: string | null | undefined): string { if (value === null || value === undefined) return '—'; try { return `${new DecimalValue(value).gt(0) ? '+' : ''}${money(value)}`; } catch { return '—'; } }
function quantity(value: string): string { try { return new DecimalValue(value).toFixed(); } catch { return value; } }
function date(value: number): string { return value ? moment.unix(value).tz(settings.timeZone).format('YYYY-MM-DD HH:mm') : '时间未提供'; }
function nowLocal(): string { return moment().tz(settings.timeZone).format('YYYY-MM-DDTHH:mm'); }
function accountName(id: string): string { return accounts.value.find(item => item.id === id)?.name || id; }
function instrumentName(id: string): string { return instrumentsList.value.find(item => item.id === id)?.name || id; }
function instrumentSymbol(id: string): string { return instrumentsList.value.find(item => item.id === id)?.symbol || '资产'; }
function quoteState(position: InvestmentPosition): string { if (!position.quote) return '暂时无法估值'; if (position.quote.fxState === 'stale') return '估值汇率过期'; if (!position.quote.fxRate) return '缺少估值汇率'; const state = position.quote.state.toUpperCase(); return ({ LIVE: '实时参考报价', REALTIME: '实时参考报价', FRESH: '实时参考报价', DELAYED: '延迟参考报价', MANUAL: '手动估值', STALE: '行情过期', UNAVAILABLE: '暂时无法估值', MISSING: '暂时无法估值' } as Record<string, string>)[state] || '参考报价'; }
function validDecimal(value: string, label: string, allowZero = true): void { if (!/^\d+(\.\d{1,18})?$/.test(value) || (!allowZero && new DecimalValue(value).lte(0))) throw new Error(`${label}应为${allowZero ? '非负' : '大于零的'}十进制数字，最多 18 位小数`); }
function parseTime(value: string): number { const parsed = moment.tz(value, 'YYYY-MM-DDTHH:mm', true, settings.timeZone); if (!parsed.isValid()) throw new Error('请选择有效的发生时间'); return parsed.unix(); }
async function load(): Promise<void> { if (loading.value) return; loading.value = true; error.value = ''; try { const result = await Promise.all([investments.summary(), investments.accounts(), investments.instruments(), investments.events(), investments.settings(), investments.history()]); summary.value = result[0]; accounts.value = result[1]; instrumentsList.value = result[2]; events.value = result[3]; await initializeSettings(result[4]); snapshots.value = result[5]; } catch (cause) { error.value = investmentError(cause); } finally { loading.value = false; } }
async function openEditor(value: string): Promise<void> { editor.value = value; editorError.value = ''; notice.value = ''; await nextTick(); editorElement.value?.focus({ preventScroll: true }); editorElement.value?.scrollIntoView({ behavior: 'smooth', block: 'start' }); }
function closeEditor(): void { if (saving.value) return; editor.value = ''; preview.value = null; editorError.value = ''; }
function openEvent(type: InvestmentEventType = 'OPENING', position?: InvestmentPosition): void { Object.assign(draft, freshEvent(), { type, exchangeRate: type === 'TRANSFER' ? '' : '1', accountId: position?.accountId || accounts.value[0]?.id || '', instrumentId: position?.instrumentId || instrumentsList.value[0]?.id || '' }); costInput.value = ''; eventTime.value = nowLocal(); settlementMode.value = 'cash'; preview.value = null; idempotencyKey.value = generateRandomUUID(); void openEditor('event'); }
function reviseEvent(event: InvestmentEvent): void { Object.assign(draft, freshEvent(), event); costInput.value = event.cost ?? ''; eventTime.value = moment.unix(event.occurredAt).tz(settings.timeZone).format('YYYY-MM-DDTHH:mm'); settlementMode.value = event.settlementInstrumentId ? 'asset' : 'cash'; preview.value = null; idempotencyKey.value = generateRandomUUID(); void openEditor('event'); }
function openQuote(instrumentId?: string): void { quoteDraft.instrumentId = instrumentId || instrumentsList.value[0]?.id || ''; quoteDraft.price = ''; quoteTime.value = nowLocal(); void openEditor('quote'); }
function askVoid(event: InvestmentEvent): void { voidCandidate.value = event; void openEditor('void'); }
async function previewEvent(): Promise<void> { saving.value = true; editorError.value = ''; try { if (!settingsInitialized) await initializeSettings(); validDecimal(draft.quantity, '数量', false); if (draft.type === 'TRANSFER' && draft.exchangeRate) validDecimal(draft.exchangeRate, '参考单价', false); validDecimal(draft.fee, '手续费'); if (['BUY', 'SELL'].includes(draft.type)) { validDecimal(draft.amount, '成交金额', false); validDecimal(draft.exchangeRate, '折算率', false); } if (draft.type === 'OPENING' && costInput.value) validDecimal(costInput.value, '成本'); draft.cost = draft.type === 'OPENING' && costInput.value ? costInput.value : null; draft.occurredAt = parseTime(eventTime.value); if (!['BUY', 'SELL'].includes(draft.type)) { draft.cashAccountId = ''; draft.settlementAccountId = ''; draft.settlementInstrumentId = ''; draft.amount = '0'; if (draft.type === 'OPENING') draft.exchangeRate = '1'; } else if (settlementMode.value === 'cash') { draft.settlementAccountId = ''; draft.settlementInstrumentId = ''; } else { draft.cashAccountId = ''; } if (draft.type !== 'TRANSFER') draft.toAccountId = ''; if (draft.type === 'OPENING') draft.fee = '0'; preview.value = await investments.preview({ ...draft }); } catch (cause) { editorError.value = investmentError(cause); } finally { saving.value = false; } }
async function mutation(action: () => Promise<unknown>, message: string): Promise<void> { if (saving.value) return; saving.value = true; editorError.value = ''; error.value = ''; try { await action(); notice.value = message; saving.value = false; closeEditor(); await load(); } catch (cause) { if (editor.value) editorError.value = investmentError(cause); else error.value = investmentError(cause); } finally { saving.value = false; } }
function invalidateDailyData(): void { dailyAccountsStore.updateAccountListInvalidState(true); dailyTransactionsStore.updateTransactionListInvalidState(true); dailyTransactionsStore.updateTransactionReconciliationStatementInvalidState(true); dailyOverviewStore.updateTransactionOverviewInvalidState(true); dailyStatisticsStore.updateTransactionStatisticsInvalidState(true); }
function saveEvent(): Promise<void> { return mutation(async () => { await investments.saveEvent({ ...draft }, idempotencyKey.value); invalidateDailyData(); discardLocalDraft(); }, draft.id ? '投资流水已修订，持仓与成本已重新计算。' : '投资流水已入账。'); }
function createAccount(): Promise<void> { return mutation(async () => { await investments.createAccount(accountDraft.name.trim(), accountDraft.kind); accountDraft.name = ''; }, '投资账户已创建。'); }
function createInstrument(): Promise<void> { return mutation(async () => { await investments.createInstrument({ ...instrumentDraft, name: instrumentDraft.name.trim(), symbol: instrumentDraft.symbol.trim() }); instrumentDraft.name = ''; instrumentDraft.symbol = ''; }, '私人资产已创建，可录入持仓与手动报价。'); }
function saveQuote(): Promise<void> { return mutation(async () => { validDecimal(quoteDraft.price, '价格', false); await investments.manualQuote(quoteDraft.instrumentId, quoteDraft.price, parseTime(quoteTime.value)); }, '手动报价已保存。'); }
function restoreAutomaticQuote(): Promise<void> { return mutation(() => investments.automaticQuote(quoteDraft.instrumentId), '已移除手动报价。支持的资产将使用自动行情；不支持的资产显示为暂未估值。'); }
function voidEvent(): Promise<void> { return mutation(async () => { if (voidCandidate.value) { await investments.voidEvent(voidCandidate.value); invalidateDailyData(); } }, '投资流水已撤销，关联资金与持仓已重算。'); }
function saveSettings(): Promise<void> { return mutation(async () => { if (!moment.tz.zone(settings.timeZone)) throw new Error('请输入有效时区，例如 Asia/Shanghai'); await investments.saveSettings({ ...settings }); }, '投资设置已保存。'); }
async function downloadCSV(): Promise<void> { saving.value = true; error.value = ''; try { const csv = await investments.export(); const url = URL.createObjectURL(new Blob(['\uFEFF', csv.replace(/^\uFEFF/, '')], { type: 'text/csv;charset=utf-8;' })); const link = document.createElement('a'); link.href = url; link.download = `CYLedger-投资业务-${moment().format('YYYY-MM-DD')}.csv`; document.body.appendChild(link); link.click(); link.remove(); setTimeout(() => URL.revokeObjectURL(url), 1000); notice.value = '投资业务 CSV 已生成。'; } catch (cause) { error.value = investmentError(cause); } finally { saving.value = false; } }
function saveLocalDraft(): void {
    try {
        localStorage.setItem(localDraftKey, JSON.stringify({ event: { ...draft }, cost: costInput.value, time: eventTime.value, settlementMode: settlementMode.value, requestKey: idempotencyKey.value }));
        hasLocalDraft.value = true;
        notice.value = '草稿已保存在此浏览器，尚未同步、尚未入账。联网后仍需预览并确认。退出登录时会清除此草稿。';
    } catch { editorError.value = '浏览器无法保存草稿，请保持页面打开并在联网后重试。'; }
}
function discardLocalDraft(): void { localStorage.removeItem(localDraftKey); hasLocalDraft.value = false; }
function restoreLocalDraft(): void {
    try {
        const saved = JSON.parse(localStorage.getItem(localDraftKey) || 'null') as { event: InvestmentEvent; cost: string; time: string; settlementMode: string; requestKey: string } | null;
        if (!saved?.event) return;
        Object.assign(draft, freshEvent(), saved.event); costInput.value = saved.cost; eventTime.value = saved.time; settlementMode.value = saved.settlementMode; idempotencyKey.value = saved.requestKey || generateRandomUUID(); preview.value = null; void openEditor('event');
    } catch { discardLocalDraft(); error.value = '本地草稿无法读取，请重新录入。'; }
}
function downloadPositions(): void {
    const safe = (value: unknown): string => {
        const raw = value === null || value === undefined ? '' : String(value);
        const escaped = /^[=+@\t\r]/.test(raw) ? `'${raw}` : raw;
        return `"${escaped.replace(/"/g, '""')}"`;
    };
    const rows: unknown[][] = [['投资账户', '资产ID', '资产名称', '代码', '数量', '剩余成本CNY', '成本已知', '平均成本CNY', '已实现净损益CNY', '市值CNY', '未实现盈亏CNY', '报价状态', '报价来源', '源报价时间', '估值汇率', '汇率日期']];
    for (const p of summary.value?.positions || []) rows.push([accountName(p.accountId), p.instrumentId, instrumentName(p.instrumentId), instrumentSymbol(p.instrumentId), p.quantity, p.cost, p.costKnown, p.averageCost, p.realizedPnl, p.marketValue, p.unrealizedPnl, quoteState(p), p.quote?.source, p.quote?.sourceTime ? date(p.quote.sourceTime) : '', p.quote?.fxRate, p.quote?.fxDate]);
    const url = URL.createObjectURL(new Blob(['\uFEFF', rows.map(row => row.map(safe).join(',')).join('\r\n')], { type: 'text/csv;charset=utf-8;' }));
    const link = document.createElement('a'); link.href = url; link.download = `CYLedger-当前持仓-${moment().format('YYYY-MM-DD')}.csv`; document.body.appendChild(link); link.click(); link.remove(); setTimeout(() => URL.revokeObjectURL(url), 1000);
}
watch(() => draft.type, () => { preview.value = null; });
let timer: ReturnType<typeof setInterval> | undefined;
onMounted(() => { void load(); timer = setInterval(async () => { if (document.hidden || saving.value || loading.value || editor.value) return; try { summary.value = await investments.summary(); } catch (cause) { error.value = investmentError(cause); } }, 10000); });
onUnmounted(() => { if (timer) clearInterval(timer); });
defineExpose({ reload: load });
</script>

<style scoped>
.cy-wealth{--cy-teal:#12786f;--cy-ink:#223d3a;--cy-muted:#687875;--cy-line:#dfe8e5;--cy-surface:#fff;--cy-soft:#f3f7f6;--cy-warn:#8b5c18;color:var(--cy-ink);font-family:"Segoe UI","Microsoft YaHei",sans-serif;width:100%;max-width:1160px;margin:auto;padding:26px 28px 50px;font-size:14px;line-height:1.6;box-sizing:border-box}.cy-wealth *{box-sizing:border-box}.cy-wealth h1,.cy-wealth h2,.cy-wealth h3,.cy-wealth p,.cy-wealth dl,.cy-wealth dd{margin:0}.cy-wealth h1{font-size:clamp(23px,2.4vw,32px);font-weight:650;letter-spacing:-.6px}.cy-wealth h2{font-size:18px;font-weight:650}.cy-wealth h3{font-size:15px}.cy-wealth button,.cy-wealth input,.cy-wealth select,.cy-wealth textarea{font:inherit}.cy-wealth button{cursor:pointer;width:auto;display:inline-flex;align-items:center;justify-content:center;margin:0}.cy-wealth button:disabled{cursor:wait;opacity:.55}.cy-wealth button:focus-visible,.cy-wealth summary:focus-visible{outline:3px solid #7fc7bd;outline-offset:3px}.cy-heading,.cy-section-heading,.cy-position-top,.cy-cash-row,.cy-event-footer{display:flex;align-items:center;justify-content:space-between;gap:16px}.cy-heading{margin:0 0 28px;align-items:center}.cy-eyebrow{color:var(--cy-teal);font-size:10px;font-weight:700;letter-spacing:2px;margin-bottom:8px!important}.cy-muted,.cy-field-note{color:var(--cy-muted)}.cy-field-note{font-size:12px;line-height:1.7}.cy-heading .cy-muted{margin-top:7px}.cy-button{min-height:44px;border:1px solid var(--cy-line);border-radius:8px;background:var(--cy-surface);color:var(--cy-ink);padding:9px 15px;font-weight:600;white-space:normal}.cy-primary{background:var(--cy-teal);border-color:var(--cy-teal);color:#fff}.cy-danger{background:#9f3f32;border-color:#9f3f32;color:white}.cy-button:hover{filter:brightness(.96)}.cy-tabs{display:flex;gap:6px;border-bottom:1px solid var(--cy-line);margin-bottom:24px;flex-wrap:wrap}.cy-tabs button{background:none;border:0;border-bottom:3px solid transparent;padding:11px 14px;color:var(--cy-muted);min-height:48px}.cy-tabs button[aria-current]{border-bottom-color:var(--cy-teal);color:var(--cy-teal);font-weight:700}.cy-tabs .cy-refresh{margin-left:auto;font-size:12px}.cy-panel{border:1px solid var(--cy-line);background:var(--cy-surface);border-radius:12px;padding:24px;margin-bottom:20px;min-width:0}.cy-section-heading{margin-bottom:18px;flex-wrap:wrap}.cy-section-heading .cy-muted{font-size:12px;margin-top:4px}.cy-overview{display:grid;grid-template-columns:1.05fr 1.2fr;gap:30px;border-top:3px solid var(--cy-teal);padding-top:28px;padding-bottom:28px}.cy-net{border-right:1px solid var(--cy-line);padding-right:20px}.cy-currency{font-size:10px;border:1px solid var(--cy-line);padding:1px 5px;border-radius:4px;margin-left:6px}.cy-total{font-family:"Bahnschrift","Segoe UI",sans-serif;font-variant-numeric:tabular-nums;font-size:clamp(27px,3.8vw,43px);line-height:1.3;font-weight:500;letter-spacing:-1.2px;overflow-wrap:anywhere;margin:8px 0!important}.cy-summary-grid,.cy-position-metrics{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:18px 24px}.cy-wealth dt{color:var(--cy-muted);font-size:12px}.cy-wealth dd{font-variant-numeric:tabular-nums;overflow-wrap:anywhere;font-size:17px;font-weight:600}.cy-summary-grid dd{margin-top:5px}.cy-segment{display:flex;background:var(--cy-soft);border-radius:7px;padding:3px}.cy-segment button{border:0;min-height:38px;padding:6px 12px;background:none;border-radius:5px;color:var(--cy-muted);font-size:12px}.cy-segment button[aria-pressed=true]{background:var(--cy-surface);color:var(--cy-teal);box-shadow:0 1px 3px #00000010}.cy-position-group>h3{padding:13px 0 9px;color:var(--cy-muted);font-weight:500;font-size:12px;border-bottom:1px solid var(--cy-line)}.cy-position{padding:20px 0;border-bottom:1px solid var(--cy-line)}.cy-position:last-child{border-bottom:0;padding-bottom:0}.cy-position-top{align-items:start;flex-wrap:wrap}.cy-position-top strong{font-size:16px}.cy-position-top .cy-muted{font-size:12px;margin-top:3px}.cy-align-end{text-align:right;display:flex;flex-direction:column;max-width:100%;overflow-wrap:anywhere}.cy-value{font-variant-numeric:tabular-nums;font-size:21px!important}.cy-status{font-size:10px;color:var(--cy-teal);margin-top:3px}.cy-status-warning{color:var(--cy-warn)}.cy-position-metrics{grid-template-columns:repeat(4,minmax(0,1fr));margin-top:18px!important}.cy-position-metrics dd{font-size:14px;font-weight:500;margin-top:4px}.cy-position details{margin-top:12px}.cy-position summary{color:var(--cy-muted);cursor:pointer;font-size:12px;padding:10px 0;min-height:40px}.cy-cash-row{padding:15px 0;border-bottom:1px solid var(--cy-line);overflow-wrap:anywhere}.cy-cash-row:last-child{border-bottom:0}.cy-cash-row .cy-muted,.cy-cash-row small{font-size:12px}.cy-cash-row>div{min-width:0}.cy-empty{padding:40px 12px;text-align:center;color:var(--cy-muted)}.cy-empty h3{color:var(--cy-ink);font-size:18px;margin:6px 0}.cy-empty p{max-width:360px;margin:8px auto 20px}.cy-empty-mark{width:44px;height:44px;display:inline-flex;align-items:center;justify-content:center;border-radius:50%;background:var(--cy-soft);color:var(--cy-teal);font-size:26px}.cy-fields{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:18px;border:0;padding:0;margin:18px 0;min-width:0}.cy-fields label{display:flex;flex-direction:column;gap:6px;color:var(--cy-ink);font-size:13px;min-width:0}.cy-fields input,.cy-fields select,.cy-fields textarea{display:block;width:100%;min-width:0;min-height:46px;padding:11px 12px;border:1px solid #c9d8d3;border-radius:7px;background:var(--cy-surface);color:var(--cy-ink);outline:none;-webkit-appearance:auto}.cy-fields input:focus,.cy-fields select:focus,.cy-fields textarea:focus{border-color:var(--cy-teal);box-shadow:0 0 0 3px #12786f15}.cy-fields input:read-only{background:var(--cy-soft)}.cy-full{grid-column:1/-1}.cy-actions{display:flex;gap:10px;align-items:center;flex-wrap:wrap;margin-top:18px}.cy-message{padding:12px 16px;border-radius:7px;background:#eaf5f2;color:#236959;margin-bottom:16px!important;font-size:13px;overflow-wrap:anywhere}.cy-message button{border:0;background:transparent;color:inherit;text-decoration:underline;padding:4px;min-height:36px}.cy-error{background:#fbeeea;color:#9b4030}.cy-warning{background:#faf2e3;color:var(--cy-warn)}.cy-editor{border-color:#7eafa6;scroll-margin-top:20px}.cy-editor:focus{outline:none}.cy-preview{background:var(--cy-soft);padding:20px;border-radius:9px}.cy-preview h3{font-size:17px;margin-bottom:12px}.cy-preview>p{margin:8px 0}.cy-preview-positions{display:grid;gap:10px;margin:18px 0}.cy-preview-positions>div{display:flex;flex-wrap:wrap;gap:5px 20px;padding:12px;border:1px solid var(--cy-line);border-radius:6px;font-size:12px}.cy-preview-positions strong{width:100%;font-size:13px}.cy-event{padding:22px 0;border-bottom:1px solid var(--cy-line)}.cy-event:last-child{border-bottom:0}.cy-event>.cy-field-note{margin:9px 0}.cy-event-footer{margin-top:12px;font-size:12px}.cy-event-footer .cy-button{margin-left:6px;font-size:12px;min-height:40px;padding:6px 12px}.cy-voided{opacity:.65}.cy-wealth small{color:var(--cy-muted)}
.cy-chart-grid{display:grid;grid-template-columns:1fr 1fr;gap:20px}.cy-allocation-bar{height:14px;display:flex;overflow:hidden;border-radius:5px;margin:22px 0 18px}.cy-allocation-legend>div{display:flex;align-items:center;justify-content:space-between;gap:10px;margin:9px 0}.cy-allocation-legend dt{display:flex;align-items:center;gap:7px}.cy-allocation-legend i{width:7px;height:7px;border-radius:50%;display:inline-block}.cy-allocation-legend dd{font-size:12px;font-weight:500}.cy-allocation-legend dd span{color:var(--cy-muted);margin-left:8px}.cy-history-range,.cy-history-dates{display:flex;justify-content:space-between;font-size:10px;color:var(--cy-muted);gap:8px}.cy-history-range{margin-top:20px}.cy-history-chart{width:100%;display:block;margin:5px 0}.cy-chart-grid .cy-empty{padding:24px 0}.cy-chart-grid h2{margin-bottom:4px}
@media(max-width:850px){.cy-chart-grid{grid-template-columns:1fr;gap:0}}
@media(max-width:700px){.cy-wealth{padding:20px 14px 32px}.cy-heading{align-items:flex-start;flex-wrap:wrap;margin-bottom:18px;gap:13px}.cy-heading h1{font-size:25px}.cy-heading>.cy-button{padding:8px 14px}.cy-tabs{gap:0;margin-bottom:18px}.cy-tabs button{padding:10px 7px;font-size:12px}.cy-tabs .cy-refresh{padding:10px 5px}.cy-panel{padding:18px 16px;border-radius:10px;margin-bottom:14px}.cy-overview{grid-template-columns:1fr;gap:20px}.cy-net{border:0;border-bottom:1px solid var(--cy-line);padding:0 0 20px}.cy-total{font-size:36px}.cy-summary-grid{gap:18px}.cy-summary-grid dd{font-size:17px}.cy-position-metrics{grid-template-columns:repeat(2,minmax(0,1fr));gap:13px}.cy-fields{grid-template-columns:1fr;gap:16px}.cy-fields input,.cy-fields select,.cy-fields textarea{font-size:16px}.cy-section-heading{gap:12px}.cy-section-heading h2{font-size:17px}.cy-preview{padding:16px}.cy-event-footer{flex-wrap:wrap}.cy-position-top>div{min-width:0;overflow-wrap:anywhere}.cy-cash-row{gap:12px}.cy-cash-row .cy-button{padding:7px 10px;font-size:12px;flex-shrink:0}}
:global(.dark) .cy-wealth,:global(.v-theme--dark) .cy-wealth{--cy-teal:#62c9b7;--cy-ink:#dce9e5;--cy-muted:#a0b5ae;--cy-line:#354a43;--cy-surface:#1c2b26;--cy-soft:#253a31;--cy-warn:#ecc689}.dark .cy-primary{color:#163d33}@media(prefers-reduced-motion:reduce){.cy-wealth *{scroll-behavior:auto!important}}
</style>
