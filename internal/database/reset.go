package database

// Reset closes and clears the singleton. Intended for tests only.
func Reset() {
	if dbInstance != nil {
		_ = dbInstance.db.Close()
		dbInstance = nil
	}
}
