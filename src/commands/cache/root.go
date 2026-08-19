package cache

type Root struct {
	Add    Add    `cmd:"add" help:"Add artifacts directly to the cache."`
	View   View   `cmd:"view" aliases:"ls" default:"withargs" help:"View cached assets. This is run if no subcommand is specified (e.g. nvm cache)."`
	Remove Remove `cmd:"remove" aliases:"rm" help:"Remove cached assets."`
}

// func (c *Cache) Run() error {
// 	cacheDir, err := http.GetCacheDir()
// 	if err != nil {
// 		return err
// 	}

// 	// Read all entries in the cache directory
// 	entries, err := os.ReadDir(cacheDir)
// 	if err != nil {
// 		return err
// 	}

// 	// Remove each file/directory in the cache
// 	for _, entry := range entries {
// 		entryPath := filepath.Join(cacheDir, entry.Name())
// 		if err := os.RemoveAll(entryPath); err != nil {
// 			return err
// 		}
// 	}

// 	fmt.Printf("Cache cleared: %s\n", cacheDir)
// 	return nil
// }
