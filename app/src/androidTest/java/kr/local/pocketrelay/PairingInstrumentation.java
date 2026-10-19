package kr.local.pocketrelay;

import android.app.Activity;
import android.app.Instrumentation;
import android.content.Intent;
import android.content.SharedPreferences;
import android.net.Uri;
import android.os.Bundle;
import java.util.concurrent.atomic.AtomicReference;

/** Real-device import test. It never presses Connect and never changes saved settings. */
public class PairingInstrumentation extends Instrumentation {
    Bundle arguments;
    @Override public void onCreate(Bundle args){super.onCreate(args);arguments=args;start();}
    @Override public void onStart(){
        Bundle result=new Bundle();MainActivity activity=null;
        try{
            if(arguments!=null&&"true".equals(arguments.getString("backgroundOnly"))){
                android.content.Context c=getTargetContext();
                android.app.job.JobScheduler scheduler=c.getSystemService(android.app.job.JobScheduler.class);
                new RecoveryReceiver().onReceive(c,new Intent(Intent.ACTION_MY_PACKAGE_REPLACED));
                android.app.job.JobInfo periodic=scheduler.getPendingJob(100);
                if(periodic==null||!periodic.isPersisted()||periodic.getIntervalMillis()!=900000L)throw new AssertionError("Periodic recovery failed");
                if(scheduler.getPendingJob(101)==null)throw new AssertionError("Immediate recovery failed");
                if(Store.prefs(c).getBoolean("sms",false)&&SmsSync.allowed(c)&&scheduler.getPendingJob(102)==null)throw new AssertionError("SMS watch recovery failed");
                SyncJob.schedule(c,true);SyncJob.watchSms(c);
                if(!periodic.equals(scheduler.getPendingJob(100)))throw new AssertionError("Periodic job changed");
                result.putString("stream","PASS: update recovery restores persisted periodic work, immediate work and enabled SMS watcher.\n");
                finish(Activity.RESULT_OK,result);return;
            }
            testResyncQueue();
            SharedPreferences p=Store.prefs(getTargetContext());String oldURL=p.getString("url",""),oldToken=p.getString("token","");boolean oldUpload=p.getBoolean("upload",false);
            Intent intent=new Intent(Intent.ACTION_VIEW,Uri.parse("pocketrelay://pair#v=1&server=https%3A%2F%2Fexample.com&key=test-only-0123456789%2B%26"));
            intent.setPackage(getTargetContext().getPackageName());intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK|Intent.FLAG_ACTIVITY_CLEAR_TASK);
            activity=(MainActivity)startActivitySync(intent);waitForIdleSync();MainActivity a=activity;AtomicReference<Throwable> failure=new AtomicReference<>();
            runOnMainSync(()->{try{
                if(!"https://example.com".equals(a.server.getText().toString())||!"test-only-0123456789+&".equals(a.token.getText().toString())||!"서버".equals(a.currentPage)||a.manualConnection.getVisibility()!=android.view.View.VISIBLE)throw new AssertionError("Cold-start import failed");
                a.onNewIntent(new Intent(Intent.ACTION_VIEW,Uri.parse("pocketrelay://pair#v=1&server=https%3A%2F%2Fexample.org&key=second-test-0123456789")));
                if(!"https://example.org".equals(a.server.getText().toString())||!"second-test-0123456789".equals(a.token.getText().toString()))throw new AssertionError("Warm import failed");
                if(!oldURL.equals(p.getString("url",""))||!oldToken.equals(p.getString("token",""))||oldUpload!=p.getBoolean("upload",false))throw new AssertionError("Import changed saved settings");
            }catch(Throwable e){failure.set(e);}});
            if(failure.get()!=null)throw new AssertionError(failure.get());
            String network="";
            if(arguments!=null&&arguments.containsKey("testServer")){
                String endpoint=arguments.getString("testServer"),key=arguments.getString("testKey");
                Transport.endpoint(endpoint);
                java.net.HttpURLConnection connection=(java.net.HttpURLConnection)new java.net.URL(endpoint+"/v1/pair/check").openConnection();
                connection.setConnectTimeout(15000);connection.setReadTimeout(15000);connection.setInstanceFollowRedirects(false);connection.setRequestMethod("POST");connection.setRequestProperty("Authorization","Bearer "+key);
                try{if(connection.getResponseCode()!=200)throw new AssertionError("Phone HTTPS authentication failed");}finally{connection.disconnect();}
                network=" Phone HTTPS tunnel authentication also passed.";
            }
            result.putString("stream","PASS: resync clears only recoverable sent markers and preserves pending records; deep-link resolution, cold/warm auto-fill, server navigation; original credentials and upload setting unchanged."+network+"\n");finish(Activity.RESULT_OK,result);
        }catch(Throwable e){result.putString("stream","FAIL: "+e.getClass().getSimpleName()+" "+e.getMessage()+"\n");finish(Activity.RESULT_CANCELED,result);}
        finally{if(activity!=null){MainActivity a=activity;runOnMainSync(a::finish);}}
    }
    void testResyncQueue() throws Exception {
        String name="server-sync-instrumentation.db";
        getTargetContext().deleteDatabase(name);
        try(Store db=new Store(getTargetContext(),name)){
            String[] kinds={"sms","recording","contacts","kakao","file","sms"};
            for(int i=0;i<kinds.length;i++){
                String id=Store.hash("resync-test-"+i);
                db.enqueue(id,kinds[i],new org.json.JSONObject().put("message","test"),null);
                if(i<5)db.getWritableDatabase().execSQL("UPDATE queue SET sent=1,payload='{}' WHERE id=?",new Object[]{id});
            }
            if(db.resetRecoverableSent()!=3)throw new AssertionError("Wrong reset count");
            for(int i=0;i<6;i++)if(db.exists(Store.hash("resync-test-"+i))!=(i>=3))throw new AssertionError("Pending or nonrecoverable history changed");
            if(db.resetRecoverableSent()!=0)throw new AssertionError("Reset not idempotent");
        } finally {getTargetContext().deleteDatabase(name);}
    }
}
