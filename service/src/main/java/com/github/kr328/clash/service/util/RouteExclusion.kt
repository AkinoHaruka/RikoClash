package com.github.kr328.clash.service.util

import java.math.BigInteger
import java.net.InetAddress

internal data class RouteSpec(
    val address: InetAddress,
    val prefix: Int,
)

/**
 * Builds Android VpnService routes for all addresses except the configured CIDRs.
 * Android's Builder has include-routes, but no public exclude-route API, so a
 * default route is represented as the CIDR complement of the exclusions.
 */
internal fun routeComplement(raw: String?): List<RouteSpec> {
    return routeSubtract(
        base = listOf("0.0.0.0/0", "::/0"),
        raw = raw,
    )
}

/**
 * Removes the configured CIDRs from an arbitrary set of allowed base routes.
 * This is used when Android's private-network bypass mode supplies a public
 * route set instead of a single default route.
 */
internal fun routeSubtract(base: Collection<String>, raw: String?): List<RouteSpec> {
    val exclusions = parseRanges(raw)
    val byBits = exclusions.groupBy { it.bits }

    return base
        .map(::parseRange)
        .flatMap { range -> subtract(range, byBits[range.bits].orEmpty()) }
}

private data class AddressRange(
    val start: BigInteger,
    val end: BigInteger,
    val bits: Int,
)

private fun parseRanges(raw: String?): List<AddressRange> {
    return raw.orEmpty()
        .split(',', ';', '\n', '\r')
        .map(String::trim)
        .filter(String::isNotEmpty)
        .map(::parseRange)
}

private fun parseRange(value: String): AddressRange {
    val separator = value.lastIndexOf('/')
    require(separator > 0 && separator < value.lastIndex) { "Invalid CIDR: $value" }

    val host = value.substring(0, separator)
    require(host.contains('.') || host.contains(':')) { "CIDR must contain an IP address: $value" }

    val address = InetAddress.getByName(host)
    val bits = address.address.size * 8
    val prefix = value.substring(separator + 1).toIntOrNull()
        ?: throw IllegalArgumentException("Invalid CIDR prefix: $value")
    require(prefix in 0..bits) { "CIDR prefix out of range: $value" }

    val max = BigInteger.ONE.shiftLeft(bits).subtract(BigInteger.ONE)
    val hostMask = BigInteger.ONE.shiftLeft(bits - prefix).subtract(BigInteger.ONE)
    val network = BigInteger(1, address.address).and(max.xor(hostMask))

    return AddressRange(network, network.add(hostMask), bits)
}

private fun subtract(base: AddressRange, exclusions: List<AddressRange>): List<RouteSpec> {
    val result = mutableListOf<RouteSpec>()
    var cursor = base.start

    for (range in merge(exclusions)) {
        if (range.end < base.start || range.start > base.end) continue

        val start = range.start.max(base.start)
        val end = range.end.min(base.end)

        if (start > cursor) {
            result += cover(cursor, start.subtract(BigInteger.ONE), base.bits)
        }

        if (end >= cursor) {
            cursor = end.add(BigInteger.ONE)
        }

        if (cursor > base.end) break
    }

    if (cursor <= base.end) {
        result += cover(cursor, base.end, base.bits)
    }

    return result
}

private fun merge(input: List<AddressRange>): List<AddressRange> {
    if (input.isEmpty()) return emptyList()

    val sorted = input.sortedWith(compareBy<AddressRange> { it.bits }.thenBy { it.start })
    val merged = mutableListOf<AddressRange>()

    for (range in sorted) {
        val current = merged.lastOrNull()
        if (current == null || current.bits != range.bits || range.start > current.end.add(BigInteger.ONE)) {
            merged += range
        } else if (range.end > current.end) {
            merged[merged.lastIndex] = current.copy(end = range.end)
        }
    }

    return merged
}

private fun cover(start: BigInteger, end: BigInteger, bits: Int): List<RouteSpec> {
    val result = mutableListOf<RouteSpec>()
    var cursor = start

    while (cursor <= end) {
        val alignmentBits = if (cursor == BigInteger.ZERO) {
            bits
        } else {
            cursor.lowestSetBit.coerceAtMost(bits)
        }
        val remainingBits = end.subtract(cursor).add(BigInteger.ONE).bitLength() - 1
        val blockBits = minOf(alignmentBits, remainingBits)
        val prefix = bits - blockBits

        result += RouteSpec(toAddress(cursor, bits), prefix)
        cursor = cursor.add(BigInteger.ONE.shiftLeft(blockBits))
    }

    return result
}

private fun toAddress(value: BigInteger, bits: Int): InetAddress {
    val size = bits / 8
    val source = value.toByteArray()
    val result = ByteArray(size)
    val copyLength = minOf(source.size, size)
    source.copyInto(result, destinationOffset = size - copyLength, startIndex = source.size - copyLength)
    return InetAddress.getByAddress(result)
}