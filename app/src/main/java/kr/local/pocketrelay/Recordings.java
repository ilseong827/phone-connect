package kr.local.pocketrelay;

import android.content.*;
import android.database.Cursor;
import android.net.Uri;
import android.provider.DocumentsContract;
import android.media.MediaMetadataRetriever;
import org.json.*;
import java.util.*;

class Recordings {
    static class Entry {
        Uri uri;String name,mime;long size,modified;
        Entry(Uri u,String n,String m,long s,long t){uri=u;name=n;mime=m;size=s;modified=t;}
        String id(){return Store.hash(uri+"|"+size+"|"+modified);}
    }
    static List<Entry> list(Context c,Uri root)throws Exception {
        List<Entry> out=new ArrayList<>();walk(c,root,DocumentsContract.getTreeDocumentId(root),out,0);return out;
    }
    static void walk(Context c,Uri root,String id,List<Entry> out,int depth)throws Exception {
        if(depth>16)throw new Exception("폴더 깊이 제한");
        Uri children=DocumentsContract.buildChildDocumentsUriUsingTree(root,id);
        String[] cols={"document_id","_display_name","mime_type","_size","last_modified"};
        try(Cursor r=c.getContentResolver().query(children,cols,null,null,null)){
            if(r==null)throw new Exception("폴더 조회 실패");
            while(r.moveToNext()){
                String doc=r.getString(0),name=r.getString(1),mime=r.getString(2);
                if(DocumentsContract.Document.MIME_TYPE_DIR.equals(mime)){walk(c,root,doc,out,depth+1);continue;}
                String lower=name==null?"":name.toLowerCase(Locale.ROOT);
                if((mime!=null&&mime.startsWith("audio/"))||lower.matches(".*\\.(m4a|mp3|wav|aac|amr|3ga|ogg|flac)$"))
                    out.add(new Entry(DocumentsContract.buildDocumentUriUsingTree(root,doc),name,mime,r.getLong(3),r.getLong(4)));
            }
        }
    }
    static void scan(Context c,Store db)throws Exception {
        SharedPreferences p=Store.prefs(c);
        if(!p.getBoolean("recordings",false))return;
        String folder=p.getString("folder","");if(folder.isEmpty())return;
        long cutoff=p.getLong("cutoff",Long.MAX_VALUE);
        int unknown=0;
        for(Entry f:list(c,Uri.parse(folder))){
            if(Thread.currentThread().isInterrupted())return;
            if(f.modified<=0){unknown++;continue;}
            if(f.modified<cutoff||f.size<=0||db.exists(f.id()))continue;
            if(!db.stable(f.uri.toString(),f.size,f.modified)||System.currentTimeMillis()-f.modified<60000)continue;
            JSONObject data=new JSONObject().put("fileName",f.name).put("mimeType",f.mime).put("size",f.size)
                .put("fileModifiedAt",f.modified).put("recordedAt",JSONObject.NULL).put("discoveredAt",System.currentTimeMillis());
            MediaMetadataRetriever m=new MediaMetadataRetriever();
            try {
                m.setDataSource(c,f.uri);String duration=m.extractMetadata(MediaMetadataRetriever.METADATA_KEY_DURATION);
                data.put("durationMs",duration==null?JSONObject.NULL:Long.parseLong(duration));
            }catch(Exception ignored){data.put("durationMs",JSONObject.NULL);}finally{try{m.release();}catch(Exception ignored){}}
            db.enqueue(f.id(),"recording",data,f.uri.toString());
        }
        p.edit().putInt("unknownDates",unknown).apply();
    }
}
