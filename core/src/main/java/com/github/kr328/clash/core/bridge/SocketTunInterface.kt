package com.github.kr328.clash.core.bridge

internal class SocketTunInterface(
    private val protectSocket: (Int) -> Boolean,
    private val resolveUid: (Int, String, String) -> Int,
) : TunInterface {
    override fun markSocket(fd: Int): Boolean = protectSocket(fd)

    override fun querySocketUid(protocol: Int, source: String, target: String): Int =
        resolveUid(protocol, source, target)
}
