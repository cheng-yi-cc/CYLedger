package com.cyledger.android;

import android.app.AlarmManager;
import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.net.Uri;
import org.json.JSONArray;
import org.json.JSONObject;

/** Local due reminders contain no credentials and run independently of WebView. */
public final class ReminderReceiver extends BroadcastReceiver {
    static final String CHANNEL = "ledger_due";
    static final String PREFS = "due_reminders";
    static PendingIntent alarm(Context context, String id) {
        Intent intent = new Intent(context, ReminderReceiver.class);
        intent.setAction("com.cyledger.android.DUE");
        intent.setData(Uri.parse("cyledger://due/" + Uri.encode(id)));
        intent.putExtra("id", id);
        return PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
    }
    static JSONArray stored(Context context) {
        try { return new JSONArray(context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getString("items", "[]")); }
        catch (Exception ignored) { return new JSONArray(); }
    }
    static synchronized void replace(Context context, String payload) {
        try {
            if (payload.length() > 200000) return;
            JSONArray items = new JSONArray(payload);
            if (items.length() > 1000) return;
            for (int i = 0; i < items.length(); i++) {
                JSONObject item = items.getJSONObject(i);
                if (item.getString("id").length() > 80 || item.getString("title").length() > 80 || item.getString("text").length() > 300 || item.getLong("at") <= 0) return;
            }
            AlarmManager manager = context.getSystemService(AlarmManager.class);
            JSONArray old = stored(context);
            for (int i = 0; i < old.length(); i++) manager.cancel(alarm(context, old.getJSONObject(i).getString("id")));
            context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit().putString("items", items.toString()).apply();
            reschedule(context);
        } catch (Exception ignored) { /* Invalid input cannot replace the saved schedule. */ }
    }
    static synchronized void reschedule(Context context) {
        try {
            JSONArray items = stored(context);long now = System.currentTimeMillis();
            AlarmManager manager = context.getSystemService(AlarmManager.class);
            for (int i = 0; i < items.length(); i++) {
                JSONObject item = items.getJSONObject(i);long at = item.getLong("at");
                if (at > now) manager.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, at, alarm(context, item.getString("id")));
            }
        } catch (Exception ignored) { }
    }
    @Override public void onReceive(Context context, Intent intent) {
        if (Intent.ACTION_BOOT_COMPLETED.equals(intent.getAction()) || Intent.ACTION_MY_PACKAGE_REPLACED.equals(intent.getAction()) || Intent.ACTION_TIME_CHANGED.equals(intent.getAction())) {
            reschedule(context);return;
        }
        if (!"com.cyledger.android.DUE".equals(intent.getAction())) return;
        String id = intent.getStringExtra("id");
        try {
            JSONArray items = stored(context);
            for (int i = 0; i < items.length(); i++) {
                JSONObject item = items.getJSONObject(i);
                if (!item.getString("id").equals(id) || item.getLong("at") > System.currentTimeMillis()) continue;
                NotificationManager manager = context.getSystemService(NotificationManager.class);
                if (!manager.areNotificationsEnabled()) return;
                manager.createNotificationChannel(new NotificationChannel(CHANNEL, "还款与定存到期", NotificationManager.IMPORTANCE_DEFAULT));
                Intent open = new Intent(context, MainActivity.class).addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP | Intent.FLAG_ACTIVITY_SINGLE_TOP);
                PendingIntent content = PendingIntent.getActivity(context, 0, open, PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
                Notification notification = new Notification.Builder(context, CHANNEL).setSmallIcon(android.R.drawable.ic_dialog_info)
                    .setContentTitle(item.getString("title")).setContentText(item.getString("text")).setStyle(new Notification.BigTextStyle().bigText(item.getString("text")))
                    .setVisibility(Notification.VISIBILITY_PRIVATE).setAutoCancel(true).setContentIntent(content).build();
                manager.notify(id, 0, notification);
                items.remove(i);context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit().putString("items", items.toString()).apply();return;
            }
        } catch (Exception ignored) { }
    }
}
