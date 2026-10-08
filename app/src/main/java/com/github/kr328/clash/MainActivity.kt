package com.github.kr328.clash

import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import android.os.PersistableBundle
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.result.contract.ActivityResultContracts.RequestPermission
import androidx.core.app.ActivityCompat
import androidx.core.content.ContextCompat
import androidx.core.content.pm.ShortcutInfoCompat
import androidx.core.content.pm.ShortcutManagerCompat
import androidx.core.graphics.drawable.IconCompat
import com.github.kr328.clash.common.constants.Intents
import com.github.kr328.clash.common.util.intent
import com.github.kr328.clash.common.util.ticker
import com.github.kr328.clash.design.MainDesign
import com.github.kr328.clash.design.ui.ToastDuration
import com.github.kr328.clash.util.startClashService
import com.github.kr328.clash.util.stopClashService
import com.github.kr328.clash.util.withClash
import com.github.kr328.clash.util.withProfile
import com.github.kr328.clash.core.bridge.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.isActive
import kotlinx.coroutines.selects.select
import kotlinx.coroutines.withContext
import android.net.Uri
import androidx.appcompat.app.AlertDialog
import org.json.JSONArray
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL
import java.util.concurrent.TimeUnit
import com.github.kr328.clash.design.R as DesignR

class MainActivity : BaseActivity<MainDesign>() {
    override suspend fun main() {
        val design = MainDesign(this)

        setContentDesign(design)

        design.fetch()

        val ticker = ticker(TimeUnit.SECONDS.toMillis(1))

        while (isActive) {
            select<Unit> {
                events.onReceive {
                    when (it) {
                        Event.ActivityStart,
                        Event.ServiceRecreated,
                        Event.ClashStop, Event.ClashStart,
                        Event.ProfileLoaded, Event.ProfileChanged -> design.fetch()
                        else -> Unit
                    }
                }
                design.requests.onReceive {
                    when (it) {
                        MainDesign.Request.ToggleStatus -> {
                            if (clashRunning)
                                stopClashService()
                            else
                                design.startClash()
                        }
                        MainDesign.Request.OpenProxy ->
                            startActivity(ProxyActivity::class.intent)
                        MainDesign.Request.OpenProfiles ->
                            startActivity(ProfilesActivity::class.intent)
                        MainDesign.Request.OpenProviders ->
                            startActivity(ProvidersActivity::class.intent)
                        MainDesign.Request.OpenLogs -> {
                            if (LogcatService.running) {
                                startActivity(LogcatActivity::class.intent)
                            } else {
                                startActivity(LogsActivity::class.intent)
                            }
                        }
                        MainDesign.Request.OpenSettings ->
                            startActivity(SettingsActivity::class.intent)
                        MainDesign.Request.OpenHelp ->
                            startActivity(HelpActivity::class.intent)
                        MainDesign.Request.OpenAbout ->
                            design.showAbout(queryAppVersionName())
                        MainDesign.Request.CheckUpdate ->
                            checkAppUpdate(design)
                    }
                }
                if (clashRunning) {
                    ticker.onReceive {
                        design.fetchTraffic()
                    }
                }
            }
        }
    }

    private suspend fun MainDesign.fetch() {
        setClashRunning(clashRunning)

        val state = withClash {
            queryTunnelState()
        }
        val providers = withClash {
            queryProviders()
        }

        setMode(state.mode)
        setHasProviders(providers.isNotEmpty())

        withProfile {
            setProfileName(queryActive()?.name)
        }
    }

    private suspend fun MainDesign.fetchTraffic() {
        withClash {
            setForwarded(queryTrafficTotal())
        }
    }

    private suspend fun MainDesign.startClash() {
        val active = withProfile { queryActive() }

        if (active == null || !active.imported) {
            showToast(DesignR.string.no_profile_selected, ToastDuration.Long) {
                setAction(DesignR.string.profiles) {
                    startActivity(ProfilesActivity::class.intent)
                }
            }

            return
        }

        val vpnRequest = startClashService()

        try {
            if (vpnRequest != null) {
                val result = startActivityForResult(
                    ActivityResultContracts.StartActivityForResult(),
                    vpnRequest
                )

                if (result.resultCode == RESULT_OK)
                    startClashService()
            }
        } catch (e: Exception) {
            design?.showToast(DesignR.string.unable_to_start_vpn, ToastDuration.Long)
        }
    }

    private suspend fun queryAppVersionName(): String {
        return withContext(Dispatchers.IO) {
            packageManager.getPackageInfo(packageName, 0).versionName + "\n" + Bridge.nativeCoreVersion().replace("_", "-")
        }
    }

