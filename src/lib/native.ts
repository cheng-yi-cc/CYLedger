// Set by the Android host before the Vue bundle runs. Browser deployments keep
// their existing authentication and do not gain an anonymous API session.
export function isNativePersonalMode(): boolean {
    return typeof window !== 'undefined' && (window as Window & { __CYLEDGER_PERSONAL__?: boolean }).__CYLEDGER_PERSONAL__ === true;
}

declare global { interface Window { CYLedgerAppearance?: { setTheme(color:string,dark:boolean):void }; CYLedgerLocal?: { status():string; settings(json:string):void; action(action:string,payload:string):void; preferences(json:string):void; backups():string; backup(settings:string):void; restore():void; exportBackup(name:string):void; deleteBackup(name:string):void } } }

export function syncNativeAppearance():void {
    if (!isNativePersonalMode()) return;
    requestAnimationFrame(()=>{
        const page=document.querySelector('.page-current.cy-mobile-surface')||document.documentElement;
        const dark=document.documentElement.classList.contains('dark');
        const raw=getComputedStyle(page).getPropertyValue('--cy-card').trim()||(dark?'#212b36':'#ffffff');
        const color=/^#[0-9a-f]{3}$/i.test(raw)?'#'+raw.slice(1).split('').map(c=>c+c).join(''):raw;
        window.CYLedgerAppearance?.setTheme(color,dark);
    });
}

export function localBackupSettings():string { const values:Record<string,string>={};for(const key of Object.keys(localStorage))if(key==='ebk_app_settings'||key.startsWith('cy_ledger_experience_')){const value=localStorage.getItem(key);if(value){try{JSON.parse(value);values[key]=value;}catch{/* Ignore malformed browser preferences. */}}}return JSON.stringify(values); }
export function syncNativeSettings():void { window.CYLedgerLocal?.settings(localBackupSettings()); }
