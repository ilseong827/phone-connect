'use strict';
const $=id=>document.getElementById(id);
const state={records:new Map(),cursor:0,mode:'chats',selected:null,query:'',busy:false,groups:[]};
const names=new Map();let namesVersion=0;
function contactLabel(e){return (e.address||'').split(',').map(number=>{number=number.trim();return names.get((e.deviceId||'')+'|'+number)||number;}).join(', ')||'번호 미확인';}
const clock=new Intl.DateTimeFormat('ko-KR',{hour:'numeric',minute:'2-digit'});
const calendar=new Intl.DateTimeFormat('ko-KR',{year:'numeric',month:'long',day:'numeric'});
function el(tag,cls,text){const n=document.createElement(tag);if(cls)n.className=cls;if(text!==undefined)n.textContent=text;return n;}
function stamp(e){return Number(e.messageTime||e.receivedAt||e.fileModifiedAt||e.discoveredAt||0);}
function time(t){return t>0&&Number.isFinite(t)?clock.format(t):'시각 미확인';}
function day(t){return t>0&&Number.isFinite(t)?calendar.format(t):'날짜 미확인';}
function recordingPlayer(e,card){
 const transcriptButton=el('button','','통화 내용 / 전사 상태 확인'),transcript=el('pre','transcript','');transcript.style.whiteSpace='pre-wrap';
 transcriptButton.addEventListener('click',async()=>{transcript.textContent='확인 중…';try{const r=await fetch('/api/record/'+e.id);if(!r.ok)throw Error();const data=await r.json(),t=data.transcript;transcript.textContent=!t?'아직 전사되지 않았습니다. 서버에서 pocketrelay transcribe --all을 실행하세요.':t.status==='done'?'자동 전사 · 원음과 다를 수 있습니다\n\n'+t.text:({queued:'전사 대기 중',processing:'음성을 텍스트로 변환 중',failed:'전사 실패 · 서버 doctor 설정 확인'}[t.status]||t.status);}catch{transcript.textContent='전사 상태를 불러오지 못했습니다.'}});card.append(transcriptButton,transcript);
 const audio=el('audio');audio.controls=true;audio.preload='metadata';audio.src='/audio/'+e.id+'?format=wav';
 const actions=el('div','audio-actions'),play=el('button','','재생'),restart=el('button','','다시 불러오기');
 const status=el('p','audio-status','웹 재생용 녹음 준비 중… 긴 파일은 처음에 시간이 걸릴 수 있습니다.');status.setAttribute('role','status');
 let failure='';
 function update(){
  play.textContent=audio.paused?'재생':'일시정지';
  if(failure){status.textContent=failure;return;}
  if(audio.error){status.textContent=({1:'재생이 중단됐습니다.',2:'녹음 전송 오류입니다.',3:'브라우저가 녹음을 해독하지 못했습니다.',4:'브라우저가 이 녹음 형식을 지원하지 않습니다.'}[audio.error.code]||'녹음 재생 오류입니다.')+' 다시 불러오기를 눌러 주세요.';return;}
  const position=Number.isFinite(audio.currentTime)?audio.currentTime.toFixed(1):'0.0',duration=Number.isFinite(audio.duration)?audio.duration.toFixed(1):'?';
  status.textContent=(audio.ended?'재생 완료':audio.paused?'재생 대기':'재생 중')+' · '+position+' / '+duration+'초'+(audio.muted||audio.volume===0?' · 음소거됨':'');
 }
 play.addEventListener('click',async()=>{failure='';if(!audio.paused){audio.pause();return;}try{await audio.play();}catch(error){failure='재생 실패: '+error.name+' · '+error.message;update();}});
 restart.addEventListener('click',()=>{failure='';audio.src='/audio/'+e.id+'?format=wav&retry='+Date.now();audio.load();status.textContent='웹 재생용 녹음을 다시 불러오는 중…';});
 for(const event of ['loadedmetadata','canplay','play','pause','timeupdate','ended','error','volumechange'])audio.addEventListener(event,update);
 actions.append(play,restart);const link=el('a','','원본 파일 다운로드');link.href='/audio/'+e.id+'?download=1';card.append(audio,actions,status,link);
}
function groupInfo(e){
 if(e.type==='file')return {key:'file:'+e.id,title:e.fileName||'공유한 파일',uncertain:false};
 if(e.type==='sms')return {key:'sms:'+(e.threadId||e.address),title:contactLabel(e),uncertain:false};
 if(e.type==='recording')return {key:'audio:'+e.id,title:e.fileName||'이름 없는 녹음',uncertain:false};
 if(e.chatRoom)return {key:'room:'+e.chatRoom,title:e.chatRoom,uncertain:false};
 if(e.notificationTitle)return {key:'title:'+e.notificationTitle,title:e.notificationTitle,uncertain:true};
 return {key:'unknown',title:'채팅방 미확인',uncertain:true};
}
function collect(){const groups=new Map();for(const e of state.records.values()){
 if(e.type!==({chats:'kakao',sms:'sms',audio:'recording',files:'file'}[state.mode]))continue;
 const info=groupInfo(e);if(!groups.has(info.key))groups.set(info.key,{...info,messages:[]});groups.get(info.key).messages.push(e);
 }for(const g of groups.values()){g.messages.sort((a,b)=>stamp(a)-stamp(b)||a.sequence-b.sequence);g.last=g.messages[g.messages.length-1];}
 return [...groups.values()].sort((a,b)=>stamp(b.last)-stamp(a.last)||b.last.sequence-a.last.sequence);}
