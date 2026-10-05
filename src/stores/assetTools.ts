import {ref,watch} from 'vue';
import {defineStore} from 'pinia';
import {assetTools,defaultAssetPreferences,type AssetPreferences} from '@/lib/asset-tools.ts';
import {useUserStore} from '@/stores/user.ts';
export const useAssetToolsStore=defineStore('assetTools',()=>{
 const preferences=ref<AssetPreferences>(defaultAssetPreferences()),loaded=ref(false);let pending:Promise<void>|undefined;
 const user=useUserStore();let generation=0;
 watch(()=>user.currentUserBasicInfo?.username,()=>{generation++;loaded.value=false;preferences.value=defaultAssetPreferences();pending=undefined;window.CYLedgerReminders?.clear();});
 async function load(force=false):Promise<void>{if(pending)return pending;if(loaded.value&&!force)return;const version=generation;pending=assetTools.preferences().then(p=>{if(version===generation){preferences.value=p;loaded.value=true;}}).finally(()=>{if(version===generation)pending=undefined;});return pending;}
 async function save(value:AssetPreferences):Promise<void>{preferences.value=await assetTools.savePreferences(value);loaded.value=true;}
 function available(kind:'cash'|'portfolio',id:string,bookId:string):boolean{return !preferences.value.rules[`${kind}:${id}`]?.disabledBooks.includes(bookId);}
 return{preferences,loaded,load,save,available};
});
