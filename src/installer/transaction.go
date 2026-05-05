package installer

type Transaction struct {
	version         string
	installDir      string
	cacheFile       string
	installed       bool
	cached          bool
	installedNew    bool
	cachedNew       bool
	installBackup   string
	backupRestored  bool
	backupDiscarded bool
}
