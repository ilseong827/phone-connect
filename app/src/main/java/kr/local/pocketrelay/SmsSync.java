package kr.local.pocketrelay;

import android.Manifest;
import android.content.*;
import android.content.pm.PackageManager;
import android.database.Cursor;
import android.net.Uri;
import android.provider.Telephony;
import org.json.*;
import java.util.*;
import java.io.*;

class SmsSync {
    static boolean allowed(Context c){return c.checkSelfPermission(Manifest.permission.READ_SMS)==PackageManager.PERMISSION_GRANTED;}
    static synchronized String device(Context c){String id=Store.prefs(c).getString("deviceId","");if(id.isEmpty()){id=UUID.randomUUID().toString();Store.prefs(c).edit().putString("deviceId",id).commit();}return id;}
    static int count(Context c,long cutoff){int n=0;
        try(Cursor r=c.getContentResolver().query(Telephony.Sms.CONTENT_URI,new String[]{"_id"},"date>=? AND type IN (1,2)",new String[]{String.valueOf(cutoff)},null)){if(r!=null)n+=r.getCount();}
        try(Cursor r=c.getContentResolver().query(Telephony.Mms.CONTENT_URI,new String[]{"_id"},"date>=? AND msg_box IN (1,2) AND m_type IN (128,132)",new String[]{String.valueOf(cutoff/1000)},null)){if(r!=null)n+=r.getCount();}
        return n;
    }
    static void scan(Context c,Store db)throws Exception {
        if(!Store.prefs(c).getBoolean("sms",false))return;
        if(!allowed(c)){Store.prefs(c).edit().putString("smsStatus","문자 읽기 권한이 필요합니다.").apply();return;}
        long cutoff=Store.prefs(c).getLong("smsCutoff",Long.MAX_VALUE);String device=device(c);int queued=0;
        String[] columns={"_id","thread_id","address","date","type","body","sub_id"};
        try(Cursor r=c.getContentResolver().query(Telephony.Sms.CONTENT_URI,columns,"date>=? AND type IN (1,2)",new String[]{String.valueOf(cutoff)},"date ASC")){
            if(r!=null)while(r.moveToNext()){
                if(Thread.currentThread().isInterrupted()||!Store.prefs(c).getBoolean("sms",false))return;
                String id=Store.hash(device+"|sms|"+r.getLong(0));if(db.exists(id))continue;
                boolean sent=r.getInt(4)==2;String address=r.getString(2);
                JSONObject p=base(device,r.getLong(1),address,r.getLong(3),sent,"SMS").put("message",r.getString(5)==null?"":r.getString(5)).put("subscriptionId",r.getInt(6)).put("attachments",new JSONArray());
                db.enqueue(id,"sms",p,null);if(++queued>=200)break;
            }
        }
        String[] mmsCols={"_id","thread_id","date","msg_box","sub"};
        try(Cursor r=c.getContentResolver().query(Telephony.Mms.CONTENT_URI,mmsCols,"date>=? AND msg_box IN (1,2) AND m_type IN (128,132)",new String[]{String.valueOf(cutoff/1000)},"date ASC")){
            if(r!=null)while(r.moveToNext()){
                if(Thread.currentThread().isInterrupted()||!Store.prefs(c).getBoolean("sms",false))return;
                long messageId=r.getLong(0),date=r.getLong(2)*1000;String id=Store.hash(device+"|mms|"+messageId);if(db.exists(id)||System.currentTimeMillis()-date<10000)continue;
                boolean sent=r.getInt(3)==2;List<String> addresses=new ArrayList<>();
                try(Cursor a=c.getContentResolver().query(Uri.parse("content://mms/"+messageId+"/addr"),new String[]{"address","type"},null,null,null)){
                    if(a!=null)while(a.moveToNext()){String address=a.getString(0);int type=a.getInt(1);if(address!=null&&!address.equals("insert-address-token")&&((sent&&(type==151||type==130))||(!sent&&type==137)))addresses.add(address);}
                }
                String address=android.text.TextUtils.join(", ",addresses);if(address.isEmpty())address="번호 미확인";
                StringBuilder body=new StringBuilder();JSONArray images=new JSONArray();boolean hasPart=false;int imageCount=0;
                try(Cursor parts=c.getContentResolver().query(Uri.parse("content://mms/"+messageId+"/part"),new String[]{"_id","ct","text","_data"},null,null,null)){
                    if(parts!=null)while(parts.moveToNext()){
                        hasPart=true;String mime=parts.getString(1);Uri part=Uri.parse("content://mms/part/"+parts.getString(0));
                        if("text/plain".equals(mime)){
                            String text=parts.getString(2);
                            if(text==null&&parts.getString(3)!=null){try(InputStream in=c.getContentResolver().openInputStream(part)){if(in!=null){ByteArrayOutputStream out=new ByteArrayOutputStream();byte[] bytes=new byte[4096];int n;while((n=in.read(bytes))!=-1&&out.size()<1024*1024)out.write(bytes,0,n);text=out.toString("UTF-8");}}}
                            if(text!=null){if(body.length()>0)body.append('\n');body.append(text);}
                        }else if(mime!=null&&mime.startsWith("image/")){
                            if(++imageCount<=10)images.put(Images.fromUri(c,part,mime,"mmsOriginal"));else images.put(Images.missing(mime,"mmsOriginal","메시지당 첨부 10개 제한"));
                        }else if(mime!=null&&!mime.equals("application/smil")){if(++imageCount<=10)images.put(Images.fromUri(c,part,mime,"mmsAttachment"));else images.put(Images.missing(mime,"mmsAttachment","메시지당 첨부 10개 제한"));}
                    }
                }
                if(!hasPart)continue; // An MMS notification can precede its download.
                JSONObject p=base(device,r.getLong(1),address,date,sent,"MMS").put("message",body.toString()).put("subject",r.getString(4)).put("attachments",images);
                db.enqueue(id,"sms",p,null);if(++queued>=400)break;
            }
        }
        Store.prefs(c).edit().putString("smsStatus","문자 확인 완료 · 이번에 "+queued+"건 수집").apply();
    }
    static JSONObject base(String device,long thread,String address,long date,boolean sent,String protocol)throws JSONException {
        return new JSONObject().put("deviceId",device).put("threadId",device+":"+thread).put("chatRoom",address).put("address",address)
            .put("sender",sent?"나":address).put("direction",sent?"outgoing":"incoming").put("messageTime",date).put("receivedAt",date)
            .put("protocol",protocol).put("source","androidTelephony").put("capturedAt",System.currentTimeMillis());
    }
}
