import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, disposePinia, setActivePinia } from 'pinia';
import moment from 'moment-timezone';
import { useLedgerScopeStore } from '../ledgerScope.ts';

vi.mock('../user.ts', () => ({ useUserStore: () => ({ currentUserBasicInfo: null }) }));

let pinia: ReturnType<typeof createPinia>;
beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-01-31T15:59:59Z'));
    vi.stubGlobal('window', { setInterval, clearInterval, addEventListener: vi.fn(), removeEventListener: vi.fn() });
    vi.stubGlobal('document', { addEventListener: vi.fn(), removeEventListener: vi.fn() });
    vi.spyOn(moment.tz, 'guess').mockReturnValue('Asia/Shanghai');
    pinia = createPinia();
    setActivePinia(pinia);
});
afterEach(() => { disposePinia(pinia); vi.restoreAllMocks(); vi.useRealTimers(); vi.unstubAllGlobals(); });

describe('device calendar clock', () => {
    it('rolls today into the next month without overwriting a selected historical calendar date', () => {
        const scope = useLedgerScopeStore();
        scope.month = '2025-12';
        scope.selectedDay = '2025-12-10';
        expect(scope.currentDay).toBe('2026-01-31');
        vi.advanceTimersByTime(1000);
        expect(scope.currentDay).toBe('2026-02-01');
        expect(scope.month).toBe('2025-12');
        expect(scope.selectedDay).toBe('2025-12-10');
    });
    it('picks up a system timezone change when the app resumes', () => {
        const scope = useLedgerScopeStore();
        vi.setSystemTime(new Date('2026-02-01T01:00:00Z'));
        scope.syncClock();
        expect(scope.currentDay).toBe('2026-02-01');
        vi.mocked(moment.tz.guess).mockReturnValue('America/Los_Angeles');
        scope.syncClock();
        expect(scope.timeZone).toBe('America/Los_Angeles');
        expect(scope.currentDay).toBe('2026-01-31');
    });
});
