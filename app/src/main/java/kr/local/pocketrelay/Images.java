package kr.local.pocketrelay;

import android.content.Context;
import android.net.Uri;
import android.graphics.Bitmap;
import android.util.Base64;
import org.json.*;
import java.io.*;

class Images {
    static final int MAX_BYTES=100*1024*1024;
    static JSONObject fromUri(Context c,Uri uri,String mime,String source) {
        File temporary=null;
        try(InputStream in=c.getContentResolver().openInputStream(uri)) {
            if(in==null)return missing(mime,source,"파일을 열 수 없음");
            File dir=new File(c.getFilesDir(),"attachments");if(!dir.isDirectory()&&!dir.mkdirs())throw new IOException("storage");
            temporary=File.createTempFile("copy-",".tmp",dir);long size=0;java.security.MessageDigest digest=java.security.MessageDigest.getInstance("SHA-256");
            try(FileOutputStream out=new FileOutputStream(temporary)){byte[] b=new byte[65536];int n;while((n=in.read(b))!=-1){size+=n;if(size>MAX_BYTES)return missing(mime,source,"파일당 100MB 제한 초과");out.write(b,0,n);digest.update(b,0,n);}out.getFD().sync();}
            StringBuilder id=new StringBuilder();for(byte b:digest.digest())id.append(String.format("%02x",b));
            File file=new File(dir,id+".blob");if(!file.exists()&&!temporary.renameTo(file))throw new IOException("save");
            String name="첨부파일";
            try(android.database.Cursor r=c.getContentResolver().query(uri,new String[]{android.provider.OpenableColumns.DISPLAY_NAME},null,null,null)){if(r!=null&&r.moveToFirst()&&r.getString(0)!=null)name=r.getString(0);}catch(Exception ignored){}
            try{String resolved=c.getContentResolver().getType(uri);if(resolved!=null&&!resolved.endsWith("/")&&!resolved.endsWith("/*"))mime=resolved;}catch(Exception ignored){}
            return new JSONObject().put("mimeType",mime==null?"application/octet-stream":mime).put("source",source).put("fileName",name).put("fileId",id.toString()).put("localFile",id.toString()).put("size",size);
        }catch(SecurityException ex){return missing(mime,source,"카카오톡 또는 제공 앱이 파일 읽기 권한을 주지 않았습니다. 해당 파일에서 공유 → 내 기록 보내기를 이용하세요.");}
        catch(Exception ex){return missing(mime,source,"파일을 읽거나 저장하지 못했습니다. 원본을 다운로드한 뒤 공유해 주세요.");}
        finally{if(temporary!=null)temporary.delete();}
    }
    static JSONObject fromBitmap(Bitmap image) {
        try{int max=Math.max(image.getWidth(),image.getHeight());Bitmap scaled=max>1280?Bitmap.createScaledBitmap(image,Math.max(1,image.getWidth()*1280/max),Math.max(1,image.getHeight()*1280/max),true):image;
            ByteArrayOutputStream out=new ByteArrayOutputStream();scaled.compress(Bitmap.CompressFormat.JPEG,85,out);if(scaled!=image)scaled.recycle();return encoded(out.toByteArray(),"image/jpeg","notificationPreview");
        }catch(Exception e){return missing("image/jpeg","notificationPreview","미리보기 변환 실패");}
    }
    static JSONObject encoded(byte[] bytes,String mime,String source)throws JSONException {
        if(bytes.length>8*1024*1024)return missing(mime,source,"미리보기 크기 초과");
        return new JSONObject().put("mimeType",mime).put("source",source).put("data",Base64.encodeToString(bytes,Base64.NO_WRAP)).put("size",bytes.length);
    }
    static JSONObject missing(String mime,String source,String reason) {
        try{return new JSONObject().put("mimeType",mime).put("source",source).put("unavailable",reason);}catch(JSONException e){throw new IllegalStateException(e);}
    }
}
