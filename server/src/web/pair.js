'use strict';
(()=>{
 const $=id=>document.getElementById(id),params=new URLSearchParams(location.hash.slice(1));
 try{
  const raw=params.get('server')||'',key=params.get('key')||'',server=new URL(raw);
  if(params.get('v')!=='1'||server.protocol!=='https:'||!server.hostname||server.username||server.password||server.search||server.hash||raw.length>2048||key.length<16||key.length>512||/[\r\n\x00-\x1f]/.test(key))throw Error();
  const base=server.href.replace(/\/$/,'');
  const fragment=new URLSearchParams({v:'1',server:base,key}).toString();
  const deep='pocketrelay://pair#'+fragment;
  $('server').value=base;$('key').value=key;$('settings').classList.remove('hidden');
  $('open').href=deep;
  $('apk').href=base+'/app.apk';
  new QRCode($('qr'),{text:deep,width:320,height:320,correctLevel:QRCode.CorrectLevel.M});
  $('status').textContent='휴대폰 앱의 서버 → QR로 연결을 누르고 아래 QR을 찍으세요.';
  $('show-key').addEventListener('click',()=>{$('key').type=$('key').type==='password'?'text':'password';$('show-key').textContent=$('key').type==='password'?'인증키 보기':'인증키 숨기기'});
  for(const name of ['server','key'])$('copy-'+name).addEventListener('click',async()=>{try{await navigator.clipboard.writeText($(name).value);$('status').textContent='복사했습니다.'}catch{$(name).focus();$(name).select();$('status').textContent='선택한 값을 직접 복사하세요.'}});
 }catch{$('status').textContent='연결 정보가 없거나 올바르지 않습니다. 서버에서 pocketrelay --pair를 실행하고 출력된 pairLink를 여세요.'}
})();
