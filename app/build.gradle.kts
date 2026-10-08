import java.net.URL
import java.nio.file.Files
import java.nio.file.StandardCopyOption
import java.security.MessageDigest
import java.util.Properties

plugins {
    kotlin("android")
    kotlin("kapt")
    id("com.android.application")
}

dependencies {
    compileOnly(project(":hideapi"))

    implementation(project(":core"))
    implementation(project(":service"))
    implementation(project(":design"))
    implementation(project(":common"))

    implementation(libs.kotlin.coroutine)
    implementation(libs.androidx.core)
    implementation(libs.androidx.activity)
    implementation(libs.androidx.fragment)
    implementation(libs.androidx.appcompat)
    implementation(libs.androidx.coordinator)
    implementation(libs.androidx.recyclerview)
    implementation(libs.google.material)
    implementation(libs.quickie.bundled)
    implementation(libs.androidx.activity.ktx)
}

tasks.getByName("clean", type = Delete::class) {
    delete(file("release"))
}

val geoFilesDownloadDir = "src/main/assets"
val geoLockFile = rootProject.file("gradle/geodata.lock.properties")

task("downloadGeoFiles") {

    inputs.file(geoLockFile)

    fun verify(file: File, size: Long, sha256: String) {
        check(file.length() == size) { "Locked asset size mismatch: ${file.name}" }
        val digest = MessageDigest.getInstance("SHA-256")
        file.inputStream().use { input ->
            val buffer = ByteArray(65536)
            while (true) {
                val count = input.read(buffer)
                if (count < 0) break
                digest.update(buffer, 0, count)
            }
        }
        val actual = digest.digest().joinToString("") { "%02x".format(it) }
        check(actual == sha256) { "Locked asset SHA-256 mismatch: ${file.name}" }
    }

    doLast {
        val lock = Properties().apply { geoLockFile.inputStream().use(::load) }
        lock.getProperty("files").split(",").forEach { outputFileName ->
            val downloadUrl = lock.getProperty("$outputFileName.url")
            val expectedSize = lock.getProperty("$outputFileName.size").toLong()
            val expectedHash = lock.getProperty("$outputFileName.sha256")
            val outputPath = file("$geoFilesDownloadDir/$outputFileName")
            if (outputPath.exists()) {
                verify(outputPath, expectedSize, expectedHash)
            } else {
                check(!gradle.startParameter.isOffline) {
                    "Locked asset missing in offline mode: $outputFileName"
                }
                outputPath.parentFile.mkdirs()
                val temporary = File.createTempFile("$outputFileName-", ".part", outputPath.parentFile)
                try {
                    val connection = URL(downloadUrl).openConnection().apply {
                        connectTimeout = 30000
                        readTimeout = 60000
                    }
                    connection.getInputStream().use { input ->
                        Files.copy(input, temporary.toPath(), StandardCopyOption.REPLACE_EXISTING)
                    }
                    verify(temporary, expectedSize, expectedHash)
                    Files.move(temporary.toPath(), outputPath.toPath(), StandardCopyOption.REPLACE_EXISTING)
                } finally {
                    temporary.delete()
                }
            }
            println("Verified locked asset: $outputFileName")
        }
    }
}

afterEvaluate {
    tasks.named("preBuild") { dependsOn("downloadGeoFiles") }
}
