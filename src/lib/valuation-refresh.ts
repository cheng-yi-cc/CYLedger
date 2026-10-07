import { investments } from '@/lib/investments.ts';
import type { WealthSummary } from '@/models/investment.ts';

// Read the shared valuation cache while a detail page is visible. This does
// not request upstream prices or reload the account's transaction history.
export function createValuationRefresh(apply: (summary: WealthSummary) => void) {
    let timer: ReturnType<typeof setInterval> | undefined;
    let generation = 0;
    let pending = false;
    let active = false;

    function stop(): void {
        clearInterval(timer);
        timer = undefined;
        active = false;
        document.removeEventListener('visibilitychange', resume);
        generation++;
    }

    async function tick(): Promise<void> {
        if (!active || document.hidden || pending) return;
        const current = generation;
        pending = true;
        try {
            const summary = await investments.summary();
            if (active && current === generation) apply(summary);
        } catch {
            // Keep the last observation, including its source time and status.
        } finally {
            pending = false;
            if (active && current !== generation) void tick();
        }
    }
    function resume(): void { if (!document.hidden) void tick(); }

    function start(): void {
        stop();
        active = true;
        document.addEventListener('visibilitychange', resume);
        timer = setInterval(() => void tick(), 5000);
        void tick();
    }

    return { start, stop };
}
