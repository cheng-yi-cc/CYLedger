import { describe, expect, it } from 'vitest';
import moment from 'moment-timezone';
import { wealthTrendPoints } from '../wealth-trend.ts';
import type { WealthSnapshot } from '@/models/investment.ts';

const zone = 'Asia/Shanghai';
function snapshot(date: string, patch: Partial<WealthSnapshot> = {}, timeZone = zone): WealthSnapshot {
    return { id: date, recordedAt: moment.tz(date, timeZone).unix(), netAssets: '12345.67890123456789', valuedAssets: '12500', complete: true, invalidated: false, ...patch };
}
describe('asset trend history boundaries', () => {
    it('keeps exact values and a single snapshot without inventing a past or future balance', () => {
        const row = snapshot('2026-10-07 12:30');
        expect(wealthTrendPoints([row], 'net', zone)).toEqual([{ at: row.recordedAt, amount: row.netAssets }]);
    });
    it('sorts history without mutating it and breaks the curve across an unrecorded day', () => {
        const rows = [snapshot('2026-10-07 09:00'), snapshot('2026-10-04 23:00')];
        const points = wealthTrendPoints(rows, 'net', zone);
        expect(points.map(point => moment.unix(point.at).tz(zone).format('MM-DD HH:mm'))).toEqual(['10-04 23:00', '10-05 00:00', '10-07 09:00']);
        expect(points[1]?.amount).toBeNull();
        expect(rows[0]?.id).toBe('2026-10-07 09:00');
    });
    it('preserves unknown and invalidated values while allowing a known zero', () => {
        const rows = [snapshot('2026-10-04', { complete: false }), snapshot('2026-10-05', { netAssets: null }), snapshot('2026-10-06', { invalidated: true }), snapshot('2026-10-07', { netAssets: '0' })];
        expect(wealthTrendPoints(rows, 'net', zone).map(point => point.amount)).toEqual([null, null, null, '0']);
        expect(wealthTrendPoints(rows, 'valued', zone).map(point => point.amount)).toEqual(['12500', '12500', null, '12500']);
    });
    it('uses accounting calendar days rather than 24-hour intervals across DST', () => {
        const timeZone = 'America/New_York';
        const rows = [snapshot('2026-11-01 00:01', {}, timeZone), snapshot('2026-11-02 23:59', {}, timeZone)];
        expect(wealthTrendPoints(rows, 'net', timeZone)).toHaveLength(2);
        const gap = wealthTrendPoints([rows[0]!, snapshot('2026-11-03 00:01', {}, timeZone)], 'net', timeZone);
        expect(gap[1]).toEqual({ at: moment.tz('2026-11-02', timeZone).unix(), amount: null });
    });
});
