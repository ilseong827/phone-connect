package kr.local.pocketrelay;
import android.Manifest;
import android.content.*;
import android.content.pm.PackageManager;
import android.database.Cursor;
import android.net.Uri;
import android.provider.*;
import org.json.*;
import java.util.*;

class ContactNames {
    static boolean allowed(Context c){return c.checkSelfPermission(Manifest.permission.READ_CONTACTS)==PackageManager.PERMISSION_GRANTED;}
    static void scan(Context c,Store db)throws Exception{
        if(!allowed(c)||!Store.prefs(c).getBoolean("sms",false))return;
        // Only look up numbers belonging to the user's selected SMS/MMS history.
        Set<String> numbers=new TreeSet<>();long cutoff=Store.prefs(c).getLong("smsCutoff",Long.MAX_VALUE);
        try(Cursor r=c.getContentResolver().query(Telephony.Sms.CONTENT_URI,new String[]{"address"},"date>=? AND type IN (1,2)",new String[]{String.valueOf(cutoff)},null)){if(r!=null)while(r.moveToNext())if(r.getString(0)!=null)numbers.add(r.getString(0));}
        try(Cursor r=c.getContentResolver().query(Telephony.Mms.CONTENT_URI,new String[]{"_id"},"date>=? AND msg_box IN (1,2)",new String[]{String.valueOf(cutoff/1000)},null)){
            if(r!=null)while(r.moveToNext())try(Cursor a=c.getContentResolver().query(Uri.parse("content://mms/"+r.getLong(0)+"/addr"),new String[]{"address"},null,null,null)){if(a!=null)while(a.moveToNext()){String n=a.getString(0);if(n!=null&&!n.equals("insert-address-token"))numbers.add(n);}}
        }
        String device=SmsSync.device(c);JSONArray batch=new JSONArray();
        for(String number:numbers){
            if(Thread.currentThread().isInterrupted()||!Store.prefs(c).getBoolean("sms",false))return;
            String name="";
            Uri uri=Uri.withAppendedPath(ContactsContract.PhoneLookup.CONTENT_FILTER_URI,Uri.encode(number));
            try(Cursor r=c.getContentResolver().query(uri,new String[]{ContactsContract.PhoneLookup.DISPLAY_NAME},null,null,null)){if(r!=null&&r.moveToFirst()&&r.getString(0)!=null)name=r.getString(0);}
            // Include an empty value to clear a name that was removed from Contacts.
            String key="contact-"+Store.hash(device+"|"+number),value=Store.hash(name);
            if(value.equals(Store.prefs(c).getString(key,"")))continue;
            batch.put(new JSONObject().put("address",number).put("displayName",name).put("cacheKey",key).put("cacheValue",value));
            if(batch.length()==200){enqueue(c,db,device,batch);batch=new JSONArray();}
        }
        if(batch.length()>0)enqueue(c,db,device,batch);
    }
    static void enqueue(Context c,Store db,String device,JSONArray values)throws Exception{
        JSONArray entries=new JSONArray();for(int i=0;i<values.length();i++){JSONObject v=values.getJSONObject(i);entries.put(new JSONObject().put("address",v.getString("address")).put("displayName",v.getString("displayName")));}
        JSONObject event=new JSONObject().put("deviceId",device).put("entries",entries).put("capturedAt",System.currentTimeMillis());
        db.enqueue(Store.hash(UUID.randomUUID().toString()),"contacts",event,null);
        SharedPreferences.Editor editor=Store.prefs(c).edit();for(int i=0;i<values.length();i++){JSONObject v=values.getJSONObject(i);editor.putString(v.getString("cacheKey"),v.getString("cacheValue"));}editor.apply();
    }
}
