import { useI18n } from '@/locales/helpers.ts';
import type { ErrorResponse } from '@/core/api.ts';
import { investmentError } from '@/lib/investments.ts';

export function useManagementFeedback() {
    const { te: translateError } = useI18n();
    function errorText(cause: unknown): string {
        if (cause && typeof cause === 'object') {
            if ('error' in cause && cause.error && typeof cause.error === 'object') {
                return translateError(cause as { error: ErrorResponse }) || '操作失败，请重试';
            }
            if ('message' in cause && typeof cause.message === 'string') return translateError(cause.message);
        }
        return investmentError(cause);
    }
    return { errorText };
}
