import type { LedgerEntry } from '@/lib/mobile-ledger.ts';
import moment from 'moment-timezone';
import {zipSync,strToU8} from 'fflate';
import services from '@/lib/services.ts';

export function downloadLedgerFile(data: BlobPart, filename: string, type: string): void {
    const url = URL.createObjectURL(new Blob([data], { type }));
    const anchor = document.createElement('a'); anchor.classList.add('external'); anchor.href = url; anchor.download = filename;
    document.body.append(anchor); anchor.click(); setTimeout(()=>anchor.remove(),5000);
    // Android reads blob downloads asynchronously through the system document picker.
    setTimeout(() => URL.revokeObjectURL(url), 120_000);
}
function csvCell(value: string, numeric = false): string {
    const protectedValue = !numeric && /^[=+\-@\t\r]/.test(value) ? `'${value}` : value;
    return `"${protectedValue.replaceAll('"', '""')}"`;
}
export function ledgerEntryRows(entries: LedgerEntry[], timeZone: string): string[][] {
    const rows = [['账单ID','账本','日期','类型','分类','子分类','账户','币种','金额','账户2','账户2币种','账户2金额','优惠','手续费','标签','备注','计入收支','地点','约定还款日期']];
    for (const item of entries) rows.push([item.id,item.bookName || '',moment.unix(item.time).tz(timeZone).format('YYYY-MM-DD HH:mm:ss'),({1:'余额调整',2:'收入',3:'支出',4:'转账'} as Record<number,string>)[item.type] || '',item.primaryCategory,item.title,item.sourceAccountName||item.account,item.currency,item.amount,item.destinationAccount || '',item.destinationCurrency||'',item.destinationAmount||'',item.discountAmount||'0',item.transferFeeAmount||'0',item.tags.join(';'),item.comment,item.excludeFromStatistics?'否':'是',item.location||'',item.debtDueDate||'']);
    return rows;
}
export function ledgerEntriesCSV(entries: LedgerEntry[], timeZone: string): string {
    const rows=ledgerEntryRows(entries,timeZone);
    return '\ufeff'+rows.map((row,index) => row.map((cell,column) => csvCell(cell, index > 0 && [8,11,12,13].includes(column))).join(',')).join('\r\n');
}
export function exportLedgerEntries(entries: LedgerEntry[], timeZone: string): void {
    downloadLedgerFile(ledgerEntriesCSV(entries,timeZone),`CYLedger-账单-${moment().format('YYYYMMDD-HHmmss')}.csv`,'text/csv;charset=utf-8');
}

function xml(value:string):string{return value.replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;').replace(/[\u0000-\u0008\u000B\u000C\u000E-\u001F]/g,'');}
export function ledgerWorkbook(rows:string[][]):Uint8Array {
    // Inline strings preserve decimal precision and cannot execute spreadsheet formulas.
    const sheet='<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews><sheetData>'+rows.map((row,i)=>`<row r="${i+1}">`+row.map(value=>`<c t="inlineStr"><is><t xml:space="preserve">${xml(value)}</t></is></c>`).join('')+'</row>').join('')+'</sheetData></worksheet>';
    const files:Record<string,Uint8Array>={};
    files['[Content_Types].xml']=strToU8('<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>');
    files['_rels/.rels']=strToU8('<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>');
    files['xl/workbook.xml']=strToU8('<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="账本导出" sheetId="1" r:id="rId1"/></sheets></workbook>');
    files['xl/_rels/workbook.xml.rels']=strToU8('<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>');
    files['xl/worksheets/sheet1.xml']=strToU8(sheet);return zipSync(files,{level:6});
}
export function exportLedgerWorkbook(rows:string[][],name='账单'):void{const bytes=ledgerWorkbook(rows);downloadLedgerFile(bytes.slice().buffer,`CYLedger-${name}-${moment().format('YYYYMMDD-HHmmss')}.xlsx`,'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet');}
export async function exportLedgerPictures(entries:LedgerEntry[],timeZone:string,progress:(value:string)=>void):Promise<void>{
    const files:Record<string,Uint8Array>={},pictures=entries.flatMap(e=>(e.pictures||[]).map(p=>({entry:e,picture:p})));let total=0;
    if(!pictures.length)throw Error('所选范围没有账单图片');
    for(let i=0;i<pictures.length;i++){const item=pictures[i]!;progress(`正在整理图片 ${i+1} / ${pictures.length}`);const response=await fetch(services.getTransactionPictureUrlWithToken(item.picture.originalUrl));if(!response.ok)throw Error('账单图片读取失败，请重试');const bytes=new Uint8Array(await response.arrayBuffer());total+=bytes.length;if(total>18*1024*1024)throw Error('图片超过 18 MB，请缩小日期范围分批导出');const ext=response.headers.get('content-type')?.includes('png')?'png':response.headers.get('content-type')?.includes('webp')?'webp':'jpg';files[`${item.entry.day}-${item.entry.id}-${i+1}.${ext}`]=bytes;}
    files['账单.csv']=strToU8(ledgerEntriesCSV(entries,timeZone));const bytes=zipSync(files,{level:0});downloadLedgerFile(bytes.slice().buffer,`CYLedger-账单图片-${moment().format('YYYYMMDD-HHmmss')}.zip`,'application/zip');
}
