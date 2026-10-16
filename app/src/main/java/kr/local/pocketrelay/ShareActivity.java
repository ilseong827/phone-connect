package kr.local.pocketrelay;

import android.app.Activity;
import android.content.Intent;
import android.net.Uri;
import android.os.Bundle;
import android.widget.*;
import org.json.*;
import java.util.*;

/** Explicit Android sharing grants read access to the selected files only. */
public class ShareActivity extends Activity {
    final ArrayList<Uri> uris=new ArrayList<>();
    public void onCreate(Bundle state){super.onCreate(state);
        Intent intent=getIntent();
        if(Intent.ACTION_SEND_MULTIPLE.equals(intent.getAction())){ArrayList<Uri> list=intent.getParcelableArrayListExtra(Intent.EXTRA_STREAM);if(list!=null)uris.addAll(list);}
        else{Uri uri=intent.getParcelableExtra(Intent.EXTRA_STREAM);if(uri!=null)uris.add(uri);}
        if(intent.getClipData()!=null)for(int i=0;i<intent.getClipData().getItemCount();i++){Uri uri=intent.getClipData().getItemAt(i).getUri();if(uri!=null&&!uris.contains(uri))uris.add(uri);}
        LinearLayout box=new LinearLayout(this);box.setOrientation(LinearLayout.VERTICAL);box.setPadding(40,60,40,40);ScrollView scroll=new ScrollView(this);scroll.addView(box);setContentView(scroll);
        TextView title=new TextView(this);title.setText("파일 보내기");title.setTextSize(24);box.addView(title);
        TextView info=new TextView(this);info.setText("선택한 "+uris.size()+"개 파일을 내 PC로 보냅니다. GIF·사진·영상·PDF·Word·Excel 등 형식 제한 없이 보관합니다. 파일당 100MB, 한 번에 10개까지입니다.\n\n채팅방·발신자·원래 메시지 시각은 공유 기능에서 제공되지 않아 자동으로 붙이지 않습니다. PC의 ‘파일’ 메뉴에 공유 시각으로 표시합니다.");box.addView(info);
        EditText note=new EditText(this);note.setHint("파일 설명 (선택)");box.addView(note);
        Button send=new Button(this);send.setText("내 PC로 보내기");box.addView(send);
        if(uris.isEmpty()||uris.size()>10){send.setEnabled(false);info.append("\n파일 1~10개를 선택해서 다시 공유해 주세요.");}
        send.setOnClickListener(v->{send.setEnabled(false);String description=note.getText().toString();info.setText("선택한 파일을 앱에 복사하고 있습니다…");
            new Thread(()->{try(Store db=new Store(this)){int added=0,failed=0;for(Uri uri:uris){if(!"content".equals(uri.getScheme())){failed++;continue;}JSONObject file=Images.fromUri(this,uri,intent.getType(),"androidShare");JSONArray files=new JSONArray().put(file);long now=System.currentTimeMillis();JSONObject p=new JSONObject().put("message",description).put("source","androidShare").put("capturedAt",now).put("messageTime",now).put("attachments",files).put("fileName",file.optString("fileName","공유한 파일"));db.enqueue(Store.hash(UUID.randomUUID().toString()),"file",p,null);if(file.has("unavailable"))failed++;else added++;}
                SyncJob.schedule(this,false);final String result=added+"개 전송 대기 · "+failed+"개 파일 접근 실패\n서버 전송이 켜져 있으면 자동으로 보냅니다.";runOnUiThread(()->{info.setText(result);send.setText("닫기");send.setEnabled(true);send.setOnClickListener(w->finish());});
            }catch(Exception ex){runOnUiThread(()->{info.setText("파일 저장 실패. 저장 공간을 확인한 뒤 다시 공유해 주세요.");send.setText("닫기");send.setEnabled(true);send.setOnClickListener(w->finish());});}}).start();
        });
    }
}
