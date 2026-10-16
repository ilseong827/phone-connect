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
