package kr.local.pocketrelay;

import android.content.*;
import android.service.notification.NotificationListenerService;

/** Restore event subscriptions after reboot or an in-place app update. */
public class RecoveryReceiver extends BroadcastReceiver {
    @Override public void onReceive(Context context,Intent intent){
        String action=intent.getAction();
        if(!Intent.ACTION_BOOT_COMPLETED.equals(action)&&!Intent.ACTION_MY_PACKAGE_REPLACED.equals(action))return;
        SyncJob.schedule(context,true);
        SyncJob.watchSms(context);
        SyncJob.schedule(context,false);
        if(Store.prefs(context).getBoolean("capture",false))
            NotificationListenerService.requestRebind(new ComponentName(context,KakaoListener.class));
    }
}
