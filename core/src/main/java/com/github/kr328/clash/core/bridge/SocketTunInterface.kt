package com.github.kr328.clash.core.bridge

internal class SocketTunInterface(
    private val protectSocket: (Int) -> Boolean,
    private val resolveUid: (Int, String, String) -> Int,
) : TunInterface {
    override fun markSocket(fd: Int): Boolean =
        runCatching { protectSocket(fd) }.getOrDefault(false)

    override fun querySocketUid(protocol: Int, source: String, target: String): Int =
        runCatching { resolveUid(protocol, source, target) }.getOrDefault(-1)
}
