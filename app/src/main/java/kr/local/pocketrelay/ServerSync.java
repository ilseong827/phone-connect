package kr.local.pocketrelay;

import android.content.Context;
import android.content.SharedPreferences;

final class ServerSync {
    // The random upload key survives tunnel URL changes. A new key means a new
    // destination (or key rotation); re-upload is safe because ids are idempotent.
    static boolean changed(String previous,String next){return !next.equals(previous);}
    static void save(Context c,String url,String key,boolean wifi,boolean force)throws Exception{
        Transport.endpoint(url);
        if(key.length()<16||key.contains("\n")||key.contains("\r"))throw new Exception("인증키를 확인하세요.");
        synchronized(SyncJob.SERVER_LOCK){
            SharedPreferences p=Store.prefs(c);
            String identity=Store.hash("destination|"+key);
            boolean reset=force||changed(p.getString("syncDestination",""),identity);
            // Disable sends before changing credentials. Failure leaves a retryable,
            // paused state rather than sending with half-updated credentials.
            p.edit().putBoolean("upload",false).commit();
            Secrets.save(c,key);
            if(reset){try(Store db=new Store(c)){db.resetRecoverableSent();}}
            SharedPreferences.Editor edit=p.edit().putString("url",url).putBoolean("wifi",wifi)
                .putString("syncDestination",identity).putBoolean("upload",true);
            if(reset){
                for(String name:p.getAll().keySet())if(name.startsWith("contact-"))edit.remove(name);
                edit.putString("status","현재 서버로 문자·녹음을 선택한 기간부터 다시 동기화합니다. 이전 카톡은 기존 서버 데이터 이전이 필요합니다.");
            }
            if(!edit.commit())throw new Exception("연결 설정 저장 실패. 다시 저장해 주세요.");
        }
        SyncJob.schedule(c,false);
    }
}
