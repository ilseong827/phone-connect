package kr.local.pocketrelay;

import android.app.*;
import android.content.*;
import android.graphics.Color;
import android.graphics.Typeface;
import android.graphics.drawable.GradientDrawable;
import android.net.Uri;
import android.os.*;
import android.provider.Settings;
import android.text.InputType;
import android.view.*;
import android.widget.*;
import java.util.*;
import java.util.concurrent.*;

public class MainActivity extends Activity {
    LinearLayout body;TextView status,folderLabel;EditText server,token;Spinner weeks;Switch wifi;
    final ExecutorService executor=Executors.newSingleThreadExecutor();
    int ink=Color.rgb(25,38,46),green=Color.rgb(20,107,86);String selectedFolder="";
    SharedPreferences prefs;
    String currentPage="상태";
    TextView smsStatus;
    int dp(int n){return (int)(n*getResources().getDisplayMetrics().density);}
    public void onCreate(Bundle b){super.onCreate(b);prefs=Store.prefs(this);getWindow().setStatusBarColor(Color.rgb(244,247,245));getWindow().setNavigationBarColor(Color.rgb(244,247,245));
        ScrollView scroll=new ScrollView(this);scroll.setFillViewport(true);scroll.setBackgroundColor(Color.rgb(244,247,245));
        body=new LinearLayout(this);body.setOrientation(LinearLayout.VERTICAL);body.setPadding(dp(22),dp(32),dp(22),dp(32));scroll.addView(body);setContentView(scroll);
        body.setOnApplyWindowInsetsListener((v,in)->{v.setPadding(dp(22),dp(16)+in.getSystemWindowInsetTop(),dp(22),dp(24)+in.getSystemWindowInsetBottom());return in;});
        label(body,"POCKET RELAY",12,green,true);label(body,"내 기록 보내기",30,ink,true);label(body,"내 폰의 기록을, 내가 정한 곳으로.",15,ink,false);
        LinearLayout state=card();label(state,"연결 상태",18,ink,true);status=label(state,"",14,ink,false);
        button(state,"상태 새로고침",this::refresh);
        button(state,"전송 대기 기록 보기",this::showPending);
        LinearLayout kakao=card();label(kakao,"01   카카오톡",20,ink,true);
        label(kakao,"발신자 · 채팅방 · 메시지 · 수신 시간\n알림에 없는 정보는 ‘확인 불가’로 보냅니다. 과거 대화는 가져오지 않습니다. 파일은 알림이 접근 주소를 제공할 때만 자동 수집합니다. 사진·GIF·영상·문서 모두 지원하며, 알림에 없는 파일은 카카오톡 또는 내 파일의 공유 → 내 기록 보내기로 전달하세요. 파일당 100MB까지입니다.",14,ink,false);
        Switch capture=new Switch(this);capture.setText("새 카톡 알림 수집");capture.setChecked(prefs.getBoolean("capture",false));kakao.addView(capture);
        capture.setOnCheckedChangeListener((v,on)->prefs.edit().putBoolean("capture",on).apply());
        button(kakao,"카카오톡 알림 접근 허용",()->startActivity(new Intent(Settings.ACTION_NOTIFICATION_LISTENER_SETTINGS)));
        LinearLayout audio=card();label(audio,"02   녹음 파일",20,ink,true);
        label(audio,"Voice Recorder 또는 통화 녹음 폴더를 선택하세요. 파일은 복사 전송하며 원본을 삭제하지 않습니다.",14,ink,false);
        folderLabel=label(audio,"",13,green,false);button(audio,"녹음 폴더 선택",()->{
            Intent i=new Intent(Intent.ACTION_OPEN_DOCUMENT_TREE);i.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION|Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION);startActivityForResult(i,1);
        });
        label(audio,"처음 가져올 기간 · 파일 수정 시각 기준",14,ink,true);
        weeks=new Spinner(this);weeks.setAdapter(new ArrayAdapter<>(this,android.R.layout.simple_spinner_dropdown_item,new String[]{"기존 녹음 제외 · 지금부터","최근 1주","최근 2주","최근 4주","직접 입력"}));weeks.setSelection(2);audio.addView(weeks);
        button(audio,"파일 수·용량 확인 후 시작",()->{
            if(weeks.getSelectedItemPosition()==4){EditText input=new EditText(this);input.setInputType(InputType.TYPE_CLASS_NUMBER);input.setHint("주 수 (1~520)");new AlertDialog.Builder(this).setTitle("최근 몇 주를 가져올까요?").setView(input).setPositiveButton("확인",(d,w)->{try{int n=Integer.parseInt(input.getText().toString());if(n<1||n>520)throw new Exception();preview(n);}catch(Exception e){toast("1~520 사이의 주 수를 입력하세요.");}}).setNegativeButton("취소",null).show();}
            else preview(new int[]{0,1,2,4}[weeks.getSelectedItemPosition()]);
        });
        button(audio,"녹음 자동 수집 중지",()->{prefs.edit().putBoolean("recordings",false).apply();refresh();});
        label(audio,"녹음은 약 15분 주기로 확인합니다. 배터리 절약·절전 상태에서는 더 늦어질 수 있습니다. 크기가 1분 이상 유지된 파일만 전송합니다.",12,ink,false);
        LinearLayout sms=card();label(sms,"내 문자 모아보기",20,ink,true);
        label(sms,"SMS · MMS 문자함의 받은 문자와 보낸 문자를 함께 가져옵니다. 사진 MMS도 포함합니다. RCS 채팅과 카톡 발신은 포함되지 않습니다.",14,ink,false);
        smsStatus=label(sms,"",13,green,false);
        label(sms,"주소록 이름을 표시하려면 연락처 읽기를 허용하세요. 선택한 문자에 등장하는 번호의 이름만 PC로 보냅니다. 이미 전송한 문자에도 이름이 반영됩니다.",13,ink,false);
        button(sms,"주소록 이름 연결 · 연락처 권한",()->{
            if(ContactNames.allowed(this)){syncNow();return;}
            requestPermissions(new String[]{android.Manifest.permission.READ_CONTACTS},21);
        });
        button(sms,"문자 읽기 권한 허용",()->{
            if(SmsSync.allowed(this)){toast("이미 문자 읽기가 허용되어 있습니다.");return;}
            requestPermissions(new String[]{android.Manifest.permission.READ_SMS},20);
        });
        label(sms,"처음 가져올 기간",14,ink,true);
        Spinner smsWeeks=new Spinner(this);smsWeeks.setAdapter(new ArrayAdapter<>(this,android.R.layout.simple_spinner_dropdown_item,new String[]{"지금부터","최근 1주","최근 2주","최근 4주","저장된 문자 전체"}));smsWeeks.setSelection(2);sms.addView(smsWeeks);
        button(sms,"문자 개수 확인 후 시작",()->{
            if(!SmsSync.allowed(this)){toast("먼저 문자 읽기 권한을 허용해 주세요.");return;}
            int index=smsWeeks.getSelectedItemPosition();long cutoff=index==4?0:System.currentTimeMillis()-new int[]{0,1,2,4}[index]*7L*24*60*60*1000;
            executor.execute(()->{try{int count=SmsSync.count(this,cutoff);runOnUiThread(()->new AlertDialog.Builder(this).setTitle("문자 수집 시작").setMessage("선택한 범위의 문자 "+count+"건과 이후 새 수·발신 문자를 가져옵니다. MMS 첨부는 파일당 100MB, 메시지당 10개까지 전송합니다.").setPositiveButton("시작",(d,w)->{prefs.edit().putBoolean("sms",true).putLong("smsCutoff",cutoff).apply();SyncJob.watchSms(this);SyncJob.schedule(this,false);refresh();}).setNegativeButton("취소",null).show());}catch(Exception e){runOnUiThread(()->toast("문자함을 읽을 수 없습니다. 권한을 확인해 주세요."));}});
        });
        button(sms,"문자 자동 수집 중지",()->{prefs.edit().putBoolean("sms",false).apply();SyncJob.watchSms(this);refresh();});
        button(sms,"문자 지금 확인",()->syncNow());
        label(sms,"문자함 변경 시 수집을 예약하고, 약 15분 주기로 다시 확인합니다. 절전 상태에서는 지연될 수 있습니다. 작성 중·실패한 문자는 보낸 문자로 표시하지 않습니다.",12,ink,false);
        LinearLayout network=card();label(network,"03   받을 곳",20,ink,true);
        label(network,"수신 서버는 나중에 연결해도 됩니다. 그동안 카톡은 앱 안에 보관하고, 녹음은 원래 폴더에서 전송을 기다립니다.",14,ink,false);
        server=edit(network,"https://내-서버-주소",false);server.setText(prefs.getString("url",""));
        token=edit(network,"인증키 · 저장된 키는 빈칸이면 유지",true);
        wifi=new Switch(this);wifi.setText("Wi-Fi에서만 전송");wifi.setChecked(prefs.getBoolean("wifi",true));network.addView(wifi);
        button(network,"수신 주소 저장 · 전송 켜기",()->saveServer());
        button(network,"전송 일시 중지",()->{prefs.edit().putBoolean("upload",false).apply();refresh();});
        button(network,"지금 확인하고 전송",()->{
            status.setText("파일과 전송 대기열을 확인 중입니다…");
            executor.execute(()->{try{SyncJob.run(this);}catch(Exception e){prefs.edit().putString("status","전송 대기 · "+e.getMessage()).apply();}runOnUiThread(this::refresh);});
        });
        button(state,"지금 전체 동기화",this::syncNow);
        LinearLayout privacy=card();label(privacy,"내가 제어하는 기록",18,ink,true);
        label(privacy,"수집과 전송은 각각 끌 수 있습니다. 전송 완료 후 메시지 본문과 첨부는 기기 대기열에서 제거됩니다. 녹음 원본은 그대로 남습니다.",13,ink,false);
        button(privacy,"대기 중인 기록 삭제",()->new AlertDialog.Builder(this).setTitle("대기열을 비울까요?").setMessage("카톡·문자·녹음 수집과 전송을 중지하고 미전송 기록을 삭제합니다. 녹음 원본은 삭제하지 않습니다.").setPositiveButton("삭제",(d,w)->{
            prefs.edit().putBoolean("capture",false).putBoolean("recordings",false).putBoolean("sms",false).putBoolean("upload",false).apply();capture.setChecked(false);SyncJob.watchSms(this);
            SharedPreferences.Editor contactCache=prefs.edit();for(String key:prefs.getAll().keySet())if(key.startsWith("contact-"))contactCache.remove(key);contactCache.apply();
            executor.execute(()->{try(Store db=new Store(this)){db.clearPending();}runOnUiThread(this::refresh);});
        }).setNegativeButton("취소",null).show());
        installNavigation(state,kakao,sms,audio,network,privacy);
        SyncJob.schedule(this,true);SyncJob.watchSms(this);refresh();
    }
    void syncNow(){status.setText("문자·녹음·전송 대기열 확인 중…");executor.execute(()->{try{SyncJob.run(this);}catch(Exception e){prefs.edit().putString("status","전송 대기 · "+e.getMessage()).apply();}runOnUiThread(this::refresh);});}
    void installNavigation(LinearLayout state,LinearLayout kakao,LinearLayout sms,LinearLayout audio,LinearLayout network,LinearLayout privacy){
        LinearLayout root=new LinearLayout(this);root.setOrientation(LinearLayout.HORIZONTAL);root.setBackgroundColor(Color.rgb(244,247,245));
        LinearLayout rail=new LinearLayout(this);rail.setOrientation(LinearLayout.VERTICAL);rail.setPadding(dp(5),dp(20),dp(5),dp(12));rail.setBackgroundColor(Color.rgb(22,62,48));root.addView(rail,new LinearLayout.LayoutParams(dp(86),-1));
        label(rail,"p.",30,Color.WHITE,true);
        ScrollView pageScroll=new ScrollView(this);pageScroll.setFillViewport(true);LinearLayout.LayoutParams right=new LinearLayout.LayoutParams(0,-1,1);root.addView(pageScroll,right);
        LinearLayout content=new LinearLayout(this);content.setOrientation(LinearLayout.VERTICAL);content.setPadding(dp(12),dp(18),dp(12),dp(16));pageScroll.addView(content);
        TextView pageTitle=label(content,"상태",26,ink,true);label(content,"내 기록 보내기 · v0.3",11,green,false);
        FrameLayout pages=new FrameLayout(this);content.addView(pages,new LinearLayout.LayoutParams(-1,-2));
        String[] names={"상태","카카오톡","문자","녹음","서버"};LinearLayout[] cards={state,kakao,sms,audio,network};LinearLayout[] containers=new LinearLayout[5];Button[] buttons=new Button[5];
        for(int i=0;i<5;i++){
            LinearLayout page=new LinearLayout(this);page.setOrientation(LinearLayout.VERTICAL);containers[i]=page;
            ((android.view.ViewGroup)cards[i].getParent()).removeView(cards[i]);page.addView(cards[i]);pages.addView(page,new FrameLayout.LayoutParams(-1,-2));page.setVisibility(i==0?View.VISIBLE:View.GONE);
            if(i==0){((android.view.ViewGroup)privacy.getParent()).removeView(privacy);page.addView(privacy);}
            Button nav=new Button(this);nav.setText(names[i]);nav.setTextSize(12);nav.setAllCaps(false);nav.setMinWidth(0);nav.setMinimumWidth(0);nav.setPadding(0,dp(8),0,dp(8));nav.setTextColor(Color.WHITE);nav.setBackgroundColor(i==0?Color.rgb(52,96,75):Color.TRANSPARENT);rail.addView(nav,new LinearLayout.LayoutParams(-1,dp(58)));buttons[i]=nav;
            final int selected=i;nav.setOnClickListener(v->{currentPage=names[selected];pageTitle.setText(currentPage);for(int j=0;j<5;j++){containers[j].setVisibility(j==selected?View.VISIBLE:View.GONE);buttons[j].setBackgroundColor(j==selected?Color.rgb(52,96,75):Color.TRANSPARENT);}pageScroll.scrollTo(0,0);refresh();});
        }
        root.setOnApplyWindowInsetsListener((v,in)->{v.setPadding(0,in.getSystemWindowInsetTop(),0,in.getSystemWindowInsetBottom());return in;});
        setContentView(root);
    }
    LinearLayout card(){LinearLayout c=new LinearLayout(this);c.setOrientation(LinearLayout.VERTICAL);c.setPadding(dp(18),dp(18),dp(18),dp(18));GradientDrawable bg=new GradientDrawable();bg.setColor(Color.WHITE);bg.setCornerRadius(dp(20));c.setBackground(bg);LinearLayout.LayoutParams lp=new LinearLayout.LayoutParams(-1,-2);lp.topMargin=dp(18);body.addView(c,lp);return c;}
    TextView label(LinearLayout box,String text,int size,int color,boolean bold){TextView v=new TextView(this);v.setText(text);v.setTextSize(size);v.setTextColor(color);v.setPadding(0,dp(5),0,dp(9));v.setLineSpacing(dp(3),1);if(bold)v.setTypeface(null,Typeface.BOLD);box.addView(v);return v;}
    void button(LinearLayout box,String text,Runnable run){Button b=new Button(this);b.setText(text);b.setAllCaps(false);b.setTextColor(green);box.addView(b,new LinearLayout.LayoutParams(-1,-2));b.setOnClickListener(v->run.run());}
    EditText edit(LinearLayout box,String hint,boolean password){EditText e=new EditText(this);e.setHint(hint);e.setTextSize(14);e.setSingleLine(true);e.setInputType(password?InputType.TYPE_CLASS_TEXT|InputType.TYPE_TEXT_VARIATION_PASSWORD:InputType.TYPE_CLASS_TEXT|InputType.TYPE_TEXT_VARIATION_URI);e.setImportantForAutofill(View.IMPORTANT_FOR_AUTOFILL_NO);box.addView(e);return e;}
    void saveServer(){try{
        String url=server.getText().toString().trim();Transport.endpoint(url);
        String key=token.getText().toString().trim();if(key.isEmpty())key=Secrets.read(this);if(key.length()<16||key.contains("\n")||key.contains("\r"))throw new Exception("인증키는 줄바꿈 없이 16자 이상 입력하세요.");
        Secrets.save(this,key);prefs.edit().putString("url",url).putBoolean("wifi",wifi.isChecked()).putBoolean("upload",true).apply();token.setText("");SyncJob.schedule(this,false);refresh();toast("저장했습니다.");
    }catch(Exception e){toast(e.getMessage());}}
    void preview(int count){
        String folder=selectedFolder.isEmpty()?prefs.getString("folder",""):selectedFolder;if(folder.isEmpty()){toast("먼저 녹음 폴더를 선택하세요.");return;}
        long cutoff=System.currentTimeMillis()-count*7L*24*60*60*1000;
        toast("녹음 파일을 확인하고 있습니다.");
        executor.execute(()->{try{
            java.util.List<Recordings.Entry> files=Recordings.list(this,Uri.parse(folder));long bytes=0;int number=0,unknown=0;
            for(Recordings.Entry f:files){if(f.modified<=0){unknown++;continue;}if(f.modified>=cutoff){bytes+=f.size;number++;}}
            String summary=number+"개 · "+android.text.format.Formatter.formatFileSize(this,bytes)+"\n\n이후 새로 저장되는 녹음도 자동으로 가져옵니다.\n날짜를 알 수 없어 제외된 파일: "+unknown+"개\n첫 전송은 파일 안정성 확인 후 진행됩니다.";
            runOnUiThread(()->new AlertDialog.Builder(this).setTitle("이 범위로 시작할까요?").setMessage(summary).setPositiveButton("시작",(d,w)->{prefs.edit().putString("folder",folder).putLong("cutoff",cutoff).putBoolean("recordings",true).apply();SyncJob.schedule(this,false);refresh();}).setNegativeButton("취소",null).show());
        }catch(Exception e){runOnUiThread(()->toast("폴더를 읽을 수 없습니다. 다시 선택해 주세요."));}});
    }
    @Override protected void onActivityResult(int request,int result,Intent data){super.onActivityResult(request,result,data);if(request==1&&result==RESULT_OK&&data!=null&&data.getData()!=null){try{
        Uri uri=data.getData();getContentResolver().takePersistableUriPermission(uri,Intent.FLAG_GRANT_READ_URI_PERMISSION);selectedFolder=uri.toString();folderLabel.setText("선택됨 · "+uri.getLastPathSegment()+"\n기간을 확인한 뒤 시작하세요.");
    }catch(Exception e){toast("폴더 접근 권한을 저장하지 못했습니다.");}}}
    void refresh(){if(status==null)return;try(Store db=new Store(this)){
        String enabled=Settings.Secure.getString(getContentResolver(),"enabled_notification_listeners");boolean access=enabled!=null&&enabled.contains(new ComponentName(this,KakaoListener.class).flattenToString());
        status.setText("카톡 접근 권한: "+(access?"허용":"설정 필요")+"\n문자 수집: "+(prefs.getBoolean("sms",false)?"켜짐":"꺼짐")+"\n녹음 자동 수집: "+(prefs.getBoolean("recordings",false)?"켜짐":"꺼짐")+"\n외부 전송: "+(prefs.getBoolean("upload",false)?"켜짐":"꺼짐")+"\n\n"+db.counts()+"\n"+prefs.getString("status","수신 서버 없이 수집부터 시작할 수 있습니다.")+"\n날짜 미확인 녹음: "+prefs.getInt("unknownDates",0)+"개");
        if(smsStatus!=null)smsStatus.setText("문자 권한: "+(SmsSync.allowed(this)?"허용":"허용 필요")+"\n주소록 권한: "+(ContactNames.allowed(this)?"허용":"허용 필요")+"\n자동 수집: "+(prefs.getBoolean("sms",false)?"켜짐":"꺼짐")+"\n"+prefs.getString("smsStatus","기간을 선택하고 수집을 시작하세요."));
        if(selectedFolder.isEmpty())folderLabel.setText(prefs.getString("folder","").isEmpty()?"아직 선택한 폴더가 없습니다.":"선택 폴더 · "+Uri.parse(prefs.getString("folder","")).getLastPathSegment());
    }}
    @Override protected void onResume(){super.onResume();refresh();}
    @Override public void onRequestPermissionsResult(int request,String[] permissions,int[] results){super.onRequestPermissionsResult(request,permissions,results);if(request==20){refresh();toast(SmsSync.allowed(this)?"문자 읽기 허용됨 · 기간을 선택하고 시작하세요.":"문자 권한이 허용되지 않았습니다.");}if(request==21){refresh();if(ContactNames.allowed(this)){toast("주소록 이름 연결 중입니다.");syncNow();}else toast("연락처 권한이 없으면 번호로 표시합니다.");}}
    void toast(String text){Toast.makeText(this,text,Toast.LENGTH_LONG).show();}
    void showPending(){
        StringBuilder text=new StringBuilder();
        try(Store db=new Store(this);android.database.Cursor r=db.getReadableDatabase().rawQuery("SELECT kind,payload FROM queue WHERE sent=0 ORDER BY created DESC LIMIT 20",null)){
            while(r.moveToNext()){
                org.json.JSONObject p=new org.json.JSONObject(db.readPayload(r.getString(1)));
                if(r.getString(0).equals("kakao"))text.append("[카톡] ").append(p.isNull("sender")?"발신자 확인 불가":p.optString("sender")).append("\n채팅방: ").append(p.isNull("chatRoom")?"확인 불가":p.optString("chatRoom")).append("\n알림 제목: ").append(p.optString("notificationTitle")).append("\n").append(p.optString("message")).append("\n수신: ").append(new java.util.Date(p.optLong("receivedAt")));
                else if(r.getString(0).equals("sms"))text.append("[문자 · ").append(p.optString("direction").equals("outgoing")?"보냄":"받음").append("] ").append(p.optString("address")).append("\n").append(p.optString("message")).append("\n첨부 ").append(p.optJSONArray("attachments")==null?0:p.optJSONArray("attachments").length()).append("개");
                else if(r.getString(0).equals("file"))text.append("[공유 파일] ").append(p.optString("fileName")).append("\n").append(p.optString("message"));
                else if(r.getString(0).equals("contacts"))text.append("[주소록 이름 연결] ").append(p.optJSONArray("entries").length()).append("개 번호");
                else text.append("[녹음] ").append(p.optString("fileName")).append("\n").append(android.text.format.Formatter.formatFileSize(this,p.optLong("size")));
                text.append("\n\n");
            }
        }catch(Exception e){toast("대기열을 읽지 못했습니다.");return;}
        TextView content=new TextView(this);content.setText(text.length()==0?"전송 대기 중인 기록이 없습니다.":text.toString());content.setPadding(dp(20),dp(12),dp(20),dp(12));content.setTextIsSelectable(true);
        ScrollView scroll=new ScrollView(this);scroll.addView(content);
        new AlertDialog.Builder(this).setTitle("최근 대기 기록 · 최대 20개").setView(scroll).setPositiveButton("닫기",null).show();
    }
}
