import { createWorker, PSM } from 'tesseract.js';
import moment from 'moment-timezone';
import { LedgerDecimal } from '@/lib/ledger-display.ts';
import type { ImportTransactionResponse } from '@/models/imported_transaction.ts';

// OCR is bundled and runs on this device. Never send a bill image to a remote API.
export async function recognizeBillImages(files: File[], zone: string, progress: (text:string)=>void): Promise<{text:string;items:ImportTransactionResponse[]}> {
    const base=new URL('js/ocr/',document.baseURI).href;
    const worker=await createWorker(['chi_sim','eng'],1,{workerPath:base+'worker.min.js',corePath:base,langPath:base,gzip:true,logger:event=>progress(event.status==='recognizing text'?`正在识别 ${Math.round(event.progress*100)}%`:'正在准备离线识别')});
    let text='';
    try { await worker.setParameters({tessedit_pageseg_mode:PSM.AUTO});for(let i=0;i<files.length;i++){progress(`正在识别第 ${i+1} / ${files.length} 张`);const result=await worker.recognize(files[i]!);text+='\n'+result.data.text;} }
    finally { await worker.terminate(); }
    return {text,items:parseBillOCR(text,zone)};
}
export function parseBillOCR(text:string,zone:string):ImportTransactionResponse[]{
    const items:ImportTransactionResponse[]=[];
    let date=moment().tz(zone).format('YYYY-MM-DD'),context='';
    for(const raw of text.split('\n')){
        const line=raw.trim();if(!line)continue;
        const stamp=line.match(/(20\d{2})\s*[-年/.]\s*(\d{1,2})\s*[-月/.]\s*(\d{1,2})/);
        if(stamp)date=`${stamp[1]}-${stamp[2]!.padStart(2,'0')}-${stamp[3]!.padStart(2,'0')}`;
        const amounts=[...line.matchAll(/(?:[¥￥]|[+\-])\s*(\d{1,10}(?:,\d{3})*\.\d{2})\b/g)];
        if(amounts.length!==1||/合计|总计|余额|共\s*\d+\s*笔/.test(line)){context=line;continue;}
        const match=amounts[0]!,amount=new LedgerDecimal(match[1]!.replaceAll(',','')).mul(100);
        if(!amount.isInteger()||amount.gt('9007199254740991'))continue;
        const timePart=line.match(/\b\d{1,2}:\d{2}(?::\d{2})?\b/)?.[0]||'12:00:00';
        const time=moment.tz(`${date} ${timePart}`,['YYYY-MM-DD H:mm:ss','YYYY-MM-DD H:mm'],true,zone);if(!time.isValid())continue;
        const comment=line.replace(match[0],'').trim()||context;
        items.push({type:/\+|收入|收款/.test(match[0]+line)?2:3,categoryId:'',originalCategoryName:'',time:time.unix(),utcOffset:time.utcOffset(),sourceAccountId:'',originalSourceAccountName:'',originalSourceAccountCurrency:'CNY',sourceAmount:amount.toNumber(),tagIds:[],originalTagNames:[],comment:comment.slice(0,255)});
    }
    return items;
}
