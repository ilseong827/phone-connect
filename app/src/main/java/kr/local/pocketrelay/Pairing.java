package kr.local.pocketrelay;

import java.net.URI;
import java.net.URLDecoder;
import java.util.HashMap;
import java.util.Map;

/** Pure parser: importing a QR never persists credentials or enables uploads. */
final class Pairing {
    final String server,key;
    private Pairing(String server,String key){this.server=server;this.key=key;}
    static Pairing parse(String link) {
        try {
            if(link==null||link.length()>8192)throw new Exception();
            URI uri=new URI(link);
            if(!"pocketrelay".equals(uri.getScheme())||!"pair".equals(uri.getHost())||uri.getUserInfo()!=null||uri.getPort()!=-1||uri.getRawQuery()!=null||!(uri.getPath()==null||uri.getPath().isEmpty())||uri.getRawFragment()==null)throw new Exception();
            Map<String,String> values=new HashMap<>();
            for(String item:uri.getRawFragment().split("&")){
                String[] pair=item.split("=",2);if(pair.length!=2)throw new Exception();
                String name=URLDecoder.decode(pair[0],"UTF-8"),value=URLDecoder.decode(pair[1],"UTF-8");
                if(values.put(name,value)!=null)throw new Exception();
            }
            if(!"1".equals(values.get("v")))throw new Exception();
            String server=values.get("server"),key=values.get("key");
            if(server==null||server.length()>2048||key==null||key.length()<16||key.length()>512)throw new Exception();
            for(int i=0;i<key.length();i++)if(Character.isISOControl(key.charAt(i)))throw new Exception();
            URI target=new URI(server);
            if(!"https".equals(target.getScheme())||target.getHost()==null||target.getHost().isEmpty()||target.getUserInfo()!=null||target.getRawQuery()!=null||target.getRawFragment()!=null)throw new Exception();
            while(server.endsWith("/"))server=server.substring(0,server.length()-1);
            return new Pairing(server,key);
        } catch(Exception e){throw new IllegalArgumentException("올바른 Phone Connect 연결 QR이 아닙니다.");}
    }
}