function list(){state.groups=collect();$('count').textContent=state.mode==='files'?`${state.groups.length}개 파일`:state.mode!=='audio'?`${state.groups.length}개 대화 · ${state.groups.reduce((n,g)=>n+g.messages.length,0)}개 메시지`:`${state.groups.length}개 녹음`;
 const visible=state.groups.filter(g=>(g.title+' '+g.messages.map(e=>(e.address||'')+' '+(e.sender||'')+' '+(e.message||'')).join(' ')).toLowerCase().includes(state.query));$('list').replaceChildren();
 for(const g of visible){const b=el('button','room'+(g.key===state.selected?' selected':''));b.setAttribute('aria-pressed',String(g.key===state.selected));b.append(el('span','avatar',state.mode!=='audio'?g.title.slice(0,1):'♫'));const copy=el('span','copy');copy.append(el('strong','',g.title),el('span','preview',state.mode!=='audio'?g.last.message||'(내용 없음)':`${(Number(g.last.size||0)/1048576).toFixed(1)} MB · ${day(stamp(g.last))}`));b.append(copy,el('time','',time(stamp(g.last))));b.addEventListener('click',()=>{state.selected=g.key;$('app').classList.add('open');list();thread(true);});$('list').append(b);}
 if(!visible.length)$('list').append(el('div','notice',state.query?'검색 결과가 없습니다.':'아직 수신한 기록이 없습니다.'));
}
function thread(force=false){const area=$('thread'),g=state.groups.find(g=>g.key===state.selected);const signature=g?g.key+'|'+namesVersion+'|'+g.messages.map(e=>e.id).join(','):state.mode+'empty';if(!force&&area.dataset.signature===signature)return;
 const bottom=area.scrollHeight-area.scrollTop-area.clientHeight<90,top=area.scrollTop;area.dataset.signature=signature;area.replaceChildren();
 if(!g){$('title').textContent=state.mode==='files'?'공유 파일 보관함':state.mode!=='audio'?'나의 대화 수신함':'나의 녹음 보관함';$('subtitle').textContent='왼쪽 목록에서 기록을 선택하세요';const empty=el('div','empty');empty.append(el('div','symbol',state.mode!=='audio'?'▤':'♫'),el('h3','',state.mode==='files'?'파일을 선택해 주세요':state.mode!=='audio'?'대화를 선택해 주세요':'녹음을 선택해 주세요'),el('p','',state.mode!=='audio'?(state.mode==='files'?'휴대폰에서 공유 → 내 기록 보내기로 전달한 파일입니다.':state.mode==='sms'?'주소록 이름으로 문자를 모읍니다. 연락처 권한이 없거나 미등록 번호이면 번호로 표시합니다.':'받은 카톡 알림이 대화방별로 모입니다. 새 메시지는 자동으로 반영됩니다.'):'원본 녹음을 선택해 바로 재생할 수 있습니다.'));area.append(empty);$('jump').classList.add('hidden');return;}
 $('title').textContent=g.title;$('subtitle').textContent=state.mode==='sms'?`${g.last.address||''} · ${g.messages.length}개 메시지`:state.mode==='files'?'직접 공유 · 표시된 시간은 공유 시각':state.mode==='audio'?'저장된 녹음 원본':g.uncertain?'알림 제목 기준 묶음 · 실제 채팅방은 미확인':`${g.messages.length}개 메시지 · ${state.mode==='sms'?'문자 대화 기준':'채팅방 이름 기준'}`;
 let previous='';for(const e of g.messages){const date=day(stamp(e));if(date!==previous){const d=el('div','day');d.append(el('span','',date));area.append(d);previous=date;}
 if(state.mode==='audio'){const card=el('article','record');card.append(el('h3','',e.fileName||'녹음'),el('p','',`${(Number(e.size||0)/1048576).toFixed(2)} MB · ${e.durationMs?Math.round(e.durationMs/1000)+'초':'길이 미확인'} · 파일 수정 ${time(stamp(e))}`));if(/^[a-f0-9]{64}$/.test(e.id))recordingPlayer(e,card);area.append(card);continue;}
 const outgoing=e.type==='sms'&&e.direction==='outgoing';
 const row=el('article','msg'+(outgoing?' outgoing':''));row.append(el('span','avatar',outgoing?'나':e.type==='sms'?contactLabel(e).slice(0,1):e.type==='file'?'▧':(e.sender||'?').slice(0,1)));const body=el('div','body');body.append(el('div','sender',outgoing?'나 · 보낸 문자':e.type==='file'?'직접 공유한 파일':e.type==='sms'?contactLabel(e):e.sender||'발신자 미확인'));const line=el('div','line');const bubble=el('div','bubble');
 if(e.message)bubble.append(el('div','',e.message));
 for(const a of e.attachments||[]){
  const id=a.fileId||a.imageId,mime=a.mimeType||'',name=a.fileName||'첨부파일';
  if(/^[a-f0-9]{64}$/.test(id||'')){
   const url='/file/'+id;
   if(['image/jpeg','image/png','image/gif','image/webp'].includes(mime)){const link=el('a','attachment');link.href=url+'?preview=1';link.target='_blank';link.rel='noopener';const img=el('img');img.src=link.href;img.loading='lazy';img.alt=a.source==='notificationPreview'?'알림 미리보기':name;link.append(img);bubble.append(link);}
   else if(['video/mp4','video/webm','audio/mpeg','audio/ogg','audio/wav'].includes(mime)){const media=el(mime.startsWith('video/')?'video':'audio','attachment-media');media.controls=true;media.preload='metadata';media.src=url+'?preview=1';bubble.append(media);}
   const download=el('a','attachment',`${name} · ${(Number(a.size||0)/1048576).toFixed(2)} MB · 다운로드`);download.href=url;download.setAttribute('download',name);bubble.append(download);
   if(a.source==='notificationAttachment'||a.source==='notificationPreview')bubble.append(el('div','attachment-note','알림 제공 파일 · 원본 또는 미리보기 여부는 카카오톡 제공 내용에 따라 다릅니다.'));
  }
  else bubble.append(el('div','attachment-missing','첨부 수집 실패: '+(a.unavailable==='Unsupported image format'?'이전 버전의 파일 형식 판별 오류입니다. 파일을 다시 공유해 주세요.':a.unavailable||'파일 주소 또는 접근 권한이 없습니다.') ));
 }
 if(!bubble.childNodes.length)bubble.textContent='(내용 없음)';line.append(bubble,el('time','',time(stamp(e))));body.append(line);row.append(body);area.append(row);
 }if(force||bottom){area.scrollTop=area.scrollHeight;$('jump').classList.add('hidden');}else{area.scrollTop=top;$('jump').classList.remove('hidden');}
}
function mode(m){state.mode=m;state.selected=null;state.query='';$('search').value='';$('app').classList.remove('open');$('heading').textContent=({chats:'카카오톡',sms:'문자',audio:'녹음',files:'파일'}[m]);$('search').placeholder=m!=='audio'?'대화방 또는 메시지 검색':'녹음 파일 검색';for(const id of ['chats','sms','audio','files']){$(id).classList.toggle('active',id===m);$(id).setAttribute('aria-pressed',String(id===m));}$('foot').replaceChildren(el('b','',m==='chats'?'카카오톡 알림':m==='sms'?'SMS · MMS':m==='files'?'직접 공유한 파일':'녹음 원본'),document.createTextNode(m==='chats'?'알림이 제공하는 첨부를 표시합니다. 없는 파일은 휴대폰에서 공유해 주세요.':m==='sms'?'받은 문자와 보낸 문자 · MMS 사진 포함 · RCS 제외':m==='files'?'공유 시각 기준 · 원래 채팅방·발신자·메시지 시각은 제공되지 않습니다.':'목록에 표시된 날짜는 파일 수정 시각입니다.'));list();thread(true);}
async function poll(){if(state.busy)return;state.busy=true;try{let more=true,changed=false;while(more){const response=await fetch('/api/events?after='+state.cursor,{cache:'no-store'});if(!response.ok)throw Error();const data=await response.json();for(const e of data.events){if(e.type==='contacts'){for(const item of e.entries||[])names.set((e.deviceId||'')+'|'+item.address,item.displayName||'');namesVersion++;}else state.records.set(e.id,e);changed=true;}state.cursor=data.cursor;more=data.more;}$('live').textContent='● 자동 수신 중';$('live').classList.remove('error');if(changed){list();thread();}}catch{$('live').textContent='● 연결 재시도 중';$('live').classList.add('error');}finally{state.busy=false;}}
$('chats').addEventListener('click',()=>mode('chats'));$('audio').addEventListener('click',()=>mode('audio'));$('sms').addEventListener('click',()=>mode('sms'));$('files').addEventListener('click',()=>mode('files'));$('back').addEventListener('click',()=>$('app').classList.remove('open'));$('search').addEventListener('input',e=>{state.query=e.target.value.toLowerCase();list();});$('jump').addEventListener('click',()=>{$('thread').scrollTop=$('thread').scrollHeight;$('jump').classList.add('hidden');});$('close').addEventListener('click',()=>$('settings').close());
$('connect').addEventListener('click',async()=>{
 $('settings').showModal();$('connection-qr').replaceChildren();$('qr-note').textContent='연결 정보를 불러오는 중…';$('apk').classList.add('hidden');
 try{const r=await fetch('/api/connection',{cache:'no-store'});if(!r.ok)throw Error();const c=await r.json();$('url').textContent=c.url||'주소 준비 중';$('token').textContent=c.token||'인증키 없음';
  const u=new URL(c.url);if(u.protocol!=='https:'||u.username||u.password||u.search||u.hash||!c.token||c.token.length<16)throw Error();
  const payload='pocketrelay://pair#'+new URLSearchParams({v:'1',server:c.url.replace(/\/$/,''),key:c.token}).toString();
  new QRCode($('connection-qr'),{text:payload,width:280,height:280,correctLevel:QRCode.CorrectLevel.M});$('connection-qr').removeAttribute('title');
  $('qr-note').textContent='휴대폰 앱 → 서버 → QR로 연결을 누르고 이 QR 코드를 찍으세요. 서버 주소와 인증키가 바로 입력됩니다.';
  $('apk').href=c.url.replace(/\/$/,'')+'/app.apk';$('apk').classList.remove('hidden');
 }catch{$('qr-note').textContent='QR을 만들 수 없습니다. 서버 주소와 인증키를 확인하거나 직접 입력하세요.';}
});
mode('chats');poll();setInterval(poll,3000);
if(location.hash==='#connection')$('connect').click();

async function dotStatus(){try{const r=await fetch('/api/dot-connection',{cache:'no-store'});const ready=r.ok;document.getElementById('dot-config').classList.toggle('hidden',!ready);document.getElementById('dot-note').textContent=ready?'Dot 연결 파일을 받아 Dot에게 전달하세요.':'조회 서버 주소를 준비 중입니다. 잠시 후 자동으로 다시 확인합니다.';}catch{document.getElementById('dot-note').textContent='조회 서버 연결을 확인할 수 없습니다.';}}
document.getElementById('connect').addEventListener('click',dotStatus);
setInterval(()=>{if(document.getElementById('settings').open)dotStatus();},5000);
