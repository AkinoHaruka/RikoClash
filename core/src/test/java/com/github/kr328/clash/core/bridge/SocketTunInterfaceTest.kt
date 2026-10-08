package com.github.kr328.clash.core.bridge

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class SocketTunInterfaceTest {
    @Test
    fun deniedSocketIsReportedToNativeCaller() {
        var protectedFd = -1
        val callbacks = SocketTunInterface({ fd -> protectedFd = fd; false }) { _, _, _ -> -1 }
        assertFalse(callbacks.markSocket(42))
        assertEquals(42, protectedFd)
    }

    @Test
    fun protectedSocketCanProceed() {
        val callbacks = SocketTunInterface({ it == 17 }) { _, _, _ -> -1 }
        assertTrue(callbacks.markSocket(17))
        assertFalse(callbacks.markSocket(18))
    }
}
