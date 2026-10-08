package app

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	appLock          sync.RWMutex
	appVersionName   string
	platformVersion  int
	installedAppsUid = map[int]string{}
)

func ApplyVersionName(versionName string) {
	appLock.Lock()
	defer appLock.Unlock()
	appVersionName = versionName
}

func ApplyPlatformVersion(version int) {
	appLock.Lock()
	defer appLock.Unlock()
	platformVersion = version
}

func VersionName() string {
	appLock.RLock()
	defer appLock.RUnlock()
	return appVersionName
}

func PlatformVersion() int {
	appLock.RLock()
	defer appLock.RUnlock()
	return platformVersion
}

func NotifyInstallAppsChanged(uidList string) {
	uids := map[int]string{}

	for _, item := range strings.Split(uidList, ",") {
		kv := strings.Split(item, ":")
		if len(kv) == 2 {
			uid, err := strconv.Atoi(kv[0])
			if err != nil {
				continue
			}

			uids[uid] = kv[1]
		}
	}

	appLock.Lock()
	installedAppsUid = uids
	appLock.Unlock()
}

func QueryAppByUid(uid int) string {
	appLock.RLock()
	defer appLock.RUnlock()
	return installedAppsUid[uid]
}

func NotifyTimeZoneChanged(name string, offset int) {
	time.Local = time.FixedZone(name, offset)
}