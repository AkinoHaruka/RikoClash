package com.github.kr328.clash.core.util

import java.net.InetAddress
import java.net.InetSocketAddress

fun parseInetSocketAddress(address: String): InetSocketAddress {
    return runCatching {
        val trimmed = address.trim()
        if (trimmed.isEmpty()) {
            return@runCatching InetSocketAddress(InetAddress.getLoopbackAddress(), 0)
        }

        // Bracketed IPv6: e.g. [2001:db8::1]:8080 or [::1]
        if (trimmed.startsWith('[')) {
            val closeIndex = trimmed.indexOf(']')
            if (closeIndex > 0) {
                val host = trimmed.substring(1, closeIndex)
                val port = if (closeIndex + 2 <= trimmed.length && trimmed[closeIndex + 1] == ':') {
                    trimmed.substring(closeIndex + 2).toIntOrNull() ?: 0
                } else {
                    0
                }
                val inet = InetAddress.getByName(host)
                return@runCatching InetSocketAddress(inet, port.coerceIn(0, 65535))
            }
        }

        val lastColon = trimmed.lastIndexOf(':')
        val firstColon = trimmed.indexOf(':')

        // Multiple colons without brackets: e.g. 2001:db8::1:8080 or 2001:db8::1
        if (firstColon != -1 && firstColon != lastColon) {
            val portCandidate = trimmed.substring(lastColon + 1).toIntOrNull()
            if (portCandidate != null && portCandidate in 1..65535) {
                val hostCandidate = trimmed.substring(0, lastColon)
                val inet = runCatching { InetAddress.getByName(hostCandidate) }.getOrNull()
                if (inet != null) {
                    return@runCatching InetSocketAddress(inet, portCandidate)
                }
            }
            // Fallback: parse entire string as IPv6 without port
            val inet = InetAddress.getByName(trimmed)
            return@runCatching InetSocketAddress(inet, 0)
        }

        // Single colon: host:port, e.g. 192.168.1.1:8080 or example.com:443
        if (lastColon != -1) {
            val host = trimmed.substring(0, lastColon)
            val port = trimmed.substring(lastColon + 1).toIntOrNull() ?: 0
            val inet = InetAddress.getByName(host)
            return@runCatching InetSocketAddress(inet, port.coerceIn(0, 65535))
        }

        // No colon: host only, e.g. 192.168.1.1 or example.com
        val inet = InetAddress.getByName(trimmed)
        InetSocketAddress(inet, 0)
    }.getOrElse {
        InetSocketAddress(InetAddress.getLoopbackAddress(), 0)
    }
}