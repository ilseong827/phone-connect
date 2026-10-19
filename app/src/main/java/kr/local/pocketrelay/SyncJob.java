package kr.local.pocketrelay;
import android.app.job.*;
import android.content.*;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicBoolean;

public class SyncJob extends JobService {
    static final AtomicBoolean RUNNING=new AtomicBoolean();
    static final Object SERVER_LOCK=new Object();
    ExecutorService executor=Executors.newSingleThreadExecutor();
    final ConcurrentHashMap<Integer,Future<?>> tasks=new ConcurrentHashMap<>();
    static void schedule(Context c,boolean periodic){
        if(periodic&&c.getSystemService(JobScheduler.class).getPendingJob(100)!=null)return;
        JobInfo.Builder b=new JobInfo.Builder(periodic?100:101,new ComponentName(c,SyncJob.class));
        if(periodic)b.setPeriodic(15*60*1000L).setPersisted(true);
        else b.setMinimumLatency(1000).setBackoffCriteria(30000,JobInfo.BACKOFF_POLICY_EXPONENTIAL);
        // Scan offline too; pending uploads survive network errors.
        c.getSystemService(JobScheduler.class).schedule(b.build());
    }
    static void run(Context c)throws Exception{
        synchronized(SERVER_LOCK){runLocked(c);}
    }
    private static void runLocked(Context c)throws Exception{
        if(!RUNNING.compareAndSet(false,true))return;
        Store.prefs(c).edit().putLong("lastSyncStarted",System.currentTimeMillis()).apply();
        try(Store db=new Store(c)){
            String scanError=null;
            try{Recordings.scan(c,db);}catch(Exception e){scanError="녹음 폴더 확인 필요: "+e.getClass().getSimpleName();}
            try{SmsSync.scan(c,db);}catch(Exception e){scanError="문자 확인 실패: "+e.getClass().getSimpleName();Store.prefs(c).edit().putString("smsStatus",scanError).apply();}
            try{ContactNames.scan(c,db);}catch(Exception e){scanError="주소록 이름 확인 실패: "+e.getClass().getSimpleName();}
            Transport.send(c,db);
            db.cleanupAttachments();
            Store.prefs(c).edit().putString("status",scanError!=null?scanError:"마지막 확인: "+new java.text.SimpleDateFormat("MM/dd HH:mm",java.util.Locale.KOREA).format(new java.util.Date())).apply();
        }finally{Store.prefs(c).edit().putLong("lastSyncFinished",System.currentTimeMillis()).apply();RUNNING.set(false);}
    }
    static void watchSms(Context c){watchSms(c,false);}
    static void watchSms(Context c,boolean rearm){
        if(!Store.prefs(c).getBoolean("sms",false)||!SmsSync.allowed(c)){c.getSystemService(JobScheduler.class).cancel(102);return;}
        if(!rearm&&c.getSystemService(JobScheduler.class).getPendingJob(102)!=null)return;
        JobInfo.Builder b=new JobInfo.Builder(102,new ComponentName(c,SyncJob.class));
        b.addTriggerContentUri(new JobInfo.TriggerContentUri(android.provider.Telephony.Sms.CONTENT_URI,JobInfo.TriggerContentUri.FLAG_NOTIFY_FOR_DESCENDANTS));
        b.addTriggerContentUri(new JobInfo.TriggerContentUri(android.provider.Telephony.Mms.CONTENT_URI,JobInfo.TriggerContentUri.FLAG_NOTIFY_FOR_DESCENDANTS));
        b.setTriggerContentUpdateDelay(5000).setTriggerContentMaxDelay(30000);
        c.getSystemService(JobScheduler.class).schedule(b.build());
    }
    public boolean onStartJob(JobParameters p){
        tasks.put(p.getJobId(),executor.submit(()->{boolean retry=false;try{run(this);}catch(Exception e){retry=true;Store.prefs(this).edit().putString("status","전송 대기 · "+e.getMessage()).apply();}
            if(Thread.currentThread().isInterrupted())return;
            jobFinished(p,retry&&p.getJobId()!=102);if(p.getJobId()==102)watchSms(this,true);
            if(!retry&&Store.prefs(this).getBoolean("upload",false)){try(Store db=new Store(this)){if(db.hasPending())schedule(this,false);}}
        }));return true;
    }
    public boolean onStopJob(JobParameters p){Future<?> task=tasks.remove(p.getJobId());if(task!=null)task.cancel(true);return true;}
    public void onDestroy(){executor.shutdownNow();super.onDestroy();}
}
