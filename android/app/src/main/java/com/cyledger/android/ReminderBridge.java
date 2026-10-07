package com.cyledger.android;
import android.Manifest;
import android.app.Activity;
import android.app.NotificationManager;
import android.net.Uri;
import android.os.Build;
import android.webkit.JavascriptInterface;
import android.webkit.WebView;
import org.json.JSONArray;
import org.json.JSONObject;

final class ReminderBridge {
    private final Activity activity;private final WebView web;
    ReminderBridge(Activity activity,WebView web){this.activity=activity;this.web=web;}
    private boolean trusted(){Uri uri=Uri.parse(web.getUrl()==null?"":web.getUrl());int port=activity.getPackageName().endsWith(".qa")?18762:18761;return "http".equals(uri.getScheme())&&"127.0.0.1".equals(uri.getHost())&&uri.getPort()==port;}
    @JavascriptInterface public String status(){return activity.getSystemService(NotificationManager.class).areNotificationsEnabled()?"granted":"denied";}
    @JavascriptInterface public void requestPermission(){activity.runOnUiThread(()->{if(trusted()&&Build.VERSION.SDK_INT>=33)activity.requestPermissions(new String[]{Manifest.permission.POST_NOTIFICATIONS},701);});}
    @JavascriptInterface public void replace(String json){activity.runOnUiThread(()->{if(trusted())ReminderReceiver.replace(activity,json);});}
    @JavascriptInterface public void clear(){replace("[]");}
    @JavascriptInterface public void test(){activity.runOnUiThread(()->{if(!trusted())return;try{JSONArray items=ReminderReceiver.stored(activity);items.put(new JSONObject().put("id","test-notification").put("at",System.currentTimeMillis()+8000).put("title","OpenBill 提醒测试").put("text","系统到期提醒已正常发送"));ReminderReceiver.replace(activity,items.toString());}catch(Exception ignored){}});}
}
