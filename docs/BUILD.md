# RikoClash 构建

固定输入见 `gradle/toolchains.lock.json`、`gradle/geodata.lock.properties` 和 Gradle wrapper 校验值。核心固定为 `88dcbf7f1614a67c3b36b848ee3592dfa92ada36`，不跟随远端自动更新。

Windows PowerShell，先准备 Android SDK 35、NDK 29.0.14206865、CMake 3.22.1，再执行：

```powershell
./scripts/Initialize-Toolchains.ps1
./scripts/Build-Android.ps1 -Tasks @(':app:assembleAlphaDebug', ':core:testAlphaDebugUnitTest', ':app:lintAlphaDebug')
```

工具链安装到忽略的 `.local/toolchains`；环境变量仅对当前构建进程生效。Go 使用本工程 `.local/go-mod-cache`。依赖和资产已缓存时可增加 `-Offline`，缺失依赖会失败；完整离线构建尚未验收。

规则资产记录 2026-10-04 官方发布文件的大小及 SHA-256；上游仅提供可变 `latest` 下载入口，因此缓存经校验后复用，远端内容变化会失败而不自动升级锁定版本。`clean` 不删除这些资产。不存在或摘要错误的文件不能进入构建。

Debug 包为 `asia.riko.clash.alpha.debug`，保留 Android 开发签名；Release 为独立包身份，必须在忽略的 `signing.properties` 中指定 `keystore.file` 及签名属性。本工程移除继承的上游签名文件，不将其用作 Riko 发布身份。缺少 Release 配置时构建明确失败，不能退回 Debug 签名。Debug 与 Release 不承诺原地升级或私有数据自动迁移。

2026-10-05：assembleAlphaDebug、2 项 Socket protect 单元测试及 lintAlphaDebug 成功；Lint 0 错误、48 项继承警告。缺少签名的 assembleAlphaRelease 在任务执行前被拒绝。详细日志位于本机忽略目录 `.local`，不作为可公开的凭据载体。

本次 arm64 产物：`app/build/outputs/apk/alpha/debug/riko-clash-2.11.35-alpha-arm64-v8a-debug.apk`，81,315,419 字节；SHA-256：`cb5febfeabd7cc2dcd84ba1e235f937878bb31c3d848cbc8b6d4e9eaf7633573`。其他 ABI 和 universal 同时构建；实际手机只验收 arm64。
