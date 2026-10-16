package kr.local.pocketrelay;
import org.junit.Test;
import static org.junit.Assert.*;

public class ServerSyncTest {
    @Test public void sameDestinationDoesNotReplayForUrlChange(){assertFalse(ServerSync.changed("key-A","key-A"));}
    @Test public void newDestinationAndLegacyUpgradeReplayRecoverableHistory(){assertTrue(ServerSync.changed("key-A","key-B"));assertTrue(ServerSync.changed("","key-A"));}
}
