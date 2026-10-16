package kr.local.pocketrelay;
import org.junit.Test;
import static org.junit.Assert.*;
public class PairingTest {
    private final String valid="pocketrelay://pair#v=1&server=https%3A%2F%2Fexample.com&key=0123456789abcdef%2B%26";
    @Test public void parsesEncodedKey(){Pairing p=Pairing.parse(valid);assertEquals("https://example.com",p.server);assertEquals("0123456789abcdef+&",p.key);}
    @Test public void rejectsUnsafeLinks(){String[] bad={valid.replace("https%3A","http%3A"),valid+"&key=duplicate",valid.replace("v=1","v=2"),valid.replace("pocketrelay://pair","https://pair"),valid.replace("example.com","user%40example.com"),valid+"%0A",valid.replace("0123456789abcdef%2B%26","short"),"pocketrelay://pair?key=secret",null};for(String s:bad){try{Pairing.parse(s);fail("accepted invalid pairing");}catch(IllegalArgumentException expected){}}}
}
