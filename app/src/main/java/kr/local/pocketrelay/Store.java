package kr.local.pocketrelay;

import android.content.*;
import android.database.Cursor;
import android.database.sqlite.*;
import org.json.*;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;

public class Store extends SQLiteOpenHelper {
    final java.io.File payloadDir;
    public Store(Context c) { this(c,"relay.db"); }
    Store(Context c,String name) { super(c, name, null, 1); payloadDir=new java.io.File(c.getFilesDir(),name.equals("relay.db")?"pending":"test-pending"); }
    public void onCreate(SQLiteDatabase d) {
        d.execSQL("CREATE TABLE queue(id TEXT PRIMARY KEY, kind TEXT NOT NULL, payload TEXT NOT NULL, uri TEXT, sent INTEGER DEFAULT 0, attempted INTEGER DEFAULT 0, created INTEGER NOT NULL)");
        d.execSQL("CREATE TABLE files(uri TEXT PRIMARY KEY, size INTEGER, modified INTEGER, stable_since INTEGER)");
    }
    public void onUpgrade(SQLiteDatabase d, int a, int b) {}
    static android.content.SharedPreferences prefs(Context c) { return c.getSharedPreferences("settings", 0); }
    static String hash(String value) {
        try { byte[] b=MessageDigest.getInstance("SHA-256").digest(value.getBytes(StandardCharsets.UTF_8)); StringBuilder s=new StringBuilder(); for(byte x:b)s.append(String.format("%02x",x)); return s.toString(); }
        catch(Exception e){ throw new IllegalStateException(e); }
    }
    void enqueue(String id, String kind, JSONObject payload, String uri) throws JSONException {
        if(exists(id))return;
        payload.put("id",id).put("schemaVersion",1).put("type",kind);
        String text=payload.toString();
        if(text.length()>128*1024){
            try{if(!payloadDir.isDirectory()&&!payloadDir.mkdirs())throw new java.io.IOException("pending directory");
                android.util.AtomicFile file=new android.util.AtomicFile(new java.io.File(payloadDir,id+".json"));java.io.FileOutputStream out=null;
                try{out=file.startWrite();out.write(text.getBytes(StandardCharsets.UTF_8));file.finishWrite(out);}catch(java.io.IOException ex){file.failWrite(out);throw ex;}
                text=new JSONObject().put("payloadFile",id).toString();
            }catch(java.io.IOException ex){throw new IllegalStateException("첨부 저장 실패",ex);}
        }
        ContentValues v=new ContentValues(); v.put("id",id);v.put("kind",kind);v.put("payload",text);v.put("uri",uri);v.put("created",System.currentTimeMillis());
        getWritableDatabase().insertWithOnConflict("queue",null,v,SQLiteDatabase.CONFLICT_IGNORE);
    }
    String readPayload(String text)throws Exception{
        String id=new JSONObject(text).optString("payloadFile","");if(id.isEmpty())return text;
        if(!id.matches("[0-9a-f]{64}"))throw new java.io.IOException("Invalid payload reference");
        return new String(new android.util.AtomicFile(new java.io.File(payloadDir,id+".json")).readFully(),StandardCharsets.UTF_8);
    }
    void removePayload(String id){if(id.matches("[0-9a-f]{64}"))new android.util.AtomicFile(new java.io.File(payloadDir,id+".json")).delete();}
    void clearPending(){try(Cursor r=getReadableDatabase().rawQuery("SELECT id FROM queue WHERE sent=0",null)){while(r.moveToNext())removePayload(r.getString(0));}getWritableDatabase().delete("queue","sent=0",null);cleanupAttachments();}
    void cleanupAttachments(){
        // A grace period protects copies still being prepared by another collector.
        java.util.Set<String> needed=new java.util.HashSet<>();
        try(Cursor r=getReadableDatabase().rawQuery("SELECT payload FROM queue WHERE sent=0",null)){
            while(r.moveToNext()){JSONArray files=new JSONObject(readPayload(r.getString(0))).optJSONArray("attachments");if(files!=null)for(int i=0;i<files.length();i++)needed.add(files.getJSONObject(i).optString("localFile"));}
        }catch(Exception ex){return;}
        java.io.File dir=new java.io.File(payloadDir.getParentFile(),"attachments");java.io.File[] files=dir.listFiles();if(files==null)return;
        for(java.io.File file:files)if(file.getName().matches("[a-f0-9]{64}\\.blob")&&!needed.contains(file.getName().substring(0,64))&&System.currentTimeMillis()-file.lastModified()>3600000)file.delete();
    }
    boolean stable(String uri,long size,long modified) {
        long now=System.currentTimeMillis();
        try(Cursor r=getReadableDatabase().rawQuery("SELECT size,modified,stable_since FROM files WHERE uri=?",new String[]{uri})) {
            if(r.moveToFirst() && r.getLong(0)==size && r.getLong(1)==modified) return now-r.getLong(2)>=60000;
        }
        ContentValues v=new ContentValues();v.put("uri",uri);v.put("size",size);v.put("modified",modified);v.put("stable_since",now);
        getWritableDatabase().insertWithOnConflict("files",null,v,SQLiteDatabase.CONFLICT_REPLACE);return false;
    }
    boolean exists(String id) {try(Cursor r=getReadableDatabase().rawQuery("SELECT 1 FROM queue WHERE id=?",new String[]{id})){return r.moveToFirst();}}
    int resetRecoverableSent(){
        // Keep unsent payloads and notification tombstones. SMS/MMS and original
        // recordings can be reconstructed from their providers on the next scan.
        return getWritableDatabase().delete("queue","sent=1 AND kind IN ('sms','recording','contacts')",null);
    }
    boolean hasPending(){try(Cursor r=getReadableDatabase().rawQuery("SELECT 1 FROM queue WHERE sent=0 LIMIT 1",null)){return r.moveToFirst();}}
    String counts() {
        try(Cursor r=getReadableDatabase().rawQuery("SELECT kind,sent,count(*) FROM queue GROUP BY kind,sent",null)) {
            StringBuilder s=new StringBuilder(); while(r.moveToNext())s.append(r.getString(0).equals("kakao")?"카톡":r.getString(0).equals("sms")?"문자":r.getString(0).equals("file")?"파일":r.getString(0).equals("contacts")?"연락처 이름":"녹음").append(r.getInt(1)==1?" 전송 완료 ":" 대기 ").append(r.getInt(2)).append("건\n");return s.length()==0?"아직 수집된 기록이 없습니다.":s.toString();
        }
    }
}