    private data class UpdateInfo(
        val hasNewVersion: Boolean,
        val currentVersion: String,
        val latestVersion: String,
        val changelog: String,
        val downloadUrl: String,
        val releasePageUrl: String,
    )

    private suspend fun checkAppUpdate(design: MainDesign) {
        design.showToast(DesignR.string.checking_update, ToastDuration.Short)
        val info = withContext(Dispatchers.IO) {
            fetchUpdateInfo()
        }

        if (info == null) {
            design.showToast(DesignR.string.check_update_failed, ToastDuration.Long)
            return
        }

        if (!info.hasNewVersion) {
            design.showToast(DesignR.string.already_latest_version, ToastDuration.Short)
            return
        }

        val message = buildString {
            append(getString(DesignR.string.current_version_format, info.currentVersion))
            append("\n")
            append(getString(DesignR.string.latest_version_format, info.latestVersion))
            if (info.changelog.isNotBlank()) {
                append("\n\n")
                append(info.changelog)
            }
        }

        AlertDialog.Builder(this@MainActivity)
            .setTitle(DesignR.string.new_version_found)
            .setMessage(message)
            .setPositiveButton(DesignR.string.download_update) { _, _ ->
                val target = info.downloadUrl.ifEmpty { info.releasePageUrl }
                try {
                    startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(target)))
                } catch (_: Exception) {
                }
            }
            .setNegativeButton(DesignR.string.cancel, null)
            .show()
    }

    private fun fetchUpdateInfo(): UpdateInfo? {
        return try {
            val endpoint = URL("https://api.github.com/repos/AkinoHaruka/RikoClash/releases")
            val conn = (endpoint.openConnection() as HttpURLConnection).apply {
                connectTimeout = 8000
                readTimeout = 8000
                setRequestProperty("Accept", "application/vnd.github.v3+json")
                setRequestProperty("User-Agent", "RikoClash-Android")
            }

            if (conn.responseCode !in 200..299) {
                conn.disconnect()
                return null
            }

            val jsonText = conn.inputStream.bufferedReader().use { it.readText() }
            conn.disconnect()

            val array = JSONArray(jsonText)
            if (array.length() == 0) return null

            val latestRelease = array.getJSONObject(0)
            val tagName = latestRelease.optString("tag_name", "").trim()
            val releaseName = latestRelease.optString("name", "").trim()
            val body = latestRelease.optString("body", "").trim()
            val releaseUrl = latestRelease.optString("html_url", "https://github.com/AkinoHaruka/RikoClash/releases")
            val assets = latestRelease.optJSONArray("assets") ?: JSONArray()

            val currentVersionName = packageManager.getPackageInfo(packageName, 0).versionName ?: "0.0.0"
            val isCurrentDebug = packageName.endsWith(".debug") || currentVersionName.contains("debug", ignoreCase = true)

            val supportedAbis = Build.SUPPORTED_ABIS ?: arrayOf("arm64-v8a")
            var matchedDownloadUrl = ""

            // 1. Prefer matching device ABI + exact build type (debug -> debug, release -> release)
            for (abi in supportedAbis) {
                for (i in 0 until assets.length()) {
                    val asset = assets.getJSONObject(i)
                    val name = asset.optString("name", "")
                    if (!name.endsWith(".apk", ignoreCase = true) || !name.contains("-$abi-", ignoreCase = true)) {
                        continue
                    }
                    val isAssetDebug = name.contains("-debug", ignoreCase = true)
                    if (isCurrentDebug == isAssetDebug) {
                        matchedDownloadUrl = asset.optString("browser_download_url", "")
                        break
                    }
                }
                if (matchedDownloadUrl.isNotEmpty()) break
            }

            // 2. Fallback to alternative build type for matching device ABI
            if (matchedDownloadUrl.isEmpty()) {
                for (abi in supportedAbis) {
                    for (i in 0 until assets.length()) {
                        val asset = assets.getJSONObject(i)
                        val name = asset.optString("name", "")
                        if (name.endsWith(".apk", ignoreCase = true) && name.contains("-$abi-", ignoreCase = true)) {
                            matchedDownloadUrl = asset.optString("browser_download_url", "")
                            break
                        }
                    }
                    if (matchedDownloadUrl.isNotEmpty()) break
                }
            }

            // 3. Fallback to any APK matching current build type
            if (matchedDownloadUrl.isEmpty()) {
                for (i in 0 until assets.length()) {
                    val asset = assets.getJSONObject(i)
                    val name = asset.optString("name", "")
                    if (name.endsWith(".apk", ignoreCase = true)) {
                        val isAssetDebug = name.contains("-debug", ignoreCase = true)
                        if (isCurrentDebug == isAssetDebug) {
                            matchedDownloadUrl = asset.optString("browser_download_url", "")
                            break
                        }
                    }
                }
            }

            // 4. Fallback to any available APK
            if (matchedDownloadUrl.isEmpty()) {
                for (i in 0 until assets.length()) {
                    val asset = assets.getJSONObject(i)
                    val name = asset.optString("name", "")
                    if (name.endsWith(".apk", ignoreCase = true)) {
                        matchedDownloadUrl = asset.optString("browser_download_url", "")
                        break
                    }
                }
            }

            val hasNew = isVersionNewer(tagName.ifEmpty { releaseName }, currentVersionName)

            UpdateInfo(
                hasNewVersion = hasNew,
                currentVersion = currentVersionName,
                latestVersion = tagName.ifEmpty { releaseName },
                changelog = body,
                downloadUrl = matchedDownloadUrl,
                releasePageUrl = releaseUrl,
            )
        } catch (e: Exception) {
            null
        }
    }

    private fun isVersionNewer(remote: String, current: String): Boolean {
        fun extractNumbers(s: String): List<Int> {
            val match = Regex("""(\d+)\.(\d+)\.(\d+)""").find(s)
            return if (match != null) {
                match.groupValues.drop(1).map { it.toIntOrNull() ?: 0 }
            } else {
                Regex("""\d+""").findAll(s).map { it.value.toIntOrNull() ?: 0 }.toList()
            }
        }

        val remoteNums = extractNumbers(remote)
        val currentNums = extractNumbers(current)

        val size = maxOf(remoteNums.size, currentNums.size)
        for (i in 0 until size) {
            val r = remoteNums.getOrElse(i) { 0 }
            val c = currentNums.getOrElse(i) { 0 }
            if (r > c) return true
            if (r < c) return false
        }

        val cleanRemote = remote.trimStart('v', 'V').trim()
        val cleanCurrent = current.trimStart('v', 'V').trim()
        return cleanRemote.isNotEmpty() && cleanCurrent.isNotEmpty() && cleanRemote != cleanCurrent && cleanRemote.contains("alpha", ignoreCase = true) && !cleanCurrent.contains("debug", ignoreCase = true)
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            val requestPermissionLauncher =
                registerForActivityResult(RequestPermission()
                ) { isGranted: Boolean ->
                }
            if (ContextCompat.checkSelfPermission(
                    this,
                    android.Manifest.permission.POST_NOTIFICATIONS
                ) != PackageManager.PERMISSION_GRANTED) {
                requestPermissionLauncher.launch(android.Manifest.permission.POST_NOTIFICATIONS)
            }
        }
        setupShortcuts()
    }

    private fun setupShortcuts() {
        // Skip dynamic shortcut setup when the app icon is hidden.
        if (uiStore.hideAppIcon) return

        val flags = Intent.FLAG_ACTIVITY_NEW_TASK or
            Intent.FLAG_ACTIVITY_EXCLUDE_FROM_RECENTS or
            Intent.FLAG_ACTIVITY_NO_ANIMATION

        val toggle = ShortcutInfoCompat.Builder(this, "toggle_clash")
            .setShortLabel(getString(DesignR.string.shortcut_toggle_short))
            .setLongLabel(getString(DesignR.string.shortcut_toggle_long))
            .setIcon(IconCompat.createWithResource(this, R.drawable.ic_toggle_all))
            .setIntent(
                Intent(Intents.ACTION_TOGGLE_CLASH)
                    .setClassName(this, ExternalControlActivity::class.java.name)
                    .addFlags(flags)
            )
            .setRank(0)
            .build()

        val start = ShortcutInfoCompat.Builder(this, "start_clash")
            .setShortLabel(getString(DesignR.string.shortcut_start_short))
            .setLongLabel(getString(DesignR.string.shortcut_start_long))
            .setIcon(IconCompat.createWithResource(this, R.drawable.ic_toggle_on))
            .setIntent(
                Intent(Intents.ACTION_START_CLASH)
                    .setClassName(this, ExternalControlActivity::class.java.name)
                    .addFlags(flags)
            )
            .setRank(1)
            .build()

        val stop = ShortcutInfoCompat.Builder(this, "stop_clash")
            .setShortLabel(getString(DesignR.string.shortcut_stop_short))
            .setLongLabel(getString(DesignR.string.shortcut_stop_long))
            .setIcon(IconCompat.createWithResource(this, R.drawable.ic_toggle_off))
            .setIntent(
                Intent(Intents.ACTION_STOP_CLASH)
                    .setClassName(this, ExternalControlActivity::class.java.name)
                    .addFlags(flags)
            )
            .setRank(2)
            .build()

        ShortcutManagerCompat.setDynamicShortcuts(this, listOf(toggle, start, stop))
    }
}
