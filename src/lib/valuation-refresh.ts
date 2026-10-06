import { investments } from '@/lib/investments.ts';
import type { WealthSummary } from '@/models/investment.ts';

// Read the shared valuation cache while a detail page is visible. This does
// not request upstream prices or reload the account's transaction history.
export function createValuationRefresh(apply: (summary: WealthSummary) => void) {
    let timer: ReturnType<typeof setInterval> | undefined;
    let generation = 0;
    let pending = false;

    function stop(): void {
        clearInterval(timer);
        timer = undefined;
        generation++;
    }

    function start(): void {
        stop();
        const current = generation;
        timer = setInterval(async () => {
            if (document.hidden || pending) return;
            pending = true;
            try {
                const summary = await investments.summary();
                if (current === generation) apply(summary);
            } catch {
                // A failed background read must preserve the last observation.
            } finally {
                pending = false;
            }
        }, 15000);
    }

    return { start, stop };
}
