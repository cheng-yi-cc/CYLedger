package com.cyledger.android;
import android.service.quicksettings.TileService;import android.app.PendingIntent;import android.content.Intent;import android.os.Build;
public final class LedgerTileService extends TileService {
 @Override public void onClick(){super.onClick();Intent intent=new Intent(this,MainActivity.class).putExtra("quickEntry",true).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK|Intent.FLAG_ACTIVITY_SINGLE_TOP|Intent.FLAG_ACTIVITY_CLEAR_TOP);if(Build.VERSION.SDK_INT>=34)startActivityAndCollapse(PendingIntent.getActivity(this,410,intent,PendingIntent.FLAG_UPDATE_CURRENT|PendingIntent.FLAG_IMMUTABLE));else startActivityAndCollapse(intent);}
}
