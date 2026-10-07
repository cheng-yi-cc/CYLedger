package com.cyledger.android;

import android.animation.ValueAnimator;
import android.content.Context;
import android.graphics.Canvas;
import android.graphics.Color;
import android.graphics.Paint;
import android.graphics.Path;
import android.graphics.RectF;
import android.graphics.Typeface;
import android.view.View;
import android.view.animation.DecelerateInterpolator;

/** 启动期间绘制品牌图形；页面就绪即可退出，不人为延长启动。 */
public final class OpenBillSplash extends View {
    private static final int INK = Color.rgb(21, 63, 53);
    private static final int PAPER = Color.rgb(247, 244, 235);
    private static final int SAGE = Color.rgb(169, 207, 181);
    private final Paint paint = new Paint(Paint.ANTI_ALIAS_FLAG);
    private final Path left = new Path();
    private final Path right = new Path();
    private final Path lines = new Path();
    private ValueAnimator animator;
    private float progress;

    public OpenBillSplash(Context context) {
        super(context);
        setBackgroundColor(PAPER);
        setContentDescription("OpenBill，正在打开账本");
        setImportantForAccessibility(View.IMPORTANT_FOR_ACCESSIBILITY_YES);
        setClickable(true);
        left.moveTo(52, 40); left.cubicTo(44, 34, 37, 33, 29, 35);
        left.lineTo(29, 65); left.cubicTo(37, 64, 45, 67, 52, 73); left.close();
        right.moveTo(56, 40); right.cubicTo(64, 34, 71, 33, 79, 35);
        right.lineTo(79, 65); right.cubicTo(71, 64, 63, 67, 56, 73); right.close();
        lines.moveTo(35, 44); lines.cubicTo(39, 44, 43, 45, 46, 47);
        lines.moveTo(35, 51); lines.cubicTo(39, 51, 43, 52, 46, 54);
    }

    @Override protected void onAttachedToWindow() {
        super.onAttachedToWindow();
        if (!ValueAnimator.areAnimatorsEnabled()) { progress = 1f; invalidate(); return; }
        if (progress >= 1f) return;
        animator = ValueAnimator.ofFloat(progress, 1f);
        animator.setDuration(Math.max(1, (long) (850 * (1f - progress))));
        animator.setInterpolator(new DecelerateInterpolator());
        animator.addUpdateListener(value -> { progress = (float) value.getAnimatedValue(); invalidate(); });
        animator.start();
    }

    @Override protected void onDetachedFromWindow() {
        if (animator != null) { animator.cancel(); animator = null; }
        super.onDetachedFromWindow();
    }

    @Override protected void onDraw(Canvas canvas) {
        super.onDraw(canvas);
        float density = getResources().getDisplayMetrics().density;
        float size = Math.min(116 * density, getWidth() * .3f);
        float centerY = getHeight() * .43f;
        canvas.save();
        canvas.translate((getWidth() - size) / 2, centerY - size / 2);
        canvas.scale(size / 108, size / 108);
        paint.setStyle(Paint.Style.FILL); paint.setColor(INK);
        canvas.drawRoundRect(new RectF(0, 0, 108, 108), 25, 25, paint);
        paint.setStyle(Paint.Style.STROKE); paint.setStrokeWidth(3.5f);
        paint.setStrokeCap(Paint.Cap.ROUND); paint.setColor(SAGE);
        canvas.drawArc(new RectF(17, 17, 91, 91), -54, -320 * progress, false, paint);
        canvas.save();
        canvas.scale(.15f + .85f * progress, 1, 54, 54);
        paint.setStyle(Paint.Style.FILL); paint.setColor(PAPER); canvas.drawPath(left, paint);
        paint.setColor(SAGE); canvas.drawPath(right, paint);
        paint.setStyle(Paint.Style.STROKE); paint.setStrokeWidth(2.5f); paint.setColor(INK);
        canvas.drawPath(lines, paint);
        canvas.restore(); canvas.restore();
        paint.setStyle(Paint.Style.FILL); paint.setColor(INK); paint.setTextAlign(Paint.Align.CENTER);
        paint.setTypeface(Typeface.create("sans-serif-medium", Typeface.NORMAL));
        paint.setTextSize(30 * density);
        canvas.drawText("OpenBill", getWidth() / 2f, centerY + size / 2 + 49 * density, paint);
        paint.setTypeface(Typeface.create("sans-serif", Typeface.NORMAL));
        paint.setTextSize(13 * density); paint.setColor(Color.rgb(104, 121, 110));
        canvas.drawText("打开账本，理清生活", getWidth() / 2f, centerY + size / 2 + 78 * density, paint);
    }
}
