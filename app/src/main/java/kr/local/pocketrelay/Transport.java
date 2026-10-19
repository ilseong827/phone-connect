package kr.local.pocketrelay;

import android.content.*;
import android.database.Cursor;
import android.net.Uri;
import java.io.*;
import java.net.*;
import java.nio.charset.StandardCharsets;
import org.json.*;

class Transport {
    static URI endpoint(String value)throws Exception {
        URI u=new URI(value.trim());
        if(!"https".equalsIgnoreCase(u.getScheme())||u.getHost()==null||u.getUserInfo()!=null||u.getQuery()!=null||u.getFragment()!=null)
            throw new Exception("HTTPS 서버 주소를 입력하세요. 사용자 정보·쿼리·#은 사용할 수 없습니다.");
        return u;
    }
    static void send(Context c,Store db)throws Exception {
        SharedPreferences p=Store.prefs(c);
        if(!p.getBoolean("upload",false))return;
        android.net.ConnectivityManager cm=c.getSystemService(android.net.ConnectivityManager.class);
        android.net.NetworkCapabilities caps=cm.getNetworkCapabilities(cm.getActiveNetwork());
        if(caps==null||!caps.hasCapability(android.net.NetworkCapabilities.NET_CAPABILITY_VALIDATED))throw new IOException("인터넷 연결을 기다립니다");
        if(p.getBoolean("wifi",true)&&!caps.hasTransport(android.net.NetworkCapabilities.TRANSPORT_WIFI))throw new IOException("Wi-Fi 연결을 기다립니다");
        String base=p.getString("url",""),token=Secrets.read(c);
        endpoint(base);if(token.isEmpty())throw new Exception("인증키가 없습니다.");
        try(Cursor r=db.getReadableDatabase().rawQuery("SELECT id,kind,payload,uri FROM queue WHERE sent=0 ORDER BY CASE WHEN kind='contacts' THEN 0 ELSE 1 END,attempted,created LIMIT 30",null)){
            while(r.moveToNext()){
                if(Thread.currentThread().isInterrupted()||!p.getBoolean("upload",false))return;
                String id=r.getString(0),kind=r.getString(1),payload=db.readPayload(r.getString(2));
                JSONObject prepared=new JSONObject(payload);JSONArray attachments=prepared.optJSONArray("attachments");
                if(attachments!=null)for(int i=0;i<attachments.length();i++){
                    JSONObject a=attachments.getJSONObject(i);String local=a.optString("localFile","");
                    if(!local.isEmpty()){uploadAttachment(c,base,token,a);a.remove("localFile");}
                }
                payload=prepared.toString();
                ContentValues attempt=new ContentValues();attempt.put("attempted",System.currentTimeMillis());
                db.getWritableDatabase().update("queue",attempt,"id=?",new String[]{id});
                if(kind.equals("recording") && prepared.optLong("size")>32L*1024*1024){
                    uploadRecording(c,base,token,id,prepared,Uri.parse(r.getString(3)));
                    ContentValues done=new ContentValues();done.put("sent",1);done.put("payload","{}");done.putNull("uri");
                    db.getWritableDatabase().update("queue",done,"id=?",new String[]{id});db.removePayload(id);
                    continue;
                }
                HttpURLConnection con=(HttpURLConnection)new URL(base.replaceAll("/+$","")+"/v1/"+(kind.equals("recording")?"recordings":"events")).openConnection();
                con.setInstanceFollowRedirects(false);con.setConnectTimeout(15000);con.setReadTimeout(60000);con.setRequestMethod("POST");con.setDoOutput(true);
                con.setRequestProperty("Authorization","Bearer "+token);con.setRequestProperty("Idempotency-Key",id);
                try {
                    if(!kind.equals("recording")){
                        byte[] bytes=payload.getBytes(StandardCharsets.UTF_8);
                        con.setRequestProperty("Content-Type","application/json; charset=utf-8");con.setFixedLengthStreamingMode(bytes.length);
                        try(OutputStream out=con.getOutputStream()){out.write(bytes);}
                    }else{
                        JSONObject meta=new JSONObject(payload);Uri uri=Uri.parse(r.getString(3));
                        // Verify metadata again immediately before uploading.
                        try(Cursor f=c.getContentResolver().query(uri,new String[]{"_size","last_modified"},null,null,null)) {
                            if(f==null||!f.moveToFirst())throw new IOException("녹음 파일에 접근할 수 없습니다");
                            if(f.getLong(0)!=meta.getLong("size")||f.getLong(1)!=meta.getLong("fileModifiedAt")) {
                                db.getWritableDatabase().delete("queue","id=?",new String[]{id});continue;
                            }
                        }
                        String boundary="Relay"+id;
                        con.setRequestProperty("Content-Type","multipart/form-data; boundary="+boundary);con.setChunkedStreamingMode(65536);
                        try(InputStream in=c.getContentResolver().openInputStream(uri);OutputStream out=con.getOutputStream()){
                            if(in==null)throw new IOException("녹음 파일 열기 실패");
                            write(out,"--"+boundary+"\r\nContent-Disposition: form-data; name=\"metadata\"\r\nContent-Type: application/json\r\n\r\n"+payload+"\r\n--"+boundary+"\r\nContent-Disposition: form-data; name=\"file\"; filename=\"recording.bin\"\r\nContent-Type: application/octet-stream\r\n\r\n");
                            byte[] buf=new byte[65536];int n;long total=0;
                            while((n=in.read(buf))!=-1){if(Thread.currentThread().isInterrupted()||!p.getBoolean("upload",false))throw new IOException("전송 중지");out.write(buf,0,n);total+=n;}
                            if(total!=meta.getLong("size"))throw new IOException("녹음 크기가 변경됨");
                            write(out,"\r\n--"+boundary+"--\r\n");
                        }
                    }
                    int code=con.getResponseCode();
                    if(code<200||code>=300)throw new IOException("서버 응답 HTTP "+code);
                    String ack;
                    try(InputStream in=con.getInputStream();ByteArrayOutputStream bytes=new ByteArrayOutputStream()){
                        byte[] buffer=new byte[1024];int n;while((n=in.read(buffer))!=-1){bytes.write(buffer,0,n);if(bytes.size()>8192)throw new IOException("응답 크기 초과");}ack=bytes.toString("UTF-8");
                    }
                    JSONObject a=new JSONObject(ack);
                    if(!a.optBoolean("accepted")||!id.equals(a.optString("id")))throw new IOException("서버 저장 확인 응답이 일치하지 않습니다");
                    ContentValues v=new ContentValues();v.put("sent",1);v.put("payload","{}");v.putNull("uri");
                    db.getWritableDatabase().update("queue",v,"id=?",new String[]{id});
                    db.removePayload(id);
                    db.cleanupAttachments();
                }finally{con.disconnect();}
            }
        }
    }
    static void write(OutputStream out,String s)throws IOException{out.write(s.getBytes(StandardCharsets.UTF_8));}
    static void verifyRecording(Context c,Uri uri,JSONObject meta)throws Exception{
        try(Cursor f=c.getContentResolver().query(uri,new String[]{"_size","last_modified"},null,null,null)){
            if(f==null||!f.moveToFirst()||f.getLong(0)!=meta.getLong("size")||f.getLong(1)!=meta.getLong("fileModifiedAt"))throw new IOException("녹음 파일이 변경되었거나 접근할 수 없습니다");
        }
    }
    static JSONObject chunkRequest(String base,String token,String id,String action,byte[] body,String hash)throws Exception{
        HttpURLConnection con=(HttpURLConnection)new URL(base.replaceAll("/+$","")+"/v1/recording-upload/"+action).openConnection();
        try{
            con.setInstanceFollowRedirects(false);con.setConnectTimeout(15000);con.setReadTimeout(120000);con.setRequestMethod("POST");con.setDoOutput(true);
            con.setRequestProperty("Authorization","Bearer "+token);con.setRequestProperty("Idempotency-Key",id);
            con.setRequestProperty("Content-Type",hash==null?"application/json":"application/octet-stream");
            if(hash!=null)con.setRequestProperty("X-Chunk-SHA256",hash);
            con.setFixedLengthStreamingMode(body.length);
            try(OutputStream out=con.getOutputStream()){out.write(body);}
            int code=con.getResponseCode();if(code!=200)throw new IOException("분할 전송 HTTP "+code);
            try(InputStream in=con.getInputStream();ByteArrayOutputStream out=new ByteArrayOutputStream()){
                byte[] buf=new byte[1024];int n;while((n=in.read(buf))!=-1){out.write(buf,0,n);if(out.size()>8192)throw new IOException("응답 크기 초과");}
                JSONObject response=new JSONObject(out.toString("UTF-8"));if(!id.equals(response.optString("id")))throw new IOException("전송 확인 ID 불일치");return response;
            }
        }finally{con.disconnect();}
    }
    static void uploadRecording(Context c,String base,String token,String id,JSONObject meta,Uri uri)throws Exception{
        verifyRecording(c,uri,meta);
        JSONObject state=chunkRequest(base,token,id,"start",meta.toString().getBytes(StandardCharsets.UTF_8),null);
        if(state.optBoolean("accepted"))return;
        int chunk=state.getInt("chunkSize"),next=state.getInt("nextChunk");long size=meta.getLong("size");
        if(chunk!=8*1024*1024||next<0||(long)next>(size+chunk-1)/chunk)throw new IOException("잘못된 분할 전송 응답");
        try(InputStream in=c.getContentResolver().openInputStream(uri)){
            if(in==null)throw new IOException("녹음 파일 열기 실패");
            long offset=Math.min(size,(long)next*chunk),skip=offset;
            while(skip>0){long n=in.skip(skip);if(n==0){if(in.read()==-1)throw new EOFException();n=1;}skip-=n;}
            while(offset<size){
                if(Thread.currentThread().isInterrupted()||!Store.prefs(c).getBoolean("upload",false))throw new IOException("전송 중지");
                byte[] data=new byte[(int)Math.min(chunk,size-offset)];int pos=0;
                while(pos<data.length){int n=in.read(data,pos,data.length-pos);if(n<0)throw new EOFException();pos+=n;}
                byte[] digest=java.security.MessageDigest.getInstance("SHA-256").digest(data);StringBuilder hash=new StringBuilder();for(byte b:digest)hash.append(String.format(java.util.Locale.ROOT,"%02x",b&255));
                chunkRequest(base,token,id,"chunk?index="+next,data,hash.toString());offset+=data.length;next++;
                Store.prefs(c).edit().putString("status","녹음 분할 전송 · "+(offset*100/size)+"%").apply();
            }
        }
        verifyRecording(c,uri,meta);
        if(!chunkRequest(base,token,id,"complete",new byte[0],null).optBoolean("accepted"))throw new IOException("녹음 저장 확인 실패");
    }
    static void uploadAttachment(Context c,String base,String token,JSONObject a)throws Exception{
        String id=a.getString("fileId"),local=a.getString("localFile");
        if(!id.matches("[a-f0-9]{64}")||!id.equals(local))throw new IOException("Invalid attachment reference");
        File file=new File(new File(c.getFilesDir(),"attachments"),local+".blob");
        if(!file.isFile()||file.length()!=a.getLong("size"))throw new IOException("첨부 파일을 다시 공유해 주세요");
        HttpURLConnection con=(HttpURLConnection)new URL(base.replaceAll("/+$","")+"/v1/attachments/"+id).openConnection();
        try{con.setInstanceFollowRedirects(false);con.setConnectTimeout(15000);con.setReadTimeout(90000);con.setRequestMethod("POST");con.setDoOutput(true);con.setFixedLengthStreamingMode(file.length());
            con.setRequestProperty("Authorization","Bearer "+token);con.setRequestProperty("Idempotency-Key",id);con.setRequestProperty("Content-Type","application/octet-stream");
            try(InputStream in=new FileInputStream(file);OutputStream out=con.getOutputStream()){byte[] b=new byte[65536];int n;while((n=in.read(b))!=-1){if(Thread.currentThread().isInterrupted()||!Store.prefs(c).getBoolean("upload",false))throw new IOException("전송 중지");out.write(b,0,n);}}
            if(con.getResponseCode()!=200)throw new IOException("첨부 전송 HTTP "+con.getResponseCode());
            try(InputStream in=con.getInputStream();ByteArrayOutputStream out=new ByteArrayOutputStream()){byte[] b=new byte[1024];int n;while((n=in.read(b))!=-1){out.write(b,0,n);if(out.size()>8192)throw new IOException("Invalid response");}JSONObject ack=new JSONObject(out.toString("UTF-8"));if(!ack.optBoolean("accepted")||!id.equals(ack.optString("id")))throw new IOException("첨부 저장 확인 실패");}
        }finally{con.disconnect();}
    }
}
