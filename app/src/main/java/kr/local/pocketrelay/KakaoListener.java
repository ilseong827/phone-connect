package kr.local.pocketrelay;

import android.app.Notification;
import android.os.Bundle;
import android.service.notification.*;
import org.json.*;

public class KakaoListener extends NotificationListenerService {
    @Override public void onNotificationPosted(StatusBarNotification sbn) {
        if(!sbn.getPackageName().equals("com.kakao.talk") || !Store.prefs(this).getBoolean("capture",false))return;
        Notification n=sbn.getNotification();
        if((n.flags & Notification.FLAG_GROUP_SUMMARY)!=0)return;
        try(Store db=new Store(this)) {
            Bundle e=n.extras;
            String title=str(e.getCharSequence(Notification.EXTRA_TITLE));
            String room=str(e.getCharSequence(Notification.EXTRA_CONVERSATION_TITLE));
            android.os.Parcelable[] bundles=e.getParcelableArray(Notification.EXTRA_MESSAGES);
            java.util.List<Notification.MessagingStyle.Message> messages=java.util.Collections.emptyList();
            if(android.os.Build.VERSION.SDK_INT>=30&&bundles!=null)messages=Notification.MessagingStyle.Message.getMessagesFromBundleArray(bundles);
            if(!messages.isEmpty()) {
                for(Notification.MessagingStyle.Message m:messages) {
                    if(m.getText()==null&&m.getDataUri()==null)continue;
                    String sender=m.getSenderPerson()==null?null:str(m.getSenderPerson().getName());
                    String id=Store.hash(sbn.getKey()+"|"+room+"|"+sender+"|"+(m.getTimestamp()>0?m.getTimestamp():sbn.getPostTime())+"|"+m.getText()+"|"+m.getDataUri());
                    if(db.exists(id))continue;
                    JSONObject p=base(sbn,title,room,sender,m.getText()==null?"사진 알림":m.getText().toString(),m.getTimestamp()>0?m.getTimestamp():sbn.getPostTime(),"messagingStyle");
                    JSONArray images=new JSONArray();
                    if(m.getDataUri()!=null)images.put(Images.fromUri(this,m.getDataUri(),m.getDataMimeType(),"notificationAttachment"));
                    if(images.length()==0&&m==messages.get(messages.size()-1)&&e.getParcelable(Notification.EXTRA_PICTURE) instanceof android.graphics.Bitmap)images.put(Images.fromBitmap((android.graphics.Bitmap)e.getParcelable(Notification.EXTRA_PICTURE)));
                    p.put("attachments",images);
                    db.enqueue(id,"kakao",p,null);
                }
            } else {
                String body=str(e.getCharSequence(Notification.EXTRA_BIG_TEXT));
                if(body==null)body=str(e.getCharSequence(Notification.EXTRA_TEXT));
                if(body==null&&!(e.getParcelable(Notification.EXTRA_PICTURE) instanceof android.graphics.Bitmap))return;
                if(body==null)body="사진 알림";
                // A plain title can mean sender OR room. Preserve it without guessing.
                JSONObject p=base(sbn,title,room,null,body,sbn.getPostTime(),"plainNotification");
                JSONArray images=new JSONArray();android.os.Parcelable picture=e.getParcelable(Notification.EXTRA_PICTURE);
                if(picture instanceof android.graphics.Bitmap)images.put(Images.fromBitmap((android.graphics.Bitmap)picture));
                p.put("attachments",images);
                db.enqueue(Store.hash(sbn.getKey()+"|"+sbn.getPostTime()+"|"+title+"|"+body),"kakao",p,null);
            }
            SyncJob.schedule(this,false);
        } catch(Exception ex){Store.prefs(this).edit().putString("status","카톡 수집 실패: "+ex.getClass().getSimpleName()).apply();}
    }
    private JSONObject base(StatusBarNotification s,String title,String room,String sender,String body,long time,String source)throws JSONException {
        return new JSONObject().put("sender",sender==null?JSONObject.NULL:sender).put("chatRoom",room==null?JSONObject.NULL:room)
            .put("message",body).put("messageTime",time).put("receivedAt",s.getPostTime()).put("capturedAt",System.currentTimeMillis())
            .put("notificationTitle",title==null?JSONObject.NULL:title).put("source",source).put("direction","incoming");
    }
    static String str(CharSequence s){return s==null?null:s.toString();}
}
